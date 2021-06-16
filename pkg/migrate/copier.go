package migrate

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/safe-waters/docker-lock/pkg/generate/parse"
)

type copier struct {
	downstreamPrefixes []string
}

// NewCopier returns an ICopier.
func NewCopier(downstreamPrefixes []string) ICopier {
	return &copier{downstreamPrefixes: downstreamPrefixes}
}

// Copy copies an image to other repositories defined by downstream prefixes.
// For each downstream prefix, it prepends the last part of an image line
// with the prefix.
//
// For instance:
// Given an image line such as docker.io/library/ubuntu:bionic@sha256:122...
// and a prefix of `myrepo`, Copy will push the exact same contents of the
// image Line to myrepo/ubuntu:bionic@sha256:122.
func (c *copier) Copy(image parse.IImage, done <-chan struct{}) error {
	if image == nil || reflect.ValueOf(image).IsNil() {
		return errors.New("'image' cannot be nil")
	}

	var (
		waitGroup sync.WaitGroup
		errCh     = make(chan error)
	)

	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()

		for _, prefix := range c.downstreamPrefixes {
			prefix := prefix

			waitGroup.Add(1)

			go func() {
				defer waitGroup.Done()

				var (
					src = image.ImageLine()
					dst = c.imageLineWithoutHostPrefix(prefix, src)
				)

				if err := crane.Copy(src, dst); err != nil {
					select {
					case errCh <- fmt.Errorf(
						"unable to copy '%s' to '%s': err: '%v'", src, dst, err,
					):
					case <-done:
					}

					return
				}
			}()
		}
	}()

	go func() {
		waitGroup.Wait()
		close(errCh)
	}()

	for err := range errCh {
		return err
	}

	return nil
}

func (c *copier) imageLineWithoutHostPrefix(
	downstreamPrefix string, imageLine string,
) string {
	var (
		fields   = strings.Split(imageLine, "/")
		lastPart = fields[len(fields)-1]
	)

	if i := strings.Index(lastPart, "@"); i != -1 {
		lastPart = lastPart[:i]
	}

	return fmt.Sprintf("%s/%s", downstreamPrefix, lastPart)
}
