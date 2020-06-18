package firstparty

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/michaelperel/docker-lock/registry"
)

// InternalWrapper is a registry wrapper for internal registries.
type InternalWrapper struct {
	client      *registry.HTTPClient
	prefix      string
	stripPrefix bool
}

// internalTokenResponse contains the bearer token required to
// query the container registry for a digest.
type internalTokenResponse struct {
	Token string `json:"token"`
}

// init registers InternalWrapper for use by docker-lock
// if INTERNAL_REGISTRY_URL is set.
func init() { //nolint: gochecknoinits
	constructor := func(
		client *registry.HTTPClient,
		_ string,
	) (registry.Wrapper, error) {
		stripPrefix, err := strconv.ParseBool(
			os.Getenv("INTERNAL_STRIP_PREFIX"),
		)
		if err != nil {
			err = fmt.Errorf("cannot register InternalWrapper: %s", err)
			return nil, err
		}

		w, err := NewInternalWrapper(
			client, os.Getenv("INTERNAL_PREFIX"), stripPrefix,
			os.Getenv("INTERNAL_REGISTRY_URL"), os.Getenv("INTERNAL_TOKEN_URL"),
		)
		if err != nil {
			err = fmt.Errorf("cannot register InternalWrapper: %s", err)
		}

		return w, err
	}

	constructors = append(constructors, constructor)
}

// NewInternalWrapper creates an InternalWrapper. If tokenURL is blank,
// the wrapper will not acquire a token before requesting the digest.
// If stripPrefix is true, the prefix will not be considered part of
// the repo name in API calls. registryURL must be set.
func NewInternalWrapper(
	client *registry.HTTPClient,
	prefix string,
	stripPrefix bool,
	registryURL string,
	tokenURL string,
) (*InternalWrapper, error) {
	if registryURL == "" {
		return nil, fmt.Errorf("internal registry url is empty")
	}

	w := &InternalWrapper{
		prefix:      prefix,
		stripPrefix: stripPrefix,
	}

	if client == nil {
		w.client = &registry.HTTPClient{
			Client:      &http.Client{},
			RegistryURL: fmt.Sprintf("%s/v2", registryURL),
			TokenURL:    tokenURL,
		}
	}

	return w, nil
}

// Digest queries the container registry for the digest given a repo and ref.
func (w *InternalWrapper) Digest(repo string, ref string) (string, error) {
	if w.stripPrefix {
		repo = strings.Replace(repo, w.Prefix(), "", 1)
	}

	url := fmt.Sprintf("%s/%s/manifests/%s", w.client.RegistryURL, repo, ref)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	if w.client.TokenURL != "" {
		var t string

		t, err = w.token(repo)
		if err != nil {
			return "", err
		}

		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", t))
	}

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
// required to query the container registry for a digest. All occurrences
// of the string "<REPO>" in the TokenURL will be replaced by the contents
// repo.
func (w *InternalWrapper) token(repo string) (string, error) {
	url := strings.ReplaceAll(w.client.TokenURL, "<REPO>", repo)

	resp, err := http.Get(url) // nolint: gosec
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	d := json.NewDecoder(resp.Body)

	t := internalTokenResponse{}
	if err = d.Decode(&t); err != nil {
		return "", err
	}

	return t.Token, nil
}

// Prefix returns the registry prefix that identifies the internal
// registry.
func (w *InternalWrapper) Prefix() string {
	return w.prefix
}
