package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GeneratorFlags are all possible flags to initialize a Generator.
type GeneratorFlags struct {
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

// NewGeneratorFlags create a Generator.
func NewGeneratorFlags(
	baseDir, lockfileName, configFile, envFile string,
	dockerfiles, composefiles, dockerfileGlobs, composefileGlobs []string,
	dockerfileRecursive, composefileRecursive, dockerfileEnvBuildArgs bool,
) (*GeneratorFlags, error) {
	baseDir = convertStringToSlash(baseDir)
	configFile = convertStringToSlash(configFile)
	envFile = convertStringToSlash(envFile)
	convertStringSliceToSlash(dockerfiles)
	convertStringSliceToSlash(composefiles)
	convertStringSliceToSlash(dockerfileGlobs)
	convertStringSliceToSlash(composefileGlobs)
	if err := validateFlags(
		baseDir, lockfileName,
		dockerfiles, composefiles, dockerfileGlobs, composefileGlobs,
	); err != nil {
		return nil, err
	}
	return &GeneratorFlags{
		BaseDir:                baseDir,
		LockfileName:           lockfileName,
		ConfigFile:             configFile,
		EnvFile:                envFile,
		Dockerfiles:            dockerfiles,
		Composefiles:           composefiles,
		DockerfileGlobs:        dockerfileGlobs,
		ComposefileGlobs:       composefileGlobs,
		DockerfileRecursive:    dockerfileRecursive,
		ComposefileRecursive:   composefileRecursive,
		DockerfileEnvBuildArgs: dockerfileEnvBuildArgs,
	}, nil
}

func convertStringToSlash(s string) string {
	return filepath.ToSlash(s)
}

func convertStringSliceToSlash(s []string) {
	for i := range s {
		s[i] = filepath.ToSlash(s[i])
	}
}

func validateFlags(
	baseDir, lockfileName string,
	dockerfiles, composefiles, dockerfileGlobs, composefileGlobs []string,
) error {
	if err := validateBaseDir(baseDir); err != nil {
		return err
	}
	if err := validateLockfileName(lockfileName); err != nil {
		return err
	}
	for _, ps := range [][]string{dockerfiles, composefiles} {
		if err := validateInputPaths(baseDir, ps); err != nil {
			return err
		}
	}
	for _, gs := range [][]string{dockerfileGlobs, composefileGlobs} {
		if err := validateGlobs(gs); err != nil {
			return err
		}
	}
	return nil
}

func validateBaseDir(baseDir string) error {
	if filepath.IsAbs(baseDir) {
		return fmt.Errorf(
			"'%s' base-dir does not support absolute paths", baseDir,
		)
	}
	if strings.HasPrefix(filepath.Join(".", baseDir), "..") {
		return fmt.Errorf(
			"'%s' base-dir is outside the current working directory", baseDir,
		)
	}
	fi, err := os.Stat(baseDir)
	if err != nil {
		return err
	}
	if mode := fi.Mode(); !mode.IsDir() {
		return fmt.Errorf(
			"'%s' base-dir is not sub directory "+
				"of the current working directory",
			baseDir,
		)
	}
	return nil
}

func validateLockfileName(lName string) error {
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
