package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spiffe/aws-spiffe-workload-helper/internal"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

func newJWTCredentialProcessCmd() (*cobra.Command, error) {
	sf := &sharedJWTFlags{}
	cmd := &cobra.Command{
		Use:   "jwt-credential-process",
		Short: `Exchanges an JWT SVID for a short-lived set of AWS credentials using AWS AssumeRoleWithWebIdentity. Compatible with the AWS credential process functionality.`,
		Long:  `Exchanges an JWT SVID for a short-lived set of AWS credentials using AWS AssumeRoleWithWebIdentity. It returns the credentials to STDOUT, in the format expected by AWS SDKs and CLIs when invoking an external credential process.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := workloadapi.New(
				ctx,
				workloadapi.WithAddr(sf.workloadAPIAddr),
				workloadapi.WithLogger(internal.NewSPIFFESlogAdapter(slog.Default())),
			)
			if err != nil {
				return fmt.Errorf("creating workload api client: %w", err)
			}
			defer func() {
				if err := client.Close(); err != nil {
					slog.Warn("Failed to close workload API client", "error", err)
				}
			}()

			params := jwtsvid.Params{
				Audience: sf.audience,
			}
			svids, err := client.FetchJWTSVIDs(ctx, params)
			if err != nil {
				return fmt.Errorf("fetching jwt: %w", err)
			}
			svid := svids[0]
			if sf.hint != "" {
				hints := make([]string, len(svids))
				found := false
				for i, s := range svids {
					if s.Hint == sf.hint {
						found = true
						svid = s
						break
					}
					hints[i] = s.Hint
				}
				if !found {
					return fmt.Errorf("could not find the specified SVID. Available hints [%s]", strings.Join(hints, ", "))
				}
			} else if len(svids) > 1 {
				slog.Warn("Received multiple SVIDs, but, no hint matcher was set. Selecting the first SVID.")
			}
			// TODO(strideynet): Implement SVID selection mechanism, for now,
			// we'll just use the first returned SVID (a.k.a the default).
			slog.Debug("Fetched JWT SVID", "svid", jwtSVIDValue(svid))

			credentials, err := exchangeJWTSVIDForAWSCredentials(sf, svid)
			if err != nil {
				return fmt.Errorf("exchanging JWT SVID for AWS credentials: %w", err)
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
