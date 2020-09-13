package generate_test

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/safe-waters/docker-lock/generate"
	"github.com/safe-waters/docker-lock/registry"
	"github.com/safe-waters/docker-lock/registry/contrib"
	"github.com/safe-waters/docker-lock/registry/firstparty"
)

func TestNewUpdater(t *testing.T) {
	t.Parallel()

	if _, err := generate.NewUpdater(nil); err == nil {
		t.Fatal("expected err, got nil")
	}

	server := mockServer(t, nil)
	defer server.Close()

	wrapperManager := defaultWrapperManager(t, server)
	if _, err := generate.NewUpdater(wrapperManager); err != nil {
		t.Fatalf("expected nil, got err: %v", err)
	}
}

func TestUpdater(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name                      string
		DockerfileImages          []*generate.DockerfileImage
		ComposefileImages         []*generate.ComposefileImage
		ExpectedNumNetworkCalls   uint64
		ExpectedDockerfileImages  []*generate.DockerfileImage
		ExpectedComposefileImages []*generate.ComposefileImage
	}{
		{
			Name: "Dockerfile Image Without Digest",
			DockerfileImages: []*generate.DockerfileImage{
				{
					Image: &generate.Image{
						Name: "busybox",
						Tag:  "latest",
					},
				},
			},
			ExpectedNumNetworkCalls: 1,
			ExpectedDockerfileImages: []*generate.DockerfileImage{
				{
					Image: &generate.Image{
						Name:   "busybox",
						Tag:    "latest",
						Digest: busyboxLatestSHA,
					},
				},
			},
		},
		{
			Name: "Dockerfile Image With Digest",
			DockerfileImages: []*generate.DockerfileImage{
				{
					Image: &generate.Image{
						Name:   "busybox",
						Tag:    "latest",
						Digest: busyboxLatestSHA,
					},
				},
			},
			ExpectedNumNetworkCalls: 0,
			ExpectedDockerfileImages: []*generate.DockerfileImage{
				{
					Image: &generate.Image{
						Name:   "busybox",
						Tag:    "latest",
						Digest: busyboxLatestSHA,
					},
				},
			},
		},
		{
			Name: "Composefile Image Without Digest",
			ComposefileImages: []*generate.ComposefileImage{
				{
					Image: &generate.Image{
						Name: "busybox",
						Tag:  "latest",
					},
				},
			},
			ExpectedNumNetworkCalls: 1,
			ExpectedComposefileImages: []*generate.ComposefileImage{
				{
					Image: &generate.Image{
						Name:   "busybox",
						Tag:    "latest",
						Digest: busyboxLatestSHA,
					},
				},
			},
		},
		{
			Name: "Composefile Image With Digest",
			ComposefileImages: []*generate.ComposefileImage{
				{
					Image: &generate.Image{
						Name:   "busybox",
						Tag:    "latest",
						Digest: busyboxLatestSHA,
					},
				},
			},
			ExpectedNumNetworkCalls: 0,
			ExpectedComposefileImages: []*generate.ComposefileImage{
				{
					Image: &generate.Image{
						Name:   "busybox",
						Tag:    "latest",
						Digest: busyboxLatestSHA,
					},
				},
			},
		},
		{
			Name: "No Duplicate Network Calls",
			DockerfileImages: []*generate.DockerfileImage{
				{
					Image: &generate.Image{
						Name:   "busybox",
						Tag:    "latest",
						Digest: busyboxLatestSHA,
					},
				},
				{
					Image: &generate.Image{
						Name: "busybox",
						Tag:  "latest",
					},
				},
			},
			ComposefileImages: []*generate.ComposefileImage{
				{
					Image: &generate.Image{
						Name:   "busybox",
						Tag:    "latest",
						Digest: busyboxLatestSHA,
					},
				},
				{
					Image: &generate.Image{
						Name: "busybox",
						Tag:  "latest",
					},
				},
			},
			ExpectedNumNetworkCalls: 1,
			ExpectedDockerfileImages: []*generate.DockerfileImage{
				{
					Image: &generate.Image{
						Name:   "busybox",
						Tag:    "latest",
						Digest: busyboxLatestSHA,
					},
				},
				{
					Image: &generate.Image{
						Name:   "busybox",
						Tag:    "latest",
						Digest: busyboxLatestSHA,
					},
				},
			},
			ExpectedComposefileImages: []*generate.ComposefileImage{
				{
					Image: &generate.Image{
						Name:   "busybox",
						Tag:    "latest",
						Digest: busyboxLatestSHA,
					},
				},
				{
					Image: &generate.Image{
						Name:   "busybox",
						Tag:    "latest",
						Digest: busyboxLatestSHA,
					},
				},
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			var gotNumNetworkCalls uint64

			server := mockServer(t, &gotNumNetworkCalls)
			defer server.Close()

			wrapperManager := defaultWrapperManager(t, server)

			updater, err := generate.NewUpdater(wrapperManager)
			if err != nil {
				t.Fatal(err)
			}

			done := make(chan struct{})

			dockerfileImages := make(
				chan *generate.DockerfileImage, len(test.DockerfileImages),
			)

			for _, dockerfileImage := range test.DockerfileImages {
				dockerfileImages <- dockerfileImage
			}
			close(dockerfileImages)

			composefileImages := make(
				chan *generate.ComposefileImage, len(test.ComposefileImages),
			)

			for _, composefileImage := range test.ComposefileImages {
				composefileImages <- composefileImage
			}
			close(composefileImages)

			updatedDockerfileImages, updatedComposefileImages := updater.UpdateDigests( // nolint: lll
				dockerfileImages, composefileImages, done,
			)

			var gotDockerfileImages []*generate.DockerfileImage
			var gotComposefileImages []*generate.ComposefileImage

			errs := make(chan error)
			var waitGroup sync.WaitGroup

			waitGroup.Add(1)

			go func() {
				defer waitGroup.Done()

				for dockerfileImage := range updatedDockerfileImages {
					if dockerfileImage.Err != nil {
						select {
						case <-done:
						case errs <- dockerfileImage.Err:
						}

						return
					}
					gotDockerfileImages = append(
						gotDockerfileImages, dockerfileImage,
					)
				}
			}()

			waitGroup.Add(1)

			go func() {
				defer waitGroup.Done()

				for composefileImage := range updatedComposefileImages {
					if composefileImage.Err != nil {
						select {
						case <-done:
						case errs <- composefileImage.Err:
						}

						return
					}
					gotComposefileImages = append(
						gotComposefileImages, composefileImage,
					)
				}
			}()

			go func() {
				waitGroup.Wait()
				close(errs)
			}()

			for err := range errs {
				close(done)
				t.Fatal(err)
			}

			assertDockerfileImagesEqual(
				t, test.ExpectedDockerfileImages, gotDockerfileImages,
			)

			assertComposefileImagesEqual(
				t, test.ExpectedComposefileImages, gotComposefileImages,
			)

			assertNumNetworkCallsEqual(
				t, test.ExpectedNumNetworkCalls, gotNumNetworkCalls,
			)
		})
	}
}

func assertNumNetworkCallsEqual(t *testing.T, expected uint64, got uint64) {
	t.Helper()

	if expected != got {
		t.Fatalf("expected %d network calls, got %d", expected, got)
	}
}

func defaultWrapperManager(
	t *testing.T,
	server *httptest.Server,
) *registry.WrapperManager {
	t.Helper()

	client := &registry.HTTPClient{
		Client:      server.Client(),
		RegistryURL: server.URL,
		TokenURL:    server.URL + "?scope=repository%s",
	}
	configPath := defaultConfigPath()

	defaultWrapper, err := firstparty.DefaultWrapper(client, configPath)
	if err != nil {
		t.Fatal(err)
	}

	wrapperManager := registry.NewWrapperManager(defaultWrapper)
	wrapperManager.Add(firstparty.AllWrappers(client, configPath)...)
	wrapperManager.Add(contrib.AllWrappers(client, configPath)...)

	return wrapperManager
}

func defaultConfigPath() string {
	if homeDir, err := os.UserHomeDir(); err == nil {
		configPath := filepath.Join(homeDir, ".docker", "config.json")

		if _, err := os.Stat(configPath); err != nil {
			return ""
		}

		return configPath
	}

	return ""
}
