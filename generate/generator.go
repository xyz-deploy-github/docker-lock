// Package generate provides functions to generate a Lockfile.
package generate

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/michaelperel/docker-lock/registry"
)

// Generator generates Lockfiles. DockerfileEnvBuildArgs determines whether
// environment variables should be used as build args in Dockerfiles.
type Generator struct {
	DockerfilePaths        []string
	ComposefilePaths       []string
	DockerfileEnvBuildArgs bool
	LockfileName           string
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

type registryResponse struct {
	im   *Image // Contains a non-empty digest
	line string // line that created the image, e.g. python:3.6@sha256:25a189...
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
func NewGenerator(flags *Flags) (*Generator, error) {
	dPaths, cPaths, err := collectDandCPaths(flags)
	if err != nil {
		return nil, err
	}

	return &Generator{
		DockerfilePaths:        dPaths,
		ComposefilePaths:       cPaths,
		DockerfileEnvBuildArgs: flags.DockerfileEnvBuildArgs,
		LockfileName:           flags.LockfileName,
	}, nil
}

// GenerateLockfile creates a Lockfile and writes its bytes to an io.Writer.
func (g *Generator) GenerateLockfile(
	wm *registry.WrapperManager,
	w io.Writer,
) error {
	doneCh := make(chan struct{})
	pilCh := g.parseFiles(doneCh)

	// Multiple parsedImageLines could contain the same line
	// For instance, 3 parsedImageLines' lines could be:
	// 		(1) ubuntu:latest
	//	 	(2) ubuntu
	// 		(3) ubuntu
	// As the 3 lines are the same, only query the registry once
	// for the digest.
	lineToImCh, lineToPils, err := g.getImsWithDigests(
		wm, pilCh, doneCh,
	)
	if err != nil {
		close(doneCh)
		return err
	}

	// Apply the registries' results to all parsedImageLines.
	dIms, cIms, err := g.applyDigests(
		lineToImCh, lineToPils,
	)
	if err != nil {
		return err
	}

	lfile := NewLockfile(dIms, cIms)

	return lfile.Write(w)
}

func (g *Generator) getImsWithDigests(
	wm *registry.WrapperManager,
	pilCh <-chan *parsedImageLine,
	doneCh <-chan struct{},
) (<-chan *registryResponse, map[string][]*parsedImageLine, error) {
	uniqLines := map[string]bool{}
	lineToPils := map[string][]*parsedImageLine{}
	lineToImCh := make(chan *registryResponse)
	wg := sync.WaitGroup{}

	for pil := range pilCh {
		if pil.err != nil {
			return nil, nil, pil.err
		}

		lineToPils[pil.line] = append(lineToPils[pil.line], pil)

		if !uniqLines[pil.line] {
			uniqLines[pil.line] = true

			wg.Add(1)

			go g.queryRegistry(pil, wm, lineToImCh, doneCh, &wg)
		}
	}

	go func() {
		wg.Wait()
		close(lineToImCh)
	}()

	return lineToImCh, lineToPils, nil
}

func (g *Generator) queryRegistry(
	pil *parsedImageLine,
	wm *registry.WrapperManager,
	lineToImCh chan<- *registryResponse,
	doneCh <-chan struct{},
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	im := g.convertLineToIm(pil.line)

	if im.Digest == "" {
		w := wm.GetWrapper(im.Name)

		var err error

		digest, err := w.GetDigest(im.Name, im.Tag)
		if err != nil {
			extraErrInfo := ""
			if pil.dPath != "" {
				extraErrInfo = fmt.Sprintf("from '%s': ", pil.dPath)
			}

			if pil.cPath != "" {
				extraErrInfo = fmt.Sprintf(
					"%sfrom '%s': from service '%s': ", extraErrInfo, pil.cPath,
					pil.svcName,
				)
			}

			err = fmt.Errorf("%s%v", extraErrInfo, err)

			select {
			case <-doneCh:
			case lineToImCh <- &registryResponse{err: err}:
			}

			return
		}

		im.Digest = digest
	}

	select {
	case <-doneCh:
	case lineToImCh <- &registryResponse{im: im, line: pil.line}:
	}
}

func (g *Generator) applyDigests(
	lineToImCh <-chan *registryResponse,
	lineToPils map[string][]*parsedImageLine,
) (map[string][]*DockerfileImage, map[string][]*ComposefileImage, error) {
	dImsCh := make(chan map[string][]*DockerfileImage)
	cImsCh := make(chan map[string][]*ComposefileImage)
	wg := sync.WaitGroup{}

	for res := range lineToImCh {
		if res.err != nil {
			return nil, nil, res.err
		}

		wg.Add(1)

		go g.applyDigestsPerLine(
			res.im, lineToPils[res.line], dImsCh, cImsCh, &wg,
		)
	}

	go func() {
		wg.Wait()
		close(dImsCh)
		close(cImsCh)
	}()

	dIms, cIms := g.convertImsChToSl(dImsCh, cImsCh)

	return dIms, cIms, nil
}

func (g *Generator) applyDigestsPerLine(
	im *Image,
	pils []*parsedImageLine,
	dImsCh chan<- map[string][]*DockerfileImage,
	cImsCh chan<- map[string][]*ComposefileImage,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	dIms := map[string][]*DockerfileImage{}
	cIms := map[string][]*ComposefileImage{}

	for _, pil := range pils {
		if pil.cPath == "" {
			dIm := &DockerfileImage{Image: im, pos: pil.pos}
			dPath := pil.dPath
			dIms[dPath] = append(dIms[dPath], dIm)
		} else {
			cIm := &ComposefileImage{
				Image:          im,
				ServiceName:    pil.svcName,
				DockerfilePath: pil.dPath,
				pos:            pil.pos,
			}
			cPath := pil.cPath
			cIms[cPath] = append(cIms[cPath], cIm)
		}
	}

	if len(dIms) > 0 {
		dImsCh <- dIms
	}

	if len(cIms) > 0 {
		cImsCh <- cIms
	}
}

func (g *Generator) convertImsChToSl(
	dImsCh <-chan map[string][]*DockerfileImage,
	cImsCh <-chan map[string][]*ComposefileImage,
) (map[string][]*DockerfileImage, map[string][]*ComposefileImage) {
	dIms := map[string][]*DockerfileImage{}
	cIms := map[string][]*ComposefileImage{}

	for {
		select {
		case dRes, ok := <-dImsCh:
			if ok {
				for p, ims := range dRes {
					dIms[p] = append(dIms[p], ims...)
				}
			} else {
				dImsCh = nil
			}
		case cRes, ok := <-cImsCh:
			if ok {
				for p, ims := range cRes {
					cIms[p] = append(cIms[p], ims...)
				}
			} else {
				cImsCh = nil
			}
		}

		if dImsCh == nil && cImsCh == nil {
			break
		}
	}

	return dIms, cIms
}

func (g *Generator) convertLineToIm(l string) *Image {
	tagSeparator := -1
	digestSeparator := -1

loop:
	for i, c := range l {
		switch c {
		case ':':
			tagSeparator = i
		case '@':
			digestSeparator = i
			break loop
		}
	}

	var name, tag, digest string

	// 4 valid cases
	switch {
	case tagSeparator != -1 && digestSeparator != -1:
		// ubuntu:18.04@sha256:9b1702...
		name = l[:tagSeparator]
		tag = l[tagSeparator+1 : digestSeparator]
		digest = l[digestSeparator+1+len("sha256:"):]
	case tagSeparator != -1 && digestSeparator == -1:
		// ubuntu:18.04
		name = l[:tagSeparator]
		tag = l[tagSeparator+1:]
	case tagSeparator == -1 && digestSeparator != -1:
		// ubuntu@sha256:9b1702...
		name = l[:digestSeparator]
		digest = l[digestSeparator+1+len("sha256:"):]
	case tagSeparator == -1 && digestSeparator == -1:
		// ubuntu
		name = l
		tag = "latest"
	}

	return &Image{Name: name, Tag: tag, Digest: digest}
}
