// Package contrib provides functions for getting digests from
// registries supported by docker-lock's community.
package contrib

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/michaelperel/docker-lock/registry"
)

// MCRWrapper is a registry wrapper for Microsoft Container Registry.
type MCRWrapper struct {
	client *registry.HTTPClient
}

// NewMCRWrapper creates an MCRWrapper.
func NewMCRWrapper(client *registry.HTTPClient) *MCRWrapper {
	w := &MCRWrapper{}

	if client == nil {
		w.client = &registry.HTTPClient{
			Client:      &http.Client{},
			RegistryURL: fmt.Sprintf("https://%sv2", w.Prefix()),
		}
	}

	return w
}

// init registers MCRWrapper for use by docker-lock.
func init() { //nolint: gochecknoinits
	constructor := func(
		client *registry.HTTPClient,
		_ string,
	) (registry.Wrapper, error) {
		return NewMCRWrapper(client), nil
	}

	constructors = append(constructors, constructor)
}

// Digest queries the container registry for the digest given a repo and ref.
func (w *MCRWrapper) Digest(repo string, ref string) (string, error) {
	repo = strings.Replace(repo, w.Prefix(), "", 1)

	url := fmt.Sprintf("%s/%s/manifests/%s", w.client.RegistryURL, repo, ref)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add(
		"Accept", "application/vnd.docker.distribution.manifest.v2+json",
	)

	resp, err := w.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	digest := resp.Header.Get("Docker-Content-Digest")

	if digest == "" {
		return "", fmt.Errorf("no digest found for '%s:%s'", repo, ref)
	}

	return strings.TrimPrefix(digest, "sha256:"), nil
}

// Prefix returns the registry prefix that identifies MCR.
func (w *MCRWrapper) Prefix() string {
	return "mcr.microsoft.com/"
}
