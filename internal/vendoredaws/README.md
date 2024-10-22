The code within this package is a partial vendoring of
https://github.com/aws/rolesanywhere-credential-helper/tree/main/aws_signing_helper

The original source is licensed under Apache 2.0, this license can be found in
`LICENSE`.

This code was vendored to break the dependency of `aws_signing_package` on
https://github.com/miekg/pkcs11, which requires CGO to build.

An issue is open with the upstream repository to break apart the packages to
avoid this dependency, at which point this vendoring will be obselete:
https://github.com/aws/rolesanywhere-credential-helper/issues/86