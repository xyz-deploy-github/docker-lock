package generate

import (
	"encoding/json"
	"io"
	"path/filepath"

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
// FROM busybox:latest@sha256:dd97a3f...
// could be represented as:
// Image{Name: busybox, Tag: latest, Digest: dd97a3f...}.
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

type digestResponse struct {
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
	return &Generator{
		DockerfilePaths:        dPaths,
		ComposefilePaths:       cPaths,
		DockerfileEnvBuildArgs: dArgs,
		OutPath:                oPath,
	}, nil
}

// GenerateLockfile creates a Lockfile and writes its bytes to an io.Writer.
func (g *Generator) GenerateLockfile(
	wm *registry.WrapperManager,
	w io.Writer,
) error {
	var (
		doneCh = make(chan struct{})
		pilCh  = g.parseFiles(doneCh)
	)
	dIms, cIms, err := g.convertParsedImageLines(wm, pilCh, doneCh)
	if err != nil {
		close(doneCh)
		return err
	}
	lFile := NewLockfile(dIms, cIms)
	return lFile.Write(w)
}

func (g *Generator) convertParsedImageLines(
	wm *registry.WrapperManager,
	pilCh <-chan *parsedImageLine,
	doneCh <-chan struct{},
) (map[string][]*DockerfileImage, map[string][]*ComposefileImage, error) {
	var (
		uniqLines  = map[string]bool{}
		lineToPils = map[string][]*parsedImageLine{}
		dResCh     = make(chan *digestResponse)
	)
	for pil := range pilCh {
		if pil.err != nil {
			return nil, nil, pil.err
		}
		lineToPils[pil.line] = append(lineToPils[pil.line], pil)
		if !uniqLines[pil.line] {
			uniqLines[pil.line] = true
			go g.getDigest(pil, wm, dResCh, doneCh)
		}
	}
	var (
		dIms = map[string][]*DockerfileImage{}
		cIms = map[string][]*ComposefileImage{}
	)
	for i := 0; i < len(uniqLines); i++ {
		res := <-dResCh
		if res.err != nil {
			return nil, nil, res.err
		}
		for _, pil := range lineToPils[res.line] {
			if pil.cPath == "" {
				dIm := &DockerfileImage{Image: res.im, pos: pil.pos}
				dPath := pil.dPath
				dIms[dPath] = append(dIms[dPath], dIm)
			} else {
				cIm := &ComposefileImage{
					Image:          res.im,
					ServiceName:    pil.svcName,
					DockerfilePath: pil.dPath,
					pos:            pil.pos,
				}
				cPath := pil.cPath
				cIms[cPath] = append(cIms[cPath], cIm)
			}
		}
	}
	close(dResCh)
	return dIms, cIms, nil
}

func (g *Generator) getDigest(
	pil *parsedImageLine,
	wm *registry.WrapperManager,
	qResCh chan<- *digestResponse,
	doneCh <-chan struct{},
) {
	var (
		tagSeparator    = -1
		digestSeparator = -1
	)
	for i, c := range pil.line {
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
	switch {
	case tagSeparator != -1 && digestSeparator != -1:
		// ubuntu:18.04@sha256:9b1702...
		name = pil.line[:tagSeparator]
		tag = pil.line[tagSeparator+1 : digestSeparator]
		digest = pil.line[digestSeparator+1+len("sha256:"):]
	case tagSeparator != -1 && digestSeparator == -1:
		// ubuntu:18.04
		name = pil.line[:tagSeparator]
		tag = pil.line[tagSeparator+1:]
	case tagSeparator == -1 && digestSeparator != -1:
		// ubuntu@sha256:9b1702...
		name = pil.line[:digestSeparator]
		digest = pil.line[digestSeparator+1+len("sha256:"):]
	default:
		// ubuntu
		name = pil.line
		tag = "latest"
	}
	if digest == "" {
		wrapper := wm.GetWrapper(name)
		var err error
		digest, err = wrapper.GetDigest(name, tag)
		if err != nil {
			select {
			case <-doneCh:
			case qResCh <- &digestResponse{err: err}:
			}
			return
		}
	}
	select {
	case <-doneCh:
	case qResCh <- &digestResponse{
		im: &Image{
			Name:   name,
			Tag:    tag,
			Digest: digest,
		},
		line: pil.line,
	}:
	}
}
