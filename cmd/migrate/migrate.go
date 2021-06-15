// Package migrate provides the "migrate" command.
package migrate

import (
	"errors"
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

			migrater, err := SetupMigrater(flags)
			if err != nil {
				return err
			}

			reader, err := os.Open(flags.LockfileName)
			if err != nil {
				return err
			}
			defer reader.Close()

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

	err := migrateCmd.MarkFlagRequired("prefix")
	if err != nil {
		return nil, err
	}

	return migrateCmd, nil
}

// SetupMigrater creates a Migrater configured for docker-lock's cli.
func SetupMigrater(flags *Flags) (migrate.IMigrater, error) {
	if flags == nil {
		return nil, errors.New("'flags' cannot be nil")
	}

	if _, err := os.Stat(flags.LockfileName); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"lockfile '%s' does not exist", flags.LockfileName,
			)
		}

		return nil, err
	}

	copier := migrate.NewCopier(flags.Prefix)

	migrater, err := migrate.NewMigrater(copier)
	if err != nil {
		return nil, err
	}

	return migrater, nil
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
