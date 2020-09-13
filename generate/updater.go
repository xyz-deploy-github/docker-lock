package generate

import (
	"errors"
	"sync"

	"github.com/safe-waters/docker-lock/registry"
)

// Updater replaces image digests with the most recent ones
// from their registries.
type Updater struct {
	WrapperManager *registry.WrapperManager
}

// IUpdater provides an interface for Updater's exported
// methods, which are used by Generator.
type IUpdater interface {
	UpdateDigests(
		dockerfileImages <-chan *DockerfileImage,
		composefileImages <-chan *ComposefileImage,
		done <-chan struct{},
	) (<-chan *DockerfileImage, <-chan *ComposefileImage)
}

type digestUpdate struct {
	image  *Image
	digest string
	err    error
}

// NewUpdater returns an Updater after validating its fields.
func NewUpdater(wrapperManger *registry.WrapperManager) (*Updater, error) {
	if wrapperManger == nil {
		return nil, errors.New("wrapperManager cannot be nil")
	}

	return &Updater{WrapperManager: wrapperManger}, nil
}

// UpdateDigests queries registries for digests of images that do not
// already specify their digests. It updates images with those
// digests.
func (u *Updater) UpdateDigests( // nolint: gocyclo
	dockerfileImages <-chan *DockerfileImage,
	composefileImages <-chan *ComposefileImage,
	done <-chan struct{},
) (<-chan *DockerfileImage, <-chan *ComposefileImage) {
	outputDockerfileImages := make(chan *DockerfileImage)
	outputComposefileImages := make(chan *ComposefileImage)
	waitGroup := &sync.WaitGroup{}

	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()

		dockerfileImagesCache := map[Image][]*DockerfileImage{}
		composefileImagesCache := map[Image][]*ComposefileImage{}

		imagesToUpdate := map[Image]*Image{}
		digestUpdateCh := make(chan digestUpdate)

		var updateWaitGroup sync.WaitGroup

		var numChannels int
		if dockerfileImages != nil {
			numChannels++
		}

		if composefileImages != nil {
			numChannels++
		}

		var numChannelsDrained int

		for {
			select {
			case dockerfileImage, ok := <-dockerfileImages:
				if !ok {
					dockerfileImages = nil
					numChannelsDrained++

					break
				}

				if dockerfileImage.Err != nil {
					select {
					case <-done:
					case outputDockerfileImages <- &DockerfileImage{
						Err: dockerfileImage.Err,
					}:
					}

					return
				}

				if imagesToUpdate[*dockerfileImage.Image] == nil {
					imagesToUpdate[*dockerfileImage.Image] = dockerfileImage.Image // nolint: lll

					if dockerfileImage.Image.Digest == "" {
						updateWaitGroup.Add(1)

						go u.queryRegistry(
							dockerfileImage.Image, digestUpdateCh,
							done, &updateWaitGroup,
						)
					}
				}

				dockerfileImage = &DockerfileImage{
					Image:    imagesToUpdate[*dockerfileImage.Image],
					Position: dockerfileImage.Position,
					Path:     dockerfileImage.Path,
				}

				dockerfileImagesCache[*dockerfileImage.Image] = append(
					dockerfileImagesCache[*dockerfileImage.Image],
					dockerfileImage,
				)
			case composefileImage, ok := <-composefileImages:
				if !ok {
					composefileImages = nil
					numChannelsDrained++

					break
				}

				if composefileImage.Err != nil {
					select {
					case <-done:
					case outputComposefileImages <- &ComposefileImage{
						Err: composefileImage.Err,
					}:
					}

					return
				}

				if imagesToUpdate[*composefileImage.Image] == nil {
					imagesToUpdate[*composefileImage.Image] = composefileImage.Image // nolint: lll

					if composefileImage.Image.Digest == "" {
						updateWaitGroup.Add(1)

						go u.queryRegistry(
							composefileImage.Image, digestUpdateCh,
							done, &updateWaitGroup,
						)
					}
				}

				composefileImage = &ComposefileImage{
					Image:          imagesToUpdate[*composefileImage.Image],
					DockerfilePath: composefileImage.DockerfilePath,
					Position:       composefileImage.Position,
					ServiceName:    composefileImage.ServiceName,
					Path:           composefileImage.Path,
				}

				composefileImagesCache[*composefileImage.Image] = append(
					composefileImagesCache[*composefileImage.Image],
					composefileImage,
				)
			}

			if numChannelsDrained >= numChannels {
				break
			}
		}

		go func() {
			updateWaitGroup.Wait()
			close(digestUpdateCh)
		}()

		for update := range digestUpdateCh {
			if update.err != nil {
				if _, ok := dockerfileImagesCache[*update.image]; ok {
					select {
					case <-done:
					case outputDockerfileImages <- &DockerfileImage{
						Err: update.err,
					}:
					}

					return
				}

				if _, ok := composefileImagesCache[*update.image]; ok {
					select {
					case <-done:
					case outputComposefileImages <- &ComposefileImage{
						Err: update.err,
					}:
					}

					return
				}
			}

			update.image.Digest = update.digest
		}

		var cacheWaitGroup sync.WaitGroup

		cacheWaitGroup.Add(1)

		go func() {
			defer cacheWaitGroup.Done()

			for _, dockerfileImages := range dockerfileImagesCache {
				for _, dockerfileImage := range dockerfileImages {
					select {
					case <-done:
					case outputDockerfileImages <- dockerfileImage:
					}
				}
			}
		}()

		cacheWaitGroup.Add(1)

		go func() {
			defer cacheWaitGroup.Done()

			for _, composefileImages := range composefileImagesCache {
				for _, composefileImage := range composefileImages {
					select {
					case <-done:
					case outputComposefileImages <- composefileImage:
					}
				}
			}
		}()

		cacheWaitGroup.Wait()
	}()

	go func() {
		waitGroup.Wait()
		close(outputDockerfileImages)
		close(outputComposefileImages)
	}()

	return outputDockerfileImages, outputComposefileImages
}

func (u *Updater) queryRegistry(
	image *Image,
	digestUpdateCh chan<- digestUpdate,
	done <-chan struct{},
	waitGroup *sync.WaitGroup,
) {
	defer waitGroup.Done()

	wrapper := u.WrapperManager.Wrapper(image.Name)

	digest, err := wrapper.Digest(image.Name, image.Tag)
	if err != nil {
		select {
		case <-done:
		case digestUpdateCh <- digestUpdate{err: err, image: image}:
		}

		return
	}

	select {
	case <-done:
	case digestUpdateCh <- digestUpdate{image: image, digest: digest}:
	}
}
