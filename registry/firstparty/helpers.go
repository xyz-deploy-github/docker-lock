package firstparty

import "github.com/michaelperel/docker-lock/registry"

// GetDefaultWrapper returns a wrapper for images without a prefix.
func GetDefaultWrapper(
	configPath string,
	client *registry.HTTPClient,
) (registry.Wrapper, error) {
	return NewDockerWrapper(configPath, client)
}

// GetAllWrappers returns all wrappers officially supported
// by the maintainers of docker-lock.
func GetAllWrappers(
	configPath string,
	client *registry.HTTPClient,
) ([]registry.Wrapper, error) {
	dw, err := NewDockerWrapper(configPath, client)
	if err != nil {
		return nil, err
	}
	aw, err := NewACRWrapper(configPath, client)
	if err != nil {
		return nil, err
	}
	return []registry.Wrapper{dw, aw}, nil
}
