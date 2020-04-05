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
}

// NewFlags creates flags for a Generator.
func NewFlags(
	bDir, lName, configFile, envFile string,
	dfiles, cfiles, dGlobs, cGlobs []string,
	dRecursive, cRecursive, dfileEnvBuildArgs bool,
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
	}, nil
}

func convertStrToSlash(s string) string {
	return filepath.ToSlash(s)
}

func convertStrSlToSlash(s []string) []string {
	sl := make([]string, len(s))

	copy(sl, s)

	for i := range sl {
		sl[i] = filepath.ToSlash(sl[i])
	}

	return sl
}

func validateFlags(
	bDir, lName string,
	dfiles, cfiles, dGlobs, cGlobs []string,
) error {
	if err := validateBDir(bDir); err != nil {
		return err
	}

	if err := validateLName(lName); err != nil {
		return err
	}

	for _, ps := range [][]string{dfiles, cfiles} {
		if err := validateInputPaths(bDir, ps); err != nil {
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

func validateBDir(bDir string) error {
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

func validateLName(lName string) error {
	if strings.Contains(lName, "/") {
		return fmt.Errorf(
			"'%s' lockfile-name cannot contain slashes", lName,
		)
	}

	return nil
}

func validateInputPaths(bDir string, inputPaths []string) error {
	for _, p := range inputPaths {
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

func validateGlobs(globs []string) error {
	for _, g := range globs {
		if filepath.IsAbs(g) {
			return fmt.Errorf("'%s' globs do not support absolute paths", g)
		}
	}

	return nil
}
