package generate_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	cmd_generate "github.com/safe-waters/docker-lock/cmd/generate"
	"github.com/safe-waters/docker-lock/generate"
	"github.com/safe-waters/docker-lock/generate/collect"
	"github.com/safe-waters/docker-lock/generate/parse"
)

const busyboxLatestSHA = "bae015c28bc7cdee3b7ef20d35db4299e3068554a769070950229d9f53f58572" // nolint: lll
const golangLatestSHA = "6cb55c08bbf44793f16e3572bd7d2ae18f7a858f6ae4faa474c0a6eae1174a5d"  // nolint: lll
const redisLatestSHA = "09c33840ec47815dc0351f1eca3befe741d7105b3e95bc8fdb9a7e4985b9e1e5"   // nolint: lll

type DockerfileImageWithoutStructTags struct {
	*parse.Image
	Position int
	Path     string
	Err      error
}

type ComposefileImageWithoutStructTags struct {
	*parse.Image
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

func assertDefaultValuesForOmittedJSONReadFromLockfile(
	t *testing.T,
	got *generate.Lockfile,
) {
	t.Helper()

	var buf bytes.Buffer
	if err := got.Write(&buf); err != nil {
		t.Fatal(err)
	}

	var readInLockfile generate.Lockfile
	if err := json.Unmarshal(buf.Bytes(), &readInLockfile); err != nil {
		t.Fatal(err)
	}

	for _, images := range readInLockfile.DockerfileImages {
		for _, image := range images {
			if image.Position != 0 {
				t.Fatal(
					"Written output contains unexpected key 'Position'",
				)
			}

			if image.Path != "" {
				t.Fatal(
					"Written output contains unexpected key 'Path'",
				)
			}

			if image.Err != nil {
				t.Fatal(
					"Written output contains unexpected key 'Err'",
				)
			}
		}
	}

	for _, images := range readInLockfile.ComposefileImages {
		for _, image := range images {
			if image.Position != 0 {
				t.Fatal(
					"Written output contains unexpected key 'Position'",
				)
			}

			if image.Path != "" {
				t.Fatal(
					"Written output contains unexpected key 'Path'",
				)
			}

			if image.Err != nil {
				t.Fatal(
					"Written output contains unexpected key 'Err'",
				)
			}
		}
	}
}

func assertConcretePathCollector(
	t *testing.T,
	pathCollector *generate.PathCollector,
	done <-chan struct{},
) {
	t.Helper()

	dockerfilePaths, composefilePaths := pathCollector.CollectPaths(done)

	if pathCollector.DockerfileCollector != nil && dockerfilePaths == nil {
		t.Fatal("expected non nil dockerfilePaths")
	}

	if pathCollector.DockerfileCollector == nil && dockerfilePaths != nil {
		t.Fatal("expected nil dockerfilePaths")
	}

	if pathCollector.ComposefileCollector != nil && composefilePaths == nil {
		t.Fatal("expected non nil composefilePaths")
	}

	if pathCollector.ComposefileCollector == nil && composefilePaths != nil {
		t.Fatal("expected nil composefilePaths")
	}
}

func assertConcreteImageParser(
	t *testing.T,
	imageParser *generate.ImageParser,
	done <-chan struct{}) {
	t.Helper()

	var dockerfilePaths chan *collect.PathResult

	var composefilePaths chan *collect.PathResult

	dockerfileImages, composefileImages := imageParser.ParseFiles(
		dockerfilePaths, composefilePaths, done,
	)

	if imageParser.DockerfileImageParser != nil && dockerfileImages == nil {
		t.Fatal("expected non nil dockerfileImages")
	}

	if imageParser.DockerfileImageParser == nil && dockerfileImages != nil {
		t.Fatal("expected nil dockerfileImages")
	}

	if imageParser.ComposefileImageParser != nil && composefileImages == nil {
		t.Fatal("expected non nil composefileImages")
	}

	if imageParser.ComposefileImageParser == nil && composefileImages != nil {
		t.Fatal("expected nil composefileImages")
	}
}

func assertConcreteImageDigestUpdater(
	t *testing.T,
	updater *generate.ImageDigestUpdater,
	done <-chan struct{},
) {
	t.Helper()

	var dockerfileImages chan *parse.DockerfileImage

	var composefileImages chan *parse.ComposefileImage

	updatedDockerfileImages, updatedComposefileImages := updater.UpdateDigests( // nolint: lll
		dockerfileImages, composefileImages, done,
	)

	if updater.DockerfileImageDigestUpdater != nil &&
		updatedDockerfileImages == nil {
		t.Fatal("expected non nil updatedDockerfileImages")
	}

	if updater.DockerfileImageDigestUpdater == nil &&
		updatedDockerfileImages != nil {
		t.Fatal("expected nil updatedDockerfileImages")
	}

	if updater.ComposefileImageDigestUpdater != nil &&
		updatedComposefileImages == nil {
		t.Fatal("expected non nil updatedComposefileImages")
	}

	if updater.ComposefileImageDigestUpdater == nil &&
		updatedComposefileImages != nil {
		t.Fatal("expected nil updatedComposefileImages")
	}
}

func copyDockerfileImagesToDockerfileImagesWithoutStructTags(
	t *testing.T,
	dockerfileImages []*parse.DockerfileImage,
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
	composefileImages []*parse.ComposefileImage,
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

// nolint: unparam
func makeFlags(
	t *testing.T,
	baseDir string,
	lockfileName string,
	configPath string,
	envPath string,
	dockerfilePaths []string,
	composefilePaths []string,
	dockerfileGlobs []string,
	composefileGlobs []string,
	dockerfileRecursive bool,
	composefileRecursive bool,
	dockerfileExcludeAll bool,
	composefileExcludeAll bool,
) *cmd_generate.Flags {
	t.Helper()

	flags, err := cmd_generate.NewFlags(
		baseDir, lockfileName, configPath, envPath, dockerfilePaths,
		composefilePaths, dockerfileGlobs, composefileGlobs,
		dockerfileRecursive, composefileRecursive,
		dockerfileExcludeAll, composefileExcludeAll,
	)
	if err != nil {
		t.Fatal(err)
	}

	return flags
}
