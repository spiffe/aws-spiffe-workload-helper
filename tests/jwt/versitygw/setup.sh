#!/bin/bash -e

SCRIPT="$(readlink -f "$0")"
SCRIPTPATH="$(dirname "${SCRIPT}")"
BASEPATH="${SCRIPTPATH}/../../../"

GITHUB_STEP_SUMMARY="${GITHUB_STEP_SUMMARY:-/tmp/summary}"

# The OIDC/STS features this test needs are on versitygw main but not yet in a
# release, so the image is built from a pinned commit. The chart (0.4.5+) has
# the pod extension hooks and volume-backed TLS this test relies on.
VERSITYGW_REF="${VERSITYGW_REF:-af503ee29716f3b401bf055b434c02f9b7140d59}"
VERSITYGW_CHART_VERSION="${VERSITYGW_CHART_VERSION:-0.4.5}"
VERSITYGW_SRC="$(mktemp -d)"

teardown() {
cat <<EOF >>"$GITHUB_STEP_SUMMARY"
#### PODS
$(kubectl get pods -A)

#### versitygw gateway
$(kubectl logs -n versitygw -l app.kubernetes.io/name=versitygw --all-containers --prefix --tail=-1)

#### versitygw iam
$(kubectl logs -n versitygw -l app.kubernetes.io/component=iam-server --all-containers --prefix --tail=-1)

#### admin
$(kubectl logs admin-0 --all-containers --prefix --tail=-1)

#### spire-agent
$(kubectl logs -n spire-server -l app.kubernetes.io/name=agent --all-containers --prefix --tail=200)
EOF
rm -rf "$VERSITYGW_SRC"
}

trap 'EC=$? && trap - SIGTERM && teardown $EC' SIGINT SIGTERM EXIT

kubectl version

if [ -n "${VERSITYGW_IMAGE:-}" ]; then
  IMAGE_REPOSITORY="${VERSITYGW_IMAGE%:*}"
  IMAGE_TAG="${VERSITYGW_IMAGE##*:}"
else
  IMAGE_REPOSITORY=versitygw
  IMAGE_TAG="ci-${VERSITYGW_REF:0:12}"
  git clone https://github.com/versity/versitygw.git "$VERSITYGW_SRC"
  git -C "$VERSITYGW_SRC" checkout "$VERSITYGW_REF"
  docker build -t "${IMAGE_REPOSITORY}:${IMAGE_TAG}" "$VERSITYGW_SRC"
  kind load docker-image "${IMAGE_REPOSITORY}:${IMAGE_TAG}" --name "${KIND_CLUSTER:-$(kind get clusters | head -1)}"
fi

helm upgrade --install -n spire-server spire-crds spire-crds --repo https://spiffe.github.io/helm-charts-hardened/ --create-namespace
helm upgrade --install -n spire-server spire spire --repo https://spiffe.github.io/helm-charts-hardened/ -f "${SCRIPTPATH}/spire-values.yaml" --wait

kubectl create namespace --dry-run=client -o yaml versitygw | kubectl apply -f -
kubectl apply -f "${SCRIPTPATH}/configmaps.yaml"
kubectl apply -f "${SCRIPTPATH}/admin.yaml"
kubectl apply -f "${SCRIPTPATH}/test.yaml"

helm upgrade --install versitygw -n versitygw oci://ghcr.io/versity/versitygw/charts/versitygw --version "${VERSITYGW_CHART_VERSION}" -f "${SCRIPTPATH}/versitygw-values.yaml" \
  --set image.repository="${IMAGE_REPOSITORY}" --set image.tag="${IMAGE_TAG}" --wait --timeout 10m

kubectl rollout status statefulset/admin --timeout=300s
kubectl exec -i admin-0 -c main -- bash -c 'until [ -x ~/setup.sh ]; do sleep 1; done; ~/setup.sh'
