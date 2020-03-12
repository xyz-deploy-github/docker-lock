package registry

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	c "github.com/docker/docker-credential-helpers/client"
)

// DockerWrapper is a registry wrapper for Docker Hub. It supports public
// and private repositories.
type DockerWrapper struct {
	ConfigFile string
	Client     *HTTPClient
	authCreds  *dockerAuthCredentials
}

type dockerTokenResponse struct {
	Token string `json:"token"`
}

type dockerConfig struct {
	Auths struct {
		Index struct {
			Auth string `json:"auth"`
		} `json:"https://index.docker.io/v1/"`
	} `json:"auths"`
	CredsStore string `json:"credsStore"`
}

type dockerAuthCredentials struct {
	username string
	password string
}

// NewDockerWrapper creates a DockerWrapper from docker's config.json.
func NewDockerWrapper(
	configPath string,
	client *HTTPClient,
) (*DockerWrapper, error) {
	if client == nil {
		client = &HTTPClient{
			Client:        &http.Client{},
			BaseDigestURL: "https://registry-1.docker.io/v2",
			BaseTokenURL:  "https://auth.docker.io/token",
		}
	}
	w := &DockerWrapper{ConfigFile: configPath, Client: client}
	authCreds, err := w.getAuthCredentials()
	if err != nil {
		return nil, err
	}
	w.authCreds = authCreds
	return w, nil
}

// GetDigest gets the digest from a name and tag. The workflow for
// authenticating with private repositories:
// (1) if "DOCKER_USERNAME" and "DOCKER_PASSWORD" are set, use them.
// (2) Otherwise, try to get credentials from docker's config file. This method
// requires the user to have logged in with the 'docker login' command
// beforehand.
func (w *DockerWrapper) GetDigest(name string, tag string) (string, error) {
	// Docker-Content-Digest is the root of the hash chain
	// https://github.com/docker/distribution/issues/1662
	var names []string
	if strings.Contains(name, "/") {
		names = []string{name, "library/" + name}
	} else {
		names = []string{"library/" + name, name}
	}
	for _, name := range names {
		token, err := w.getToken(name)
		if err != nil {
			return "", err
		}
		url := fmt.Sprintf(
			"%s/%s/manifests/%s", w.Client.BaseDigestURL, name, tag,
		)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
		req.Header.Add(
			"Accept", "application/vnd.docker.distribution.manifest.v2+json",
		)
		resp, err := w.Client.Client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		digest := resp.Header.Get("Docker-Content-Digest")
		if digest != "" {
			return strings.TrimPrefix(digest, "sha256:"), nil
		}
	}
	return "", errors.New("no digest found")
}

func (w *DockerWrapper) getToken(name string) (string, error) {
	url := fmt.Sprintf(
		"%s?scope=repository:%s:pull&service=registry.docker.io",
		w.Client.BaseTokenURL,
		name,
	)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	if w.authCreds.username != "" && w.authCreds.password != "" {
		req.SetBasicAuth(w.authCreds.username, w.authCreds.password)
	}
	resp, err := w.Client.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	var t dockerTokenResponse
	if err = decoder.Decode(&t); err != nil {
		return "", err
	}
	return t.Token, nil
}

func (w *DockerWrapper) getAuthCredentials() (*dockerAuthCredentials, error) {
	var (
		username = os.Getenv("DOCKER_USERNAME")
		password = os.Getenv("DOCKER_PASSWORD")
	)
	if username != "" && password != "" {
		return &dockerAuthCredentials{
			username: username,
			password: password,
		}, nil
	}
	if w.ConfigFile == "" {
		return &dockerAuthCredentials{}, nil
	}
	confByt, err := ioutil.ReadFile(w.ConfigFile)
	if err != nil {
		return nil, err
	}
	var conf dockerConfig
	if err = json.Unmarshal(confByt, &conf); err != nil {
		return nil, err
	}
	authByt, err := base64.StdEncoding.DecodeString(conf.Auths.Index.Auth)
	if err != nil {
		return nil, err
	}
	authString := string(authByt)
	if authString != "" {
		auth := strings.Split(authString, ":")
		return &dockerAuthCredentials{username: auth[0], password: auth[1]}, nil
	} else if conf.CredsStore != "" {
		authCreds, err := w.getAuthCredentialsFromCredsStore(conf.CredsStore)
		if err != nil {
			return &dockerAuthCredentials{}, nil
		}
		return authCreds, nil
	}
	return &dockerAuthCredentials{}, nil
}

func (w *DockerWrapper) getAuthCredentialsFromCredsStore(
	credsStore string,
) (authCreds *dockerAuthCredentials, err error) {
	credsStore = fmt.Sprintf("%s-%s", "docker-credential", credsStore)
	defer func() {
		if err := recover(); err != nil {
			authCreds = &dockerAuthCredentials{}
			return
		}
	}()
	p := c.NewShellProgramFunc(credsStore)
	credResponse, err := c.Get(p, "https://index.docker.io/v1/")
	if err != nil {
		return authCreds, err
	}
	return &dockerAuthCredentials{
		username: credResponse.Username,
		password: credResponse.Secret,
	}, nil
}

// Prefix returns an empty string since images on Docker Hub do not use a
// prefix, unlike third party registries.
func (w *DockerWrapper) Prefix() string {
	return ""
}
