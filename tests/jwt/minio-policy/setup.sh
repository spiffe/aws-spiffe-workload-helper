#!/bin/bash

SCRIPT="$(readlink -f "$0")"
SCRIPTPATH="$(dirname "${SCRIPT}")"
BASEPATH="${SCRIPTPATH}/../../../"

helm upgrade --install -n spire-server spire-crds spire-crds --repo https://spiffe.github.io/helm-charts-hardened/ --create-namespace
helm upgrade --install -n spire-server spire spire --repo https://spiffe.github.io/helm-charts-hardened/ -f "${SCRIPTPATH}/spire-values.yaml" --wait
kubectl apply -f "${SCRIPTPATH}/test.yaml"
if ! helm upgrade --install minio -n minio --create-namespace oci://registry-1.docker.io/bitnamicharts/minio -f "${SCRIPTPATH}/minio-values.yaml"; then
	echo "MinIO install failed. Provisioning job diagnostics:"
	kubectl get pods -n minio -o wide
	kubectl describe job -n minio minio-provisioning
	kubectl logs -n minio -l job-name=minio-provisioning --all-containers --prefix --tail=-1
	kubectl logs -n minio -l job-name=minio-provisioning --all-containers --prefix --tail=-1 --previous
	kubectl logs -n minio deploy/minio --tail=200
	exit 1
fi
kubectl rollout restart -n minio deployment/minio
kubectl rollout status -n minio deployment/minio
