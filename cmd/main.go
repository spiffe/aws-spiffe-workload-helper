package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
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

	var (
		roleARN         string
		region          string
		profileARN      string
		sessionDuration time.Duration
		trustAnchorARN  string
		roleSessionName string
	)
	x509CredentialProcessCmd := &cobra.Command{
		Use:   "x509-credential-process",
		Short: "TODO", // TODO(strideynet): Helpful, short description.
		Long:  `TODO`, // TODO(strideynet): Helpful, long description.
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Hello, World!")
		},
	}
	rootCmd.AddCommand(x509CredentialProcessCmd)
	// TODO(strideynet): Review flag help strings.
	x509CredentialProcessCmd.Flags().StringVar(&roleARN, "role-arn", "", "TODO. Required.")
	x509CredentialProcessCmd.MarkFlagRequired("role-arn")
	x509CredentialProcessCmd.Flags().StringVar(&region, "region", "", "TODO")
	x509CredentialProcessCmd.Flags().StringVar(&profileARN, "profile-arn", "", "TODO. Required.")
	x509CredentialProcessCmd.MarkFlagRequired("profile-arn")
	x509CredentialProcessCmd.Flags().DurationVar(&sessionDuration, "session-duration", 0, "TODO")
	x509CredentialProcessCmd.Flags().StringVar(&trustAnchorARN, "trust-anchor-arn", "", "TODO. Required.")
	x509CredentialProcessCmd.MarkFlagRequired("trust-anchor-arn")
	x509CredentialProcessCmd.Flags().StringVar(&roleSessionName, "role-session-name", "", "TODO")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
