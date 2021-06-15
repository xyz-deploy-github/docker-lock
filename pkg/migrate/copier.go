package migrate

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/safe-waters/docker-lock/pkg/generate/parse"
)

type copier struct {
	prefix string
}

// NewCopier returns an ICopier.
func NewCopier(prefix string) ICopier {
	return &copier{prefix: prefix}
}

// Copy copies an image to another repository. It prepends the last
// part of an image's path with the prefix.
//
// For instance:
// Given an image such as docker.io/library/ubuntu:bionic@sha256:122...
// and a prefix of `myrepo`, Copy will push the exact same contents of the
// image to myrepo/ubuntu:bionic@sha256:122.
func (c *copier) Copy(image parse.IImage) error {
	if image == nil || reflect.ValueOf(image).IsNil() {
		return errors.New("cannot copy nil image")
	}

	src := image.ImageLine()
	dst := c.imageLineWithoutHostPrefix(src)

	if err := crane.Copy(src, dst); err != nil {
		return fmt.Errorf("unable to copy '%s' to '%s'", src, dst)
	}

	return nil
}

func (c *copier) imageLineWithoutHostPrefix(imageLine string) string {
	fields := strings.Split(imageLine, "/")
	return fmt.Sprintf("%s/%s", c.prefix, fields[len(fields)-1])
}
