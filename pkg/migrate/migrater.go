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

type migrater struct {
	copier ICopier
}

// NewMigrater returns an IMigrater after ensuring copier is not nil.
func NewMigrater(copier ICopier) (IMigrater, error) {
	if copier == nil || reflect.ValueOf(copier).IsNil() {
		return nil, errors.New("'copier' cannot be nil")
	}

	return &migrater{copier: copier}, nil
}

// Migrate copies all images referenced in a lockfile to another
// registry.
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

		imageLineCache := map[string]struct{}{}

		for kind := range lockfile {
			for _, images := range lockfile[kind] {
				for _, image := range images {
					parsedImage, err := m.parseImageFromLockfile(image)
					if err != nil {
						select {
						case errCh <- err:
						case <-doneCh:
						}

						return
					}

					if parsedImage.Name() == "scratch" {
						continue
					}

					imageLine := parsedImage.ImageLine()

					if _, ok := imageLineCache[imageLine]; ok {
						continue
					}

					imageLineCache[imageLine] = struct{}{}

					waitGroup.Add(1)

					go func() {
						defer waitGroup.Done()

						if err := m.copier.Copy(imageLine); err != nil {
							select {
							case errCh <- err:
							case <-doneCh:
							}

							return
						}
					}()
				}
			}
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

func (m *migrater) parseImageFromLockfile(
	lockfileImage interface{},
) (parse.IImage, error) {
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
