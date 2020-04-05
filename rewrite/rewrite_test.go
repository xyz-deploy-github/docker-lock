package rewrite

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

type test struct {
	lPath      string
	dPaths     []string
	cPaths     []string
	shouldFail bool
}

var dTestDir = filepath.Join("testdata", "docker")  //nolint: gochecknoglobals
var cTestDir = filepath.Join("testdata", "compose") //nolint: gochecknoglobals
var tmpDir = filepath.Join("testdata", "tmp")       //nolint: gochecknoglobals

func TestRewriter(t *testing.T) {
	tests := getTests()
	for name, tc := range tests {
		tc := tc

		t.Run(name, func(t *testing.T) {
			flags, err := NewFlags(tc.lPath, "got", tmpDir)
			if err != nil {
				t.Fatal(err)
			}

			r, err := NewRewriter(flags)
			if err != nil {
				t.Fatal(err)
			}

			if err := r.Rewrite(); err != nil && !tc.shouldFail {
				t.Fatal(err)
			}

			if tc.shouldFail {
				return
			}

			if err := cmpRewrites(tc.dPaths, tc.cPaths); err != nil {
				t.Fatal(err)
			}

			if err := rmGotPaths(tc.dPaths, tc.cPaths); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Dockerfile tests

// dLocalArg replaces the ARG referenced in the FROM instruction with the image.
func dLocalArg() *test {
	bDir := filepath.Join(dTestDir, "localarg")
	lPath := filepath.Join(bDir, "docker-lock.json")

	return &test{
		lPath:      lPath,
		dPaths:     []string{filepath.Join(bDir, "Dockerfile")},
		cPaths:     []string{},
		shouldFail: false,
	}
}

// dBuildStage replaces newly defined images
// in the case of multi-stage builds. For instance:
// # Dockerfile
// FROM busybox AS busy
// FROM busy AS anotherbusy
// should only replace the first busybox.
func dBuildStage() *test {
	bDir := filepath.Join(dTestDir, "buildstage")
	lPath := filepath.Join(bDir, "docker-lock.json")

	return &test{
		lPath:      lPath,
		dPaths:     []string{filepath.Join(bDir, "Dockerfile")},
		cPaths:     []string{},
		shouldFail: false,
	}
}

// dMoreIms ensures that when there are
// more images in a Dockerfile than in a Lockfile, an error occurs.
func dMoreImsDockerfile() *test {
	bDir := filepath.Join(dTestDir, "moreimagesdockerfile")
	lPath := filepath.Join(bDir, "docker-lock.json")

	return &test{
		lPath:      lPath,
		dPaths:     []string{filepath.Join(bDir, "Dockerfile")},
		cPaths:     []string{},
		shouldFail: true,
	}
}

// dMoreImsLockfile ensures that when there are
// more images in a Lockfile than in a Dockerfile, an error occurs.
func dMoreImsLockfile() *test {
	bDir := filepath.Join(dTestDir, "moreimageslockfile")
	lPath := filepath.Join(bDir, "docker-lock.json")

	return &test{
		lPath:      lPath,
		dPaths:     []string{filepath.Join(bDir, "Dockerfile")},
		cPaths:     []string{},
		shouldFail: true,
	}
}

// cIm replaces the image line with the image.
func cIm() *test {
	bDir := filepath.Join(cTestDir, "image")
	lPath := filepath.Join(bDir, "docker-lock.json")

	return &test{
		lPath:      lPath,
		dPaths:     []string{},
		cPaths:     []string{filepath.Join(bDir, "docker-compose.yml")},
		shouldFail: false,
	}
}

// cEnv replaces the environment variable
// referenced in the image line with the image.
func cEnv() *test {
	bDir := filepath.Join(cTestDir, "env")
	lPath := filepath.Join(bDir, "docker-lock.json")

	return &test{
		lPath:      lPath,
		dPaths:     []string{},
		cPaths:     []string{filepath.Join(bDir, "docker-compose.yml")},
		shouldFail: false,
	}
}

// cDockerfile ensures that Dockerfiles
// referenced in docker-compose files are rewritten.
func cDockerfile() *test {
	bDir := filepath.Join(cTestDir, "dockerfile")
	lPath := filepath.Join(bDir, "docker-lock.json")

	return &test{
		lPath: lPath,
		dPaths: []string{
			filepath.Join(bDir, "dockerfile", "Dockerfile"),
		},
		cPaths:     []string{},
		shouldFail: false,
	}
}

// cSort ensures that
// multiple Dockerfiles with multi-stage builds, referenced by docker-compose
// files, are rewritten in the proper order.
func cSort() *test {
	bDir := filepath.Join(cTestDir, "sort")
	lPath := filepath.Join(bDir, "docker-lock.json")

	return &test{
		lPath: lPath,
		dPaths: []string{
			filepath.Join(bDir, "sort", "Dockerfile-one"),
			filepath.Join(bDir, "sort", "Dockerfile-two"),
		},
		cPaths:     []string{},
		shouldFail: false,
	}
}

// cAssortment tests rewrite for a collection of arbitrary
// docker-compose files and Dockerfiles.
func cAssortment() *test {
	bDir := filepath.Join(cTestDir, "assortment")
	lPath := filepath.Join(bDir, "docker-lock.json")

	return &test{
		lPath: lPath,
		dPaths: []string{
			filepath.Join(bDir, "build", "Dockerfile"),
			filepath.Join(bDir, "context", "Dockerfile"),
			filepath.Join(bDir, "dockerfile", "Dockerfile"),
		},
		cPaths: []string{
			filepath.Join(bDir, "docker-compose.yml"),
			filepath.Join(bDir, "docker-compose.yaml"),
		},
		shouldFail: false,
	}
}

// cDiffNumSvcs ensures that if a lockfile and docker-compose file have
// a different number of services, rewrite will fail.
func cDiffNumSvcs() *test {
	bDir := filepath.Join(cTestDir, "diffnumsvcs")
	lPath := filepath.Join(bDir, "docker-lock.json")

	return &test{
		lPath:      lPath,
		dPaths:     []string{},
		cPaths:     []string{filepath.Join(bDir, "docker-compose.yml")},
		shouldFail: true,
	}
}

// cDiffNamedSvcs ensures that if a lockfile and docker-compose file have
// different named services, rewrite will fail.
func cDiffNamedSvcs() *test {
	bDir := filepath.Join(cTestDir, "diffnamedsvcs")
	lPath := filepath.Join(bDir, "docker-lock.json")

	return &test{
		lPath:      lPath,
		dPaths:     []string{},
		cPaths:     []string{filepath.Join(bDir, "docker-compose.yml")},
		shouldFail: true,
	}
}

// Helpers

func getTests() map[string]*test {
	tests := map[string]*test{
		"dLocalArg":          dLocalArg(),
		"dBuildStage":        dBuildStage(),
		"dMoreImsDockerfile": dMoreImsDockerfile(),
		"dMoreImsLockfile":   dMoreImsLockfile(),
		"cIm":                cIm(),
		"cEnv":               cEnv(),
		"cDockerfile":        cDockerfile(),
		"cSort":              cSort(),
		"cAssortment":        cAssortment(),
		"cDiffNumSvcs":       cDiffNumSvcs(),
		"cDiffNamedSvcs":     cDiffNamedSvcs(),
	}

	return tests
}

func cmpRewrites(dPaths, cPaths []string) error {
	if err := cmpDfileRewrites(dPaths); err != nil {
		return err
	}

	if err := cmpCfileRewrites(cPaths); err != nil {
		return err
	}

	return nil
}

func cmpDfileRewrites(paths []string) error {
	for _, p := range paths {
		gotPath := getSuffixPath(p, "got")

		gotByt, err := ioutil.ReadFile(gotPath)
		if err != nil {
			return err
		}

		wantPath := getSuffixPath(p, "want")

		wantByt, err := ioutil.ReadFile(wantPath)
		if err != nil {
			return err
		}

		if !bytes.Equal(gotByt, wantByt) {
			return fmt.Errorf("files %s and %s differ", gotPath, wantPath)
		}

		gotStatements := strings.Split(string(gotByt), "\n")
		wantStatements := strings.Split(string(wantByt), "\n")

		if len(gotStatements) != len(wantStatements) {
			return fmt.Errorf(
				"%s and %s have a different number of lines", gotPath, wantPath,
			)
		}

		for j := range gotStatements {
			if gotStatements[j] != wantStatements[j] {
				return fmt.Errorf(
					"got %s, want %s", gotStatements[j], wantStatements[j],
				)
			}
		}
	}

	return nil
}

func cmpCfileRewrites(paths []string) error {
	for _, p := range paths {
		gotPath := getSuffixPath(p, "got")
		wantPath := getSuffixPath(p, "want")

		gotByt, err := ioutil.ReadFile(gotPath)
		if err != nil {
			return err
		}

		wantByt, err := ioutil.ReadFile(wantPath)
		if err != nil {
			return err
		}

		gotComp := compose{}
		if err := yaml.Unmarshal(gotByt, &gotComp); err != nil {
			return err
		}

		wantComp := compose{}
		if err := yaml.Unmarshal(wantByt, &wantComp); err != nil {
			return err
		}

		if len(wantComp.Services) != len(gotComp.Services) {
			return fmt.Errorf(
				"%s and %s have a different number of services",
				gotPath, wantPath,
			)
		}

		for svcName := range gotComp.Services {
			gotIm := gotComp.Services[svcName].Image
			wantIm := wantComp.Services[svcName].Image

			if gotIm != wantIm {
				return fmt.Errorf("got %s, want %s", gotIm, wantIm)
			}
		}
	}

	return nil
}

func rmGotPaths(dPaths, cPaths []string) error {
	for _, paths := range [][]string{dPaths, cPaths} {
		for _, p := range paths {
			if err := os.Remove(getSuffixPath(p, "got")); err != nil {
				return err
			}
		}
	}

	return nil
}

func getSuffixPath(p, suffix string) string {
	switch {
	case strings.HasSuffix(p, ".yml"):
		return fmt.Sprintf(
			"%s-%s%s", p[:len(p)-len(".yml")], suffix, ".yml",
		)
	case strings.HasSuffix(p, ".yaml"):
		return fmt.Sprintf(
			"%s-%s%s", p[:len(p)-len(".yaml")], suffix, ".yaml",
		)
	default:
		return fmt.Sprintf("%s-%s", p, suffix)
	}
}
