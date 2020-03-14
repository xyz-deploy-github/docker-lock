package cmd

import (
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/michaelperel/docker-lock/registry"
	"github.com/michaelperel/docker-lock/verify"
	"github.com/spf13/cobra"
)

// NewVerifyCmd creates the command 'verify' used in 'docker lock verify'.
func NewVerifyCmd(client *registry.HTTPClient) *cobra.Command {
	verifyCmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify that a Lockfile is up-to-date",
		RunE: func(cmd *cobra.Command, args []string) error {
			envPath, err := cmd.Flags().GetString("env-file")
			if err != nil {
				return err
			}
			envPath = filepath.ToSlash(envPath)
			_ = godotenv.Load(envPath)
			configPath, err := cmd.Flags().GetString("config-file")
			if err != nil {
				return err
			}
			configPath = filepath.ToSlash(configPath)
			wm, err := getDefaultWrapperManager(configPath, client)
			if err != nil {
				return err
			}
			verifier, err := verify.NewVerifier(cmd)
			if err != nil {
				return err
			}
			if err := verifier.VerifyLockfile(wm); err != nil {
				return err
			}
			return nil
		},
	}
	verifyCmd.Flags().StringP(
		"lockfile-path", "l", "docker-lock.json", "Path to Lockfile",
	)
	verifyCmd.Flags().String(
		"config-file", getDefaultConfigPath(),
		"Path to config file for auth credentials",
	)
	verifyCmd.Flags().String(
		"env-file", ".env", "Path to .env file",
	)
	verifyCmd.Flags().Bool(
		"dockerfile-env-build-args", false,
		"Use environment vars as build args for Dockerfiles",
	)
	return verifyCmd
}
