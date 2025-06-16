#!/bin/bash

set -e

SCRIPT="$(readlink -f "$0")"
SCRIPTPATH="$(dirname "${SCRIPT}")"
BASEPATH="${SCRIPTPATH}/../../../"

cd "${BASEPATH}"
make
kubectl cp aws-spiffe-workload-helper test-0:/tmp/aws-spiffe-workload-helper

echo "Starting tests that should work..."
kubectl exec -i test-0 -- bash -c 'mc alias set minio http://minio.minio:9000 admin admin1234' || true
kubectl exec -i test-0 -- bash -c 'mc idp openid info minio spire'
kubectl exec -i test-0 -- bash -c 'echo "hello from $(date)" > hello.txt'
kubectl exec -i test-0 -- bash -c 'aws --endpoint-url http://minio.minio:9000 s3 cp hello.txt s3://data/test/hello.txt'
kubectl exec -i test-0 -- bash -c 'aws --endpoint-url http://minio.minio:9000 s3 cp s3://data/test/hello.txt hello2.txt'
kubectl exec -i test-0 -- bash -c 'cat hello2.txt | grep "hello from "'

echo "Starting tests that should fail..."
set +e
kubectl exec -i test-0 -- bash -c 'aws --endpoint-url http://minio.minio:9000 s3 cp hello.txt s3://data/other/hello.txt'
res=$?
if [ $res -eq 1 ]; then
	echo "Test failed as expected. Moving on..."
else
	echo "Test should have failed but didn't..."
	exit 1
fi
echo Done. Tests passed.
