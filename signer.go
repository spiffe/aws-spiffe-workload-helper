package awsspiffe

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"fmt"
	"io"

	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

// SPIFFESigner creates signatures compatible with the AWS RolesAnywhere
// API using an X509 SVID. It implements the aws_signing_helper.Signer
// interface.
type X509SVIDSigner struct {
	SVID *x509svid.SVID
}

func (s *X509SVIDSigner) Public() crypto.PublicKey {
	return s.SVID.PrivateKey.Public()
}

func (s *X509SVIDSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	// Note(strideynet):
	// As of the time of writing, it looks like the AWS signing helper will
	// only ever invoke Sign with SHA256, however, their signer implementations
	// do also support SHA384 and SHA512. It feels safest to support all three
	// here as well.
	//
	// Looking at the documentation for AWS SigV4, it looks like SHA256 is also
	// the only supported hash function today...
	var hash []byte
	switch opts.HashFunc() {
	case crypto.SHA256:
		sum := sha256.Sum256(digest)
		hash = sum[:]
	case crypto.SHA384:
		sum := sha512.Sum384(digest)
		hash = sum[:]
	case crypto.SHA512:
		sum := sha512.Sum512(digest)
		hash = sum[:]
	default:
		return nil, fmt.Errorf("unsupported hash function: %v", opts.HashFunc())
	}

	// From https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication.html
	// > RSA and EC keys are supported; RSA keys are used with the RSA PKCS#
	// > v1.5 signing algorithm. EC keys are used with the ECDSA.
	switch key := s.SVID.PrivateKey.(type) {
	case *rsa.PrivateKey:
		sig, err := rsa.SignPKCS1v15(rand, key, opts.HashFunc(), hash)
		if err != nil {
			return nil, fmt.Errorf("signing with RSA: %w", err)
		}
		return sig, nil
	case *ecdsa.PrivateKey:
		sig, err := ecdsa.SignASN1(rand, key, hash)
		if err != nil {
			return nil, fmt.Errorf("signing with ECDSA: %w", err)
		}
		return sig, nil
	default:
		return nil, fmt.Errorf("unsupported key type: %T", s.SVID.PrivateKey)
	}
}

// From https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication-sign-process.html
// > Algorithm. As described above, instead of AWS4-HMAC-SHA256, the algorithm
// > field will have the values of the form AWS4-X509-RSA-SHA256 or
// > AWS4-X509-ECDSA-SHA256, depending on whether an RSA or Elliptic Curve
// > algorithm is used. This, in turn, is determined by the key bound to the
// > signing certificate.
const (
	awsV4X509RSASHA256   = "AWS4-X509-RSA-SHA256"
	awsV4X509ECDSASHA256 = "AWS4-X509-ECDSA-SHA256"
)

// SignatureAlgorithm returns the signature algorithm of the underlying
// private key, in the representation expected by AWS.
// See https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication-sign-process.html
func (s *X509SVIDSigner) SignatureAlgorithm() (string, error) {
	switch s.SVID.PrivateKey.(type) {
	case *rsa.PrivateKey:
		return awsV4X509RSASHA256, nil
	case *ecdsa.PrivateKey:
		return awsV4X509ECDSASHA256, nil
	default:
		return "", fmt.Errorf("unsupported key type: %T", s.SVID.PrivateKey)
	}
}

func (s *X509SVIDSigner) Certificate() (*x509.Certificate, error) {
	return s.SVID.Certificates[0], nil
}

func (s *X509SVIDSigner) CertificateChain() ([]*x509.Certificate, error) {
	if len(s.SVID.Certificates) < 1 {
		return s.SVID.Certificates[1:], nil
	}
	return nil, nil
}

func (s *X509SVIDSigner) Close() {
	// Nothing to do here...
}
