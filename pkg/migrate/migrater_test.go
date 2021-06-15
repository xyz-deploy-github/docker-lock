package migrate_test

import (
	"bytes"
	"sort"
	"sync"
	"testing"

	"github.com/safe-waters/docker-lock/pkg/migrate"
)

type mockCopier struct {
	prefixes   []string
	imageLines []string
	mu         *sync.Mutex
}

func (m *mockCopier) Copy(imageLine string, done <-chan struct{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.imageLines = append(m.imageLines, imageLine)

	return nil
}

func TestMigrate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		lockfile []byte
		expected []string
	}{
		{
			name: "duplicates",
			lockfile: []byte(`{
	"dockerfiles": {
		".devcontainer/Dockerfile": [
			{
				"name": "ubuntu",
				"tag": "bionic",
				"digest": "122f"
			}
		],
		"Dockerfile.alpine": [
			{
				"name": "alpine",
				"tag": "latest",
				"digest": "826f"
			}
		],
		"Dockerfile.scratch": [
			{
				"name": "alpine",
				"tag": "latest",
				"digest": "826f"
			},
			{
				"name": "scratch",
				"tag": "",
				"digest": ""
			}
		]
	}
}`),
			expected: []string{
				"ubuntu:bionic@sha256:122f", "alpine:latest@sha256:826f",
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			prefixes := []string{"myrepo"}
			copier := &mockCopier{
				prefixes:   prefixes,
				imageLines: []string{},
				mu:         &sync.Mutex{},
			}

			migrater, err := migrate.NewMigrater(copier)
			if err != nil {
				t.Fatal(err)
			}

			lockfileReader := bytes.NewReader(test.lockfile)

			if err := migrater.Migrate(lockfileReader); err != nil {
				t.Fatal(err)
			}

			if len(test.expected) != len(copier.imageLines) {
				t.Fatalf(
					"expected '%d' imageLines, got '%d'",
					len(test.expected),
					len(copier.imageLines),
				)
			}

			sort.Strings(copier.imageLines)
			sort.Strings(test.expected)

			for i := range test.expected {
				if test.expected[i] != copier.imageLines[i] {
					t.Fatalf(
						"expected '%s', got '%s'",
						test.expected[i], copier.imageLines[i],
					)
				}
			}
		})
	}
}
