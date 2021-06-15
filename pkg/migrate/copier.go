package migrate

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
)

type copier struct {
	prefix string
}

// NewCopier returns an ICopier.
func NewCopier(prefix string) ICopier {
	return &copier{prefix: prefix}
}

// Copy copies an image to another repository. It prepends the last
// part of an image line with the prefix.
//
// For instance:
// Given an image line such as docker.io/library/ubuntu:bionic@sha256:122...
// and a prefix of `myrepo`, Copy will push the exact same contents of the
// image Line to myrepo/ubuntu:bionic@sha256:122.
func (c *copier) Copy(imageLine string) error {
	if imageLine == "" {
		return errors.New("cannot copy an empty imageLine")
	}

	src := imageLine
	dst := c.imageLineWithoutHostPrefix(src)

	if err := crane.Copy(src, dst); err != nil {
		return fmt.Errorf("unable to copy '%s' to '%s': '%v'", src, dst, err)
	}

	return nil
}

func (c *copier) imageLineWithoutHostPrefix(imageLine string) string {
	fields := strings.Split(imageLine, "/")
	return fmt.Sprintf("%s/%s", c.prefix, fields[len(fields)-1])
}
