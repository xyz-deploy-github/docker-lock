package generate_test

import (
	"testing"

	"github.com/safe-waters/docker-lock/generate"
	"github.com/safe-waters/docker-lock/generate/collect"
)

func TestPathCollector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name                 string
		DockerfileCollector  *collect.PathCollector
		ComposefileCollector *collect.PathCollector
	}{
		{
			Name: "All Nil",
		},
		{
			Name:                 "Nil DockerfileCollector",
			ComposefileCollector: &collect.PathCollector{},
		},
		{
			Name:                "Nil ComposefileCollector",
			DockerfileCollector: &collect.PathCollector{},
		},
		{
			Name:                 "Non Nil",
			DockerfileCollector:  &collect.PathCollector{},
			ComposefileCollector: &collect.PathCollector{},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			pathCollector := &generate.PathCollector{
				DockerfileCollector:  test.DockerfileCollector,
				ComposefileCollector: test.ComposefileCollector,
			}

			done := make(chan struct{})
			defer close(done)

			assertConcretePathCollector(t, pathCollector, done)
		})
	}
}
