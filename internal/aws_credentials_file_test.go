package internal

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAWSCredentialsFile_Write(t *testing.T) {
	// TODO: Add more cases:
	// - If file exists, but is a bad ini and Force moe
	// - Replace mode
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config")
	log := slog.Default()

	err := UpsertAWSCredentialsFileProfile(
		log,
		AWSCredentialsFileConfig{
			Path: configPath,
		},
		AWSCredentialsFileProfile{
			AWSAccessKeyID:     "1234567890",
			AWSSecretAccessKey: "abcdefgh",
			AWSSessionToken:    "ijklmnop",
		},
	)
	require.NoError(t, err)

	got, err := os.ReadFile(configPath)
	require.NoError(t, err)

	require.Equal(t, `[default]
aws_secret_access_key = abcdefgh
aws_access_key_id     = 1234567890
aws_session_token     = ijklmnop
`, string(got))

	t.Run("bad file", func(t *testing.T) {
		tmp := t.TempDir()
		configPath := filepath.Join(tmp, "config")
		require.NoError(t, os.WriteFile(configPath, []byte("bad ini"), 0600))
		err := UpsertAWSCredentialsFileProfile(
			log,
			AWSCredentialsFileConfig{
				Path:  configPath,
				Force: true,
			},
			AWSCredentialsFileProfile{
				AWSAccessKeyID:     "1234567890",
				AWSSecretAccessKey: "abcdefgh",
				AWSSessionToken:    "ijklmnop",
			},
		)
		require.NoError(t, err)

		got, err := os.ReadFile(configPath)
		require.NoError(t, err)

		require.Equal(t, `[default]
aws_secret_access_key = abcdefgh
aws_access_key_id     = 1234567890
aws_session_token     = ijklmnop
`, string(got))
	})
}
