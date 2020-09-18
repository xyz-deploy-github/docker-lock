package verify_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	cmd_verify "github.com/safe-waters/docker-lock/cmd/verify"
	"github.com/safe-waters/docker-lock/registry"
)

var dTestDir = filepath.Join("testdata", "docker")  // nolint: gochecknoglobals
var cTestDir = filepath.Join("testdata", "compose") // nolint: gochecknoglobals

func TestVerifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name       string
		Flags      *cmd_verify.Flags
		ShouldFail bool
	}{
		{
			Name: "Different Number of Images in Dockerfile",
			Flags: &cmd_verify.Flags{
				LockfileName: filepath.Join(
					dTestDir, "diffnumimages", "docker-lock.json",
				),
				EnvPath: ".env",
			},
			ShouldFail: true,
		},
		{
			Name: "Different Digests in Dockerfile",
			Flags: &cmd_verify.Flags{
				LockfileName: filepath.Join(
					dTestDir, "diffdigests", "docker-lock.json",
				),
				EnvPath: ".env",
			},
			ShouldFail: true,
		},
		{
			Name: "Different Number of Images in Composefile",
			Flags: &cmd_verify.Flags{
				LockfileName: filepath.Join(
					cTestDir, "diffnumimages", "docker-lock.json",
				),
				EnvPath: ".env",
			},
			ShouldFail: true,
		},
		{
			Name: "Different Digests in Composefile",
			Flags: &cmd_verify.Flags{
				LockfileName: filepath.Join(
					cTestDir, "diffdigests", "docker-lock.json",
				),
				EnvPath: ".env",
			},
			ShouldFail: true,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			var numNetworkCalls uint64

			server := mockServer(t, &numNetworkCalls)
			defer server.Close()

			client := &registry.HTTPClient{
				Client:      server.Client(),
				RegistryURL: server.URL,
				TokenURL:    server.URL + "?scope=repository%s",
			}

			verifier, err := cmd_verify.SetupVerifier(client, test.Flags)
			if err != nil {
				t.Fatal(err)
			}

			reader, err := os.Open(test.Flags.LockfileName)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()

			if err := verifier.VerifyLockfile(reader); err != nil &&
				!test.ShouldFail {
				t.Fatal(err)
			}
		})
	}
}

const busyboxLatestSHA = "bae015c28bc7cdee3b7ef20d35db4299e3068554a769070950229d9f53f58572" // nolint: lll
const golangLatestSHA = "6cb55c08bbf44793f16e3572bd7d2ae18f7a858f6ae4faa474c0a6eae1174a5d"  // nolint: lll
const redisLatestSHA = "09c33840ec47815dc0351f1eca3befe741d7105b3e95bc8fdb9a7e4985b9e1e5"   // nolint: lll

func mockServer(t *testing.T, numNetworkCalls *uint64) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(
		http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			switch url := req.URL.String(); {
			case strings.Contains(url, "scope"):
				byt := []byte(`{"token": "NOT_USED"}`)
				_, err := res.Write(byt)
				if err != nil {
					t.Fatal(err)
				}
			case strings.Contains(url, "manifests"):
				atomic.AddUint64(numNetworkCalls, 1)

				urlParts := strings.Split(url, "/")
				repo, ref := urlParts[2], urlParts[len(urlParts)-1]

				var digest string
				switch fmt.Sprintf("%s:%s", repo, ref) {
				case "busybox:latest":
					digest = busyboxLatestSHA
				case "redis:latest":
					digest = redisLatestSHA
				case "golang:latest":
					digest = golangLatestSHA
				default:
					digest = fmt.Sprintf(
						"repo %s with ref %s not defined for testing",
						repo, ref,
					)
				}

				res.Header().Set("Docker-Content-Digest", digest)
			}
		}))

	return server
}
