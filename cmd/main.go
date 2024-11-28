package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	awsspiffe "github.com/spiffe/aws-spiffe-workload-helper"
	"github.com/spiffe/aws-spiffe-workload-helper/internal"
	"github.com/spiffe/aws-spiffe-workload-helper/internal/vendoredaws"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
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

	oneShotCredentialWriteCmd, err := newX509CredentialFileCmd()
	if err != nil {
		return nil, fmt.Errorf("initializing x509-credential-file command: %w", err)
	}
	rootCmd.AddCommand(oneShotCredentialWriteCmd)

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

func newX509CredentialFileCmd() (*cobra.Command, error) {
	oneshot := false
	force := false
	replace := false
	awsCredentialsPath := ""
	sf := &sharedFlags{}
	cmd := &cobra.Command{
		Use:   "x509-credential-file",
		Short: ``,
		Long:  ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := workloadapi.New(
				ctx,
				workloadapi.WithAddr(sf.workloadAPIAddr),
			)
			if err != nil {
				return fmt.Errorf("creating workload api client: %w", err)
			}

			x509Ctx, err := client.FetchX509Context(ctx)
			if err != nil {
				return fmt.Errorf("fetching x509 context: %w", err)
			}
			svid := x509Ctx.DefaultSVID()
			slog.Debug(
				"Fetched X509 SVID",
				slog.Group("svid",
					"spiffe_id", svid.ID,
					"hint", svid.Hint,
				),
			)

			credentials, err := exchangeX509SVIDForAWSCredentials(sf, svid)
			if err != nil {
				return fmt.Errorf("exchanging X509 SVID for AWS credentials: %w", err)
			}

			// Now we write this to disk in the format that the AWS CLI/SDK
			// expects for a credentials file.
			err = internal.UpsertAWSCredentialsFileProfile(
				slog.Default(),
				internal.AWSCredentialsFileConfig{
					Path:        awsCredentialsPath,
					Force:       force,
					ReplaceFile: replace,
				},
				internal.AWSCredentialsFileProfile{
					AWSAccessKeyID:     credentials.AccessKeyId,
					AWSSecretAccessKey: credentials.SecretAccessKey,
					AWSSessionToken:    credentials.SessionToken,
				},
			)
			if err != nil {
				return fmt.Errorf("writing credentials to file: %w", err)
			}
			slog.Info("Wrote AWS credential to file", "path", "./my-credential")
			return nil
		},
		// Hidden for now as the daemon is likely more "usable"
		Hidden: true,
	}
	if err := sf.addFlags(cmd); err != nil {
		return nil, fmt.Errorf("adding shared flags: %w", err)
	}
	cmd.Flags().BoolVar(&oneshot, "oneshot", false, "If set, generate the AWS credentials file and exit. If unset, run as a daemon that continuously updates the file before expiry or when a new SVID is available.")
	cmd.Flags().StringVar(&awsCredentialsPath, "aws-credentials-path", "", "The path to the AWS credentials file to write.")
	if err := cmd.MarkFlagRequired("aws-credentials-path"); err != nil {
		return nil, fmt.Errorf("marking aws-credentials-path flag as required: %w", err)
	}
	cmd.Flags().BoolVar(&force, "force", false, "If set, failures loading the existing AWS credentials file will be ignored and the contents overwritten.")
	cmd.Flags().BoolVar(&replace, "replace", false, "If set, the AWS credentials file will be replaced if it exists. This will remove any profiles not written by this tool.")

	return cmd, nil
}

func newX509CredentialProcessCmd() (*cobra.Command, error) {
	sf := &sharedFlags{}
	cmd := &cobra.Command{
		Use:   "x509-credential-process",
		Short: `Exchanges an X509 SVID for a short-lived set of AWS credentials using AWS Roles Anywhere. Compatible with the AWS credential process functionality.`,
		Long:  `Exchanges an X509 SVID for a short-lived set of AWS credentials using the AWS Roles Anywhere API. It returns the credentials to STDOUT, in the format expected by AWS SDKs and CLIs when invoking an external credential process.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := workloadapi.New(
				ctx,
				workloadapi.WithAddr(sf.workloadAPIAddr),
			)
			if err != nil {
				return fmt.Errorf("creating workload api client: %w", err)
			}

			x509Ctx, err := client.FetchX509Context(ctx)
			if err != nil {
				return fmt.Errorf("fetching x509 context: %w", err)
			}
			// TODO(strideynet): Implement SVID selection mechanism, for now,
			// we'll just use the first returned SVID (a.k.a the default).
			svid := x509Ctx.DefaultSVID()
			slog.Debug(
				"Fetched X509 SVID",
				slog.Group("svid",
					"spiffe_id", svid.ID,
					"hint", svid.Hint,
				),
			)

			credentials, err := exchangeX509SVIDForAWSCredentials(sf, svid)
			if err != nil {
				return fmt.Errorf("exchanging X509 SVID for AWS credentials: %w", err)
			}

			out, err := json.Marshal(credentials)
			if err != nil {
				return fmt.Errorf("marshalling credentials: %w", err)
			}
			_, err = os.Stdout.Write(out)
			if err != nil {
				return fmt.Errorf("writing credentials to stdout: %w", err)
			}
			return nil
		},
	}
	if err := sf.addFlags(cmd); err != nil {
		return nil, fmt.Errorf("adding shared flags: %w", err)
	}

	return cmd, nil
}
