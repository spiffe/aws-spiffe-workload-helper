aws-spiffe-workload-helper: cmd/credential_file.go cmd/jwt_credential_process.go cmd/main.go cmd/x509_credential_process.go
	CGO_ENABLED=0 go build -o ./aws-spiffe-workload-helper cmd/*

clean:
	rm -f aws-spiffe-workload-helper

