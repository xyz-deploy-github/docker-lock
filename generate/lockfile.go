package generate

import (
	"encoding/json"
	"io"
	"sort"
	"sync"
)

// Lockfile contains DockerfileImages and ComposefileImages identified by
// their filepaths.
type Lockfile struct {
	DockerfileImages  map[string][]*DockerfileImage  `json:"dockerfiles"`
	ComposefileImages map[string][]*ComposefileImage `json:"composefiles"`
}

// NewLockfile creates a Lockfile with sorted DockerfileImages
// and ComposefilesImages.
func NewLockfile(
	dIms map[string][]*DockerfileImage,
	cIms map[string][]*ComposefileImage,
) *Lockfile {
	l := &Lockfile{DockerfileImages: dIms, ComposefileImages: cIms}
	l.sortIms()

	return l
}

func (l *Lockfile) sortIms() {
	wg := sync.WaitGroup{}

	wg.Add(1)

	go l.sortDIms(&wg)

	wg.Add(1)

	go l.sortCIms(&wg)

	wg.Wait()
}

func (l *Lockfile) sortDIms(wg *sync.WaitGroup) {
	defer wg.Done()

	for _, ims := range l.DockerfileImages {
		wg.Add(1)

		go func(ims []*DockerfileImage) {
			defer wg.Done()

			sort.Slice(ims, func(i, j int) bool {
				return ims[i].pos < ims[j].pos
			})
		}(ims)
	}
}

func (l *Lockfile) sortCIms(wg *sync.WaitGroup) {
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
					return ims[i].pos < ims[j].pos
				}
			})
		}(ims)
	}
}

// Write writes a Lockfile, formatted as indented json, to an io.Writer.
func (l *Lockfile) Write(w io.Writer) error {
	lByt, err := json.MarshalIndent(l, "", "\t")
	if err != nil {
		return err
	}

	if _, err := w.Write(lByt); err != nil {
		return err
	}

	return nil
}
