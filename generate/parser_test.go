package generate_test

import (
	"testing"

	"github.com/safe-waters/docker-lock/generate"
	"github.com/safe-waters/docker-lock/generate/parse"
)

func TestImageParser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name                   string
		DockerfileImageParser  *parse.DockerfileImageParser
		ComposefileImageParser *parse.ComposefileImageParser
	}{
		{
			Name: "All Nil",
		},
		{
			Name:                   "Nil DockerfileImageParser",
			ComposefileImageParser: &parse.ComposefileImageParser{},
		},
		{
			Name:                  "Nil ComposefileImageParser",
			DockerfileImageParser: &parse.DockerfileImageParser{},
		},
		{
			Name:                   "Non Nil",
			DockerfileImageParser:  &parse.DockerfileImageParser{},
			ComposefileImageParser: &parse.ComposefileImageParser{},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			imageParser := &generate.ImageParser{
				DockerfileImageParser:  test.DockerfileImageParser,
				ComposefileImageParser: test.ComposefileImageParser,
			}

			done := make(chan struct{})
			defer close(done)

			assertConcreteImageParser(t, imageParser, done)
		})
	}
}
