package fakespiffeapi

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
)

const (
	trustDomain = "example.org"
	spiffeID    = "spiffe://example.org/workload"
)

// CA is a test certificate authority that generates X.509 and JWT SVIDs.
type CA struct {
	CACert *x509.Certificate
	CAKey  *ecdsa.PrivateKey
	JWTKey *ecdsa.PrivateKey
}

// NewCA creates a new test CA with a self-signed root certificate.
func NewCA(t *testing.T) *CA {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating CA key: %v", err)
	}

	spiffeURI, err := url.Parse("spiffe://" + trustDomain)
	if err != nil {
		t.Fatalf("parsing SPIFFE URI: %v", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "Test CA",
		},
		URIs:                  []*url.URL{spiffeURI},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("creating CA certificate: %v", err)
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		t.Fatalf("parsing CA certificate: %v", err)
	}

	jwtKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating JWT key: %v", err)
	}

	return &CA{
		CACert: caCert,
		CAKey:  caKey,
		JWTKey: jwtKey,
	}
}

// CreateX509SVIDResponse creates a workload API X509SVIDResponse with a leaf
// certificate signed by the CA.
func (ca *CA) CreateX509SVIDResponse(t *testing.T) *workload.X509SVIDResponse {
	t.Helper()

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating leaf key: %v", err)
	}

	spiffeURI, err := url.Parse(spiffeID)
	if err != nil {
		t.Fatalf("parsing SPIFFE URI: %v", err)
	}

	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "workload",
		},
		URIs:      []*url.URL{spiffeURI},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}

	leafCertDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, ca.CACert, &leafKey.PublicKey, ca.CAKey)
	if err != nil {
		t.Fatalf("creating leaf certificate: %v", err)
	}

	leafKeyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		t.Fatalf("marshalling leaf key: %v", err)
	}

	// X509Svid is the concatenation of leaf + CA cert DER bytes
	certChainDER := append(leafCertDER, ca.CACert.Raw...)

	return &workload.X509SVIDResponse{
		Svids: []*workload.X509SVID{
			{
				SpiffeId:    spiffeID,
				X509Svid:    certChainDER,
				X509SvidKey: leafKeyDER,
				Bundle:      ca.CACert.Raw,
			},
		},
	}
}

// CreateJWTSVIDResponse creates a workload API JWTSVIDResponse with a signed
// JWT token.
func (ca *CA) CreateJWTSVIDResponse(t *testing.T, audience string) *workload.JWTSVIDResponse {
	t.Helper()

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: ca.JWTKey},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		t.Fatalf("creating JWT signer: %v", err)
	}

	now := time.Now()
	claims := jwt.Claims{
		Subject:   spiffeID,
		Audience:  jwt.Audience{audience},
		Issuer:    "test-ca",
		IssuedAt:  jwt.NewNumericDate(now),
		Expiry:    jwt.NewNumericDate(now.Add(time.Hour)),
		NotBefore: jwt.NewNumericDate(now.Add(-time.Minute)),
	}

	token, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		t.Fatalf("signing JWT: %v", err)
	}

	return &workload.JWTSVIDResponse{
		Svids: []*workload.JWTSVID{
			{
				SpiffeId: spiffeID,
				Svid:     token,
			},
		},
	}
}
