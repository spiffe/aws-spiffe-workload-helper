package fakeawsapi

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	// Canned credential values returned by the fake AWS API.
	AccessKeyID     = "ASIATESTCREDENTIAL"
	SecretAccessKey = "fake-secret-access-key"
	SessionToken    = "fake-session-token"
)

// Expiration returns a fixed expiration time for the canned credentials.
func Expiration() string {
	return time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
}

// RolesAnywhereExpectations holds expected values for Roles Anywhere request
// validation. When set, the handler asserts that the request query parameters
// carry these exact values.
type RolesAnywhereExpectations struct {
	RoleARN        string
	ProfileARN     string
	TrustAnchorARN string
}

// Config configures the fake AWS API server.
type Config struct {
	// CACert is the trust anchor used to verify SigV4-X509 signatures on
	// Roles Anywhere requests. Required for X.509 tests.
	CACert *x509.Certificate
	// RolesAnywhere holds expected values for Roles Anywhere request
	// validation. If nil, ARN query parameters are not checked.
	RolesAnywhere *RolesAnywhereExpectations
}

// Start creates a fake AWS API HTTP server that handles both:
//   - Roles Anywhere CreateSession (POST /sessions)
//   - STS AssumeRoleWithWebIdentity (POST / with Action query param)
//
// For Roles Anywhere requests, the server performs full SigV4-X509 signature
// verification using the CA certificate from cfg as the trust anchor. This
// validates the complete signing flow: certificate chain verification, serial
// number binding, canonical request construction, and ECDSA signature.
//
// The server is automatically closed when the test completes.
func Start(t *testing.T, cfg Config) *httptest.Server {
	t.Helper()

	expiration := Expiration()

	mux := http.NewServeMux()
	mux.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		rolesAnywhereHandler(t, w, r, expiration, cfg)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("Action") == "AssumeRoleWithWebIdentity" {
			stsHandler(t, w, r, expiration)
			return
		}
		http.NotFound(w, r)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func rolesAnywhereHandler(t *testing.T, w http.ResponseWriter, r *http.Request, expiration string, cfg Config) {
	t.Helper()

	// Read the body so we can compute its hash for signature verification.
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		t.Errorf("Roles Anywhere: reading request body: %v", err)
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}

	if err := verifySigV4X509(r, bodyBytes, cfg.CACert); err != nil {
		t.Errorf("Roles Anywhere: SigV4-X509 verification failed: %v", err)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// Validate that the expected ARNs were sent as query parameters.
	// The AWS SDK sends profileArn, roleArn, and trustAnchorArn as
	// querystring parameters per the CreateSessionInput struct.
	if exp := cfg.RolesAnywhere; exp != nil {
		q := r.URL.Query()
		if got := q.Get("roleArn"); got != exp.RoleARN {
			t.Errorf("Roles Anywhere: roleArn = %q, want %q", got, exp.RoleARN)
		}
		if got := q.Get("profileArn"); got != exp.ProfileARN {
			t.Errorf("Roles Anywhere: profileArn = %q, want %q", got, exp.ProfileARN)
		}
		if got := q.Get("trustAnchorArn"); got != exp.TrustAnchorARN {
			t.Errorf("Roles Anywhere: trustAnchorArn = %q, want %q", got, exp.TrustAnchorARN)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
  "credentialSet": [{
    "assumedRoleUser": {
      "arn": "arn:aws:sts::123456789012:assumed-role/test-role/session",
      "assumedRoleId": "AROA3XFRBF23:session"
    },
    "credentials": {
      "accessKeyId": %q,
      "secretAccessKey": %q,
      "sessionToken": %q,
      "expiration": %q
    },
    "roleArn": "arn:aws:iam::123456789012:role/test-role"
  }]
}`, AccessKeyID, SecretAccessKey, SessionToken, expiration)
}

func stsHandler(t *testing.T, w http.ResponseWriter, r *http.Request, expiration string) {
	t.Helper()

	if r.Method != http.MethodPost {
		t.Errorf("STS: expected POST, got %s", r.Method)
	}

	token := r.URL.Query().Get("WebIdentityToken")
	if token == "" {
		t.Errorf("STS: missing WebIdentityToken query parameter")
	}

	w.Header().Set("Content-Type", "text/xml")
	fmt.Fprintf(w, `<AssumeRoleWithWebIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleWithWebIdentityResult>
    <AssumedRoleUser>
      <Arn>arn:aws:sts::123456789012:assumed-role/test-role/session</Arn>
      <AssumeRoleId>AROA3XFRBF23:session</AssumeRoleId>
    </AssumedRoleUser>
    <Credentials>
      <AccessKeyId>%s</AccessKeyId>
      <SecretAccessKey>%s</SecretAccessKey>
      <SessionToken>%s</SessionToken>
      <Expiration>%s</Expiration>
    </Credentials>
  </AssumeRoleWithWebIdentityResult>
  <ResponseMetadata/>
</AssumeRoleWithWebIdentityResponse>`, AccessKeyID, SecretAccessKey, SessionToken, expiration)
}

// authHeader holds parsed components of a SigV4-X509 Authorization header.
type authHeader struct {
	algorithm     string
	credential    string // serialNumber/date/region/service/aws4_request
	signedHeaders []string
	signature     string // hex-encoded
}

// parseAuthorizationHeader parses an AWS SigV4-X509 Authorization header.
// Format: Algorithm Credential=..., SignedHeaders=..., Signature=...
func parseAuthorizationHeader(header string) (authHeader, error) {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return authHeader{}, fmt.Errorf("invalid format: %q", header)
	}

	auth := authHeader{algorithm: parts[0]}
	for _, field := range strings.Split(parts[1], ", ") {
		kv := strings.SplitN(field, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "Credential":
			auth.credential = kv[1]
		case "SignedHeaders":
			auth.signedHeaders = strings.Split(kv[1], ";")
		case "Signature":
			auth.signature = kv[1]
		}
	}

	if auth.credential == "" || len(auth.signedHeaders) == 0 || auth.signature == "" {
		return authHeader{}, fmt.Errorf("missing required fields in Authorization header")
	}
	return auth, nil
}

// verifySigV4X509 performs full server-side verification of a SigV4-X509
// signed request, following the process described at:
// https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication-sign-process.html
//
// It validates:
//  1. The signing certificate (from X-Amz-X509) chains to the trusted CA
//  2. The certificate serial number matches the Credential field
//  3. The canonical request is correctly reconstructed
//  4. The ECDSA signature over the string-to-sign is valid
func verifySigV4X509(r *http.Request, body []byte, caCert *x509.Certificate) error {
	if r.Method != http.MethodPost {
		return fmt.Errorf("expected POST, got %s", r.Method)
	}

	// 1. Parse the Authorization header.
	auth, err := parseAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		return fmt.Errorf("parsing Authorization: %w", err)
	}
	if auth.algorithm != "AWS4-X509-ECDSA-SHA256" {
		return fmt.Errorf("unexpected algorithm %q, expected AWS4-X509-ECDSA-SHA256", auth.algorithm)
	}

	// 2. Decode the signing certificate from the X-Amz-X509 header.
	x509B64 := r.Header.Get("X-Amz-X509")
	if x509B64 == "" {
		return fmt.Errorf("missing X-Amz-X509 header")
	}
	certDER, err := base64.StdEncoding.DecodeString(x509B64)
	if err != nil {
		return fmt.Errorf("decoding X-Amz-X509: %w", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return fmt.Errorf("parsing signing certificate: %w", err)
	}

	// 3. Verify the certificate chains to the trusted CA.
	roots := x509.NewCertPool()
	roots.AddCert(caCert)
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		return fmt.Errorf("certificate chain verification: %w", err)
	}

	// 4. Verify the serial number in the Credential matches the certificate.
	// Credential format: serialNumber/date/region/service/aws4_request
	credParts := strings.SplitN(auth.credential, "/", 2)
	if len(credParts) != 2 {
		return fmt.Errorf("invalid Credential format: %s", auth.credential)
	}
	if cert.SerialNumber.String() != credParts[0] {
		return fmt.Errorf("serial number mismatch: cert=%s credential=%s",
			cert.SerialNumber.String(), credParts[0])
	}

	// 5. Reconstruct the canonical request.
	// Format matches vendoredaws/signer.go createCanonicalRequest().
	contentHash := sha256Hex(body)
	canonicalHeaders, signedHeadersStr := buildCanonicalHeaders(r, auth.signedHeaders)

	var cr strings.Builder
	cr.WriteString(r.Method)
	cr.WriteString("\n")
	cr.WriteString(r.URL.Path)
	cr.WriteString("\n")
	cr.WriteString(buildCanonicalQueryString(r))
	cr.WriteString("\n")
	cr.WriteString(canonicalHeaders)
	cr.WriteString("\n\n")
	cr.WriteString(signedHeadersStr)
	cr.WriteString("\n")
	cr.WriteString(contentHash)

	canonicalRequestHash := sha256Hex([]byte(cr.String()))

	// 6. Reconstruct the string to sign.
	// Format matches vendoredaws/signer.go CreateStringToSign().
	scope := credParts[1] // date/region/service/aws4_request
	amzDate := r.Header.Get("X-Amz-Date")
	if amzDate == "" {
		return fmt.Errorf("missing X-Amz-Date header")
	}

	var sts strings.Builder
	sts.WriteString(auth.algorithm)
	sts.WriteString("\n")
	sts.WriteString(amzDate)
	sts.WriteString("\n")
	sts.WriteString(scope)
	sts.WriteString("\n")
	sts.WriteString(canonicalRequestHash)

	// 7. Verify the ECDSA signature.
	sigBytes, err := hex.DecodeString(auth.signature)
	if err != nil {
		return fmt.Errorf("decoding signature hex: %w", err)
	}

	ecPub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("expected ECDSA public key, got %T", cert.PublicKey)
	}

	// The X509SVIDSigner (signer.go) hashes the stringToSign with SHA256
	// before calling ecdsa.SignASN1. We must hash the same way to verify.
	hash := sha256.Sum256([]byte(sts.String()))
	if !ecdsa.VerifyASN1(ecPub, hash[:], sigBytes) {
		return fmt.Errorf("ECDSA signature verification failed")
	}

	return nil
}

// buildCanonicalHeaders reconstructs the canonical header string from the
// signed headers list, matching the format in vendoredaws/signer.go.
func buildCanonicalHeaders(r *http.Request, signedHeaders []string) (string, string) {
	sorted := make([]string, len(signedHeaders))
	copy(sorted, signedHeaders)
	sort.Strings(sorted)

	parts := make([]string, len(sorted))
	for i, h := range sorted {
		var value string
		if h == "host" {
			// Go's net/http stores the Host header in r.Host, not r.Header.
			value = r.Host
		} else {
			values := r.Header.Values(h)
			value = strings.Join(values, ",")
		}
		parts[i] = h + ":" + value
	}
	stripExcessSpaces(parts)
	return strings.Join(parts, "\n"), strings.Join(sorted, ";")
}

// buildCanonicalQueryString produces the SigV4 canonical query string.
func buildCanonicalQueryString(r *http.Request) string {
	return strings.Replace(r.URL.Query().Encode(), "+", "%20", -1)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// stripExcessSpaces removes leading/trailing whitespace and collapses multiple
// consecutive spaces. Matches vendoredaws/signer.go stripExcessSpaces().
func stripExcessSpaces(vals []string) {
	var j, k, l, m, spaces int
	for i, str := range vals {
		for j = len(str) - 1; j >= 0 && str[j] == ' '; j-- {
		}
		for k = 0; k < j && str[k] == ' '; k++ {
		}
		str = str[k : j+1]

		j = strings.Index(str, "  ")
		if j < 0 {
			vals[i] = str
			continue
		}

		buf := []byte(str)
		for k, m, l = j, j, len(buf); k < l; k++ {
			if buf[k] == ' ' {
				if spaces == 0 {
					buf[m] = buf[k]
					m++
				}
				spaces++
			} else {
				spaces = 0
				buf[m] = buf[k]
				m++
			}
		}
		vals[i] = string(buf[:m])
	}
}
