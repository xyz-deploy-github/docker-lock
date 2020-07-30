package cmd

import (
	"log"
	"os"

	"github.com/michaelperel/docker-lock/generate"
	"github.com/michaelperel/docker-lock/registry"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewGenerateCmd creates the command 'generate' used in 'docker lock generate'.
func NewGenerateCmd(client *registry.HTTPClient) (*cobra.Command, error) {
	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a Lockfile to track image digests",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags, err := generatorFlags(cmd)
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

			g, err := generate.NewGenerator(flags)
			if err != nil {
				return err
			}

			lfile, err := os.Create(g.LockfileName)
			if err != nil {
				return err
			}
			defer lfile.Close()

			if err := g.GenerateLockfile(wm, lfile); err != nil {
				return err
			}

			return nil
		},
	}
	generateCmd.Flags().StringP(
		"base-dir", "b", ".", "Top level directory to collect files from",
	)
	generateCmd.Flags().StringSliceP(
		"dockerfiles", "d", []string{}, "Path to Dockerfiles",
	)
	generateCmd.Flags().StringSliceP(
		"composefiles", "c", []string{}, "Path to docker-compose files",
	)
	generateCmd.Flags().StringP(
		"lockfile-name", "l", "docker-lock.json",
		"Lockfile name to be output in the current working directory",
	)
	generateCmd.Flags().StringSlice(
		"dockerfile-globs", []string{}, "Glob pattern to select Dockerfiles",
	)
	generateCmd.Flags().StringSlice(
		"composefile-globs", []string{},
		"Glob pattern to select docker-compose files",
	)
	generateCmd.Flags().Bool(
		"dockerfile-recursive", false, "Recursively collect Dockerfiles",
	)
	generateCmd.Flags().Bool(
		"composefile-recursive", false,
		"Recursively collect docker-compose files",
	)
	generateCmd.Flags().String(
		"config-file", defaultConfigPath(),
		"Path to config file for auth credentials",
	)
	generateCmd.Flags().StringP(
		"env-file", "e", ".env", "Path to .env file",
	)
	generateCmd.Flags().Bool(
		"dockerfile-env-build-args", false,
		"Use environment vars as build args for Dockerfiles",
	)
	generateCmd.Flags().BoolP(
		"verbose", "v", false, "Show logs",
	)

	if err := viper.BindPFlags(generateCmd.Flags()); err != nil {
		return nil, err
	}

	return generateCmd, nil
}

// generatorFlags gets values from the command and uses them to
// create Flags.
func generatorFlags(cmd *cobra.Command) (*generate.Flags, error) {
	var (
		bDir, lName, configFile, envFile                   string
		dfiles, cfiles, dGlobs, cGlobs                     []string
		dRecursive, cRecursive, dfileEnvBuildArgs, verbose bool
		err                                                error
	)

	switch viper.ConfigFileUsed() {
	case "":
		verbose, err = cmd.Flags().GetBool("verbose")
		if err != nil {
			return nil, err
		}

		bDir, err = cmd.Flags().GetString("base-dir")
		if err != nil {
			return nil, err
		}

		lName, err = cmd.Flags().GetString("lockfile-name")
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

		dfiles, err = cmd.Flags().GetStringSlice("dockerfiles")
		if err != nil {
			return nil, err
		}

		cfiles, err = cmd.Flags().GetStringSlice("composefiles")
		if err != nil {
			return nil, err
		}

		dGlobs, err = cmd.Flags().GetStringSlice("dockerfile-globs")
		if err != nil {
			return nil, err
		}

		cGlobs, err = cmd.Flags().GetStringSlice("composefile-globs")
		if err != nil {
			return nil, err
		}

		dRecursive, err = cmd.Flags().GetBool("dockerfile-recursive")
		if err != nil {
			return nil, err
		}

		cRecursive, err = cmd.Flags().GetBool("composefile-recursive")
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
		bDir = viper.GetString("base-dir")
		lName = viper.GetString("lockfile-name")
		configFile = viper.GetString("config-file")
		envFile = viper.GetString("env-file")
		dfiles = viper.GetStringSlice("dockerfiles")
		cfiles = viper.GetStringSlice("composefiles")
		dGlobs = viper.GetStringSlice("dockerfile-globs")
		cGlobs = viper.GetStringSlice("composefile-globs")
		dRecursive = viper.GetBool("dockerfile-recursive")
		cRecursive = viper.GetBool("composefile-recursive")
		dfileEnvBuildArgs = viper.GetBool("dockerfile-env-build-args")
		verbose = viper.GetBool("verbose")
	}

	return generate.NewFlags(
		bDir, lName, configFile, envFile,
		dfiles, cfiles, dGlobs, cGlobs,
		dRecursive, cRecursive, dfileEnvBuildArgs, verbose,
	)
}
