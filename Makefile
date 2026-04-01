aws-spiffe-workload-helper:
	CGO_ENABLED=0 go build -o ./aws-spiffe-workload-helper ./cmd

clean:
	rm -f aws-spiffe-workload-helper

