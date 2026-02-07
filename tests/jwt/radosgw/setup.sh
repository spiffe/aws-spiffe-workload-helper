#!/bin/bash

SCRIPT="$(readlink -f "$0")"
SCRIPTPATH="$(dirname "${SCRIPT}")"
BASEPATH="${SCRIPTPATH}/../../../"

helm upgrade --install -n spire-server spire-crds spire-crds --repo https://spiffe.github.io/helm-charts-hardened/ --create-namespace
helm upgrade --install -n spire-server spire spire --repo https://spiffe.github.io/helm-charts-hardened/ -f "${SCRIPTPATH}/spire-values.yaml" --wait
kubectl apply -f "${SCRIPTPATH}/admin.yaml"
kubectl apply -f "${SCRIPTPATH}/test.yaml"
kubectl create namespace --dry-run=client -o yaml rook-ceph | kubectl apply -f -
kubectl apply -f "${SCRIPTPATH}/rook-config-override-configmap.yaml"
helm upgrade --install -n rook-ceph rook-ceph rook-ceph --repo https://charts.rook.io/release --create-namespace -f "${SCRIPTPATH}/ceph-values.yaml" --wait
helm upgrade --install -n rook-ceph rook-ceph-cluster rook-ceph-cluster --repo https://charts.rook.io/release -f "${SCRIPTPATH}/ceph-cluster-values.yaml" --wait

kubectl wait --for=create deploy/rook-ceph-tools -n rook-ceph --timeout=60s
kubectl wait --for=jsonpath='{.status.readyReplicas}'=1 deploy/rook-ceph-tools -n rook-ceph --timeout=300s

kubectl wait --for=create deploy/rook-ceph-rgw-ceph-objectstore-a -n rook-ceph --timeout=300s
kubectl wait --for=jsonpath='{.status.readyReplicas}'=1 deploy/rook-ceph-rgw-ceph-objectstore-a -n rook-ceph --timeout=300s

kubectl exec -it -n rook-ceph deploy/rook-ceph-tools -- bash -c 'radosgw-admin --rgw-zone=ceph-objectstore user create --uid=oidc-admin --display-name="OIDC Admin" --access-key=oidc-admin --secret-key=test1234'
kubectl exec -it -n rook-ceph deploy/rook-ceph-tools -- bash -c 'radosgw-admin --rgw-zone=ceph-objectstore caps add --uid=oidc-admin --caps="oidc-provider=*"'
kubectl exec -it -n rook-ceph deploy/rook-ceph-tools -- bash -c 'radosgw-admin --rgw-zone=ceph-objectstore caps add --uid=oidc-admin --caps="roles=*"'

FOO=N5u4qNh6K8Tk1JfK
kubectl exec -it -n rook-ceph deploy/rook-ceph-tools -- bash -c "ceph config set client.rgw.ceph.objectstore 'rgw sts key' $FOO"
kubectl exec -it -n rook-ceph deploy/rook-ceph-tools -- bash -c "ceph config set client.rgw.ceph.objectstore 'rgw s3 auth use sts' true"
kubectl exec -it -n rook-ceph deploy/rook-ceph-tools -- bash -c 'ceph config set client.rgw rgw_verify_ssl false'

kubectl exec -it -n rook-ceph deploy/rook-ceph-tools -- bash -c 'ceph config set client.rgw.ceph.objectstore.a debug_rgw 20'
kubectl exec -it -n rook-ceph deploy/rook-ceph-tools -- bash -c '#ceph config set client.rgw.ceph.objectstore.a debug_ms 1'

kubectl rollout restart deploy/rook-ceph-rgw-ceph-objectstore-a -n rook-ceph

kubectl wait --for=jsonpath='{.status.readyReplicas}'=1 deploy/rook-ceph-rgw-ceph-objectstore-a -n rook-ceph --timeout=300s

kubectl exec -it admin-0 -- bash -c "~/setup.sh"
