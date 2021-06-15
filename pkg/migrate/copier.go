package migrate

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/safe-waters/docker-lock/pkg/generate/parse"
)

type ICopier interface {
	Copy(image parse.IImage) error
}

type copier struct {
	prefix string
}

func NewCopier(prefix string) ICopier {
	return &copier{prefix: prefix}
}

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
