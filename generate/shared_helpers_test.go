package generate_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/safe-waters/docker-lock/generate"
)

const busyboxLatestSHA = "bae015c28bc7cdee3b7ef20d35db4299e3068554a769070950229d9f53f58572" // nolint: lll
const golangLatestSHA = "6cb55c08bbf44793f16e3572bd7d2ae18f7a858f6ae4faa474c0a6eae1174a5d"  // nolint: lll
const redisLatestSHA = "09c33840ec47815dc0351f1eca3befe741d7105b3e95bc8fdb9a7e4985b9e1e5"   // nolint: lll

type DockerfileImageWithoutStructTags struct {
	*generate.Image
	Position int
	Path     string
	Err      error
}

type ComposefileImageWithoutStructTags struct {
	*generate.Image
	DockerfilePath string
	Position       int
	ServiceName    string
	Path           string
	Err            error
}

type LockfileWithoutStructTags struct {
	DockerfileImages  map[string][]*DockerfileImageWithoutStructTags
	ComposefileImages map[string][]*ComposefileImageWithoutStructTags
}

func writeFilesToTempDir(
	t *testing.T,
	tempDir string,
	fileNames []string,
	fileContents [][]byte,
) []string {
	t.Helper()

	if len(fileNames) != len(fileContents) {
		t.Fatalf(
			"different number of names and contents: %d names, %d contents",
			len(fileNames), len(fileContents))
	}

	fullPaths := make([]string, len(fileNames))

	for i, name := range fileNames {
		fullPath := filepath.Join(tempDir, name)

		if err := ioutil.WriteFile(
			fullPath, fileContents[i], 0777,
		); err != nil {
			t.Fatal(err)
		}

		fullPaths[i] = fullPath
	}

	return fullPaths
}

func makeDir(t *testing.T, dirPath string) {
	t.Helper()

	err := os.MkdirAll(dirPath, 0777)
	if err != nil {
		t.Fatal(err)
	}
}

func makeTempDir(t *testing.T, dirName string) string {
	t.Helper()

	dir, err := ioutil.TempDir("", dirName)
	if err != nil {
		t.Fatal(err)
	}

	return dir
}

func makeParentDirsInTempDirFromFilePaths(
	t *testing.T,
	tempDir string,
	paths []string,
) {
	t.Helper()

	for _, p := range paths {
		dir, _ := filepath.Split(p)
		fullDir := filepath.Join(tempDir, dir)

		makeDir(t, fullDir)
	}
}

func assertDockerfileImagesEqual(
	t *testing.T,
	expected []*generate.DockerfileImage,
	got []*generate.DockerfileImage,
) {
	t.Helper()

	if !reflect.DeepEqual(expected, got) {
		expectedWithoutStructTags := copyDockerfileImagesToDockerfileImagesWithoutStructTags( // nolint: lll
			t, expected,
		)

		gotWithoutStructTags := copyDockerfileImagesToDockerfileImagesWithoutStructTags( // nolint: lll
			t, got,
		)

		t.Fatalf(
			"expected %+v, got %+v",
			jsonPrettyPrint(t, expectedWithoutStructTags),
			jsonPrettyPrint(t, gotWithoutStructTags),
		)
	}
}

func assertComposefileImagesEqual(
	t *testing.T,
	expected []*generate.ComposefileImage,
	got []*generate.ComposefileImage,
) {
	t.Helper()

	if !reflect.DeepEqual(expected, got) {
		expectedWithoutStructTags := copyComposefileImagesToComposefileImagesWithoutStructTags( // nolint: lll
			t, expected,
		)

		gotWithoutStructTags := copyComposefileImagesToComposefileImagesWithoutStructTags( // nolint: lll
			t, got,
		)

		t.Fatalf(
			"expected %+v, got %+v",
			jsonPrettyPrint(t, expectedWithoutStructTags),
			jsonPrettyPrint(t, gotWithoutStructTags),
		)
	}
}

func assertLockfilesEqual(
	t *testing.T,
	expected *generate.Lockfile,
	got *generate.Lockfile,
) {
	t.Helper()

	if !reflect.DeepEqual(expected, got) {
		expectedWithoutStructTags := copyLockfileToLockfileWithoutStructTags(
			t, expected,
		)

		gotWithoutStructTags := copyLockfileToLockfileWithoutStructTags(
			t, got,
		)

		t.Fatalf(
			"expected %+v, got %+v",
			jsonPrettyPrint(t, expectedWithoutStructTags),
			jsonPrettyPrint(t, gotWithoutStructTags),
		)
	}
}

func copyDockerfileImagesToDockerfileImagesWithoutStructTags(
	t *testing.T,
	dockerfileImages []*generate.DockerfileImage,
) []*DockerfileImageWithoutStructTags {
	t.Helper()

	dockerfileImagesWithoutStructTags := make(
		[]*DockerfileImageWithoutStructTags, len(dockerfileImages),
	)

	for i, image := range dockerfileImages {
		dockerfileImagesWithoutStructTags[i] =
			&DockerfileImageWithoutStructTags{
				Image:    image.Image,
				Position: image.Position,
				Path:     image.Path,
				Err:      image.Err,
			}
	}

	return dockerfileImagesWithoutStructTags
}

func copyComposefileImagesToComposefileImagesWithoutStructTags(
	t *testing.T,
	composefileImages []*generate.ComposefileImage,
) []*ComposefileImageWithoutStructTags {
	t.Helper()

	composefileImagesWithoutStructTags := make(
		[]*ComposefileImageWithoutStructTags, len(composefileImages),
	)

	for i, image := range composefileImages {
		composefileImagesWithoutStructTags[i] =
			&ComposefileImageWithoutStructTags{
				Image:          image.Image,
				DockerfilePath: image.DockerfilePath,
				Position:       image.Position,
				ServiceName:    image.ServiceName,
				Path:           image.Path,
				Err:            image.Err,
			}
	}

	return composefileImagesWithoutStructTags
}

func copyLockfileToLockfileWithoutStructTags(
	t *testing.T,
	lockfile *generate.Lockfile,
) *LockfileWithoutStructTags {
	t.Helper()

	lockfileWithoutStructTags := &LockfileWithoutStructTags{
		ComposefileImages: map[string][]*ComposefileImageWithoutStructTags{},
		DockerfileImages:  map[string][]*DockerfileImageWithoutStructTags{},
	}

	for p := range lockfile.DockerfileImages {
		lockfileWithoutStructTags.DockerfileImages[p] = copyDockerfileImagesToDockerfileImagesWithoutStructTags( // nolint: lll
			t, lockfile.DockerfileImages[p],
		)
	}

	for p := range lockfile.ComposefileImages {
		lockfileWithoutStructTags.ComposefileImages[p] = copyComposefileImagesToComposefileImagesWithoutStructTags( // nolint: lll
			t, lockfile.ComposefileImages[p],
		)
	}

	return lockfileWithoutStructTags
}

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

func jsonPrettyPrint(t *testing.T, i interface{}) string {
	t.Helper()

	byt, err := json.MarshalIndent(i, "", "\t")
	if err != nil {
		t.Fatal(err)
	}

	return string(byt)
}
