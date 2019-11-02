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
	"github.com/michaelperel/docker-lock/registry/internal/acr"
)

// ACRWrapper is a registry wrapper for Azure Container Registry.
type ACRWrapper struct {
	ConfigFile string
	Client     *HTTPClient
	authCreds  *acrAuthCredentials
	regName    string
}

type acrAuthCredentials struct {
	username string
	password string
}

// NewACRWrapper creates an ACRWrapper from docker's config.json.
func NewACRWrapper(configPath string, client *HTTPClient) (*ACRWrapper, error) {
	w := &ACRWrapper{ConfigFile: configPath}
	w.regName = os.Getenv("ACR_REGISTRY_NAME")
	if client == nil {
		w.Client = &HTTPClient{
			Client:        &http.Client{},
			BaseDigestURL: fmt.Sprintf("https://%sv2", w.Prefix()),
			BaseTokenURL:  fmt.Sprintf("https://%soauth2/token", w.Prefix()),
		}
	}
	authCreds, err := w.getAuthCredentials()
	if err != nil {
		return nil, err
	}
	w.authCreds = authCreds
	return w, nil
}

// GetDigest gets the digest from a name and tag. The workflow for
// authenticating with private repositories:
// (1) if "ACR_USERNAME" and "ACR_PASSWORD" are set, use them.
// (2) Otherwise, try to get credentials from docker's config file. This method
// requires the user to have logged in with the 'docker login' command
// beforehand.
func (w *ACRWrapper) GetDigest(name string, tag string) (string, error) {
	name = strings.Replace(name, w.Prefix(), "", 1)
	token, err := w.getToken(name)
	url := fmt.Sprintf("%s/%s/manifests/%s", w.Client.BaseDigestURL, name, tag)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	resp, err := w.Client.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return "", errors.New("No digest found")
	}
	return strings.TrimPrefix(digest, "sha256:"), nil
}

func (w *ACRWrapper) getToken(name string) (string, error) {
	url := fmt.Sprintf("%s?service=%s.azurecr.io&scope=repository:%s:pull", w.Client.BaseTokenURL, w.regName, name)
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
	var t acr.TokenResponse
	if err = decoder.Decode(&t); err != nil {
		return "", err
	}
	return t.Token, nil
}

func (w *ACRWrapper) getAuthCredentials() (*acrAuthCredentials, error) {
	var (
		username = os.Getenv("ACR_USERNAME")
		password = os.Getenv("ACR_PASSWORD")
	)
	if username != "" && password != "" {
		return &acrAuthCredentials{username: username, password: password}, nil
	}
	if w.ConfigFile == "" {
		return &acrAuthCredentials{}, nil
	}
	confByt, err := ioutil.ReadFile(w.ConfigFile)
	if err != nil {
		return nil, err
	}
	var conf acr.Config
	if err = json.Unmarshal(confByt, &conf); err != nil {
		return nil, err
	}
	var authByt []byte
	for serverName, authInfo := range conf.Auths {
		if serverName == fmt.Sprintf("%s.azurecr.io", w.regName) {
			authByt, err = base64.StdEncoding.DecodeString(authInfo["auth"])
			if err != nil {
				return nil, err
			}
			break
		}
	}
	authString := string(authByt)
	if authString != "" {
		auth := strings.Split(authString, ":")
		return &acrAuthCredentials{username: auth[0], password: auth[1]}, nil
	} else if conf.CredsStore != "" {
		authCreds, err := w.getAuthCredentialsFromCredsStore(conf.CredsStore)
		if err != nil {
			return &acrAuthCredentials{}, nil
		}
		return authCreds, nil
	}
	return &acrAuthCredentials{}, nil
}

func (w *ACRWrapper) getAuthCredentialsFromCredsStore(credsStore string) (authCreds *acrAuthCredentials, err error) {
	credsStore = fmt.Sprintf("%s-%s", "docker-credential", credsStore)
	defer func() {
		if err := recover(); err != nil {
			authCreds = &acrAuthCredentials{}
			return
		}
	}()
	p := c.NewShellProgramFunc(credsStore)
	credResponse, err := c.Get(p, fmt.Sprintf("%s.azurecr.io", w.regName))
	if err != nil {
		return authCreds, err
	}
	return &acrAuthCredentials{username: credResponse.Username, password: credResponse.Secret}, nil
}

// Prefix returns the registry prefix that identifies ACR.
func (w *ACRWrapper) Prefix() string {
	return fmt.Sprintf("%s.azurecr.io/", w.regName)
}
