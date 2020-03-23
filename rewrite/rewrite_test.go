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
			flags, err := NewRewriterFlags(tc.lPath, "got", tmpDir)
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
			if err := compareRewrites(tc.dPaths, tc.cPaths); err != nil {
				t.Fatal(err)
			}
			if err := removeGotPaths(tc.dPaths, tc.cPaths); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Dockerfile tests

// dLocalArg replaces the ARG referenced in the FROM instruction with the image.
func dLocalArg() *test {
	baseDir := filepath.Join(dTestDir, "localarg")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	return &test{
		lPath:      lPath,
		dPaths:     []string{filepath.Join(baseDir, "Dockerfile")},
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
	baseDir := filepath.Join(dTestDir, "buildstage")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	return &test{
		lPath:      lPath,
		dPaths:     []string{filepath.Join(baseDir, "Dockerfile")},
		cPaths:     []string{},
		shouldFail: false,
	}
}

// dMoreImages ensures that when there are
// more images in a Dockerfile than in a Lockfile, an error occurs.
func dMoreImagesDockerfile() *test {
	baseDir := filepath.Join(dTestDir, "moreimagesdockerfile")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	return &test{
		lPath:      lPath,
		dPaths:     []string{filepath.Join(baseDir, "Dockerfile")},
		cPaths:     []string{},
		shouldFail: true,
	}
}

// dMoreImagesLockfile ensures that when there are
// more images in a Lockfile than in a Dockerfile, an error occurs.
func dMoreImagesLockfile() *test {
	baseDir := filepath.Join(dTestDir, "moreimageslockfile")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	return &test{
		lPath:      lPath,
		dPaths:     []string{filepath.Join(baseDir, "Dockerfile")},
		cPaths:     []string{},
		shouldFail: true,
	}
}

// cImage replaces the image line with the image.
func cImage() *test {
	baseDir := filepath.Join(cTestDir, "image")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	return &test{
		lPath:      lPath,
		dPaths:     []string{},
		cPaths:     []string{filepath.Join(baseDir, "docker-compose.yml")},
		shouldFail: false,
	}
}

// cEnv replaces the environment variable
// referenced in the image line with the image.
func cEnv() *test {
	baseDir := filepath.Join(cTestDir, "env")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	return &test{
		lPath:      lPath,
		dPaths:     []string{},
		cPaths:     []string{filepath.Join(baseDir, "docker-compose.yml")},
		shouldFail: false,
	}
}

// cDockerfile ensures that Dockerfiles
// referenced in docker-compose files are rewritten.
func cDockerfile() *test {
	baseDir := filepath.Join(cTestDir, "dockerfile")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	return &test{
		lPath:      lPath,
		dPaths:     []string{filepath.Join(baseDir, "dockerfile", "Dockerfile")},
		cPaths:     []string{},
		shouldFail: false,
	}
}

// cSort ensures that
// multiple Dockerfiles with multi-stage builds, referenced by docker-compose
// files, are rewritten in the proper order.
func cSort() *test {
	baseDir := filepath.Join(cTestDir, "sort")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	return &test{
		lPath: lPath,
		dPaths: []string{
			filepath.Join(baseDir, "sort", "Dockerfile-one"),
			filepath.Join(baseDir, "sort", "Dockerfile-two"),
		},
		cPaths:     []string{},
		shouldFail: false,
	}
}

// cAssortment tests rewrite for a collection of arbitrary
// docker-compose files and Dockerfiles.
func cAssortment() *test {
	baseDir := filepath.Join(cTestDir, "assortment")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	return &test{
		lPath: lPath,
		dPaths: []string{
			filepath.Join(baseDir, "build", "Dockerfile"),
			filepath.Join(baseDir, "context", "Dockerfile"),
			filepath.Join(baseDir, "dockerfile", "Dockerfile"),
		},
		cPaths: []string{
			filepath.Join(baseDir, "docker-compose.yml"),
			filepath.Join(baseDir, "docker-compose.yaml"),
		},
		shouldFail: false,
	}
}

// Helpers

func getTests() map[string]*test {
	tests := map[string]*test{
		"dLocalArg":             dLocalArg(),
		"dBuildStage":           dBuildStage(),
		"dMoreImagesDockerfile": dMoreImagesDockerfile(),
		"dMoreImagesLockfile":   dMoreImagesLockfile(),
		"cImage":                cImage(),
		"cEnv":                  cEnv(),
		"cDockerfile":           cDockerfile(),
		"cSort":                 cSort(),
		"cAssortment":           cAssortment(),
	}
	return tests
}

func compareRewrites(dPaths, cPaths []string) error {
	if err := compareDockerfileRewrites(dPaths); err != nil {
		return err
	}
	if err := compareComposefileRewrites(cPaths); err != nil {
		return err
	}
	return nil
}

func compareDockerfileRewrites(paths []string) error {
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
		gotLines := strings.Split(string(gotByt), "\n")
		wantLines := strings.Split(string(wantByt), "\n")
		if len(gotLines) != len(wantLines) {
			return fmt.Errorf(
				"%s and %s have a different number of lines", gotPath, wantPath,
			)
		}
		for j := range gotLines {
			if gotLines[j] != wantLines[j] {
				return fmt.Errorf("got %s, want %s", gotLines[j], wantLines[j])
			}
		}
	}
	return nil
}

func compareComposefileRewrites(paths []string) error {
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
		var gotComp compose
		if err := yaml.Unmarshal(gotByt, &gotComp); err != nil {
			return err
		}
		var wantComp compose
		if err := yaml.Unmarshal(wantByt, &wantComp); err != nil {
			return err
		}
		if len(wantComp.Services) != len(gotComp.Services) {
			return fmt.Errorf(
				"%s and %s have a different number of services",
				gotPath, wantPath,
			)
		}
		for serviceName := range gotComp.Services {
			gotImage := gotComp.Services[serviceName].Image
			wantImage := wantComp.Services[serviceName].Image
			if gotImage != wantImage {
				return fmt.Errorf("got %s, want %s", gotImage, wantImage)
			}
		}
	}
	return nil
}

func removeGotPaths(dPaths, cPaths []string) error {
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
