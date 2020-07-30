// Package cmd provides docker-lock's cli.
package cmd

import (
	"fmt"

	"github.com/michaelperel/docker-lock/registry"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewRootCmd creates the root command for docker-lock.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "docker",
		Short: "Root command for docker lock",
	}

	return rootCmd
}

// Execute creates all of docker-lock's commands, adds appropriate commands
// to each other, and executes the root command.
func Execute(client *registry.HTTPClient) error {
	if err := initViper(); err != nil {
		return err
	}

	rootCmd := NewRootCmd()

	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	lockCmd := NewLockCmd()
	versionCmd := NewVersionCmd()

	generateCmd, err := NewGenerateCmd(client)
	if err != nil {
		return err
	}

	verifyCmd, err := NewVerifyCmd(client)
	if err != nil {
		return err
	}

	rewriteCmd, err := NewRewriteCmd()
	if err != nil {
		return err
	}

	rootCmd.AddCommand(lockCmd)
	lockCmd.AddCommand(
		[]*cobra.Command{versionCmd, generateCmd, verifyCmd, rewriteCmd}...,
	)

	if err := rootCmd.Execute(); err != nil {
		return err
	}

	return nil
}

// initViper reads configuration values for docker-lock from a config
// file, if it exists. Otherwise, docker-lock will fall back to command line
// flags.
func initViper() error {
	const cfgFilePrefix = ".docker-lock"

	// works with variety of files such as .docker-lock.[yaml|json|toml] etc.
	viper.SetConfigName(cfgFilePrefix)
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("malformed %s file: %v", cfgFilePrefix, err)
		}
	}

	return nil
}
