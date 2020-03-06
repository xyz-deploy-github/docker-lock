package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is overridden in the CD pipeline to match the git tag
var Version = "v0.0.1" //nolint: gochecknoglobals

func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the docker-lock version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(Version)
			return nil
		},
	}
}