package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SharedFlags are flags that do not differ between Dockerfiles
// and docker-compose files.
type SharedFlags struct {
	BaseDir      string
	LockfileName string
	ConfigFile   string
	EnvFile      string
	Verbose      bool
}

// SpecificFlags are flags whose values differ between Dockerfiles and
// docker-compose files.
type SpecificFlags struct {
	Paths     []string
	Globs     []string
	Recursive bool
}

// DockerfileFlags are flags that determine how Dockerfiles are processed.
type DockerfileFlags struct {
	*SpecificFlags
	UseEnvAsBuildArgs bool
}

// ComposefileFlags are flags that determine how docker-compose files
// are processed.
type ComposefileFlags struct {
	*SpecificFlags
}

// Flags consist of SharedFlags, DockerfileFlags, and ComposefileFlags.
type Flags struct {
	*SharedFlags
	DockerfileFlags  *DockerfileFlags
	ComposefileFlags *ComposefileFlags
}

// NewSharedFlags preprocesses and validates the fields of SharedFlags.
func NewSharedFlags(
	bDir string,
	lName string,
	configFile string,
	envFile string,
	verbose bool,
) (*SharedFlags, error) {
	bDir = convertStrToSlash(bDir)
	configFile = convertStrToSlash(configFile)
	envFile = convertStrToSlash(envFile)

	if err := validateBaseDirectory(bDir); err != nil {
		return nil, err
	}

	if err := validateLockfileName(lName); err != nil {
		return nil, err
	}

	return &SharedFlags{
		BaseDir:      bDir,
		LockfileName: lName,
		ConfigFile:   configFile,
		EnvFile:      envFile,
		Verbose:      verbose,
	}, nil
}

// NewSpecificFlags preprocesses and validates the fields of SpecificFlags.
func NewSpecificFlags(
	bDir string,
	paths []string,
	globs []string,
	recursive bool,
) (*SpecificFlags, error) {
	paths = convertStrSlToSlash(paths)
	globs = convertStrSlToSlash(globs)

	if err := validateSuppliedPaths(bDir, paths); err != nil {
		return nil, err
	}

	if err := validateGlobs(globs); err != nil {
		return nil, err
	}

	return &SpecificFlags{
		Paths:     paths,
		Globs:     globs,
		Recursive: recursive,
	}, nil
}

// NewFlags preprocesses and validates flags for Dockerfiles and
// docker-compose files.
func NewFlags(
	bDir, lName, configFile, envFile string,
	dfiles, cfiles, dGlobs, cGlobs []string,
	dRecursive, cRecursive, dfileEnvBuildArgs, verbose bool,
) (*Flags, error) {
	sharedFlags, err := NewSharedFlags(
		bDir, lName, configFile, envFile, verbose,
	)
	if err != nil {
		return nil, err
	}

	dFlags, err := NewSpecificFlags(bDir, dfiles, dGlobs, dRecursive)
	if err != nil {
		return nil, err
	}

	cFlags, err := NewSpecificFlags(bDir, cfiles, cGlobs, cRecursive)
	if err != nil {
		return nil, err
	}

	return &Flags{
		SharedFlags:      sharedFlags,
		DockerfileFlags:  &DockerfileFlags{dFlags, dfileEnvBuildArgs},
		ComposefileFlags: &ComposefileFlags{cFlags},
	}, nil
}

// convertStrToSlash converts a filepath string to use forward slashes.
func convertStrToSlash(s string) string {
	return filepath.ToSlash(s)
}

// convertStrSlToSlash converts a slice of filepath strings to use forward
// slashes.
func convertStrSlToSlash(s []string) []string {
	sl := make([]string, len(s))

	copy(sl, s)

	for i := range sl {
		sl[i] = filepath.ToSlash(sl[i])
	}

	return sl
}

// validateBaseDirectory ensures that the base directory is not an
// absolute path and that it is a directory inside the current working
// directory.
func validateBaseDirectory(bDir string) error {
	if filepath.IsAbs(bDir) {
		return fmt.Errorf(
			"'%s' base-dir does not support absolute paths", bDir,
		)
	}

	if strings.HasPrefix(filepath.Join(".", bDir), "..") {
		return fmt.Errorf(
			"'%s' base-dir is outside the current working directory", bDir,
		)
	}

	fi, err := os.Stat(bDir)
	if err != nil {
		return err
	}

	if mode := fi.Mode(); !mode.IsDir() {
		return fmt.Errorf(
			"'%s' base-dir is not sub directory "+
				"of the current working directory",
			bDir,
		)
	}

	return nil
}

// validateLockfileName ensures that the lockfile name does not
// contain slashes.
func validateLockfileName(lName string) error {
	if strings.Contains(lName, "/") {
		return fmt.Errorf(
			"'%s' lockfile-name cannot contain slashes", lName,
		)
	}

	return nil
}

// validateSuppliedPaths ensures that supplied paths are not absolute paths
// and that they are inside the current working directory.
func validateSuppliedPaths(bDir string, suppliedPaths []string) error {
	for _, p := range suppliedPaths {
		if filepath.IsAbs(p) {
			return fmt.Errorf(
				"'%s' input paths do not support absolute paths", p,
			)
		}

		p = filepath.Join(bDir, p)

		if strings.HasPrefix(p, "..") {
			return fmt.Errorf(
				"'%s' is outside the current working directory", p,
			)
		}
	}

	return nil
}

// validateGlobs ensures that globs are not absolute paths.
func validateGlobs(globs []string) error {
	for _, g := range globs {
		if filepath.IsAbs(g) {
			return fmt.Errorf("'%s' globs do not support absolute paths", g)
		}
	}

	return nil
}
