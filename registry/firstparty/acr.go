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

// ACRWrapper is a registry wrapper for Azure Container Registry.
type ACRWrapper struct {
	ConfigurationPath string
	Client            *registry.HTTPClient
	*acrAuthCreds
	registryName string
}

// acrTokenResponse contains the bearer token required to
// query the container registry for a digest.
type acrTokenResponse struct {
	Token string `json:"access_token"`
}

// acrConfig represents the section in docker's config.json for ACR.
type acrConfig struct {
	Auths      map[string]map[string]string `json:"auths"`
	CredsStore string                       `json:"credsStore"`
}

// acrAuthCreds contains the username and password required to
// query the container registry for a digest.
type acrAuthCreds struct {
	username string
	password string
}

// NewACRWrapper creates an ACRWrapper from docker's config.json.
// For ACRWrapper to be selected by the wrapper manager, the environment
// variable ACR_REGISTRY_NAME must be set. For instance, if your image is
// stored at myregistry.azurecr.io/myimage, then
// ACR_REGISTRY_NAME=myregistry.
func NewACRWrapper(
	configPath string,
	client *registry.HTTPClient,
) (*ACRWrapper, error) {
	w := &ACRWrapper{ConfigurationPath: configPath}
	w.registryName = os.Getenv("ACR_REGISTRY_NAME")

	if client == nil {
		w.Client = &registry.HTTPClient{
			Client:        &http.Client{},
			BaseDigestURL: fmt.Sprintf("https://%sv2", w.Prefix()),
			BaseTokenURL:  fmt.Sprintf("https://%soauth2/token", w.Prefix()),
		}
	}

	ac, err := w.authCreds()
	if err != nil {
		return nil, err
	}

	w.acrAuthCreds = ac

	return w, nil
}

// Digest queries the container registry for the digest given a name and tag.
// The workflow for authenticating with private repositories:
//
// (1) if "ACR_USERNAME" and "ACR_PASSWORD" are set, use them.
//
// (2) Otherwise, try to get credentials from docker's config file.
// This method requires the user to have logged in with the
// 'docker login' command beforehand.
func (w *ACRWrapper) Digest(name string, tag string) (string, error) {
	name = strings.Replace(name, w.Prefix(), "", 1)

	t, err := w.token(name)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/%s/manifests/%s", w.Client.BaseDigestURL, name, tag)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", t))
	req.Header.Add(
		"Accept", "application/vnd.docker.distribution.manifest.v2+json",
	)

	resp, err := w.Client.Client.Do(req)
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

// token queries the container registry for a bearer token that is later
// required to query the container registry for a digest.
func (w *ACRWrapper) token(name string) (string, error) {
	url := fmt.Sprintf(
		"%s?service=%s.azurecr.io&scope=repository:%s:pull",
		w.Client.BaseTokenURL, w.registryName, name,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	if w.username != "" && w.password != "" {
		req.SetBasicAuth(w.username, w.password)
	}

	resp, err := w.Client.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	d := json.NewDecoder(resp.Body)

	t := acrTokenResponse{}
	if err = d.Decode(&t); err != nil {
		return "", err
	}

	return t.Token, nil
}

// authCreds returns the username and password required to query
// the registry for the digest. It first looks for the environment
// variables "ACR_USERNAME" and "ACR_PASSWORD". If those do not exist,
// it tries to read them from docker's config.json.
func (w *ACRWrapper) authCreds() (*acrAuthCreds, error) {
	username := os.Getenv("ACR_USERNAME")
	password := os.Getenv("ACR_PASSWORD")

	if username != "" && password != "" {
		return &acrAuthCreds{username: username, password: password}, nil
	}

	if w.ConfigurationPath == "" {
		return &acrAuthCreds{}, nil
	}

	confByt, err := ioutil.ReadFile(w.ConfigurationPath)
	if err != nil {
		return nil, err
	}

	conf := acrConfig{}
	if err = json.Unmarshal(confByt, &conf); err != nil {
		return nil, err
	}

	var authByt []byte

	for serverName, authInfo := range conf.Auths {
		if serverName == fmt.Sprintf("%s.azurecr.io", w.registryName) {
			authByt, err = base64.StdEncoding.DecodeString(authInfo["auth"])
			if err != nil {
				return nil, err
			}

			break
		}
	}

	authStr := string(authByt)

	switch {
	case authStr != "":
		auth := strings.Split(authStr, ":")
		return &acrAuthCreds{username: auth[0], password: auth[1]}, nil
	case conf.CredsStore != "":
		authCreds, err := w.authCredsFromStore(conf.CredsStore)
		if err != nil {
			return &acrAuthCreds{}, nil
		}

		return authCreds, nil
	}

	return &acrAuthCreds{}, nil
}

// authCredsFromStore reads auth creds from a creds store such as
// wincred, pass, or osxkeychain by shelling out to docker credential helper.
func (w *ACRWrapper) authCredsFromStore(
	credsStore string,
) (authCreds *acrAuthCreds, err error) {
	defer func() {
		if err := recover(); err != nil {
			authCreds = &acrAuthCreds{}
			return
		}
	}()

	credsStore = fmt.Sprintf("%s-%s", "docker-credential", credsStore)
	p := c.NewShellProgramFunc(credsStore)

	credRes, err := c.Get(p, fmt.Sprintf("%s.azurecr.io", w.registryName))
	if err != nil {
		return authCreds, err
	}

	return &acrAuthCreds{
		username: credRes.Username,
		password: credRes.Secret,
	}, nil
}

// Prefix returns the registry prefix that identifies ACR.
func (w *ACRWrapper) Prefix() string {
	return fmt.Sprintf("%s.azurecr.io/", w.registryName)
}
