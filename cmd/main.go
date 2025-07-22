package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	awsspiffe "github.com/spiffe/aws-spiffe-workload-helper"
	"github.com/spiffe/aws-spiffe-workload-helper/vendoredaws"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

var (
	version = "dev"
)

func main() {
	rootCmd, err := newRootCmd()
	if err != nil {
		slog.Error("Failed to initialize CLI", "error", err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		slog.Error("Encountered a fatal error during execution", "error", err)
		os.Exit(1)
	}
}

func newRootCmd() (*cobra.Command, error) {
	var debug bool
	rootCmd := &cobra.Command{
		Use:     "aws-spiffe-workload-helper",
		Short:   `A light-weight tool intended to assist in providing a workload with credentials for AWS using its SPIFFE identity.`,
		Long:    `A light-weight tool intended to assist in providing a workload with credentials for AWS using its SPIFFE identity.`,
		Version: version,
	}
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if debug {
			slog.SetLogLoggerLevel(slog.LevelDebug)
		}
	}

	x509CredentialProcessCmd, err := newX509CredentialProcessCmd()
	if err != nil {
		return nil, fmt.Errorf("initializing x509-credential-process command: %w", err)
	}
	rootCmd.AddCommand(x509CredentialProcessCmd)

	x509CredentialFileCmd, err := newX509CredentialFileCmd()
	if err != nil {
		return nil, fmt.Errorf("initializing x509-credential-file command: %w", err)
	}
	rootCmd.AddCommand(x509CredentialFileCmd)

	x509CredentialFileOneshotCmd, err := newX509CredentialFileOneshotCmd()
	if err != nil {
		return nil, fmt.Errorf("initializing x509-credential-file-oneshot command: %w", err)
	}
	rootCmd.AddCommand(x509CredentialFileOneshotCmd)

	JWTCredentialProcessCmd, err := newJWTCredentialProcessCmd()
	if err != nil {
		return nil, fmt.Errorf("initializing jwt-credential-process command: %w", err)
	}
	rootCmd.AddCommand(JWTCredentialProcessCmd)

	return rootCmd, nil
}

type sharedX509Flags struct {
	roleARN         string
	region          string
	profileARN      string
	sessionDuration int
	trustAnchorARN  string
	roleSessionName string
	workloadAPIAddr string
}

func (f *sharedX509Flags) addFlags(cmd *cobra.Command) error {
	cmd.Flags().StringVar(&f.roleARN, "role-arn", "", "The ARN of the role to assume. Required.")
	if err := cmd.MarkFlagRequired("role-arn"); err != nil {
		return fmt.Errorf("marking role-arn flag as required: %w", err)
	}
	cmd.Flags().StringVar(&f.region, "region", "", "Overrides AWS region to use when exchanging the SVID for AWS credentials. Optional.")
	cmd.Flags().StringVar(&f.profileARN, "profile-arn", "", "The ARN of the Roles Anywhere profile to use. Required.")
	if err := cmd.MarkFlagRequired("profile-arn"); err != nil {
		return fmt.Errorf("marking profile-arn flag as required: %w", err)
	}
	cmd.Flags().IntVar(&f.sessionDuration, "session-duration", 3600, "The duration, in seconds, of the resulting session. Optional. Can range from 15 minutes (900) to 12 hours (43200).")
	cmd.Flags().StringVar(&f.trustAnchorARN, "trust-anchor-arn", "", "The ARN of the Roles Anywhere trust anchor to use. Required.")
	if err := cmd.MarkFlagRequired("trust-anchor-arn"); err != nil {
		return fmt.Errorf("marking trust-anchor-arn flag as required: %w", err)
	}
	cmd.Flags().StringVar(&f.roleSessionName, "role-session-name", "", "The identifier for the role session. Optional.")
	cmd.Flags().StringVar(&f.workloadAPIAddr, "workload-api-addr", "", "Overrides the address of the Workload API endpoint that will be use to fetch the X509 SVID. If unspecified, the value from the SPIFFE_ENDPOINT_SOCKET environment variable will be used.")
	return nil
}

type sharedJWTFlags struct {
	roleARN         string
	audience        string
	endpoint        string
	sessionDuration int
	workloadAPIAddr string
	hint            string
}

func (f *sharedJWTFlags) addFlags(cmd *cobra.Command) error {
	cmd.Flags().StringVar(&f.audience, "audience", "", "Sets what audience will be used for the JWT. Required.")
	if err := cmd.MarkFlagRequired("audience"); err != nil {
		return fmt.Errorf("marking audience flag as required: %w", err)
	}
	cmd.Flags().StringVar(&f.endpoint, "endpoint", "", "The URL of the IAM endpoint. Required.")
	if err := cmd.MarkFlagRequired("endpoint"); err != nil {
		return fmt.Errorf("marking endpoint flag as required: %w", err)
	}
	cmd.Flags().IntVar(&f.sessionDuration, "session-duration", 3600, "The duration, in seconds, of the resulting session. Optional. Can range from 15 minutes (900) to 12 hours (43200).")
	cmd.Flags().StringVar(&f.workloadAPIAddr, "workload-api-addr", "", "Overrides the address of the Workload API endpoint that will be use to fetch the X509 SVID. If unspecified, the value from the SPIFFE_ENDPOINT_SOCKET environment variable will be used.")
	cmd.Flags().StringVar(&f.roleARN, "role-arn", "", "The ARN of the role to assume.")
	cmd.Flags().StringVar(&f.hint, "hint", "", "Hint to use to find the SVID.")
	return nil
}

func exchangeX509SVIDForAWSCredentials(
	sf *sharedX509Flags,
	svid *x509svid.SVID,
) (vendoredaws.CredentialProcessOutput, error) {
	signer := &awsspiffe.X509SVIDSigner{
		SVID: svid,
	}
	signatureAlgorithm, err := signer.SignatureAlgorithm()
	if err != nil {
		return vendoredaws.CredentialProcessOutput{}, fmt.Errorf("getting signature algorithm: %w", err)
	}
	credentials, err := vendoredaws.GenerateCredentials(&vendoredaws.CredentialsOpts{
		RoleArn:           sf.roleARN,
		ProfileArnStr:     sf.profileARN,
		Region:            sf.region,
		RoleSessionName:   sf.roleSessionName,
		TrustAnchorArnStr: sf.trustAnchorARN,
		SessionDuration:   sf.sessionDuration,
	}, signer, signatureAlgorithm)
	if err != nil {
		return vendoredaws.CredentialProcessOutput{}, fmt.Errorf("generating credentials: %w", err)
	}
	slog.Debug(
		"Generated AWS credentials",
		"expiration", credentials.Expiration,
	)
	return credentials, nil
}

type AssumeRoleWithWebIdentityResponse struct {
	XMLName                         xml.Name                        `xml:"https://sts.amazonaws.com/doc/2011-06-15/ AssumeRoleWithWebIdentityResponse"`
	AssumeRoleWithWebIdentityResult AssumeRoleWithWebIdentityResult `xml:"AssumeRoleWithWebIdentityResult"`
	ResponseMetadata                struct{}                        `xml:"ResponseMetadata"` // Empty struct if no fields are needed
}

type AssumeRoleWithWebIdentityResult struct {
	AssumedRoleUser AssumedRoleUser `xml:"AssumedRoleUser"`
	Credentials     Credentials     `xml:"Credentials"`
}

type AssumedRoleUser struct {
	Arn          string `xml:"Arn"`
	AssumeRoleId string `xml:"AssumeRoleId"`
}

type Credentials struct {
	AccessKeyId     string `xml:"AccessKeyId"`
	SecretAccessKey string `xml:"SecretAccessKey"`
	Expiration      string `xml:"Expiration"`
	SessionToken    string `xml:"SessionToken"`
}

func exchangeJWTSVIDForAWSCredentials(sf *sharedJWTFlags, svid *jwtsvid.SVID) (vendoredaws.CredentialProcessOutput, error) {
	token := svid.Marshal()
	u, err := url.Parse(sf.endpoint)
	if err != nil {
		return vendoredaws.CredentialProcessOutput{}, fmt.Errorf("error parsing URL: %v", err)
	}
	queryParams := u.Query()
	queryParams.Add("Action", "AssumeRoleWithWebIdentity")
	queryParams.Add("WebIdentityToken", token)
	queryParams.Add("Version", "2011-06-15")
	queryParams.Add("DurationSeconds", fmt.Sprintf("%d", sf.sessionDuration))
	if sf.roleARN != "" {
		queryParams.Add("RoleArn", sf.roleARN)
	}
	u.RawQuery = queryParams.Encode()
	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return vendoredaws.CredentialProcessOutput{}, fmt.Errorf("error making new request: %v", err)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return vendoredaws.CredentialProcessOutput{}, fmt.Errorf("error performing the sts request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return vendoredaws.CredentialProcessOutput{}, fmt.Errorf("error reading response: %v", err)
	}
	if resp.StatusCode != 200 {
		return vendoredaws.CredentialProcessOutput{}, fmt.Errorf("error performing the sts request: %d: %s: %s", resp.StatusCode, http.StatusText(resp.StatusCode), body)
	}
	var stsResponse AssumeRoleWithWebIdentityResponse
	err = xml.Unmarshal(body, &stsResponse)
	if err != nil {
		return vendoredaws.CredentialProcessOutput{}, fmt.Errorf("error parsing xml respopse: %v", err)
	}
	cpo := vendoredaws.CredentialProcessOutput{
		Version:         1,
		AccessKeyId:     stsResponse.AssumeRoleWithWebIdentityResult.Credentials.AccessKeyId,
		SecretAccessKey: stsResponse.AssumeRoleWithWebIdentityResult.Credentials.SecretAccessKey,
		SessionToken:    stsResponse.AssumeRoleWithWebIdentityResult.Credentials.SessionToken,
		Expiration:      stsResponse.AssumeRoleWithWebIdentityResult.Credentials.Expiration,
	}
	return cpo, nil
}

func svidValue(svid *x509svid.SVID) slog.Value {
	return slog.GroupValue(
		slog.String("id", svid.ID.String()),
		slog.String("hint", svid.Hint),
		slog.Time("expires_at", svid.Certificates[0].NotAfter),
	)
}

func jwtSVIDValue(svid *jwtsvid.SVID) slog.Value {
	return slog.GroupValue(
		slog.String("id", svid.ID.String()),
		slog.String("hint", svid.Hint),
		slog.Time("expires_at", svid.Expiry),
	)
}
