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
	lPath, configFile, envFile string,
	dfileEnvBuildArgs bool,
) (*Flags, error) {
	lPath = convertStrToSlash(lPath)
	configFile = convertStrToSlash(configFile)
	envFile = convertStrToSlash(envFile)

	return &Flags{
		LockfilePath:           lPath,
		ConfigFile:             configFile,
		EnvFile:                envFile,
		DockerfileEnvBuildArgs: dfileEnvBuildArgs,
	}, nil
}

func convertStrToSlash(s string) string {
	return filepath.ToSlash(s)
}
