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
	username := os.Getenv("DOCKER_USERNAME")
	password := os.Getenv("DOCKER_PASSWORD")

	return NewDockerWrapper(client, configPath, username, password)
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
	username := os.Getenv("DOCKER_USERNAME")
	password := os.Getenv("DOCKER_PASSWORD")

	dw, err := NewDockerWrapper(client, configPath, username, password)
	if err != nil {
		return nil, err
	}

	ws := []registry.Wrapper{dw}

	if registryName := os.Getenv("ACR_REGISTRY_NAME"); registryName != "" {
		username = os.Getenv("ACR_USERNAME")
		password = os.Getenv("ACR_PASSWORD")

		aw, err := NewACRWrapper(
			client, configPath, username, password, registryName,
		)

		if err != nil {
			return nil, err
		}

		ws = append(ws, aw)
	}

	return ws, nil
}
