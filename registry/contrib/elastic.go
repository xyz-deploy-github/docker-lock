package contrib

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/michaelperel/docker-lock/registry"
)

// ElasticWrapper is a registry wrapper for the Elasticsearch registry.
type ElasticWrapper struct {
	client *registry.HTTPClient
}

// elasticTokenResponse contains the bearer token required to
// query the container registry for a digest.
type elasticTokenResponse struct {
	Token string `json:"token"`
}

// NewElasticWrapper creates an ElasticWrapper.
func NewElasticWrapper(client *registry.HTTPClient) *ElasticWrapper {
	w := &ElasticWrapper{}

	if client == nil {
		w.client = &registry.HTTPClient{
			Client:      &http.Client{},
			RegistryURL: fmt.Sprintf("https://%sv2", w.Prefix()),
			TokenURL: fmt.Sprintf(
				"https://docker-auth.elastic.co/auth%s",
				"?scope=repository:%s:pull&service=token-service",
			),
		}
	}

	return w
}

// init registers ElasticWrapper for use by docker-lock.
func init() { //nolint: gochecknoinits
	constructor := func(
		client *registry.HTTPClient,
		_ string,
	) (registry.Wrapper, error) {
		return NewElasticWrapper(client), nil
	}

	constructors = append(constructors, constructor)
}

// Digest queries the container registry for the digest given a repo and ref.
func (w *ElasticWrapper) Digest(repo string, ref string) (string, error) {
	repo = strings.Replace(repo, w.Prefix(), "", 1)

	t, err := w.token(repo)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/%s/manifests/%s", w.client.RegistryURL, repo, ref)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", t))
	req.Header.Add(
		"Accept", "application/vnd.docker.distribution.manifest.v2+json",
	)
	req.Header.Add(
		"Accept", "application/vnd.docker.distribution.manifest.list.v2+json",
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

// token queries the container registry for a bearer token that is later
// required to query the container registry for a digest.
func (w *ElasticWrapper) token(repo string) (string, error) {
	// example repo -> "elasticsearch/elasticsearch-oss"
	url := fmt.Sprintf(w.client.TokenURL, repo)

	resp, err := http.Get(url) // nolint: gosec
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	d := json.NewDecoder(resp.Body)

	t := elasticTokenResponse{}
	if err = d.Decode(&t); err != nil {
		return "", err
	}

	return t.Token, nil
}

// Prefix returns the registry prefix that identifies the Elasticsearch
// registry.
func (w *ElasticWrapper) Prefix() string {
	return "docker.elastic.co/"
}
