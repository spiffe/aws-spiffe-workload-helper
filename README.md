# AWS SPIFFE Workload Helper

[![Apache 2.0 License](https://img.shields.io/github/license/spiffe/helm-charts)](https://opensource.org/licenses/Apache-2.0)
[![Development Phase](https://github.com/spiffe/spiffe/blob/main/.img/maturity/dev.svg)](https://github.com/spiffe/spiffe/blob/main/MATURITY.md#development)

AWS SPIFFE Workload Helper is a light-weight tool intended to assist in
providing a workload with credentials for AWS using its SPIFFE identity.

Currently, the helper only supports authenticating to AWS using an X.509 SVID
via [AWS Roles Anywhere](https://docs.aws.amazon.com/rolesanywhere/latest/userguide/introduction.html).
It provides a more native experience when using SPIFFE identities compared to
the [`rolesanywhere-credential-helper`](https://github.com/aws/rolesanywhere-credential-helper)
released by AWS.

## Usage

TODO: Link to full guide on SPIFFE website for a proper "getting started"

### Binary

TODO: ...

### Configuring AWS SDKs and CLIs

TODO: ...

### OCI Image

The `aws-spiffe-workload-helper` is also distributed within an OCI image. This
may be useful as a source of the binary if you are building your own image and
require this binary within it.

These images are published to the GitHub Container Registry: [ghcr.io/spiffe/aws-spiffe-workload-helper:latest](https://github.com/spiffe/aws-spiffe-workload-helper/pkgs/container/aws-spiffe-workload-helper)

```dockerfile
COPY --from=ghcr.io/spiffe/aws-spiffe-workload-helper:latest /ko-app/cmd /aws-spiffe-workload-helper
```

## Contributing

We welcome contributions to this project. If you require any assistance, please
get in contact via the SPIFFE Slack.

### Governance

This is a ["tiny-project"](https://github.com/spiffe/spiffe/blob/main/NEW_PROJECTS.md#tiny-projects).

Dispute resolution is handled via escalation to the [SPIFFE Steering Committee (SSC)](https://github.com/spiffe/spiffe/blob/main/GOVERNANCE.md#the-spiffe-steering-committee-ssc).
