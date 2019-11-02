package acr

// TokenResponse holds the token to get digests.
type TokenResponse struct {
	Token string `json:"access_token"`
}

// Config holds information from config.json.
type Config struct {
	Auths      map[string]map[string]string `json:"auths"`
	CredsStore string                       `json:"credsStore"`
}
