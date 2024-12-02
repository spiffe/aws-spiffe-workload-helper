package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"
	"github.com/spiffe/aws-spiffe-workload-helper/internal"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

func newX509CredentialFileOneshotCmd() (*cobra.Command, error) {
	force := false
	replace := false
	awsCredentialsPath := ""
	sf := &sharedFlags{}
	cmd := &cobra.Command{
		Use:   "x509-credential-file-oneshot",
		Short: ``,
		Long:  ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			return oneshotX509CredentialFile(
				cmd.Context(), force, replace, awsCredentialsPath, sf,
			)
		},
	}
	if err := sf.addFlags(cmd); err != nil {
		return nil, fmt.Errorf("adding shared flags: %w", err)
	}
	cmd.Flags().StringVar(&awsCredentialsPath, "aws-credentials-path", "", "The path to the AWS credentials file to write.")
	if err := cmd.MarkFlagRequired("aws-credentials-path"); err != nil {
		return nil, fmt.Errorf("marking aws-credentials-path flag as required: %w", err)
	}
	cmd.Flags().BoolVar(&force, "force", false, "If set, failures loading the existing AWS credentials file will be ignored and the contents overwritten.")
	cmd.Flags().BoolVar(&replace, "replace", false, "If set, the AWS credentials file will be replaced if it exists. This will remove any profiles not written by this tool.")

	return cmd, nil
}

func oneshotX509CredentialFile(
	ctx context.Context,
	force bool,
	replace bool,
	awsCredentialsPath string,
	sf *sharedFlags,
) error {
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
}

func newX509CredentialFileCmd() (*cobra.Command, error) {
	force := false
	replace := false
	awsCredentialsPath := ""
	sf := &sharedFlags{}
	cmd := &cobra.Command{
		Use:   "x509-credential-file",
		Short: ``,
		Long:  ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			return daemonX509CredentialFile(
				cmd.Context(), force, replace, awsCredentialsPath, sf,
			)
		},
		// Hidden for now as the daemon is likely more "usable"
		Hidden: true,
	}
	if err := sf.addFlags(cmd); err != nil {
		return nil, fmt.Errorf("adding shared flags: %w", err)
	}
	cmd.Flags().StringVar(&awsCredentialsPath, "aws-credentials-path", "", "The path to the AWS credentials file to write.")
	if err := cmd.MarkFlagRequired("aws-credentials-path"); err != nil {
		return nil, fmt.Errorf("marking aws-credentials-path flag as required: %w", err)
	}
	cmd.Flags().BoolVar(&force, "force", false, "If set, failures loading the existing AWS credentials file will be ignored and the contents overwritten.")
	cmd.Flags().BoolVar(&replace, "replace", false, "If set, the AWS credentials file will be replaced if it exists. This will remove any profiles not written by this tool.")

	return cmd, nil
}

func daemonX509CredentialFile(
	ctx context.Context,
	force bool,
	replace bool,
	awsCredentialsPath string,
	sf *sharedFlags,
) error {
	slog.Info("Starting AWS credential file daemon")
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

	slog.Debug("Fetching initial X509 SVID")
	x509Source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClient(client))
	if err != nil {
		return fmt.Errorf("creating x509 source: %w", err)
	}
	defer func() {
		if err := x509Source.Close(); err != nil {
			slog.Warn("Failed to close x509 source", "error", err)
		}
	}()

	svidUpdate := x509Source.Updated()
	svid, err := x509Source.GetX509SVID()
	if err != nil {
		return fmt.Errorf("fetching initial X509 SVID: %w", err)
	}
	slog.Debug("Fetched initial X509 SVID", slog.Group("svid",
		"spiffe_id", svid.ID,
		"hint", svid.Hint,
		"expires_at", svid.Certificates[0].NotAfter,
	))

	for {
		slog.Debug("Exchanging X509 SVID for AWS credentials")
		credentials, err := exchangeX509SVIDForAWSCredentials(sf, svid)
		if err != nil {
			return fmt.Errorf("exchanging X509 SVID for AWS credentials: %w", err)
		}
		slog.Info(
			"Successfully exchanged X509 SVID for AWS credentials",
		)

		expiresAt, err := time.Parse(time.RFC3339, credentials.Expiration)
		if err != nil {
			return fmt.Errorf("parsing expiration time: %w", err)
		}

		slog.Debug("Writing AWS credentials to file", "path", awsCredentialsPath)
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
		slog.Info("Wrote AWS credentials to file", "path", awsCredentialsPath)

		// Calculate next renewal time as 50% of the remaining time left on the
		// AWS credentials.
		// TODO(noah): This is a little crude, it may make more sense to just
		// renew on a fixed basis (e.g every minute?). We'll go with this
		// for now, and speak to consumers once it's in use to see if a
		// different mechanism may be more suitable.
		now := time.Now()
		awsTTL := expiresAt.Sub(now)
		renewIn := awsTTL / 2
		awsRenewAt := now.Add(renewIn)

		slog.Info(
			"Sleeping until a new X509 SVID is received or the AWS credentials are close to expiry",
			"aws_expires_at", expiresAt,
			"aws_ttl", awsTTL,
			"aws_renews_at", awsRenewAt,
			"svid_expires_at", svid.Certificates[0].NotAfter,
			"svid_ttl", svid.Certificates[0].NotAfter.Sub(now),
		)

		select {
		case <-time.After(time.Until(awsRenewAt)):
			slog.Info("Triggering renewal as AWS credentials are close to expiry")
		case <-svidUpdate:
			slog.Debug("Received potential X509 SVID update")
			newSVID, err := x509Source.GetX509SVID()
			if err != nil {
				return fmt.Errorf("fetching updated X509 SVID: %w", err)
			}
			slog.Info(
				"Received new X509 SVID from Workload API, will update AWS credentials",
				slog.Group("svid",
					"spiffe_id", newSVID.ID,
					"hint", newSVID.Hint,
					"expires_at", newSVID.Certificates[0].NotAfter,
				),
			)
			svid = newSVID
		case <-ctx.Done():
			return nil
		}
	}
}
