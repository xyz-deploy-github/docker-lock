package verify

import "path/filepath"

// Flags are all possible flags to initialize a Verifier.
type Flags struct {
	LockfilePath           string
	ConfigFile             string
	EnvFile                string
	DockerfileEnvBuildArgs bool
	Verbose                bool
}

// NewFlags creates flags for a Verifier.
func NewFlags(
	lPath, configFile, envFile string,
	dfileEnvBuildArgs, verbose bool,
) (*Flags, error) {
	lPath = convertStrToSlash(lPath)
	configFile = convertStrToSlash(configFile)
	envFile = convertStrToSlash(envFile)

	return &Flags{
		LockfilePath:           lPath,
		ConfigFile:             configFile,
		EnvFile:                envFile,
		DockerfileEnvBuildArgs: dfileEnvBuildArgs,
		Verbose:                verbose,
	}, nil
}

// convertStrToSlash converts a filepath string to use forward slashes.
func convertStrToSlash(s string) string {
	return filepath.ToSlash(s)
}
