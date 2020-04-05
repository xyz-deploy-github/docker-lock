package cmd

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/michaelperel/docker-lock/generate"
	"github.com/michaelperel/docker-lock/registry"
	"github.com/spf13/cobra"
)

// NewGenerateCmd creates the command 'generate' used in 'docker lock generate'.
func NewGenerateCmd(client *registry.HTTPClient) *cobra.Command {
	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a Lockfile to track image digests",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags, err := getGeneratorFlags(cmd)
			if err != nil {
				return err
			}

			_ = godotenv.Load(flags.EnvFile)

			wm, err := getDefaultWrapperManager(flags.ConfigFile, client)
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
		"compose-files", "c", []string{}, "Path to docker-compose files",
	)
	generateCmd.Flags().StringP(
		"lockfile-name", "l", "docker-lock.json",
		"Lockfile name to be output in the current working directory",
	)
	generateCmd.Flags().StringSlice(
		"dockerfile-globs", []string{}, "Glob pattern to select Dockerfiles",
	)
	generateCmd.Flags().StringSlice(
		"compose-file-globs", []string{},
		"Glob pattern to select docker-compose files",
	)
	generateCmd.Flags().Bool(
		"dockerfile-recursive", false, "Recursively collect Dockerfiles",
	)
	generateCmd.Flags().Bool(
		"compose-file-recursive", false,
		"Recursively collect docker-compose files",
	)
	generateCmd.Flags().String(
		"config-file", getDefaultConfigPath(),
		"Path to config file for auth credentials",
	)
	generateCmd.Flags().String(
		"env-file", ".env", "Path to .env file",
	)
	generateCmd.Flags().Bool(
		"dockerfile-env-build-args", false,
		"Use environment vars as build args for Dockerfiles",
	)

	return generateCmd
}

func getGeneratorFlags(cmd *cobra.Command) (*generate.Flags, error) {
	bDir, err := cmd.Flags().GetString("base-dir")
	if err != nil {
		return nil, err
	}

	lName, err := cmd.Flags().GetString("lockfile-name")
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

	dfiles, err := cmd.Flags().GetStringSlice("dockerfiles")
	if err != nil {
		return nil, err
	}

	cfiles, err := cmd.Flags().GetStringSlice("compose-files")
	if err != nil {
		return nil, err
	}

	dGlobs, err := cmd.Flags().GetStringSlice("dockerfile-globs")
	if err != nil {
		return nil, err
	}

	cGlobs, err := cmd.Flags().GetStringSlice("compose-file-globs")
	if err != nil {
		return nil, err
	}

	dRecursive, err := cmd.Flags().GetBool("dockerfile-recursive")
	if err != nil {
		return nil, err
	}

	cRecursive, err := cmd.Flags().GetBool("compose-file-recursive")
	if err != nil {
		return nil, err
	}

	dfileEnvBuildArgs, err := cmd.Flags().GetBool(
		"dockerfile-env-build-args",
	)
	if err != nil {
		return nil, err
	}

	return generate.NewFlags(
		bDir, lName, configFile, envFile,
		dfiles, cfiles, dGlobs, cGlobs,
		dRecursive, cRecursive, dfileEnvBuildArgs,
	)
}
