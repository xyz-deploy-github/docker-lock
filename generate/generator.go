package generate

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"sync"

	"github.com/michaelperel/docker-lock/registry"
	"github.com/spf13/cobra"
)

// Generator generates Lockfiles. DockerfileEnvBuildArgs determines whether
// environment variables should be used as build args in Dockerfiles.
type Generator struct {
	DockerfilePaths        []string
	ComposefilePaths       []string
	DockerfileEnvBuildArgs bool
	OutPath                string
}

// Image contains information extracted from 'FROM' instructions in Dockerfiles
// or 'image:' keys in docker-compose files. For instance,
// FROM busybox:latest@sha256:dd97a3fe6d721c5cf03abac0f50e2848dc583f7c4e41bf39102ceb42edfd1808
// could be represented as:
// Image{Name: busybox, Tag: latest, Digest: dd97a3fe6d721c5cf03abac0f50e2848dc583f7c4e41bf39102ceb42edfd1808}.
type Image struct {
	Name   string `json:"name"`
	Tag    string `json:"tag"`
	Digest string `json:"digest"`
}

// DockerfileImage contains an Image along with metadata used to generate
// a Lockfile.
type DockerfileImage struct {
	*Image
	pos int
}

// ComposefileImage contains an Image along with metadata used to generate
// a Lockfile.
type ComposefileImage struct {
	*Image
	ServiceName    string `json:"serviceName"`
	DockerfilePath string `json:"dockerfile"`
	pos            int
}

// Lockfile contains DockerfileImages and ComposefileImages identified by
// their filepaths. This data structure can be written to disk as the
// output Lockfile.
type Lockfile struct {
	DockerfileImages  map[string][]*DockerfileImage  `json:"dockerfiles"`
	ComposefileImages map[string][]*ComposefileImage `json:"composefiles"`
}

type queryResponse struct {
	im   *Image
	line string
	err  error
}

// String formats an Image as indented json.
func (i *Image) String() string {
	pretty, _ := json.MarshalIndent(i, "", "\t")
	return string(pretty)
}

// NewGenerator creates a Generator from command line flags. If no
// Dockerfiles or docker-compose files are specified as flags,
// files that match "Dockerfile", "docker-compose.yml", and
// "docker-compose.yaml" will be used. If files are specified in
// command line flags, only those files will be used.
func NewGenerator(cmd *cobra.Command) (*Generator, error) {
	oPath, err := cmd.Flags().GetString("outpath")
	if err != nil {
		return nil, err
	}
	oPath = filepath.ToSlash(oPath)
	dArgs, err := cmd.Flags().GetBool("dockerfile-env-build-args")
	if err != nil {
		return nil, err
	}
	dPaths, cPaths, err := collectPaths(cmd)
	if err != nil {
		return nil, err
	}
	return &Generator{DockerfilePaths: dPaths,
		ComposefilePaths:       cPaths,
		DockerfileEnvBuildArgs: dArgs,
		OutPath:                oPath}, nil
}

// GenerateLockfile writes a Lockfile's bytes to an io.Writer.
func (g *Generator) GenerateLockfile(wm *registry.WrapperManager, w io.Writer) error {
	var (
		doneCh = make(chan struct{})
		ilCh   = g.parseFiles(doneCh)
	)
	dIms, cIms, err := g.queryRegistries(wm, ilCh, doneCh)
	if err != nil {
		close(doneCh)
		return err
	}
	g.sortImages(dIms, cIms)
	lFile := &Lockfile{DockerfileImages: dIms, ComposefileImages: cIms}
	lFileByt, err := g.formatLockfile(lFile)
	if err != nil {
		return err
	}
	w.Write(lFileByt)
	return nil
}

func (g *Generator) queryRegistries(wm *registry.WrapperManager,
	ilCh <-chan *imageLine,
	doneCh <-chan struct{},
) (map[string][]*DockerfileImage, map[string][]*ComposefileImage, error) {

	var (
		ilReqs  = map[string]bool{}
		allIls  = map[string][]*imageLine{}
		qResCh  = make(chan *queryResponse)
		numReqs int
	)
	for il := range ilCh {
		if il.err != nil {
			return nil, nil, il.err
		}
		allIls[il.line] = append(allIls[il.line], il)
		if !ilReqs[il.line] {
			ilReqs[il.line] = true
			numReqs++
			go g.queryRegistry(il, wm, qResCh, doneCh)
		}
	}
	var (
		dIms = map[string][]*DockerfileImage{}
		cIms = map[string][]*ComposefileImage{}
	)
	for i := 0; i < numReqs; i++ {
		var res *queryResponse
		select {
		case <-doneCh:
			return nil, nil, fmt.Errorf("goroutine cancelled")
		case res = <-qResCh:
		}
		if res.err != nil {
			return nil, nil, res.err
		}
		for _, il := range allIls[res.line] {
			if il.cPath == "" {
				dIm := &DockerfileImage{Image: res.im, pos: il.pos}
				dPath := il.dPath
				dIms[dPath] = append(dIms[dPath], dIm)
			} else {
				cIm := &ComposefileImage{Image: res.im,
					ServiceName:    il.svcName,
					DockerfilePath: il.dPath,
					pos:            il.pos}
				cPath := il.cPath
				cIms[cPath] = append(cIms[cPath], cIm)
			}
		}
	}
	close(qResCh)
	return dIms, cIms, nil
}

func (g *Generator) queryRegistry(il *imageLine,
	wm *registry.WrapperManager,
	qResCh chan<- *queryResponse,
	doneCh <-chan struct{}) {

	var (
		tagSeparator    = -1
		digestSeparator = -1
	)
	for i, c := range il.line {
		if c == ':' {
			tagSeparator = i
		}
		if c == '@' {
			digestSeparator = i
			break
		}
	}
	var name, tag, digest string
	// 4 valid cases
	if tagSeparator != -1 && digestSeparator != -1 {
		// ubuntu:18.04@sha256:9b1702dcfe32c873a770a32cfd306dd7fc1c4fd134adfb783db68defc8894b3c
		name = il.line[:tagSeparator]
		tag = il.line[tagSeparator+1 : digestSeparator]
		digest = il.line[digestSeparator+1+len("sha256:"):]
	} else if tagSeparator != -1 && digestSeparator == -1 {
		// ubuntu:18.04
		name = il.line[:tagSeparator]
		tag = il.line[tagSeparator+1:]
	} else if tagSeparator == -1 && digestSeparator != -1 {
		// ubuntu@sha256:9b1702dcfe32c873a770a32cfd306dd7fc1c4fd134adfb783db68defc8894b3c
		name = il.line[:digestSeparator]
		digest = il.line[digestSeparator+1+len("sha256:"):]
	} else {
		// ubuntu
		name = il.line
		tag = "latest"
	}
	if digest == "" {
		wrapper := wm.GetWrapper(name)
		var err error
		digest, err = wrapper.GetDigest(name, tag)
		if err != nil {
			select {
			case <-doneCh:
			case qResCh <- &queryResponse{err: err}:
			}
			return
		}
	}
	select {
	case <-doneCh:
	case qResCh <- &queryResponse{im: &Image{Name: name, Tag: tag, Digest: digest}, line: il.line}:
	}
}

func (g *Generator) sortImages(dIms map[string][]*DockerfileImage, cIms map[string][]*ComposefileImage) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sortDockerfileImages(dIms)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		sortComposefileImages(cIms)
	}()
	wg.Wait()
}

func sortDockerfileImages(dIms map[string][]*DockerfileImage) {
	var wg sync.WaitGroup
	for _, ims := range dIms {
		wg.Add(1)
		go func(ims []*DockerfileImage) {
			defer wg.Done()
			sort.Slice(ims, func(i, j int) bool {
				return ims[i].pos < ims[j].pos
			})
		}(ims)
	}
	wg.Wait()
}

func sortComposefileImages(cIms map[string][]*ComposefileImage) {
	var wg sync.WaitGroup
	for _, ims := range cIms {
		wg.Add(1)
		go func(ims []*ComposefileImage) {
			defer wg.Done()
			sort.Slice(ims, func(i, j int) bool {
				if ims[i].ServiceName != ims[j].ServiceName {
					return ims[i].ServiceName < ims[j].ServiceName
				} else if ims[i].DockerfilePath != ims[i].DockerfilePath {
					return ims[i].DockerfilePath < ims[j].DockerfilePath
				} else {
					return ims[i].pos < ims[j].pos
				}
			})
		}(ims)
	}
	wg.Wait()
}

func (g *Generator) formatLockfile(lFile *Lockfile) ([]byte, error) {
	lFileByt, err := json.MarshalIndent(lFile, "", "\t")
	if err != nil {
		return nil, err
	}
	return lFileByt, nil
}
