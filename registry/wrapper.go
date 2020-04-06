// Package registry provides functions to get digests from registries.
package registry

// Wrapper defines an interface that registry wrappers implement.
type Wrapper interface {
	// GetDigest gets the digest from a name and tag.
	GetDigest(name string, tag string) (string, error)
	// Prefix returns the registry prefix and is used by the wrapper manager
	// to select which registry to use. For instance, the prefix for
	// 'dockerlocktestaccount.azurecr.io/busybox' would be
	// 'dockerlocktestaccount.azurecr.io/'.
	Prefix() string
}
