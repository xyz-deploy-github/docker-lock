package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ElasticWrapper is a registry wrapper for the Elasticsearch repository.
type ElasticWrapper struct {
	Client *HTTPClient
}

type elasticTokenResponse struct {
	Token string `json:"token"`
}

// NewElasticWrapper creates an ElasticWrapper.
func NewElasticWrapper(client *HTTPClient) *ElasticWrapper {
	w := &ElasticWrapper{}
	if client == nil {
		w.Client = &HTTPClient{
			Client:        &http.Client{},
			BaseDigestURL: fmt.Sprintf("https://%sv2", w.Prefix()),
			BaseTokenURL:  "https://docker-auth.elastic.co/auth",
		}
	}
	return w
}

// GetDigest gets the digest from a name and tag.
func (w *ElasticWrapper) GetDigest(name string, tag string) (string, error) {
	name = strings.Replace(name, w.Prefix(), "", 1)
	token, err := w.getToken(name)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s/%s/manifests/%s", w.Client.BaseDigestURL, name, tag)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
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
		return "", errors.New("no digest found")
	}
	return strings.TrimPrefix(digest, "sha256:"), nil
}

func (w *ElasticWrapper) getToken(name string) (string, error) {
	// example name -> "elasticsearch/elasticsearch-oss"
	url := fmt.Sprintf(
		"%s?scope=repository:%s:pull&service=token-service",
		w.Client.BaseTokenURL, name,
	)
	resp, err := http.Get(url) // nolint: gosec
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	var t elasticTokenResponse
	if err = decoder.Decode(&t); err != nil {
		return "", err
	}
	return t.Token, nil
}

// Prefix returns the registry prefix that identifies
// the Elasticsearch registry.
func (w *ElasticWrapper) Prefix() string {
	return "docker.elastic.co/"
}
