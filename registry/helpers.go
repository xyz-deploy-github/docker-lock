package registry

import "net/http"

// HTTPClient overrides base urls to get digests and auth tokens.
type HTTPClient struct {
	Client        *http.Client
	BaseDigestURL string
	BaseTokenURL  string
}
