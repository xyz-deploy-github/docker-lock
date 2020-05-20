package firstparty

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	c "github.com/docker/docker-credential-helpers/client"
	"github.com/michaelperel/docker-lock/registry"
)

// ACRWrapper is a registry wrapper for Azure Container Registry.
type ACRWrapper struct {
	configPath string
	client     *registry.HTTPClient
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

// NewACRWrapper creates an ACRWrapper or returns an error if not possible.
//
// registryName must not be empty. If your image is referenced by the line
// myregistry.azurecr.io/myimage, "myregistry" should be registryName's value.
//
// If username and password are defined, then they will be used for
// authentication. Otherwise, the username and password will be obtained
// from docker's config.json. For this to work, please login using
// 'docker login' such as 'docker login myregistry.azurecr.io'.
//
// If using the cli, to set the registry name, username, and password, ensure
// ACR_REGISTRY_NAME, ACR_USERNAME, and ACR_PASSWORD are set. This can
// be achieved automatically via a .env file or manually by exporting the
// environment variables. configPath can be set via cli flags.
func NewACRWrapper(
	client *registry.HTTPClient,
	configPath string,
	username string,
	password string,
	registryName string,
) (*ACRWrapper, error) {
	if registryName == "" {
		return nil, fmt.Errorf("acr registry name is empty")
	}

	w := &ACRWrapper{configPath: configPath}
	w.registryName = registryName

	if client == nil {
		w.client = &registry.HTTPClient{
			Client:      &http.Client{},
			RegistryURL: fmt.Sprintf("https://%sv2", w.Prefix()),
			TokenURL: fmt.Sprintf(
				"https://%soauth2/token%s", w.Prefix(),
				"?service=%s.azurecr.io&scope=repository:%s:pull",
			),
		}
	}

	ac, err := w.authCreds(username, password)
	if err != nil {
		return nil, err
	}

	w.acrAuthCreds = ac

	return w, nil
}

// Digest queries the container registry for the digest given a repo and ref.
func (w *ACRWrapper) Digest(repo string, ref string) (string, error) {
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
func (w *ACRWrapper) token(repo string) (string, error) {
	url := fmt.Sprintf(w.client.TokenURL, w.registryName, repo)

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

	t := acrTokenResponse{}
	if err = d.Decode(&t); err != nil {
		return "", err
	}

	return t.Token, nil
}

// authCreds returns the username and password for ACR. If username and
// password are empty, it will look in docker's config.json.
func (w *ACRWrapper) authCreds(
	username string,
	password string,
) (*acrAuthCreds, error) {
	if username != "" && password != "" {
		return &acrAuthCreds{username: username, password: password}, nil
	}

	if w.configPath == "" {
		return &acrAuthCreds{}, nil
	}

	confByt, err := ioutil.ReadFile(w.configPath)
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
