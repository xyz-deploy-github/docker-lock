package migrate

import (
	"io"

	"github.com/safe-waters/docker-lock/pkg/generate/parse"
)

// IMigrater provides an interface for migrating images in a Lockfile
// from one repository to others.
type IMigrater interface {
	Migrate(lockfileReader io.Reader) error
}

// ICopier provides an interface for copying an image from one repository
// to others.
type ICopier interface {
	Copy(image parse.IImage, done <-chan struct{}) error
}
