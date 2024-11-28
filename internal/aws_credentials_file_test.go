package internal

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAWSCredentialsFile_Write(t *testing.T) {
	log := slog.Default()

	defaultProfile := AWSCredentialsFileProfile{
		AWSAccessKeyID:     "1234567890",
		AWSSecretAccessKey: "abcdefgh",
		AWSSessionToken:    "ijklmnop",
	}

	preExistingContents := []byte(`[pre-existing]
aws_secret_access_key = foo
aws_access_key_id     = bar
aws_session_token     = bizz
`)

	tests := []struct {
		name                 string
		existingFileContents []byte
		config               AWSCredentialsFileConfig
		profile              AWSCredentialsFileProfile
		want                 []byte
		wantErr              string
	}{
		{
			name:    "no pre-existing file - default profile",
			config:  AWSCredentialsFileConfig{},
			profile: defaultProfile,
			want: []byte(`[default]
aws_secret_access_key = abcdefgh
aws_access_key_id     = 1234567890
aws_session_token     = ijklmnop
`),
		},
		{
			name: "no pre-existing file - named profile",
			config: AWSCredentialsFileConfig{
				ProfileName: "my-profile",
			},
			profile: defaultProfile,
			want: []byte(`[my-profile]
aws_secret_access_key = abcdefgh
aws_access_key_id     = 1234567890
aws_session_token     = ijklmnop
`),
		},
		{
			name:                 "pre-existing file, no profile name clash - default profile",
			config:               AWSCredentialsFileConfig{},
			profile:              defaultProfile,
			existingFileContents: preExistingContents,
			want: []byte(`[pre-existing]
aws_secret_access_key = foo
aws_access_key_id     = bar
aws_session_token     = bizz

[default]
aws_secret_access_key = abcdefgh
aws_access_key_id     = 1234567890
aws_session_token     = ijklmnop
`),
		},
		{
			name: "pre-existing file, no profile name clash - default profile with replace mode",
			config: AWSCredentialsFileConfig{
				ReplaceFile: true,
			},
			profile:              defaultProfile,
			existingFileContents: preExistingContents,
			want: []byte(`[default]
aws_secret_access_key = abcdefgh
aws_access_key_id     = 1234567890
aws_session_token     = ijklmnop
`),
		},
		{
			name: "pre-existing file, profile name clash",
			config: AWSCredentialsFileConfig{
				ProfileName: "pre-existing",
			},
			profile:              defaultProfile,
			existingFileContents: preExistingContents,
			want: []byte(`[pre-existing]
aws_secret_access_key = abcdefgh
aws_access_key_id     = 1234567890
aws_session_token     = ijklmnop
`),
		},
		{
			name:                 "pre-existing file with garbage",
			config:               AWSCredentialsFileConfig{},
			profile:              defaultProfile,
			existingFileContents: []byte(`dduhufd`),
			wantErr:              "key-value delimiter not found",
		},
		{
			name: "pre-existing file with garbage, --force",
			config: AWSCredentialsFileConfig{
				Force: true,
			},
			profile:              defaultProfile,
			existingFileContents: []byte(`dduhufd`),
			want: []byte(`[default]
aws_secret_access_key = abcdefgh
aws_access_key_id     = 1234567890
aws_session_token     = ijklmnop
`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			credentialPath := filepath.Join(tmp, "credentials")
			cfg := tt.config
			cfg.Path = credentialPath

			if tt.existingFileContents != nil {
				require.NoError(t, os.WriteFile(credentialPath, tt.existingFileContents, 0600))
			}

			err := UpsertAWSCredentialsFileProfile(
				log,
				cfg,
				tt.profile,
			)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			got, err := os.ReadFile(credentialPath)
			require.NoError(t, err)

			require.Equal(t, string(tt.want), string(got))
		})
	}
}
