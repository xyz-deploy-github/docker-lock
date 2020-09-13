package generate_test

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"

	"github.com/safe-waters/docker-lock/generate"
)

func TestLockfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name              string
		DockerfileImages  map[string][]*generate.DockerfileImage
		ComposefileImages map[string][]*generate.ComposefileImage
		Expected          *generate.Lockfile
	}{
		{
			Name:     "Nil Images",
			Expected: &generate.Lockfile{},
		},
		{
			Name: "Non Nil Images",
			DockerfileImages: map[string][]*generate.DockerfileImage{
				"Dockerfile": {
					{
						Image: &generate.Image{
							Name: "busybox",
							Tag:  "latest",
						},
						Path: "Dockerfile",
					},
				},
			},
			ComposefileImages: map[string][]*generate.ComposefileImage{
				"docker-compose.yml": {
					{
						Image: &generate.Image{
							Name: "busybox",
							Tag:  "latest",
						},
						DockerfilePath: "Dockerfile",
						ServiceName:    "svc",
						Path:           "docker-compose.yml",
					},
				},
			},
			Expected: &generate.Lockfile{
				DockerfileImages: map[string][]*generate.DockerfileImage{
					"Dockerfile": {
						{
							Image: &generate.Image{
								Name: "busybox",
								Tag:  "latest",
							},
							Path: "Dockerfile",
						},
					},
				},
				ComposefileImages: map[string][]*generate.ComposefileImage{
					"docker-compose.yml": {
						{
							Image: &generate.Image{
								Name: "busybox",
								Tag:  "latest",
							},
							DockerfilePath: "Dockerfile",
							ServiceName:    "svc",
							Path:           "docker-compose.yml",
						},
					},
				},
			},
		},
		{
			Name: "Nil Dockerfile Images",
			ComposefileImages: map[string][]*generate.ComposefileImage{
				"docker-compose.yml": {
					{
						Image: &generate.Image{
							Name: "busybox",
							Tag:  "latest",
						},
						DockerfilePath: "Dockerfile",
						ServiceName:    "svc",
						Path:           "docker-compose.yml",
					},
				},
			},
			Expected: &generate.Lockfile{
				ComposefileImages: map[string][]*generate.ComposefileImage{
					"docker-compose.yml": {
						{
							Image: &generate.Image{
								Name: "busybox",
								Tag:  "latest",
							},
							DockerfilePath: "Dockerfile",
							ServiceName:    "svc",
							Path:           "docker-compose.yml",
						},
					},
				},
			},
		},
		{
			Name: "Nil Composefile Images",
			DockerfileImages: map[string][]*generate.DockerfileImage{
				"Dockerfile": {
					{
						Image: &generate.Image{
							Name: "busybox",
							Tag:  "latest",
						},
						Path: "Dockerfile",
					},
				},
			},
			Expected: &generate.Lockfile{
				DockerfileImages: map[string][]*generate.DockerfileImage{
					"Dockerfile": {
						{
							Image: &generate.Image{
								Name: "busybox",
								Tag:  "latest",
							},
							Path: "Dockerfile",
						},
					},
				},
			},
		},
		{
			Name: "Sorted Images",
			DockerfileImages: map[string][]*generate.DockerfileImage{
				"Dockerfile": {
					{
						Image: &generate.Image{
							Name: "busybox",
							Tag:  "latest",
						},
						Position: 1,
						Path:     "Dockerfile",
					},
					{
						Image: &generate.Image{
							Name: "busybox",
							Tag:  "latest",
						},
						Position: 0,
						Path:     "Dockerfile",
					},
				},
			},
			ComposefileImages: map[string][]*generate.ComposefileImage{
				"docker-compose.yml": {
					{
						Image: &generate.Image{
							Name: "busybox",
							Tag:  "latest",
						},
						DockerfilePath: "Dockerfile",
						Position:       1,
						ServiceName:    "svc",
						Path:           "docker-compose.yml",
					},
					{
						Image: &generate.Image{
							Name: "busybox",
							Tag:  "latest",
						},
						DockerfilePath: "Dockerfile",
						Position:       0,
						ServiceName:    "svc",
						Path:           "docker-compose.yml",
					},
				},
			},
			Expected: &generate.Lockfile{
				DockerfileImages: map[string][]*generate.DockerfileImage{
					"Dockerfile": {
						{
							Image: &generate.Image{
								Name: "busybox",
								Tag:  "latest",
							},
							Position: 0,
							Path:     "Dockerfile",
						},
						{
							Image: &generate.Image{
								Name: "busybox",
								Tag:  "latest",
							},
							Position: 1,
							Path:     "Dockerfile",
						},
					},
				},
				ComposefileImages: map[string][]*generate.ComposefileImage{
					"docker-compose.yml": {
						{
							Image: &generate.Image{
								Name: "busybox",
								Tag:  "latest",
							},
							DockerfilePath: "Dockerfile",
							Position:       0,
							ServiceName:    "svc",
							Path:           "docker-compose.yml",
						},
						{
							Image: &generate.Image{
								Name: "busybox",
								Tag:  "latest",
							},
							DockerfilePath: "Dockerfile",
							Position:       1,
							ServiceName:    "svc",
							Path:           "docker-compose.yml",
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			done := make(chan struct{})

			var dockerfileImages chan *generate.DockerfileImage

			var waitGroup sync.WaitGroup

			if len(test.DockerfileImages) != 0 {
				dockerfileImages = make(chan *generate.DockerfileImage)

				waitGroup.Add(1)

				go func() {
					defer waitGroup.Done()

					for _, images := range test.DockerfileImages {
						for _, image := range images {
							select {
							case <-done:
								return
							case dockerfileImages <- image:
							}
						}
					}
				}()
			}

			var composefileImages chan *generate.ComposefileImage

			if len(test.ComposefileImages) != 0 {
				composefileImages = make(chan *generate.ComposefileImage)

				waitGroup.Add(1)

				go func() {
					defer waitGroup.Done()

					for _, images := range test.ComposefileImages {
						for _, image := range images {
							select {
							case <-done:
								return
							case composefileImages <- image:
							}
						}
					}
				}()
			}

			go func() {
				waitGroup.Wait()

				if dockerfileImages != nil {
					close(dockerfileImages)
				}

				if composefileImages != nil {
					close(composefileImages)
				}
			}()

			got, err := generate.NewLockfile(
				dockerfileImages, composefileImages, done,
			)
			if err != nil {
				t.Fatal(err)
			}

			assertLockfilesEqual(t, test.Expected, got)

			assertDefaultValuesForOmittedJSONReadFromLockfile(t, got)
		})
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
