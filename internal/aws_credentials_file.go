package internal

import (
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/ini.v1"
)

type AWSCredentialsFileConfig struct {
	Path        string
	ProfileName string
	Force       bool
	ReplaceFile bool
}

type AWSCredentialsFileProfile struct {
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSSessionToken    string
}

func (p AWSCredentialsFileProfile) Validate() error {
	// TODO: Validate
	return nil
}

// UpsertAWSCredentialsFileProfile writes the provided AWS credentials profile to the AWS credentials file.
// See https://docs.aws.amazon.com/cli/v1/userguide/cli-configure-files.html
func UpsertAWSCredentialsFileProfile(
	log *slog.Logger,
	cfg AWSCredentialsFileConfig,
	p AWSCredentialsFileProfile,
) error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("validating aws credentials file profile: %w", err)
	}

	f, err := ini.Load(cfg.Path)
	if err != nil {
		if !os.IsNotExist(err) {
			if !cfg.Force {
				log.Error(
					"When loading the existing AWS credentials file, an error occurred. Use --force to ignore errors and attempt to overwrite.",
					"error", err,
					"path", cfg.Path,
				)
				return fmt.Errorf("loading existing aws credentials file: %w", err)
			}
			log.Warn(
				"When loading the existing AWS credentials file, an error occurred. As --force is set, the file will be overwritten.",
				"error", err,
				"path", cfg.Path,
			)
		}
		f = ini.Empty()
	}

	sectionName := "default"
	if cfg.ProfileName != "" {
		sectionName = cfg.ProfileName
	}
	sec := f.Section(sectionName)

	sec.Key("aws_secret_access_key").SetValue(p.AWSSecretAccessKey)
	sec.Key("aws_access_key_id").SetValue(p.AWSAccessKeyID)
	sec.Key("aws_session_token").SetValue(p.AWSSessionToken)

	if err := f.SaveTo(cfg.Path); err != nil {
		return fmt.Errorf("saving aws credentials file: %w", err)
	}
	return nil
}
