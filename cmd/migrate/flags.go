package migrate

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Flags holds all command line options for Dockerfiles, Composefiles,
// and Kubernetesfiles.
type Flags struct {
	LockfileName       string
	DownstreamPrefixes []string
}

// NewFlags returns Flags after validating its fields.
// lockfileName may not contain slashes.
func NewFlags(
	lockfileName string,
	downstreamPrefixes []string,
) (*Flags, error) {
	if err := validateLockfileName(lockfileName); err != nil {
		return nil, err
	}

	if len(downstreamPrefixes) == 0 {
		return nil, errors.New("'downstreamPrefixes' must be greater than 0")
	}

	for i, s := range downstreamPrefixes {
		downstreamPrefixes[i] = strings.TrimSuffix(s, "/")
	}

	return &Flags{
		LockfileName:       lockfileName,
		DownstreamPrefixes: downstreamPrefixes,
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
