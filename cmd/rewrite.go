package cmd

import (
	"github.com/michaelperel/docker-lock/rewrite"
	"github.com/spf13/cobra"
)

// NewRewriteCmd creates the command 'rewrite' used in 'docker lock rewrite'.
func NewRewriteCmd() *cobra.Command {
	rewriteCmd := &cobra.Command{
		Use:   "rewrite",
		Short: "Rewrite files referenced by a Lockfile to use image digests",
		RunE: func(cmd *cobra.Command, args []string) error {
			rewriter, err := rewrite.NewRewriter(cmd)
			if err != nil {
				return err
			}
			if err := rewriter.Rewrite(); err != nil {
				return err
			}
			return nil
		},
	}
	rewriteCmd.Flags().String("outpath", "docker-lock.json", "Path to load Lockfile")
	rewriteCmd.Flags().String("suffix",
		"",
		"Create new Dockerfiles and docker-compose files with a suffix rather than overwrite existing files")
	rewriteCmd.Flags().String("tempdir",
		"",
		"Directory where a temporary directory will be created/deleted during a rewrite transaction")
	return rewriteCmd
}
