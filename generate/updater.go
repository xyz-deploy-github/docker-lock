package generate

import (
	"github.com/safe-waters/docker-lock/generate/parse"
	"github.com/safe-waters/docker-lock/generate/update"
)

// ImageDigestUpdater contains ImageDigestUpdaters for
// DockerfileImages and ComposefileImages.
type ImageDigestUpdater struct {
	DockerImageUpdater  *update.DockerfileImageDigestUpdater
	ComposeImageUpdater *update.ComposefileImageDigestUpdater
}

// IImageDigestUpdater provides an interface for ImageDigestUpdater's exported
// methods, which are used by Generator.
type IImageDigestUpdater interface {
	UpdateDigests(
		dockerfileImages <-chan *parse.DockerfileImage,
		composefileImages <-chan *parse.ComposefileImage,
		done <-chan struct{},
	) (
		updatedDockerfileImages <-chan *parse.DockerfileImage,
		updatedComposefileImages <-chan *parse.ComposefileImage,
	)
}

// UpdateDigests updates digests for DockerfileImages and ComposefileImages.
func (i *ImageDigestUpdater) UpdateDigests(
	dockerfileImages <-chan *parse.DockerfileImage,
	composefileImages <-chan *parse.ComposefileImage,
	done <-chan struct{},
) (
	updatedDockerfileImages <-chan *parse.DockerfileImage,
	updatedComposefileImages <-chan *parse.ComposefileImage,
) {
	if i.DockerImageUpdater != nil {
		updatedDockerfileImages = i.DockerImageUpdater.UpdateDigests(
			dockerfileImages, done,
		)
	}

	if i.ComposeImageUpdater != nil {
		updatedComposefileImages = i.ComposeImageUpdater.UpdateDigests(
			composefileImages, done,
		)
	}

	return
}
