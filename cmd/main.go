package main

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/rolesanywhere-credential-helper/aws_signing_helper"
	"github.com/spf13/cobra"
	awsspiffe "github.com/spiffe/aws-spiffe-workload-helper"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

var (
	version = "dev"
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "aws-spiffe-workload-helper",
		Short:   "TODO", // TODO(strideynet): Helpful, short description.
		Long:    `TODO`, // TODO(strideynet): Helpful, long description.
		Version: version,
	}

	x509CredentialProcessCmd := newX509CredentialProcessCmd()
	rootCmd.AddCommand(x509CredentialProcessCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func newX509CredentialProcessCmd() *cobra.Command {
	var (
		roleARN         string
		region          string
		profileARN      string
		sessionDuration time.Duration
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

			signer := &awsspiffe.X509SVIDSigner{
				SVID: svid,
			}
			signatureAlgorithm, err := signer.SignatureAlgorithm()
			if err != nil {
				return fmt.Errorf("getting signature algorithm: %w", err)
			}
			aws_signing_helper.GenerateCredentials(&aws_signing_helper.CredentialsOpts{
				RoleArn:           roleARN,
				ProfileArnStr:     profileARN,
				Region:            region,
				RoleSessionName:   roleSessionName,
				TrustAnchorArnStr: trustAnchorARN,
			}, signer, signatureAlgorithm)

			return nil
		},
	}
	// TODO(strideynet): Review flag help strings.
	cmd.Flags().StringVar(&roleARN, "role-arn", "", "TODO. Required.")
	cmd.MarkFlagRequired("role-arn")
	cmd.Flags().StringVar(&region, "region", "", "TODO")
	cmd.Flags().StringVar(&profileARN, "profile-arn", "", "TODO. Required.")
	cmd.MarkFlagRequired("profile-arn")
	cmd.Flags().DurationVar(&sessionDuration, "session-duration", 0, "TODO")
	cmd.Flags().StringVar(&trustAnchorARN, "trust-anchor-arn", "", "TODO. Required.")
	cmd.MarkFlagRequired("trust-anchor-arn")
	cmd.Flags().StringVar(&roleSessionName, "role-session-name", "", "TODO")
	return cmd
}
