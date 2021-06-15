package migrate_test

import (
	"bytes"
	"sort"
	"sync"
	"testing"

	"github.com/safe-waters/docker-lock/pkg/generate/parse"
	"github.com/safe-waters/docker-lock/pkg/migrate"
)

type mockCopier struct {
	prefix     string
	imageLines []string
	mu         *sync.Mutex
}

func (m *mockCopier) Copy(image parse.IImage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.imageLines = append(m.imageLines, image.ImageLine())
	return nil
}

var lockfile = `{
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
}`

func TestMigrate(t *testing.T) {
	t.Parallel()

	prefix := "myrepo"
	copier := &mockCopier{
		prefix:     prefix,
		imageLines: []string{},
		mu:         &sync.Mutex{},
	}
	migrater, err := migrate.NewMigrater(copier)
	if err != nil {
		t.Fatal(err)
	}

	lockfileReader := bytes.NewReader([]byte(lockfile))

	if err := migrater.Migrate(lockfileReader); err != nil {
		t.Fatal(err)
	}

	expected := []string{
		"ubuntu:bionic@sha256:122f",
		"alpine:latest@sha256:826f",
		"alpine:latest@sha256:826f",
	}

	if len(copier.imageLines) != len(expected) {
		t.Fatalf(
			"expected '%d' imageLines, got '%d'",
			len(expected),
			len(copier.imageLines),
		)
	}

	sort.Strings(copier.imageLines)
	sort.Strings(expected)

	for i := range expected {
		if expected[i] != copier.imageLines[i] {
			t.Fatalf(
				"expected '%s', got '%s'", expected[i], copier.imageLines[i],
			)
		}
	}
}
