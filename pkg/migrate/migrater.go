package migrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"

	"github.com/safe-waters/docker-lock/pkg/generate/parse"
	"github.com/safe-waters/docker-lock/pkg/kind"
)

type IMigrater interface {
	Migrate(lockfileReader io.Reader) error
}

type migrater struct {
	copier ICopier
}

func NewMigrater(copier ICopier) (IMigrater, error) {
	if copier == nil || reflect.ValueOf(copier).IsNil() {
		return nil, errors.New("'copier' cannot be nil")
	}

	return &migrater{copier: copier}, nil
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

								parsedImage, err := m.parseImageFromLockfile(
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

								if err := m.copier.Copy(
									parsedImage,
								); err != nil {
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

func (m *migrater) parseImageFromLockfile(lockfileImage interface{}) (parse.IImage, error) {
	image, ok := lockfileImage.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("malformed image '%v'", lockfileImage)
	}

	name, ok := image["name"].(string)
	if !ok {
		return nil, fmt.Errorf("malformed name in image '%v'", image)
	}

	tag, ok := image["tag"].(string)
	if !ok {
		return nil, fmt.Errorf("malformed tag in image '%v'", image)
	}

	digest, ok := image["digest"].(string)
	if !ok {
		return nil, fmt.Errorf("malformed digest in image '%v'", image)
	}

	return parse.NewImage("", name, tag, digest, nil, nil), nil
}
