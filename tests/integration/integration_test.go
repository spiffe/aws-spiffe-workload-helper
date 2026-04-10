package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spiffe/aws-spiffe-workload-helper/cmd/cli"
	"github.com/spiffe/aws-spiffe-workload-helper/tests/integration/internal/fakeawsapi"
	"github.com/spiffe/aws-spiffe-workload-helper/tests/integration/internal/fakespiffeapi"
	"github.com/spiffe/aws-spiffe-workload-helper/internal/rolesanywhere"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/ini.v1"
)

const (
	testRoleARN        = "arn:aws:iam::123456789012:role/test-role"
	testProfileARN     = "arn:aws:rolesanywhere:us-east-1:123456789012:profile/test-profile"
	testTrustAnchorARN = "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/test-anchor"
)

func TestX509CredentialProcess(t *testing.T) {
	ca := fakespiffeapi.NewCA(t)
	spiffeAddr := fakespiffeapi.Start(t, fakespiffeapi.Config{
		X509Response: ca.CreateX509SVIDResponse(t),
	})
	awsSrv := fakeawsapi.Start(t, fakeawsapi.Config{
		CACert: ca.CACert,
		RolesAnywhere: &fakeawsapi.RolesAnywhereExpectations{
			RoleARN:        testRoleARN,
			ProfileARN:     testProfileARN,
			TrustAnchorARN: testTrustAnchorARN,
		},
	})

	rootCmd, err := cli.NewRootCmd("test")
	require.NoError(t, err)

	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{
		"x509-credential-process",
		"--workload-api-addr", spiffeAddr,
		"--role-arn", testRoleARN,
		"--profile-arn", testProfileARN,
		"--trust-anchor-arn", testTrustAnchorARN,
		"--region", "us-east-1",
		"--endpoint", awsSrv.URL,
	})
	require.NoError(t, rootCmd.Execute())

	var creds rolesanywhere.CredentialProcessOutput
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &creds))
	assert.Equal(t, 1, creds.Version)
	assert.Equal(t, fakeawsapi.AccessKeyID, creds.AccessKeyId)
	assert.Equal(t, fakeawsapi.SecretAccessKey, creds.SecretAccessKey)
	assert.Equal(t, fakeawsapi.SessionToken, creds.SessionToken)
	assert.NotEmpty(t, creds.Expiration)
}

func TestX509CredentialFileOneshot(t *testing.T) {
	ca := fakespiffeapi.NewCA(t)
	spiffeAddr := fakespiffeapi.Start(t, fakespiffeapi.Config{
		X509Response: ca.CreateX509SVIDResponse(t),
	})
	awsSrv := fakeawsapi.Start(t, fakeawsapi.Config{
		CACert: ca.CACert,
		RolesAnywhere: &fakeawsapi.RolesAnywhereExpectations{
			RoleARN:        testRoleARN,
			ProfileARN:     testProfileARN,
			TrustAnchorARN: testTrustAnchorARN,
		},
	})

	credFile := filepath.Join(t.TempDir(), "aws-credentials")

	rootCmd, err := cli.NewRootCmd("test")
	require.NoError(t, err)
	rootCmd.SetArgs([]string{
		"x509-credential-file-oneshot",
		"--workload-api-addr", spiffeAddr,
		"--role-arn", testRoleARN,
		"--profile-arn", testProfileARN,
		"--trust-anchor-arn", testTrustAnchorARN,
		"--region", "us-east-1",
		"--endpoint", awsSrv.URL,
		"--aws-credentials-path", credFile,
		"--replace",
	})
	require.NoError(t, rootCmd.Execute())

	f, err := ini.Load(credFile)
	require.NoError(t, err)
	sec := f.Section("default")
	assert.Equal(t, fakeawsapi.AccessKeyID, sec.Key("aws_access_key_id").String())
	assert.Equal(t, fakeawsapi.SecretAccessKey, sec.Key("aws_secret_access_key").String())
	assert.Equal(t, fakeawsapi.SessionToken, sec.Key("aws_session_token").String())
}

func TestX509CredentialFile(t *testing.T) {
	ca := fakespiffeapi.NewCA(t)
	spiffeAddr := fakespiffeapi.Start(t, fakespiffeapi.Config{
		X509Response: ca.CreateX509SVIDResponse(t),
	})
	awsSrv := fakeawsapi.Start(t, fakeawsapi.Config{
		CACert: ca.CACert,
		RolesAnywhere: &fakeawsapi.RolesAnywhereExpectations{
			RoleARN:        testRoleARN,
			ProfileARN:     testProfileARN,
			TrustAnchorARN: testTrustAnchorARN,
		},
	})

	credFile := filepath.Join(t.TempDir(), "aws-credentials")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rootCmd, err := cli.NewRootCmd("test")
	require.NoError(t, err)
	rootCmd.SetArgs([]string{
		"x509-credential-file",
		"--workload-api-addr", spiffeAddr,
		"--role-arn", testRoleARN,
		"--profile-arn", testProfileARN,
		"--trust-anchor-arn", testTrustAnchorARN,
		"--region", "us-east-1",
		"--endpoint", awsSrv.URL,
		"--aws-credentials-path", credFile,
		"--replace",
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- rootCmd.ExecuteContext(ctx)
	}()

	// Wait for the credential file to appear with valid content.
	require.Eventually(t, func() bool {
		_, err := os.Stat(credFile)
		return err == nil
	}, 15*time.Second, 100*time.Millisecond, "credential file never appeared")

	f, err := ini.Load(credFile)
	require.NoError(t, err)
	sec := f.Section("default")
	assert.Equal(t, fakeawsapi.AccessKeyID, sec.Key("aws_access_key_id").String())
	assert.Equal(t, fakeawsapi.SecretAccessKey, sec.Key("aws_secret_access_key").String())
	assert.Equal(t, fakeawsapi.SessionToken, sec.Key("aws_session_token").String())

	// Stop the daemon.
	cancel()
	require.NoError(t, <-errCh)
}

func TestJWTCredentialProcess(t *testing.T) {
	ca := fakespiffeapi.NewCA(t)
	audience := "sts.amazonaws.com"
	spiffeAddr := fakespiffeapi.Start(t, fakespiffeapi.Config{
		JWTResponse: ca.CreateJWTSVIDResponse(t, audience),
	})
	awsSrv := fakeawsapi.Start(t, fakeawsapi.Config{})

	rootCmd, err := cli.NewRootCmd("test")
	require.NoError(t, err)

	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{
		"jwt-credential-process",
		"--workload-api-addr", spiffeAddr,
		"--audience", audience,
		"--endpoint", awsSrv.URL,
	})
	require.NoError(t, rootCmd.Execute())

	var creds rolesanywhere.CredentialProcessOutput
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &creds))
	assert.Equal(t, 1, creds.Version)
	assert.Equal(t, fakeawsapi.AccessKeyID, creds.AccessKeyId)
	assert.Equal(t, fakeawsapi.SecretAccessKey, creds.SecretAccessKey)
	assert.Equal(t, fakeawsapi.SessionToken, creds.SessionToken)
	assert.NotEmpty(t, creds.Expiration)
}
