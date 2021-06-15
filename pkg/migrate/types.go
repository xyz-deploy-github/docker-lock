package migrate

import (
	"io"
)

// IMigrater provides an interface for migrating images in a Lockfile
// from one repository to another.
type IMigrater interface {
	Migrate(lockfileReader io.Reader) error
}

// ICopier provides an interface for copying an image from one repository
// to another.
type ICopier interface {
	Copy(imageLine string) error
}
