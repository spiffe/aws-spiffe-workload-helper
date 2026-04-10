package rolesanywhere

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// CredentialProcessOutput adheres to the format of credential_process output
// as specified by AWS.
type CredentialProcessOutput struct {
	Version        int    `json:"Version"`
	AccessKeyId    string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken   string `json:"SessionToken"`
	Expiration     string `json:"Expiration"`
}

// CreateSessionInput holds the parameters for the CreateSession API call.
type CreateSessionInput struct {
	RoleARN          string
	ProfileARN       string
	TrustAnchorARN   string
	Region           string
	Endpoint         string
	SessionDuration  int
	RoleSessionName  string
	Certificate      *x509.Certificate
	CertificateChain []*x509.Certificate
	Signer           crypto.Signer
	SigningAlgorithm string
}

// parseARNRegion extracts the region (4th colon-delimited field) from an ARN.
func parseARNRegion(arnStr string) (string, error) {
	parts := strings.SplitN(arnStr, ":", 5)
	if len(parts) < 5 {
		return "", fmt.Errorf("invalid ARN: %q", arnStr)
	}
	return parts[3], nil
}

// CreateSession calls the IAM Roles Anywhere CreateSession API and returns
// temporary AWS credentials.
func CreateSession(ctx context.Context, input CreateSessionInput) (CredentialProcessOutput, error) {
	// Validate that trust anchor and profile ARN regions match.
	taRegion, err := parseARNRegion(input.TrustAnchorARN)
	if err != nil {
		return CredentialProcessOutput{}, fmt.Errorf("parsing trust anchor ARN: %w", err)
	}
	profileRegion, err := parseARNRegion(input.ProfileARN)
	if err != nil {
		return CredentialProcessOutput{}, fmt.Errorf("parsing profile ARN: %w", err)
	}
	if taRegion != profileRegion {
		return CredentialProcessOutput{}, errors.New("trust anchor and profile regions don't match")
	}

	// Derive region.
	region := input.Region
	if region == "" {
		region = taRegion
	}

	// Derive endpoint.
	endpoint := input.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://rolesanywhere.%s.amazonaws.com", region)
	}

	// Build query parameters.
	reqURL := endpoint + "/sessions?profileArn=" + url.QueryEscape(input.ProfileARN) +
		"&roleArn=" + url.QueryEscape(input.RoleARN) +
		"&trustAnchorArn=" + url.QueryEscape(input.TrustAnchorARN)

	// Build JSON body.
	type requestBody struct {
		Cert            string `json:"cert"`
		DurationSeconds int    `json:"durationSeconds"`
		RoleSessionName string `json:"roleSessionName,omitempty"`
	}
	rb := requestBody{
		Cert:            base64.StdEncoding.EncodeToString(input.Certificate.Raw),
		DurationSeconds: input.SessionDuration,
	}
	if input.RoleSessionName != "" {
		rb.RoleSessionName = input.RoleSessionName
	}
	bodyBytes, err := json.Marshal(rb)
	if err != nil {
		return CredentialProcessOutput{}, fmt.Errorf("marshaling request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return CredentialProcessOutput{}, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Sign the request using SigV4-X509.
	if err := signRequest(
		req,
		bodyBytes,
		input.Signer,
		input.SigningAlgorithm,
		input.Certificate,
		input.CertificateChain,
		region,
		"rolesanywhere",
	); err != nil {
		return CredentialProcessOutput{}, fmt.Errorf("signing request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return CredentialProcessOutput{}, fmt.Errorf("performing CreateSession request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return CredentialProcessOutput{}, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return CredentialProcessOutput{}, fmt.Errorf("CreateSession returned %d: %s", resp.StatusCode, respBody)
	}

	// Parse the JSON response.
	var createSessionResp struct {
		CredentialSet []struct {
			Credentials struct {
				AccessKeyId     string `json:"accessKeyId"`
				SecretAccessKey string `json:"secretAccessKey"`
				SessionToken    string `json:"sessionToken"`
				Expiration      string `json:"expiration"`
			} `json:"credentials"`
		} `json:"credentialSet"`
	}
	if err := json.Unmarshal(respBody, &createSessionResp); err != nil {
		return CredentialProcessOutput{}, fmt.Errorf("parsing CreateSession response: %w", err)
	}
	if len(createSessionResp.CredentialSet) == 0 {
		return CredentialProcessOutput{}, errors.New("unable to obtain temporary security credentials from CreateSession")
	}

	creds := createSessionResp.CredentialSet[0].Credentials
	return CredentialProcessOutput{
		Version:        1,
		AccessKeyId:    creds.AccessKeyId,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:   creds.SessionToken,
		Expiration:     creds.Expiration,
	}, nil
}
