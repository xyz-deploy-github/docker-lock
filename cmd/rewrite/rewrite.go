// Package rewrite provides the "rewrite" command.
package rewrite

import (
	"io/ioutil"
	"log"

	"github.com/safe-waters/docker-lock/rewrite"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewRewriteCmd creates the command 'rewrite' used in 'docker lock rewrite'.
func NewRewriteCmd() (*cobra.Command, error) {
	rewriteCmd := &cobra.Command{
		Use:   "rewrite",
		Short: "Rewrite files referenced by a Lockfile to use image digests",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags, err := rewriterFlags(cmd)
			if err != nil {
				return err
			}

			configureLogger(flags.Verbose)

			log.Printf("Found flags '%+v'.", flags)

			rewriter, err := rewrite.NewRewriter(flags)
			if err != nil {
				return err
			}

			if err := rewriter.Rewrite(); err != nil {
				return err
			}

			return nil
		},
	}
	rewriteCmd.Flags().StringP(
		"lockfile-path", "l", "docker-lock.json", "Path to Lockfile",
	)
	rewriteCmd.Flags().StringP(
		"suffix", "s", "",
		"Create new Dockerfiles and docker-compose files "+
			"with a suffix rather than overwrite existing files",
	)
	rewriteCmd.Flags().StringP(
		"tempdir", "t", "",
		"Directory where a temporary directory will be created/deleted "+
			"during a rewrite transaction",
	)
	rewriteCmd.Flags().BoolP(
		"exclude-tags", "e", false, "Exclude image tags from rewritten files",
	)
	rewriteCmd.Flags().BoolP(
		"verbose", "v", false, "Show logs",
	)

	if err := viper.BindPFlags(rewriteCmd.Flags()); err != nil {
		return nil, err
	}

	return rewriteCmd, nil
}

// rewriterFlags gets values from the command and uses them to
// create Flags.
func rewriterFlags(cmd *cobra.Command) (*rewrite.Flags, error) { //nolint: dupl
	var (
		lPath, suffix, tmpDir string
		excludeTags, verbose  bool
		err                   error
	)

	switch viper.ConfigFileUsed() {
	case "":
		lPath, err = cmd.Flags().GetString("lockfile-path")
		if err != nil {
			return nil, err
		}

		suffix, err = cmd.Flags().GetString("suffix")
		if err != nil {
			return nil, err
		}

		tmpDir, err = cmd.Flags().GetString("tempdir")
		if err != nil {
			return nil, err
		}

		verbose, err = cmd.Flags().GetBool("verbose")
		if err != nil {
			return nil, err
		}

		excludeTags, err = cmd.Flags().GetBool("exclude-tags")
		if err != nil {
			return nil, err
		}
	default:
		lPath = viper.GetString("lockfile-path")
		suffix = viper.GetString("suffix")
		tmpDir = viper.GetString("tempdir")
		excludeTags = viper.GetBool("exclude-tags")
		verbose = viper.GetBool("verbose")
	}

	return rewrite.NewFlags(lPath, suffix, tmpDir, excludeTags, verbose)
}

// configureLogger configures a common logger for all subcommands. If
// verbose is not set, all logs are discarded.
func configureLogger(verbose bool) {
	if !verbose {
		log.SetOutput(ioutil.Discard)
		return
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	log.SetPrefix("[DEBUG] ")
}
