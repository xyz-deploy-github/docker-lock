package verify

import "path/filepath"

type VerifierFlags struct {
	LockfilePath           string
	ConfigFile             string
	EnvFile                string
	DockerfileEnvBuildArgs bool
}

func NewVerifierFlags(
	lockfilePath, configFile, envFile string,
	dockerfileEnvBuildArgs bool,
) (*VerifierFlags, error) {
	lockfilePath = convertStringToSlash(lockfilePath)
	configFile = convertStringToSlash(configFile)
	envFile = convertStringToSlash(envFile)
	return &VerifierFlags{
		LockfilePath:           lockfilePath,
		ConfigFile:             configFile,
		EnvFile:                envFile,
		DockerfileEnvBuildArgs: dockerfileEnvBuildArgs,
	}, nil
}

func convertStringToSlash(s string) string {
	return filepath.ToSlash(s)
}
