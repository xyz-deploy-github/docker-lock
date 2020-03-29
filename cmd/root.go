package cmd

import (
	"github.com/michaelperel/docker-lock/registry"
	"github.com/spf13/cobra"
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
	rootCmd := NewRootCmd()

	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	lockCmd := NewLockCmd()
	versionCmd := NewVersionCmd()
	generateCmd := NewGenerateCmd(client)
	verifyCmd := NewVerifyCmd(client)
	rewriteCmd := NewRewriteCmd()

	rootCmd.AddCommand(lockCmd)
	lockCmd.AddCommand(
		[]*cobra.Command{versionCmd, generateCmd, verifyCmd, rewriteCmd}...,
	)

	if err := rootCmd.Execute(); err != nil {
		return err
	}

	return nil
}
