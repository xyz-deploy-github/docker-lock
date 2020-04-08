// Package verify provides functions for verifying that an existing
// Lockfile is up-to-date.
package verify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/michaelperel/docker-lock/generate"
	"github.com/michaelperel/docker-lock/registry"
)

// Verifier ensures that a Lockfile contains up-to-date information.
type Verifier struct {
	Generator *generate.Generator
	Lockfile  *generate.Lockfile
}

// NewVerifier creates a Verifier from command line flags.
func NewVerifier(flags *Flags) (*Verifier, error) {
	lfile, err := readLfile(flags.LockfilePath)
	if err != nil {
		return nil, err
	}

	g := makeGenerator(lfile, flags.DockerfileEnvBuildArgs)

	return &Verifier{Generator: g, Lockfile: lfile}, nil
}

// VerifyLockfile generates bytes for a new Lockfile and ensures that
// the existing Lockfile contains the same information. Specifically,
// the existing Lockfile must have:
//
// (1) the same number of Dockerfiles and docker-compose files
//
// (2) the same number of images in each Dockerfile and docker-compose file
//
// (3) the same image in the proper order in each Dockerfile and docker-compose
// file
//
// If any of these checks fail, VerifyLockfile will return an error.
func (v *Verifier) VerifyLockfile(wm *registry.WrapperManager) error {
	lByt := bytes.Buffer{}
	if err := v.Generator.GenerateLockfile(wm, &lByt); err != nil {
		return err
	}

	lfile := generate.Lockfile{}
	if err := json.Unmarshal(lByt.Bytes(), &lfile); err != nil {
		return err
	}

	if err := v.verifyNumFiles(&lfile); err != nil {
		return err
	}

	if err := v.verifyIms(&lfile); err != nil {
		return err
	}

	return nil
}

func (v *Verifier) verifyNumFiles(lfile *generate.Lockfile) error {
	if len(v.Lockfile.DockerfileImages) != len(lfile.DockerfileImages) {
		return fmt.Errorf(
			"got '%d' Dockerfiles, want '%d'", len(lfile.DockerfileImages),
			len(v.Lockfile.DockerfileImages),
		)
	}

	if len(v.Lockfile.ComposefileImages) != len(lfile.ComposefileImages) {
		return fmt.Errorf(
			"got '%d' docker-compose files, want '%d'",
			len(lfile.ComposefileImages),
			len(v.Lockfile.ComposefileImages),
		)
	}

	return nil
}

func (v *Verifier) verifyIms(lfile *generate.Lockfile) error {
	for dPath := range v.Lockfile.DockerfileImages {
		if len(v.Lockfile.DockerfileImages[dPath]) !=
			len(lfile.DockerfileImages[dPath]) {
			return fmt.Errorf(
				"got '%d' images in file '%s', want '%d'",
				len(lfile.DockerfileImages[dPath]), dPath,
				len(v.Lockfile.DockerfileImages[dPath]),
			)
		}

		for i := range v.Lockfile.DockerfileImages[dPath] {
			if *v.Lockfile.DockerfileImages[dPath][i].Image !=
				*lfile.DockerfileImages[dPath][i].Image {
				return fmt.Errorf(
					"got image:\n%+v\nwant image:\n%+v",
					lfile.DockerfileImages[dPath][i].Image,
					v.Lockfile.DockerfileImages[dPath][i].Image,
				)
			}
		}
	}

	for cPath := range v.Lockfile.ComposefileImages {
		if len(v.Lockfile.ComposefileImages[cPath]) !=
			len(lfile.ComposefileImages[cPath]) {
			return fmt.Errorf(
				"got '%d' images in file '%s', want '%d'",
				len(lfile.ComposefileImages[cPath]), cPath,
				len(v.Lockfile.ComposefileImages[cPath]),
			)
		}

		for i := range v.Lockfile.ComposefileImages[cPath] {
			if *v.Lockfile.ComposefileImages[cPath][i].Image !=
				*lfile.ComposefileImages[cPath][i].Image {
				return fmt.Errorf(
					"got image:\n%+v\nwant image:\n%+v",
					lfile.ComposefileImages[cPath][i].Image,
					v.Lockfile.ComposefileImages[cPath][i].Image,
				)
			}
		}
	}

	return nil
}

func readLfile(lName string) (*generate.Lockfile, error) {
	lByt, err := ioutil.ReadFile(lName) // nolint: gosec
	if err != nil {
		return nil, err
	}

	lfile := generate.Lockfile{}
	if err := json.Unmarshal(lByt, &lfile); err != nil {
		return nil, err
	}

	return &lfile, nil
}

func makeGenerator(
	lfile *generate.Lockfile,
	dfileEnvBuildArgs bool,
) *generate.Generator {
	dPaths := make([]string, len(lfile.DockerfileImages))
	cPaths := make([]string, len(lfile.ComposefileImages))

	var i, j int

	wg := sync.WaitGroup{}

	wg.Add(1)

	go func() {
		defer wg.Done()

		for p := range lfile.DockerfileImages {
			dPaths[i] = p
			i++
		}
	}()

	wg.Add(1)

	go func() {
		defer wg.Done()

		for p := range lfile.ComposefileImages {
			cPaths[j] = p
			j++
		}
	}()

	wg.Wait()

	return &generate.Generator{
		DockerfilePaths:        dPaths,
		ComposefilePaths:       cPaths,
		DockerfileEnvBuildArgs: dfileEnvBuildArgs,
	}
}
