package verify

import "path/filepath"

// Flags are all possible flags to initialize a Verifier.
type Flags struct {
	LockfilePath           string
	ConfigFile             string
	EnvFile                string
	DockerfileEnvBuildArgs bool
}

// NewFlags creates flags for a Verifier.
func NewFlags(
	lockfilePath, configFile, envFile string,
	dockerfileEnvBuildArgs bool,
) (*Flags, error) {
	lockfilePath = convertStringToSlash(lockfilePath)
	configFile = convertStringToSlash(configFile)
	envFile = convertStringToSlash(envFile)
	return &Flags{
		LockfilePath:           lockfilePath,
		ConfigFile:             configFile,
		EnvFile:                envFile,
		DockerfileEnvBuildArgs: dockerfileEnvBuildArgs,
	}, nil
}

func convertStringToSlash(s string) string {
	return filepath.ToSlash(s)
}
