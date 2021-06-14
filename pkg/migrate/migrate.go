package migrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/safe-waters/docker-lock/pkg/generate/parse"
	"github.com/safe-waters/docker-lock/pkg/kind"
)

type IMigrater interface {
	Migrate(lockfileReader io.Reader) error
}

type migrater struct {
	prefix string
}

func NewMigrater(prefix string) IMigrater {
	return &migrater{prefix: prefix}
}

func (m *migrater) Migrate(lockfileReader io.Reader) error {
	if lockfileReader == nil || reflect.ValueOf(lockfileReader).IsNil() {
		return errors.New("'lockfileReader' cannot be nil")
	}

	var lockfile map[kind.Kind]map[string][]interface{}
	if err := json.NewDecoder(lockfileReader).Decode(&lockfile); err != nil {
		return err
	}

	var (
		waitGroup sync.WaitGroup
		errCh     = make(chan error)
		doneCh    = make(chan struct{})
	)

	defer close(doneCh)

	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()

		for kind := range lockfile {
			kind := kind

			waitGroup.Add(1)

			go func() {
				defer waitGroup.Done()

				for _, images := range lockfile[kind] {
					images := images

					waitGroup.Add(1)

					go func() {
						defer waitGroup.Done()

						for _, image := range images {
							image := image

							waitGroup.Add(1)

							go func() {
								defer waitGroup.Done()

								parsedImage, err := parseImageFromLockfile(
									image,
								)
								if err != nil {
									select {
									case errCh <- err:
									case <-doneCh:
									}

									return
								}

								if parsedImage.Name() == "scratch" {
									return
								}

								src := parsedImage.ImageLine()
								dst := imageLineWithoutHostPrefix(
									parsedImage.ImageLine(), m.prefix,
								)

								if err := crane.Copy(src, dst); err != nil {
									err = fmt.Errorf(
										"unable to copy '%s' to '%s'", src, dst,
									)

									select {
									case errCh <- err:
									case <-doneCh:
									}

									return
								}
							}()
						}
					}()
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

func parseImageFromLockfile(lockfileImage interface{}) (parse.IImage, error) {
	image, ok := lockfileImage.(map[string]interface{})
	if !ok {
		return nil, errors.New("malformed image")
	}

	name, ok := image["name"].(string)
	if !ok {
		return nil, fmt.Errorf("malformed name '%s'", name)
	}

	tag, ok := image["tag"].(string)
	if !ok {
		return nil, fmt.Errorf("malformed tag '%s'", tag)
	}

	digest, ok := image["digest"].(string)
	if !ok {
		return nil, fmt.Errorf("malformed digest '%s'", digest)
	}

	return parse.NewImage("", name, tag, digest, nil, nil), nil
}

func imageLineWithoutHostPrefix(imageLine string, prefix string) string {
	fields := strings.Split(imageLine, "/")
	return fmt.Sprintf("%s/%s", prefix, fields[len(fields)-1])
}
