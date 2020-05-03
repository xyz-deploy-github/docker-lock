package contrib

import "github.com/michaelperel/docker-lock/registry"

// AllWrappers returns all wrappers supported by the community.
func AllWrappers(client *registry.HTTPClient) ([]registry.Wrapper, error) {
	return []registry.Wrapper{
		NewElasticWrapper(client),
		NewMCRWrapper(client),
	}, nil
}
