package migrate

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Flags holds all command line options for Dockerfiles, Composefiles,
// and Kubernetesfiles.
type Flags struct {
	LockfileName string
	Prefix       string
}

// NewFlags returns Flags after validating its fields.
// lockfileName may not contain slashes.
func NewFlags(
	lockfileName string,
	prefix string,
) (*Flags, error) {
	if err := validateLockfileName(lockfileName); err != nil {
		return nil, err
	}

	prefix = strings.TrimSuffix(prefix, "/")

	return &Flags{
		LockfileName: lockfileName,
		Prefix:       prefix,
	}, nil
}

func validateLockfileName(lockfileName string) error {
	if filepath.IsAbs(lockfileName) {
		return fmt.Errorf(
			"'%s' lockfile-name does not support absolute paths", lockfileName,
		)
	}

	lockfileName = filepath.Join(".", lockfileName)

	if strings.ContainsAny(lockfileName, `/\`) {
		return fmt.Errorf(
			"'%s' lockfile-name cannot contain slashes", lockfileName,
		)
	}

	return nil
}
