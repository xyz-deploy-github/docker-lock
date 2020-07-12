package generate

import (
	"fmt"
	"log"
	"sync"

	"github.com/michaelperel/docker-lock/registry"
)

// updater updates base images' digests to their most recent digests
// according to the appropriate container registry.
type updater struct{}

// digestUpdate is used to update an Image's digest in a
// concurrently safe manner.
type digestUpdate struct {
	image  *Image
	digest string
	err    error
}

// updateDigest updates base images' digests to their most recent digests
// according to the container registries selected by the wrapper manager.
// If there are base images with duplicate names and tags, the registry
// will only be queried once. If the base image already has a digest,
// the registry will not be queried.
func (u *updater) updateDigest(
	wm *registry.WrapperManager,
	bImCh <-chan *BaseImage,
	doneCh <-chan struct{},
) (map[string][]*DockerfileImage, map[string][]*ComposefileImage, error) {
	dIms := map[string][]*DockerfileImage{}
	cIms := map[string][]*ComposefileImage{}

	// Each Image saves a pointer to an Image with the same values in its
	// fields. This saved *Image is used to update the digests of all the
	// Images that have the same values.
	// For instance, if the line 'python:3.6' appears multiple times, we only
	// want to query the registry one time. When the registry is queried,
	// the digest field pointed to by all Images with that line will be
	// updated.
	uniqBIms := map[Image]*Image{}

	// A channel whose values are consumed in a single goroutine so that
	// all digest updates are concurrently safe.
	digUpCh := make(chan digestUpdate)

	wg := sync.WaitGroup{}

	for b := range bImCh {
		if b.err != nil {
			return nil, nil, b.err
		}

		log.Printf("Found '%+v'.", b)

		// Only query the registry once per image.
		if uniqBIms[*b.Image] == nil {
			uniqBIms[*b.Image] = b.Image

			// Only query the registry if the digest is empty.
			if b.Image.Digest == "" {
				wg.Add(1)

				go u.queryContainerRegisty(b, wm, digUpCh, doneCh, &wg)
			}
		}

		// DockerfileImages and ComposefileImages' Image fields are set
		// to *Images whose digests will be updated with the result
		// from querying the container registries, if the digests are not
		// already present.
		switch im := uniqBIms[*b.Image]; b.composefilePath {
		case "":
			dIm := &DockerfileImage{Image: im, position: b.position}
			dIms[b.dockerfilePath] = append(dIms[b.dockerfilePath], dIm)
		default:
			cIm := &ComposefileImage{
				Image:          im,
				ServiceName:    b.serviceName,
				DockerfilePath: b.dockerfilePath,
				position:       b.position,
			}
			cIms[b.composefilePath] = append(cIms[b.composefilePath], cIm)
		}
	}

	go func() {
		wg.Wait()
		close(digUpCh)
	}()

	// Update the digests of *Images in the DockerfileImages and
	// ComposefileImages' Image fields in a concurrent safe manner.
	for up := range digUpCh {
		if up.err != nil {
			return nil, nil, up.err
		}

		up.image.Digest = up.digest
	}

	return dIms, cIms, nil
}

// queryContainerRegistry queries the appropriate container registry
// for the most recent digest based off of the wrapper manager.
func (u *updater) queryContainerRegisty(
	bIm *BaseImage,
	wm *registry.WrapperManager,
	digUpCh chan<- digestUpdate,
	doneCh <-chan struct{},
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	log.Printf("Querying '%+v' for a digest.", bIm)

	w := wm.Wrapper(bIm.Image.Name)

	d, err := w.Digest(bIm.Image.Name, bIm.Image.Tag)
	if err != nil {
		log.Printf("Unable to find digest for '%+v'.", bIm)

		errMsg := ""
		if bIm.dockerfilePath != "" {
			errMsg = fmt.Sprintf("from '%s': ", bIm.dockerfilePath)
		}

		if bIm.composefilePath != "" {
			errMsg = fmt.Sprintf(
				"%sfrom '%s': from service '%s': ", errMsg, bIm.composefilePath,
				bIm.serviceName,
			)
		}

		err = fmt.Errorf("%s%v", errMsg, err)

		select {
		case <-doneCh:
		case digUpCh <- digestUpdate{err: err}:
		}

		return
	}

	log.Printf("Found digest '%s' for '%+v'.", d, bIm)

	select {
	case <-doneCh:
	case digUpCh <- digestUpdate{image: bIm.Image, digest: d}:
	}
}
