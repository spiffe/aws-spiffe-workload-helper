# AWS SPIFFE Workload Helper

[![Apache 2.0 License](https://img.shields.io/github/license/spiffe/helm-charts)](https://opensource.org/licenses/Apache-2.0)
[![Development Phase](https://github.com/spiffe/spiffe/blob/main/.img/maturity/dev.svg)](https://github.com/spiffe/spiffe/blob/main/MATURITY.md#development)

AWS SPIFFE Workload Helper is a light-weight tool intended to assist in
providing a workload with credentials for AWS using its SPIFFE identity.

It provides a more native experience when using SPIFFE identities compared to
the [`rolesanywhere-credential-helper`](https://github.com/aws/rolesanywhere-credential-helper)
released by AWS, and is intended to be used in place of
`rolesanywhere-credential-helper`.

Currently, the helper only supports authenticating to AWS using an X.509 SVID
via [AWS Roles Anywhere](https://docs.aws.amazon.com/rolesanywhere/latest/userguide/introduction.html).

## Usage

### Getting Started

Follow the guidance at
<https://docs.aws.amazon.com/rolesanywhere/latest/userguide/getting-started.html>
and substitute the usage of `rolesanywhere-credential-helper` with this utility.

### Installation

#### Binary

The `aws-spiffe-workload-helper` binary is available for a range of
architectures within the
[GitHub Releases](https://github.com/spiffe/aws-spiffe-workload-helper/releases)
of this repository.

Download the appropriate artifact for your architecture, and extract the
.tar.gz. The binary can then be placed somewhere on the system where it will be
accessible to workloads that use the AWS SDKs or CLIs. It may be beneficial to
ensure it is in a location that is within your PATH.

#### OCI Image

The `aws-spiffe-workload-helper` is also distributed within an OCI image. This
may be useful as a source of the binary if you are building your own image and
require this binary within it.

These images are published to the GitHub Container Registry: [ghcr.io/spiffe/aws-spiffe-workload-helper:latest](https://github.com/spiffe/aws-spiffe-workload-helper/pkgs/container/aws-spiffe-workload-helper)

```dockerfile
COPY --from=ghcr.io/spiffe/aws-spiffe-workload-helper:latest /ko-app/cmd /aws-spiffe-workload-helper
```

### CLI Commands

#### `x509-credential-process`

The `x509-credential-process` command exchanges an X509 SVID for a short-lived
set of AWS credentials using the AWS Roles Anywhere API. It returns the
credentials to STDOUT, in the format expected by AWS SDKs and CLIs when invoking
an external credential process.

The command fetches the X509-SVID from the SPIFFE Workload API. The location of
the SPIFFE Workload API endpoint should be specified using the
`SPIFFE_ENDPOINT_SOCKET` environment variable or the `--workload-api-addr` flag.

Example usage:

```sh
$ aws-spiffe-workload-helper x509-credential-process \
    --trust-anchor-arn arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000 \
    --profile-arn arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-000000000000 \
    --role-arn arn:aws:iam::123456789012:role/example-role \
    --workload-api-addr unix:///opt/workload-api.sock
```

##### Reference

| Flag              | Required | Description                                                                                                                                                                              | Example                                                                                         |
|-------------------|----------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------|
| role-arn          | Yes      | The ARN of the role to assume. Required.                                                                                                                                                 | `arn:aws:iam::123456789012:role/example-role`                                                   |
| profile-arn       | Yes      | The ARN of the Roles Anywhere profile to use. Required.                                                                                                                                  | `arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000`       |
| trust-anchor-arn  | Yes      | The ARN of the Roles Anywhere trust anchor to use. Required.                                                                                                                             | `arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000` |
| region            | No       | Overrides AWS region to use when exchanging the SVID for AWS credentials. Optional.                                                                                                      | `us-east-1`                                                                                     |
| session-duration  | No       | The duration, in seconds, of the resulting session. Optional. Can range from 15 minutes (900) to 12 hours (43200).                                                                       | `3600`                                                                                          |
| workload-api-addr | No       | Overrides the address of the Workload API endpoint that will be use to fetch the X509 SVID. If unspecified, the value from the SPIFFE_ENDPOINT_SOCKET environment variable will be used. | `unix:///opt/my/path/workload.sock`                                                             |

#### `x509-credential-file`

The `x509-credential-file` command starts a long-lived daemon which exchanges
an X509 SVID for a short-lived set of AWS credentials using the AWS Roles
Anywhere API. It writes the credentials to a specified file in the format 
supported by AWS SDKs and CLIs as a "credential file".

It repeats this exchange process when the AWS credentials are more than 50% of
the way through their lifetime, ensuring that a fresh set of credentials are
always available.

Whilst the `x509-credentials-process` flow should be preferred as it does not 
cause credentials to be written to the filesystem, the `x509-credentials-file`
flow may be useful in scenarios where you need to provide credentials to legacy
SDKs or CLIs that do not support the `credential_process` configuration.

The command fetches the X509-SVID from the SPIFFE Workload API. The location of
the SPIFFE Workload API endpoint should be specified using the
`SPIFFE_ENDPOINT_SOCKET` environment variable or the `--workload-api-addr` flag.

```sh
$ aws-spiffe-workload-helper x509-credential-file \
    --trust-anchor-arn arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000 \
    --profile-arn arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-000000000000 \
    --role-arn arn:aws:iam::123456789012:role/example-role \
    --workload-api-addr unix:///opt/workload-api.sock \
    --aws-credentials-file /opt/my-aws-credentials-file
```

###### Reference

| Flag                 | Required | Description                                                                                                                                                                              | Example                                                                                         |
|----------------------|----------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------|
| role-arn             | Yes      | The ARN of the role to assume. Required.                                                                                                                                                 | `arn:aws:iam::123456789012:role/example-role`                                                   |
| profile-arn          | Yes      | The ARN of the Roles Anywhere profile to use. Required.                                                                                                                                  | `arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000`       |
| trust-anchor-arn     | Yes      | The ARN of the Roles Anywhere trust anchor to use. Required.                                                                                                                             | `arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000` |
| region               | No       | Overrides AWS region to use when exchanging the SVID for AWS credentials. Optional.                                                                                                      | `us-east-1`                                                                                     |
| session-duration     | No       | The duration, in seconds, of the resulting session. Optional. Can range from 15 minutes (900) to 12 hours (43200).                                                                       | `3600`                                                                                          |
| workload-api-addr    | No       | Overrides the address of the Workload API endpoint that will be use to fetch the X509 SVID. If unspecified, the value from the SPIFFE_ENDPOINT_SOCKET environment variable will be used. | `unix:///opt/my/path/workload.sock`                                                             |
| aws-credentials-path | Yes      | The path to the AWS credentials file to write.                                                                                                                                           | `/opt/my-aws-credentials-file                                                                   |
| force                | No       | If set, failures loading the existing AWS credentials file will be ignored and the contents overwritten.                                                                                 |                                                                                                 |
| replace              | No       | If set, the AWS credentials file will be replaced if it exists. This will remove any profiles not written by this tool.                                                                  |                                                                                                 |

## Configuring AWS SDKs and CLIs

To configure AWS SDKs and CLIs to use Roles Anywhere and SPIFFE for
authentication, you will modify the AWS configuration file.

By default, AWS SDKs and CLIs will expect this file to be located at 
`~/.aws/config`. This location can be customized using the `AWS_CONFIG_FILE`
environment variable.

Example configuration:

```toml
[default]
credential_process = /usr/bin/aws-spiffe-workload-helper x509-credential-process --profile-arn arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-000000000000
--trust-anchor-arn arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000 --role-arn arn:aws:iam::123456789012:role/example-role
```

You can learn more about external credential processes at
<https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html>

## Contributing

We welcome contributions to this project. If you require any assistance, please
get in contact via the SPIFFE Slack.

### Governance

This is a ["tiny-project"](https://github.com/spiffe/spiffe/blob/main/NEW_PROJECTS.md#tiny-projects).

Dispute resolution is handled via escalation to the [SPIFFE Steering Committee (SSC)](https://github.com/spiffe/spiffe/blob/main/GOVERNANCE.md#the-spiffe-steering-committee-ssc).
