package main

import (
	"log/slog"
	"os"

	"github.com/spiffe/aws-spiffe-workload-helper/cmd/cli"
)

var (
	version = "dev"
)

func main() {
	rootCmd, err := cli.NewRootCmd(version)
	if err != nil {
		slog.Error("Failed to initialize CLI", "error", err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		slog.Error("Encountered a fatal error during execution", "error", err)
		os.Exit(1)
	}
}
