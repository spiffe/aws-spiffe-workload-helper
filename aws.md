# AWS Roles Anywhere

## Useful Resources

- https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication.html
- https://docs.aws.amazon.com/rolesanywhere/latest/userguide/trust-model.html
- https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication-sign-process.html

## Constraints

End entity certificates must satisfy the following constraints to be used for authentication:
- The certificates MUST be X.509v3.
- Basic constraints MUST include CA: false.
- The key usage MUST include Digital Signature.
- The signing algorithm MUST include SHA256 or stronger. MD5 and SHA1 signing algorithms are rejected.

> RSA and EC keys are supported; RSA keys are used with the RSA PKCS# v1.5 signing algorithm. EC keys are used with the ECDSA.

