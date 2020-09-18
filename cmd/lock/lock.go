// Package lock provides the "lock" command.
package lock

import (
	"github.com/spf13/cobra"
)

// NewLockCmd creates the command 'lock' used in 'docker lock'.
func NewLockCmd() *cobra.Command {
	lockCmd := &cobra.Command{
		Use:   "lock",
		Short: "Manage image digests with Lockfiles",
	}

	return lockCmd
}
