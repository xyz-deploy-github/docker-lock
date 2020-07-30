package cmd

import (
	"log"

	"github.com/michaelperel/docker-lock/registry"
	"github.com/michaelperel/docker-lock/verify"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewVerifyCmd creates the command 'verify' used in 'docker lock verify'.
func NewVerifyCmd(client *registry.HTTPClient) (*cobra.Command, error) {
	verifyCmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify that a Lockfile is up-to-date",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags, err := verifierFlags(cmd)
			if err != nil {
				return err
			}

			configureLogger(flags.Verbose)

			log.Printf("Found flags '%+v'.", flags)

			if err = loadEnv(flags.EnvFile); err != nil {
				return err
			}

			wm, err := defaultWrapperManager(client, flags.ConfigFile)
			if err != nil {
				return err
			}

			v, err := verify.NewVerifier(flags)
			if err != nil {
				return err
			}

			if err := v.VerifyLockfile(wm); err != nil {
				return err
			}

			return nil
		},
	}
	verifyCmd.Flags().StringP(
		"lockfile-path", "l", "docker-lock.json", "Path to Lockfile",
	)
	verifyCmd.Flags().String(
		"config-file", defaultConfigPath(),
		"Path to config file for auth credentials",
	)
	verifyCmd.Flags().String(
		"env-file", ".env", "Path to .env file",
	)
	verifyCmd.Flags().Bool(
		"dockerfile-env-build-args", false,
		"Use environment vars as build args for Dockerfiles",
	)
	verifyCmd.Flags().BoolP(
		"verbose", "v", false, "Show logs",
	)

	if err := viper.BindPFlags(verifyCmd.Flags()); err != nil {
		return nil, err
	}

	return verifyCmd, nil
}

// verifierFlags gets values from the command and uses them to
// create Flags.
func verifierFlags(cmd *cobra.Command) (*verify.Flags, error) { //nolint: dupl
	var (
		lPath, configFile, envFile string
		dfileEnvBuildArgs, verbose bool
		err                        error
	)

	switch viper.ConfigFileUsed() {
	case "":
		verbose, err = cmd.Flags().GetBool("verbose")
		if err != nil {
			return nil, err
		}

		lPath, err = cmd.Flags().GetString("lockfile-path")
		if err != nil {
			return nil, err
		}

		configFile, err = cmd.Flags().GetString("config-file")
		if err != nil {
			return nil, err
		}

		envFile, err = cmd.Flags().GetString("env-file")
		if err != nil {
			return nil, err
		}

		dfileEnvBuildArgs, err = cmd.Flags().GetBool(
			"dockerfile-env-build-args",
		)
		if err != nil {
			return nil, err
		}
	default:
		lPath = viper.GetString("lockfile-path")
		configFile = viper.GetString("config-file")
		envFile = viper.GetString("env-file")
		dfileEnvBuildArgs = viper.GetBool("dockerfile-env-build-args")
		verbose = viper.GetBool("verbose")
	}

	return verify.NewFlags(
		lPath, configFile, envFile, dfileEnvBuildArgs, verbose,
	)
}
