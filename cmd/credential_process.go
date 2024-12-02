package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

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
			defer func() {
				if err := client.Close(); err != nil {
					slog.Warn("Failed to close workload API client", "error", err)
				}
			}()

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
