package cmd

import (
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
			flags, err := getVerifierFlags(cmd)
			if err != nil {
				return err
			}
			_ = godotenv.Load(flags.EnvFile)
			wm, err := getDefaultWrapperManager(flags.ConfigFile, client)
			if err != nil {
				return err
			}
			verifier, err := verify.NewVerifier(flags)
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

func getVerifierFlags(cmd *cobra.Command) (*verify.Flags, error) {
	lockfilePath, err := cmd.Flags().GetString("lockfile-path")
	if err != nil {
		return nil, err
	}
	configFile, err := cmd.Flags().GetString("config-file")
	if err != nil {
		return nil, err
	}
	envFile, err := cmd.Flags().GetString("env-file")
	if err != nil {
		return nil, err
	}
	dockerfileEnvBuildArgs, err := cmd.Flags().GetBool("dockerfile-env-build-args")
	if err != nil {
		return nil, err
	}
	return verify.NewFlags(
		lockfilePath, configFile, envFile, dockerfileEnvBuildArgs,
	)
}
