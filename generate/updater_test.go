package generate_test

import (
	"testing"

	"github.com/safe-waters/docker-lock/generate"
	"github.com/safe-waters/docker-lock/generate/update"
)

func TestImageDigestUpdater(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name                          string
		DockerfileImageDigestUpdater  *update.DockerfileImageDigestUpdater
		ComposefileImageDigestUpdater *update.ComposefileImageDigestUpdater
	}{
		{
			Name: "All Nil",
		},
		{
			Name:                          "Nil DockerfileImageDigestUpdater",
			ComposefileImageDigestUpdater: &update.ComposefileImageDigestUpdater{}, // nolint: lll
		},
		{
			Name:                         "Nil ComposefileImageDigestUpdater",
			DockerfileImageDigestUpdater: &update.DockerfileImageDigestUpdater{}, // nolint: lll
		},
		{
			Name:                          "Non Nil",
			DockerfileImageDigestUpdater:  &update.DockerfileImageDigestUpdater{},  // nolint: lll
			ComposefileImageDigestUpdater: &update.ComposefileImageDigestUpdater{}, // nolint: lll
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			updater := &generate.ImageDigestUpdater{
				DockerfileImageDigestUpdater:  test.DockerfileImageDigestUpdater,  // nolint: lll
				ComposefileImageDigestUpdater: test.ComposefileImageDigestUpdater, // nolint: lll
			}

			done := make(chan struct{})
			defer close(done)

			assertConcreteImageDigestUpdater(t, updater, done)
		})
	}
}
