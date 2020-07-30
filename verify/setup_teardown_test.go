package verify

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/safe-waters/docker-lock/registry"
)

var server = mockServer()          //nolint: gochecknoglobals
var client = &registry.HTTPClient{ //nolint: gochecknoglobals
	Client:      server.Client(),
	RegistryURL: server.URL,
	TokenURL:    server.URL + "?scope=repository%s",
}

// TestMain executes code before the tests for the package is run and after.
func TestMain(m *testing.M) {
	log.SetOutput(ioutil.Discard)

	retCode := m.Run()

	server.Close()
	os.Exit(retCode)
}

func mockServer() *httptest.Server {
	server := httptest.NewServer(
		http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
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
