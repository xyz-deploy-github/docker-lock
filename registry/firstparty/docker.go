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
	configPath string
	client     *registry.HTTPClient
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

// init registers DockerWrapper for use by docker-lock.
func init() { //nolint: gochecknoinits
	constructor := func(
		client *registry.HTTPClient,
		configPath string,
	) (registry.Wrapper, error) {
		w, err := NewDockerWrapper(
			client, configPath, os.Getenv("DOCKER_USERNAME"),
			os.Getenv("DOCKER_PASSWORD"),
		)
		if err != nil {
			err = fmt.Errorf("cannot register DockerWrapper: %s", err)
		}

		return w, err
	}

	constructors = append(constructors, constructor)
}

// NewDockerWrapper creates a DockerWrapper or returns an error
// if not possible.
//
// If username and password are defined, then they will be used for
// authentication. Otherwise, the username and password will be obtained
// from docker's config.json. For this to work, please login using
// 'docker login'.
//
// If using the cli, to set the username and password, ensure
// DOCKER_USERNAME and DOCKER_PASSWORD are set. This can be achieved
// automatically via a .env file or manually by exporting the
// environment variables. configPath can be set via cli flags.
func NewDockerWrapper(
	client *registry.HTTPClient,
	configPath string,
	username string,
	password string,
) (*DockerWrapper, error) {
	if client == nil {
		client = &registry.HTTPClient{
			Client:      &http.Client{},
			RegistryURL: "https://registry-1.docker.io/v2",
			TokenURL: fmt.Sprintf(
				"https://auth.docker.io/token%s",
				"?scope=repository:%s:pull&service=registry.docker.io",
			),
		}
	}

	w := &DockerWrapper{configPath: configPath, client: client}

	ac, err := w.authCreds(username, password)
	if err != nil {
		return nil, err
	}

	w.dockerAuthCreds = ac

	return w, nil
}

// Digest queries the container registry for the digest given a repo and ref.
func (w *DockerWrapper) Digest(repo string, ref string) (string, error) {
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
			"%s/%s/manifests/%s", w.client.RegistryURL, repo, ref,
		)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return "", err
		}

		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", t))
		req.Header.Add(
			"Accept", "application/vnd.docker.distribution.manifest.v2+json",
		)
		req.Header.Add(
			"Accept",
			"application/vnd.docker.distribution.manifest.list.v2+json",
		)

		resp, err := w.client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		digest := resp.Header.Get("Docker-Content-Digest")

		if digest != "" {
			return strings.TrimPrefix(digest, "sha256:"), nil
		}
	}

	return "", fmt.Errorf("no digest found for '%s:%s'", repo, ref)
}

// token queries the container registry for a bearer token that is later
// required to query the container registry for a digest.
func (w *DockerWrapper) token(repo string) (string, error) {
	url := fmt.Sprintf(w.client.TokenURL, repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	if w.username != "" && w.password != "" {
		req.SetBasicAuth(w.username, w.password)
	}

	resp, err := w.client.Do(req)
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
// the registry for the digest. If username and password are empty,
// it will look in docker's config.json.
func (w *DockerWrapper) authCreds(
	username string, password string,
) (*dockerAuthCreds, error) {
	if username != "" && password != "" {
		return &dockerAuthCreds{
			username: username,
			password: password,
		}, nil
	}

	if w.configPath == "" {
		return &dockerAuthCreds{}, nil
	}

	confByt, err := ioutil.ReadFile(w.configPath)
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
