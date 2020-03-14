package contrib

import "github.com/michaelperel/docker-lock/registry"

// GetAllWrappers returns all wrappers supported by the community.
func GetAllWrappers(client *registry.HTTPClient) ([]registry.Wrapper, error) {
	return []registry.Wrapper{
		NewElasticWrapper(client),
		NewMCRWrapper(client),
	}, nil
}
