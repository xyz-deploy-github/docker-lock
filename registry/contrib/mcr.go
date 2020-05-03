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
	Client *registry.HTTPClient
}

// NewMCRWrapper creates an MCRWrapper.
func NewMCRWrapper(client *registry.HTTPClient) *MCRWrapper {
	w := &MCRWrapper{}

	if client == nil {
		w.Client = &registry.HTTPClient{
			Client:        &http.Client{},
			BaseDigestURL: fmt.Sprintf("https://%sv2", w.Prefix()),
		}
	}

	return w
}

// Digest queries the container registry for the digest given a name and tag.
func (w *MCRWrapper) Digest(name string, tag string) (string, error) {
	name = strings.Replace(name, w.Prefix(), "", 1)

	url := fmt.Sprintf("%s/%s/manifests/%s", w.Client.BaseDigestURL, name, tag)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add(
		"Accept", "application/vnd.docker.distribution.manifest.v2+json",
	)

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	digest := resp.Header.Get("Docker-Content-Digest")

	if digest == "" {
		return "", fmt.Errorf("no digest found for '%s:%s'", name, tag)
	}

	return strings.TrimPrefix(digest, "sha256:"), nil
}

// Prefix returns the registry prefix that identifies MCR.
func (w *MCRWrapper) Prefix() string {
	return "mcr.microsoft.com/"
}
