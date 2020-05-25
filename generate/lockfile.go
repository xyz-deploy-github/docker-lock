package generate

import (
	"encoding/json"
	"io"
	"log"
	"sort"
	"sync"
)

// Lockfile contains DockerfileImages and ComposefileImages identified by
// their filepaths.
type Lockfile struct {
	DockerfileImages  map[string][]*DockerfileImage  `json:"dockerfiles"`
	ComposefileImages map[string][]*ComposefileImage `json:"composefiles"`
}

// NewLockfile creates a Lockfile, first sorting DockerfileImages
// and ComposefileImages.
func NewLockfile(
	dIms map[string][]*DockerfileImage,
	cIms map[string][]*ComposefileImage,
) *Lockfile {
	log.Printf("Creating Lockfile from Dockerfile Images '%+v' "+
		"and Composefile Images '%+v'.", dIms, cIms,
	)

	l := &Lockfile{DockerfileImages: dIms, ComposefileImages: cIms}
	l.sortImages()

	log.Printf("Sorted images to make Lockfile '%+v'.", l)

	return l
}

// sortImages sorts DockerfileImages and ComposefileImages.
func (l *Lockfile) sortImages() {
	wg := sync.WaitGroup{}

	wg.Add(1)

	go l.sortDockerfileImages(&wg)

	wg.Add(1)

	go l.sortComposefileImages(&wg)

	wg.Wait()
}

// sortDockerfileImages sorts by position.
func (l *Lockfile) sortDockerfileImages(wg *sync.WaitGroup) {
	defer wg.Done()

	for _, ims := range l.DockerfileImages {
		wg.Add(1)

		go func(ims []*DockerfileImage) {
			defer wg.Done()

			sort.Slice(ims, func(i, j int) bool {
				return ims[i].position < ims[j].position
			})
		}(ims)
	}
}

// sortComposefileImages sorts by ServiceName, DockerfilePath, and position,
// in that order.
func (l *Lockfile) sortComposefileImages(wg *sync.WaitGroup) {
	defer wg.Done()

	for _, ims := range l.ComposefileImages {
		wg.Add(1)

		go func(ims []*ComposefileImage) {
			defer wg.Done()

			sort.Slice(ims, func(i, j int) bool {
				switch {
				case ims[i].ServiceName != ims[j].ServiceName:
					return ims[i].ServiceName < ims[j].ServiceName
				case ims[i].DockerfilePath != ims[j].DockerfilePath:
					return ims[i].DockerfilePath < ims[j].DockerfilePath
				default:
					return ims[i].position < ims[j].position
				}
			})
		}(ims)
	}
}

// Write writes a Lockfile to an io.Writer, formatted as indented json.
func (l *Lockfile) Write(w io.Writer) error {
	lByt, err := json.MarshalIndent(l, "", "\t")
	if err != nil {
		return err
	}

	log.Printf("Writing Lockfile to '%+v'", w)

	if _, err := w.Write(lByt); err != nil {
		return err
	}

	return nil
}
