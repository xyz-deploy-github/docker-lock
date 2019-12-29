package unittests

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/michaelperel/docker-lock/registry"
)

var (
	server = getMockServer()
	client = &registry.HTTPClient{Client: server.Client(), BaseDigestURL: server.URL, BaseTokenURL: server.URL}
)

// TestMain executes code before the tests for the package is run and after.
func TestMain(m *testing.M) {
	retCode := m.Run()
	server.Close()
	os.Exit(retCode)
}

func getMockServer() *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch url := req.URL.String(); {
		// get token
		case strings.Contains(url, "scope"):
			byt := []byte(`{"token": "NOT_USED"}`)
			_, _ = rw.Write(byt)
		// get digest
		case strings.Contains(url, "manifests"):
			rw.Header().Set("Docker-Content-Digest", "sha256:NOT_USED")
		}
	}))
	return server
}
