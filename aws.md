# AWS Roles Anywhere

Some notes on authenticating to AWS using X.509 certificates through the 
AWS Roles Anywhere API.

## Useful Resources

- https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication.html
- https://docs.aws.amazon.com/rolesanywhere/latest/userguide/trust-model.html
- https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication-sign-process.html

## Important Constraints

### Certificates

> End entity certificates must satisfy the following constraints to be used for authentication:
>
> - The certificates MUST be X.509v3.
> - Basic constraints MUST include CA: false.
> - The key usage MUST include Digital Signature.
> - The signing algorithm MUST include SHA256 or stronger. MD5 and SHA1 signing algorithms are rejected.

### Keys

> RSA and EC keys are supported; RSA keys are used with the RSA PKCS# v1.5 signing algorithm. EC keys are used with the ECDSA.

This seems like a relatively small problem. The SPIFFE spec does not make 
comment on permissible key types, and therefore, an implementation of SPIFFE
could choose to use something other than EC or RSA. However, most
implementations of SPIFFE today (e.g SPIRE, Teleport Workload Identity) use
either EC or RSA as the default and indeed only support EC or RSA.

