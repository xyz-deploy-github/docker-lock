package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Flags are all possible flags to initialize a Generator.
type Flags struct {
	BaseDir                string
	LockfileName           string
	ConfigFile             string
	EnvFile                string
	Dockerfiles            []string
	Composefiles           []string
	DockerfileGlobs        []string
	ComposefileGlobs       []string
	DockerfileRecursive    bool
	ComposefileRecursive   bool
	DockerfileEnvBuildArgs bool
	Verbose                bool
}

// NewFlags creates flags for a Generator.
func NewFlags(
	bDir, lName, configFile, envFile string,
	dfiles, cfiles, dGlobs, cGlobs []string,
	dRecursive, cRecursive, dfileEnvBuildArgs, verbose bool,
) (*Flags, error) {
	bDir = convertStrToSlash(bDir)
	configFile = convertStrToSlash(configFile)
	envFile = convertStrToSlash(envFile)

	dfiles = convertStrSlToSlash(dfiles)
	cfiles = convertStrSlToSlash(cfiles)
	dGlobs = convertStrSlToSlash(dGlobs)
	cGlobs = convertStrSlToSlash(cGlobs)

	if err := validateFlags(
		bDir, lName, dfiles, cfiles, dGlobs, cGlobs,
	); err != nil {
		return nil, err
	}

	return &Flags{
		BaseDir:                bDir,
		LockfileName:           lName,
		ConfigFile:             configFile,
		EnvFile:                envFile,
		Dockerfiles:            dfiles,
		Composefiles:           cfiles,
		DockerfileGlobs:        dGlobs,
		ComposefileGlobs:       cGlobs,
		DockerfileRecursive:    dRecursive,
		ComposefileRecursive:   cRecursive,
		DockerfileEnvBuildArgs: dfileEnvBuildArgs,
		Verbose:                verbose,
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

// validateFlags validates legal values for command line flags.
func validateFlags(
	bDir, lName string,
	dfiles, cfiles, dGlobs, cGlobs []string,
) error {
	if err := validateBaseDirectory(bDir); err != nil {
		return err
	}

	if err := validateLockfileName(lName); err != nil {
		return err
	}

	for _, ps := range [][]string{dfiles, cfiles} {
		if err := validateSuppliedPaths(bDir, ps); err != nil {
			return err
		}
	}

	for _, gs := range [][]string{dGlobs, cGlobs} {
		if err := validateGlobs(gs); err != nil {
			return err
		}
	}

	return nil
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
