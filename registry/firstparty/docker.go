// Package firstparty provides functions for getting digests from
// registries supported by docker-lock's maintainers.
package firstparty

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	c "github.com/docker/docker-credential-helpers/client"
	"github.com/michaelperel/docker-lock/registry"
)

// DockerWrapper is a registry wrapper for Docker Hub. It supports public
// and private repositories.
type DockerWrapper struct {
	ConfigPath string
	Client     *registry.HTTPClient
	*dockerAuthCreds
}

// dockerTokenResponse contains the bearer token required to
// query the container registry for a digest.
type dockerTokenResponse struct {
	Token string `json:"token"`
}

// dockerConfig represents the section in docker's config.json for Docker Hub.
type dockerConfig struct {
	Auths struct {
		Index struct {
			Auth string `json:"auth"`
		} `json:"https://index.docker.io/v1/"`
	} `json:"auths"`
	CredsStore string `json:"credsStore"`
}

// dockerAuthCreds contains the username and password required to
// query the container registry for a digest.
type dockerAuthCreds struct {
	username string
	password string
}

// NewDockerWrapper creates a DockerWrapper from docker's config.json.
func NewDockerWrapper(
	configPath string,
	client *registry.HTTPClient,
) (*DockerWrapper, error) {
	if client == nil {
		client = &registry.HTTPClient{
			Client:        &http.Client{},
			BaseDigestURL: "https://registry-1.docker.io/v2",
			BaseTokenURL:  "https://auth.docker.io/token",
		}
	}

	w := &DockerWrapper{ConfigPath: configPath, Client: client}

	ac, err := w.authCreds()
	if err != nil {
		return nil, err
	}

	w.dockerAuthCreds = ac

	return w, nil
}

// Digest queries the container registry for the digest given a repo and tag.
// The workflow for authenticating with private repositories:
//
// (1) if "DOCKER_USERNAME" and "DOCKER_PASSWORD" are set, use them.
//
// (2) Otherwise, try to get credentials from docker's config file. This method
// requires the user to have logged in with the 'docker login' command
// beforehand.
func (w *DockerWrapper) Digest(repo string, tag string) (string, error) {
	// Docker-Content-Digest is the root of the hash chain
	// https://github.com/docker/distribution/issues/1662
	var repos []string

	if strings.Contains(repo, "/") {
		repos = []string{repo, "library/" + repo}
	} else {
		repos = []string{"library/" + repo, repo}
	}

	for _, repo := range repos {
		t, err := w.token(repo)
		if err != nil {
			return "", err
		}

		url := fmt.Sprintf(
			"%s/%s/manifests/%s", w.Client.BaseDigestURL, repo, tag,
		)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return "", err
		}

		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", t))
		req.Header.Add(
			"Accept", "application/vnd.docker.distribution.manifest.v2+json",
		)

		resp, err := w.Client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		digest := resp.Header.Get("Docker-Content-Digest")

		if digest != "" {
			return strings.TrimPrefix(digest, "sha256:"), nil
		}
	}

	return "", fmt.Errorf("no digest found for '%s:%s'", repo, tag)
}

// token queries the container registry for a bearer token that is later
// required to query the container registry for a digest.
func (w *DockerWrapper) token(repo string) (string, error) {
	url := fmt.Sprintf(
		"%s?scope=repository:%s:pull&service=registry.docker.io",
		w.Client.BaseTokenURL,
		repo,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	if w.username != "" && w.password != "" {
		req.SetBasicAuth(w.username, w.password)
	}

	resp, err := w.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	d := json.NewDecoder(resp.Body)

	t := dockerTokenResponse{}
	if err = d.Decode(&t); err != nil {
		return "", err
	}

	return t.Token, nil
}

// authCreds returns the username and password required to query
// the registry for the digest. It first looks for the environment
// variables "DOCKER_USERNAME" and "DOCKER_PASSWORD". If those do not exist,
// it tries to read them from docker's config.json.
func (w *DockerWrapper) authCreds() (*dockerAuthCreds, error) {
	username := os.Getenv("DOCKER_USERNAME")
	password := os.Getenv("DOCKER_PASSWORD")

	if username != "" && password != "" {
		return &dockerAuthCreds{
			username: username,
			password: password,
		}, nil
	}

	if w.ConfigPath == "" {
		return &dockerAuthCreds{}, nil
	}

	confByt, err := ioutil.ReadFile(w.ConfigPath)
	if err != nil {
		return nil, err
	}

	conf := dockerConfig{}
	if err = json.Unmarshal(confByt, &conf); err != nil {
		return nil, err
	}

	authByt, err := base64.StdEncoding.DecodeString(conf.Auths.Index.Auth)
	if err != nil {
		return nil, err
	}

	authStr := string(authByt)

	switch {
	case authStr != "":
		auth := strings.Split(authStr, ":")
		return &dockerAuthCreds{username: auth[0], password: auth[1]}, nil
	case conf.CredsStore != "":
		authCreds, err := w.authCredsFromStore(conf.CredsStore)
		if err != nil {
			return &dockerAuthCreds{}, nil
		}

		return authCreds, nil
	}

	return &dockerAuthCreds{}, nil
}

// authCredsFromStore reads auth creds from a creds store such as
// wincred, pass, or osxkeychain by shelling out to docker credential helper.
func (w *DockerWrapper) authCredsFromStore(
	credsStore string,
) (authCreds *dockerAuthCreds, err error) {
	defer func() {
		if err := recover(); err != nil {
			authCreds = &dockerAuthCreds{}
			return
		}
	}()

	credsStore = fmt.Sprintf("%s-%s", "docker-credential", credsStore)
	p := c.NewShellProgramFunc(credsStore)

	credRes, err := c.Get(p, "https://index.docker.io/v1/")
	if err != nil {
		return authCreds, err
	}

	return &dockerAuthCreds{
		username: credRes.Username,
		password: credRes.Secret,
	}, nil
}

// Prefix returns an empty string since images on Docker Hub do not use a
// prefix, unlike third party registries.
func (w *DockerWrapper) Prefix() string {
	return ""
}
