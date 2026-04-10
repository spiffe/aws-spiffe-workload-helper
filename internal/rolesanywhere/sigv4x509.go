package rolesanywhere

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// signerParams holds the parameters needed for SigV4-X509 signing.
type signerParams struct {
	OverriddenDate   time.Time
	RegionName       string
	ServiceName      string
	SigningAlgorithm string
}

const (
	aws4X509RSASHA256   = "AWS4-X509-RSA-SHA256"
	aws4X509ECDSASHA256 = "AWS4-X509-ECDSA-SHA256"
	timeFormat          = "20060102T150405Z"
	shortTimeFormat     = "20060102"
	headerAMZDate       = "X-Amz-Date"
	headerAMZX509       = "X-Amz-X509"
	headerAMZX509Chain  = "X-Amz-X509-Chain"
	headerContentSHA256 = "X-Amz-Content-Sha256"
	headerAuthorization = "Authorization"
	headerHost          = "Host"
	emptyStringSHA256   = `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`
)

// ignoredHeaderKeys lists headers excluded from the signature calculation.
var ignoredHeaderKeys = map[string]bool{
	"Authorization":   true,
	"User-Agent":      true,
	"X-Amzn-Trace-Id": true,
}

func (sp *signerParams) getFormattedSigningDateTime() string {
	return sp.OverriddenDate.UTC().Format(timeFormat)
}

func (sp *signerParams) getFormattedShortSigningDateTime() string {
	return sp.OverriddenDate.UTC().Format(shortTimeFormat)
}

func (sp *signerParams) getScope() string {
	var sb strings.Builder
	sb.WriteString(sp.getFormattedShortSigningDateTime())
	sb.WriteString("/")
	sb.WriteString(sp.RegionName)
	sb.WriteString("/")
	sb.WriteString(sp.ServiceName)
	sb.WriteString("/")
	sb.WriteString("aws4_request")
	return sb.String()
}

func certificateToString(certificate *x509.Certificate) string {
	return base64.StdEncoding.EncodeToString(certificate.Raw)
}

func certificateChainToString(certificateChain []*x509.Certificate) string {
	var sb strings.Builder
	for i, certificate := range certificateChain {
		sb.WriteString(certificateToString(certificate))
		if i != len(certificateChain)-1 {
			sb.WriteString(",")
		}
	}
	return sb.String()
}

// signRequest signs an *http.Request using SigV4-X509.
func signRequest(
	req *http.Request,
	body []byte,
	signer crypto.Signer,
	signingAlgorithm string,
	certificate *x509.Certificate,
	certificateChain []*x509.Certificate,
	region string,
	serviceName string,
) error {
	sp := signerParams{
		OverriddenDate:   time.Now(),
		RegionName:       region,
		ServiceName:      serviceName,
		SigningAlgorithm: signingAlgorithm,
	}

	// Set headers required for signing.
	req.Header.Set(headerHost, req.URL.Host)
	req.Header.Set(headerAMZDate, sp.getFormattedSigningDateTime())
	req.Header.Set(headerAMZX509, certificateToString(certificate))
	if certificateChain != nil {
		req.Header.Set(headerAMZX509Chain, certificateChainToString(certificateChain))
	}

	contentHash := calculateContentHash(body)
	canonicalRequest, signedHeadersString := createCanonicalRequest(req, body, contentHash)
	stringToSign := createStringToSign(canonicalRequest, sp)

	signatureBytes, err := signer.Sign(rand.Reader, []byte(stringToSign), crypto.SHA256)
	if err != nil {
		return fmt.Errorf("signing request: %w", err)
	}
	signature := hex.EncodeToString(signatureBytes)

	req.Header.Set(headerAuthorization, buildAuthorizationHeader(signedHeadersString, signature, certificate, sp))
	return nil
}

func calculateContentHash(body []byte) string {
	if len(body) == 0 {
		return emptyStringSHA256
	}
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

func createCanonicalQueryString(r *http.Request) string {
	return strings.Replace(r.URL.Query().Encode(), "+", "%20", -1)
}

func createCanonicalHeaderString(r *http.Request) (string, string) {
	var headers []string
	signedHeaderVals := make(http.Header)
	for k, v := range r.Header {
		canonicalKey := http.CanonicalHeaderKey(k)
		if ignoredHeaderKeys[canonicalKey] {
			continue
		}

		lowerCaseKey := strings.ToLower(k)
		if _, ok := signedHeaderVals[lowerCaseKey]; ok {
			signedHeaderVals[lowerCaseKey] = append(signedHeaderVals[lowerCaseKey], v...)
			continue
		}

		headers = append(headers, lowerCaseKey)
		signedHeaderVals[lowerCaseKey] = v
	}
	sort.Strings(headers)

	headerValues := make([]string, len(headers))
	for i, k := range headers {
		headerValues[i] = k + ":" + strings.Join(signedHeaderVals[k], ",")
	}
	stripExcessSpaces(headerValues)
	return strings.Join(headerValues, "\n"), strings.Join(headers, ";")
}

const doubleSpace = "  "

// stripExcessSpaces rewrites the string values in the slice to not contain
// multiple side-by-side spaces.
func stripExcessSpaces(vals []string) {
	var j, k, l, m, spaces int
	for i, str := range vals {
		// Trim trailing spaces
		for j = len(str) - 1; j >= 0 && str[j] == ' '; j-- {
		}

		// Trim leading spaces
		for k = 0; k < j && str[k] == ' '; k++ {
		}
		str = str[k : j+1]

		// Strip multiple spaces.
		j = strings.Index(str, doubleSpace)
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

func createCanonicalRequest(r *http.Request, body []byte, contentSHA256 string) (string, string) {
	var sb strings.Builder
	canonicalHeaderString, signedHeadersString := createCanonicalHeaderString(r)
	sb.WriteString("POST")
	sb.WriteString("\n")
	sb.WriteString("/sessions")
	sb.WriteString("\n")
	sb.WriteString(createCanonicalQueryString(r))
	sb.WriteString("\n")
	sb.WriteString(canonicalHeaderString)
	sb.WriteString("\n\n")
	sb.WriteString(signedHeadersString)
	sb.WriteString("\n")
	sb.WriteString(contentSHA256)
	hashBytes := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(hashBytes[:]), signedHeadersString
}

func createStringToSign(canonicalRequest string, sp signerParams) string {
	var sb strings.Builder
	sb.WriteString(sp.SigningAlgorithm)
	sb.WriteString("\n")
	sb.WriteString(sp.getFormattedSigningDateTime())
	sb.WriteString("\n")
	sb.WriteString(sp.getScope())
	sb.WriteString("\n")
	sb.WriteString(canonicalRequest)
	return sb.String()
}

func buildAuthorizationHeader(signedHeadersString string, signature string, certificate *x509.Certificate, sp signerParams) string {
	signingCredentials := certificate.SerialNumber.String() + "/" + sp.getScope()
	credential := "Credential=" + signingCredentials
	signerHeaders := "SignedHeaders=" + signedHeadersString
	signatureHeader := "Signature=" + signature

	var sb strings.Builder
	sb.WriteString(sp.SigningAlgorithm)
	sb.WriteString(" ")
	sb.WriteString(credential)
	sb.WriteString(", ")
	sb.WriteString(signerHeaders)
	sb.WriteString(", ")
	sb.WriteString(signatureHeader)
	return sb.String()
}
