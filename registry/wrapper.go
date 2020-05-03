// Package registry provides functions to get digests from registries.
package registry

// Wrapper defines an interface that registry wrappers implement.
type Wrapper interface {
	// Digest returns the digest from a name and tag.
	Digest(name string, tag string) (string, error)

	// Prefix returns the registry prefix and is used by the wrapper manager
	// to select which registry to use. For instance, the prefix for
	// 'dockerlocktestaccount.azurecr.io/busybox' would be
	// 'dockerlocktestaccount.azurecr.io/'.
	Prefix() string
}
