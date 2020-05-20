package registry

import "net/http"

// HTTPClient overrides base urls to get digests and auth tokens.
type HTTPClient struct {
	*http.Client
	BaseDigestURL string
	BaseTokenURL  string
}
