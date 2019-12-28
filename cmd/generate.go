package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
			if err := validateGenerateCmdFlags(cmd); err != nil {
				return err
			}
			envPath, err := cmd.Flags().GetString("env-file")
			if err != nil {
				return err
			}
			envPath = filepath.ToSlash(envPath)
			godotenv.Load(envPath)
			wrapperManager, err := getDefaultWrapperManager(cmd, client)
			if err != nil {
				return err
			}
			generator, err := generate.NewGenerator(cmd)
			if err != nil {
				return err
			}
			oFile, err := os.Create(generator.OutPath)
			if err != nil {
				return err
			}
			defer oFile.Close()
			if err := generator.GenerateLockfile(
				wrapperManager,
				oFile,
			); err != nil {
				return err
			}
			return nil
		},
	}
	generateCmd.Flags().String(
		"base-dir",
		".",
		"Top level directory to collect files from",
	)
	generateCmd.Flags().StringSlice(
		"dockerfiles",
		[]string{},
		"Path to Dockerfiles",
	)
	generateCmd.Flags().StringSlice(
		"compose-files",
		[]string{},
		"Path to docker-compose files",
	)
	generateCmd.Flags().StringSlice(
		"dockerfile-globs",
		[]string{},
		"Glob pattern to select Dockerfiles",
	)
	generateCmd.Flags().StringSlice(
		"compose-file-globs",
		[]string{},
		"Glob pattern to select docker-compose files",
	)
	generateCmd.Flags().Bool(
		"dockerfile-recursive",
		false,
		"Recursively collect Dockerfiles",
	)
	generateCmd.Flags().Bool(
		"compose-file-recursive",
		false,
		"Recursively collect docker-compose files",
	)
	generateCmd.Flags().String(
		"outpath",
		"docker-lock.json",
		"Path to save Lockfile",
	)
	generateCmd.Flags().String(
		"config-file",
		getDefaultConfigPath(),
		"Path to config file for auth credentials",
	)
	generateCmd.Flags().String(
		"env-file",
		".env",
		"Path to .env file",
	)
	generateCmd.Flags().Bool(
		"dockerfile-env-build-args",
		false,
		"Use environment vars as build args for Dockerfiles",
	)
	return generateCmd
}

func validateGenerateCmdFlags(cmd *cobra.Command) error {
	bDir, err := cmd.Flags().GetString("base-dir")
	if err != nil {
		return err
	}
	bDir = filepath.ToSlash(bDir)
	if err := validateBaseDir(bDir); err != nil {
		return err
	}
	dPaths, err := cmd.Flags().GetStringSlice("dockerfiles")
	if err != nil {
		return err
	}
	cPaths, err := cmd.Flags().GetStringSlice("compose-files")
	if err != nil {
		return err
	}
	for _, p := range [][]string{dPaths, cPaths} {
		if err := validateInputPaths(bDir, p); err != nil {
			return err
		}
	}
	dGlobs, err := cmd.Flags().GetStringSlice("dockerfile-globs")
	if err != nil {
		return err
	}
	cGlobs, err := cmd.Flags().GetStringSlice("compose-file-globs")
	if err != nil {
		return err
	}
	for _, g := range [][]string{dGlobs, cGlobs} {
		if err := validateGlobs(g); err != nil {
			return err
		}
	}
	return nil
}

func validateBaseDir(bDir string) error {
	if filepath.IsAbs(bDir) {
		return fmt.Errorf("base-dir does not support absolute paths")
	}
	bDir = filepath.ToSlash(filepath.Join(".", bDir))
	if strings.HasPrefix(bDir, "..") {
		return fmt.Errorf("base-dir is outside the current working directory")
	}
	fi, err := os.Stat(bDir)
	if err != nil {
		return err
	}
	if mode := fi.Mode(); !mode.IsDir() {
		return fmt.Errorf(
			"base-dir is not a sub directory of the current working directory",
		)
	}
	return nil
}

func validateInputPaths(bDir string, paths []string) error {
	for _, p := range paths {
		p = filepath.ToSlash(p)
		if filepath.IsAbs(p) {
			return fmt.Errorf(
				"%s dockerfiles and compose-files don't support absolute paths",
				p,
			)
		}
		p = filepath.ToSlash(filepath.Join(bDir, p))
		if strings.HasPrefix(p, "..") {
			return fmt.Errorf("%s is outside the current working directory", p)
		}
	}
	return nil
}

func validateGlobs(globs []string) error {
	for _, g := range globs {
		if filepath.IsAbs(g) {
			return fmt.Errorf("%s globs do not support absolute paths", g)
		}
	}
	return nil
}
