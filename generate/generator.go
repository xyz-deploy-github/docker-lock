// Package generate provides functions to generate a Lockfile.
package generate

import (
	"encoding/json"
	"io"

	"github.com/safe-waters/docker-lock/registry"
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
	position int
}

// ComposefileImage contains an Image along with metadata used to generate
// a Lockfile.
type ComposefileImage struct {
	*Image
	ServiceName    string `json:"serviceName"`
	DockerfilePath string `json:"dockerfile"`
	position       int
}

// String formats an Image as json.
func (i *Image) String() string {
	j, _ := json.Marshal(i)
	return string(j)
}

// NewGenerator creates a Generator from command line flags. If no
// Dockerfiles or docker-compose files are specified as flags,
// files that match "Dockerfile", "docker-compose.yml", and
// "docker-compose.yaml" will be used. If files are specified in
// command line flags, only those files will be used.
func NewGenerator(flags *Flags) (*Generator, error) {
	c := &collector{
		Flags:    flags,
		dBaseSet: map[string]struct{}{"Dockerfile": {}},
		cBaseSet: map[string]struct{}{
			"docker-compose.yml":  {},
			"docker-compose.yaml": {},
		},
	}

	dPaths, cPaths, err := c.collectPaths()
	if err != nil {
		return nil, err
	}

	return &Generator{
		DockerfilePaths:        dPaths,
		ComposefilePaths:       cPaths,
		DockerfileEnvBuildArgs: flags.DockerfileFlags.UseEnvAsBuildArgs,
		LockfileName:           flags.LockfileName,
	}, nil
}

// GenerateLockfile creates a Lockfile and writes its bytes to an io.Writer.
func (g *Generator) GenerateLockfile(
	wm *registry.WrapperManager,
	w io.Writer,
) error {
	p := &parser{
		dPaths:            g.DockerfilePaths,
		cPaths:            g.ComposefilePaths,
		dfileEnvBuildArgs: g.DockerfileEnvBuildArgs,
	}

	doneCh := make(chan struct{})
	bImCh := p.parseFiles(doneCh)

	u := &updater{}

	dIms, cIms, err := u.updateDigest(wm, bImCh, doneCh)
	if err != nil {
		close(doneCh)
		return err
	}

	lfile := NewLockfile(dIms, cIms)

	return lfile.Write(w)
}
