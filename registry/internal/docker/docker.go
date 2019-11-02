package docker

// TokenResponse holds the token to get digests.
type TokenResponse struct {
	Token string `json:"token"`
}

// Config holds information from config.json.
type Config struct {
	Auths struct {
		Index struct {
			Auth string `json:"auth"`
		} `json:"https://index.docker.io/v1/"`
	} `json:"auths"`
	CredsStore string `json:"credsStore"`
}
