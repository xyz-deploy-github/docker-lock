// Package migrate provides the "migrate" command.
package migrate

import (
	"fmt"
	"os"

	"github.com/safe-waters/docker-lock/pkg/migrate"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const namespace = "migrate"

// NewMigrateCmd creates the command 'migrate' used in 'docker lock migrate'.
func NewMigrateCmd() (*cobra.Command, error) {
	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate images referenced by a Lockfile to another registry",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return bindPFlags(cmd, []string{
				"lockfile-name",
				"prefix",
			})
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			flags, err := parseFlags()
			if err != nil {
				return err
			}

			reader, err := os.Open(flags.LockfileName)
			if err != nil {
				return err
			}
			defer reader.Close()

			migrater := migrate.NewMigrater(flags.Prefix)
			err = migrater.Migrate(reader)
			if err == nil {
				fmt.Println("successfully migrated images from lockfile!")
			}

			return err
		},
	}
	migrateCmd.Flags().String(
		"lockfile-name", "docker-lock.json", "Lockfile to read from",
	)
	migrateCmd.Flags().String(
		"prefix", "", "location for migrated images such as hostname:port/repo",
	)
	migrateCmd.MarkFlagRequired("prefix")

	return migrateCmd, nil
}

func bindPFlags(cmd *cobra.Command, flagNames []string) error {
	for _, name := range flagNames {
		if err := viper.BindPFlag(
			fmt.Sprintf("%s.%s", namespace, name), cmd.Flags().Lookup(name),
		); err != nil {
			return err
		}
	}

	return nil
}

func parseFlags() (*Flags, error) {
	var (
		lockfileName = viper.GetString(
			fmt.Sprintf("%s.%s", namespace, "lockfile-name"),
		)
		prefix = viper.GetString(
			fmt.Sprintf("%s.%s", namespace, "prefix"),
		)
	)

	return NewFlags(lockfileName, prefix)
}
