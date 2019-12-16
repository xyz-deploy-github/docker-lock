package verify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"

	"github.com/michaelperel/docker-lock/generate"
	"github.com/michaelperel/docker-lock/registry"
	"github.com/spf13/cobra"
)

// Verifier ensures that a Lockfile contains up-to-date information.
type Verifier struct {
	Generator *generate.Generator
	Lockfile  *generate.Lockfile
	oPath     string
}

// NewVerifier creates a Verifier from command line flags.
func NewVerifier(cmd *cobra.Command) (*Verifier, error) {
	oPath, err := cmd.Flags().GetString("outpath")
	if err != nil {
		return nil, err
	}
	oPath = filepath.ToSlash(oPath)
	lFile, err := readLockfile(oPath)
	if err != nil {
		return nil, err
	}
	g, err := makeGenerator(cmd, lFile)
	if err != nil {
		return nil, err
	}
	return &Verifier{Generator: g, Lockfile: lFile, oPath: oPath}, nil
}

// VerifyLockfile generates bytes for a new Lockfile and ensures that
// the existing Lockfile contains the same information. Specifically,
// the existing Lockfile must have:
// (1) the same number of Dockerfiles and docker-compose files
// (2) the same number of images in each Dockerfile and docker-compose file
// (3) the same image in the proper order in each Dockerfile and docker-compose
// file
// If any of these checks fail, VerifyLockfile will return an error.
func (v *Verifier) VerifyLockfile(
	wrapperManager *registry.WrapperManager,
) error {

	var lByt bytes.Buffer
	if err := v.Generator.GenerateLockfile(wrapperManager, &lByt); err != nil {
		return err
	}
	var lFile generate.Lockfile
	if err := json.Unmarshal(lByt.Bytes(), &lFile); err != nil {
		return err
	}
	if err := v.verifyNumFiles(&lFile); err != nil {
		return err
	}
	if err := v.verifyImages(&lFile); err != nil {
		return err
	}
	return nil
}

func (v *Verifier) verifyNumFiles(lFile *generate.Lockfile) error {
	if len(v.Lockfile.DockerfileImages) != len(lFile.DockerfileImages) {
		return fmt.Errorf(
			"got %d Dockerfiles, want %d",
			len(lFile.DockerfileImages),
			len(v.Lockfile.DockerfileImages),
		)
	}
	if len(v.Lockfile.ComposefileImages) != len(lFile.ComposefileImages) {
		return fmt.Errorf(
			"got %d docker-compose files, want %d",
			len(lFile.ComposefileImages),
			len(v.Lockfile.ComposefileImages),
		)
	}
	return nil
}

func (v *Verifier) verifyImages(lFile *generate.Lockfile) error {
	for dPath := range v.Lockfile.DockerfileImages {
		if len(v.Lockfile.DockerfileImages[dPath]) !=
			len(lFile.DockerfileImages[dPath]) {
			return fmt.Errorf(
				"got %d images in file %s, want %d",
				len(lFile.DockerfileImages[dPath]),
				dPath,
				len(v.Lockfile.DockerfileImages[dPath]),
			)
		}
		for i := range v.Lockfile.DockerfileImages[dPath] {
			if *v.Lockfile.DockerfileImages[dPath][i].Image !=
				*lFile.DockerfileImages[dPath][i].Image {
				return fmt.Errorf(
					"got image:\n%+v\nwant image:\n%+v",
					lFile.DockerfileImages[dPath][i].Image,
					v.Lockfile.DockerfileImages[dPath][i].Image,
				)
			}
		}
	}
	for cPath := range v.Lockfile.ComposefileImages {
		if len(v.Lockfile.ComposefileImages[cPath]) !=
			len(lFile.ComposefileImages[cPath]) {
			return fmt.Errorf(
				"got %d images in file %s, want %d",
				len(lFile.ComposefileImages[cPath]),
				cPath,
				len(v.Lockfile.ComposefileImages[cPath]),
			)
		}
		for i := range v.Lockfile.ComposefileImages[cPath] {
			if *v.Lockfile.ComposefileImages[cPath][i].Image !=
				*lFile.ComposefileImages[cPath][i].Image {
				return fmt.Errorf(
					"got image:\n%+v\nwant image:\n%+v",
					lFile.ComposefileImages[cPath][i].Image,
					v.Lockfile.ComposefileImages[cPath][i].Image,
				)
			}
		}
	}
	return nil
}

func readLockfile(oPath string) (*generate.Lockfile, error) {
	lByt, err := ioutil.ReadFile(oPath)
	if err != nil {
		return nil, err
	}
	var lFile generate.Lockfile
	if err := json.Unmarshal(lByt, &lFile); err != nil {
		return nil, err
	}
	return &lFile, nil
}

func makeGenerator(
	cmd *cobra.Command,
	lFile *generate.Lockfile,
) (*generate.Generator, error) {

	var (
		i      int
		j      int
		dPaths = make([]string, len(lFile.DockerfileImages))
		cPaths = make([]string, len(lFile.ComposefileImages))
		wg     sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for p := range lFile.DockerfileImages {
			dPaths[i] = p
			i++
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for p := range lFile.ComposefileImages {
			cPaths[j] = p
			j++
		}
	}()
	dArgs, err := cmd.Flags().GetBool("dockerfile-env-build-args")
	if err != nil {
		return nil, err
	}
	wg.Wait()
	return &generate.Generator{
		DockerfilePaths:        dPaths,
		ComposefilePaths:       cPaths,
		DockerfileEnvBuildArgs: dArgs,
	}, nil
}
