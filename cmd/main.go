package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/rolesanywhere-credential-helper/aws_signing_helper"
	"github.com/spf13/cobra"
	awsspiffe "github.com/spiffe/aws-spiffe-workload-helper"
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
		Short:   "TODO", // TODO(strideynet): Helpful, short description.
		Long:    `TODO`, // TODO(strideynet): Helpful, long description.
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

	return rootCmd, nil
}

func newX509CredentialProcessCmd() (*cobra.Command, error) {
	var (
		roleARN         string
		region          string
		profileARN      string
		sessionDuration int
		trustAnchorARN  string
		roleSessionName string
	)
	cmd := &cobra.Command{
		Use:   "x509-credential-process",
		Short: "TODO", // TODO(strideynet): Helpful, short description.
		Long:  `TODO`, // TODO(strideynet): Helpful, long description.
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := workloadapi.New(ctx) // TODO(strideynet): Ability to configure workload api endpoint with flag
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

			signer := &awsspiffe.X509SVIDSigner{
				SVID: svid,
			}
			signatureAlgorithm, err := signer.SignatureAlgorithm()
			if err != nil {
				return fmt.Errorf("getting signature algorithm: %w", err)
			}
			credentials, err := aws_signing_helper.GenerateCredentials(&aws_signing_helper.CredentialsOpts{
				RoleArn:           roleARN,
				ProfileArnStr:     profileARN,
				Region:            region,
				RoleSessionName:   roleSessionName,
				TrustAnchorArnStr: trustAnchorARN,
				SessionDuration:   sessionDuration,
			}, signer, signatureAlgorithm)
			if err != nil {
				return fmt.Errorf("generating credentials: %w", err)
			}
			slog.Debug(
				"Generated AWS credentials",
				"expiration", credentials.Expiration,
			)

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
	cmd.Flags().StringVar(&roleARN, "role-arn", "", "The ARN of the role to assume. Required.")
	if err := cmd.MarkFlagRequired("role-arn"); err != nil {
		return nil, fmt.Errorf("marking role-arn flag as required: %w", err)
	}
	cmd.Flags().StringVar(&region, "region", "", "The AWS region to use. Optional.")
	cmd.Flags().StringVar(&profileARN, "profile-arn", "", "The ARN of the Roles Anywhere profile to use. Required.")
	if err := cmd.MarkFlagRequired("profile-arn"); err != nil {
		return nil, fmt.Errorf("marking profile-arn flag as required: %w", err)
	}
	cmd.Flags().IntVar(&sessionDuration, "session-duration", 3600, "The duration, in seconds, of the resulting session. Optional. Can range from 15 minutes (900) to 12 hours (43200).")
	cmd.Flags().StringVar(&trustAnchorARN, "trust-anchor-arn", "", "The ARN of the Roles Anywhere trust anchor to use. Required.")
	if err := cmd.MarkFlagRequired("trust-anchor-arn"); err != nil {
		return nil, fmt.Errorf("marking trust-anchor-arn flag as required: %w", err)
	}
	cmd.Flags().StringVar(&roleSessionName, "role-session-name", "", "The identifier for the role session. Optional.")
	return cmd, nil
}
