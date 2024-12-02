package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	awsspiffe "github.com/spiffe/aws-spiffe-workload-helper"
	"github.com/spiffe/aws-spiffe-workload-helper/internal/vendoredaws"
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

	return rootCmd, nil
}

type sharedFlags struct {
	roleARN         string
	region          string
	profileARN      string
	sessionDuration int
	trustAnchorARN  string
	roleSessionName string
	workloadAPIAddr string
}

func (f *sharedFlags) addFlags(cmd *cobra.Command) error {
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

func exchangeX509SVIDForAWSCredentials(
	sf *sharedFlags,
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
