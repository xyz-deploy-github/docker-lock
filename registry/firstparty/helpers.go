package firstparty

import (
	"os"

	"github.com/michaelperel/docker-lock/registry"
)

// DefaultWrapper returns a wrapper for images without a prefix.
func DefaultWrapper(
	configPath string,
	client *registry.HTTPClient,
) (registry.Wrapper, error) {
	return NewDockerWrapper(configPath, client)
}

// AllWrappers returns all wrappers officially supported
// by the maintainers of docker-lock, that match the caller's environment.
// By default, a *DockerWrapper will always be returned. Other wrappers
// will be returned if their appropriate environment variables are set.
// Currently, this means that an *ACRWrapper will only be returned if
// the environment variable, ACR_REGISTRY_NAME, is set.
func AllWrappers(
	configPath string,
	client *registry.HTTPClient,
) ([]registry.Wrapper, error) {
	dw, err := NewDockerWrapper(configPath, client)
	if err != nil {
		return nil, err
	}

	ws := []registry.Wrapper{dw}

	if os.Getenv("ACR_REGISTRY_NAME") != "" {
		aw, err := NewACRWrapper(configPath, client)
		if err != nil {
			return nil, err
		}

		ws = append(ws, aw)
	}

	return ws, nil
}
