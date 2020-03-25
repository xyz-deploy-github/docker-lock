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
			generator, err := generate.NewGenerator(flags)
			if err != nil {
				return err
			}
			lFile, err := os.Create(generator.LockfileName)
			if err != nil {
				return err
			}
			defer lFile.Close()
			if err := generator.GenerateLockfile(wm, lFile); err != nil {
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
	baseDir, err := cmd.Flags().GetString("base-dir")
	if err != nil {
		return nil, err
	}
	lockfileName, err := cmd.Flags().GetString("lockfile-name")
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
	dockerfiles, err := cmd.Flags().GetStringSlice("dockerfiles")
	if err != nil {
		return nil, err
	}
	composefiles, err := cmd.Flags().GetStringSlice("compose-files")
	if err != nil {
		return nil, err
	}
	dockerfileGlobs, err := cmd.Flags().GetStringSlice("dockerfile-globs")
	if err != nil {
		return nil, err
	}
	composefileGlobs, err := cmd.Flags().GetStringSlice("compose-file-globs")
	if err != nil {
		return nil, err
	}
	dockerfileRecursive, err := cmd.Flags().GetBool("dockerfile-recursive")
	if err != nil {
		return nil, err
	}
	composefileRecursive, err := cmd.Flags().GetBool("compose-file-recursive")
	if err != nil {
		return nil, err
	}
	dockerfileEnvBuildArgs, err := cmd.Flags().GetBool(
		"dockerfile-env-build-args",
	)
	if err != nil {
		return nil, err
	}
	return generate.NewFlags(
		baseDir, lockfileName, configFile, envFile,
		dockerfiles, composefiles, dockerfileGlobs, composefileGlobs,
		dockerfileRecursive, composefileRecursive, dockerfileEnvBuildArgs,
	)
}
