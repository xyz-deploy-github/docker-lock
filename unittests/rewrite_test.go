package unittests

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/michaelperel/docker-lock/cmd"
	"github.com/michaelperel/docker-lock/unittests/internal/compose"
	"gopkg.in/yaml.v2"
)

type rewriteTestObject struct {
	wantPaths []string
	gotPaths  []string
	testFn    func(*testing.T, []string, []string)
}

var (
	rewriteDockerBaseDir  = filepath.Join("testdata", "rewrite", "docker")
	rewriteComposeBaseDir = filepath.Join("testdata", "rewrite", "compose")
)

// TestRewriteDockerfileArgsLocalArg replaces the ARG referenced in
// the FROM instruction with the image.
func TestRewriteDockerfileArgsLocalArg(t *testing.T) {
	baseDir := filepath.Join(rewriteDockerBaseDir, "args", "localarg")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	tOs := []rewriteTestObject{
		{
			wantPaths: []string{filepath.Join(baseDir, "Dockerfile-want")},
			gotPaths:  []string{filepath.Join(baseDir, "Dockerfile-got")},
			testFn:    checkRewriteDockerfile,
		},
	}
	testRewrite(t, lPath, tOs, false)
}

// TestRewriteDockerfileArgsBuildStage replaces newly defined images
// in the case of multi-stage builds. For instance:
// # Dockerfile
// FROM busybox AS busy
// FROM busy AS anotherbusy
// should only replace the first busybox.
func TestRewriteDockerfileArgsBuildStage(t *testing.T) {
	baseDir := filepath.Join(rewriteDockerBaseDir, "args", "buildstage")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	tOs := []rewriteTestObject{
		{
			wantPaths: []string{filepath.Join(baseDir, "Dockerfile-want")},
			gotPaths:  []string{filepath.Join(baseDir, "Dockerfile-got")},
			testFn:    checkRewriteDockerfile,
		},
	}
	testRewrite(t, lPath, tOs, false)
}

// TestRewriteMoreDockerfileImages ensures that when there are
// more images in a Dockerfile than in a Lockfile, an error occurs.
func TestRewriteMoreDockerfileImages(t *testing.T) {
	baseDir := filepath.Join(rewriteDockerBaseDir, "fail", "moreImagesDockerfile")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	tOs := []rewriteTestObject{
		{
			wantPaths: []string{filepath.Join(baseDir, "Dockerfile-want")},
			gotPaths:  []string{filepath.Join(baseDir, "Dockerfile-got")},
			testFn:    checkRewriteDockerfile,
		},
	}
	testRewrite(t, lPath, tOs, true)
}

// TestRewriteMoreLockfileImages ensures that when there are
// more images in a Lockfile than in a Dockerfile, an error occurs.
func TestRewriteMoreLockfileImages(t *testing.T) {
	baseDir := filepath.Join(rewriteDockerBaseDir, "fail", "moreImagesLockfile")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	tOs := []rewriteTestObject{
		{
			wantPaths: []string{filepath.Join(baseDir, "Dockerfile-want")},
			gotPaths:  []string{filepath.Join(baseDir, "Dockerfile-got")},
			testFn:    checkRewriteDockerfile,
		},
	}
	testRewrite(t, lPath, tOs, true)
}

// TestRewriteComposefileImage replaces the image line with the image.
func TestRewriteComposefileImage(t *testing.T) {
	baseDir := filepath.Join(rewriteComposeBaseDir, "image")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	tOs := []rewriteTestObject{
		{
			wantPaths: []string{filepath.Join(baseDir, "docker-compose-want.yml")},
			gotPaths:  []string{filepath.Join(baseDir, "docker-compose-got.yml")},
			testFn:    checkRewriteComposefile,
		},
	}
	testRewrite(t, lPath, tOs, false)
}

// TestRewriteComposefileEnv replaces the environment variable
// referenced in the image line with the image.
func TestRewriteComposefileEnv(t *testing.T) {
	baseDir := filepath.Join(rewriteComposeBaseDir, "env")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	tOs := []rewriteTestObject{
		{
			wantPaths: []string{filepath.Join(baseDir, "docker-compose-want.yml")},
			gotPaths:  []string{filepath.Join(baseDir, "docker-compose-got.yml")},
			testFn:    checkRewriteComposefile,
		},
	}
	testRewrite(t, lPath, tOs, false)
}

// TestRewriteDockerfilesReferencedByComposefiles ensures that Dockerfiles
// referenced in docker-compose files are rewritten.
func TestRewriteDockerfilesReferencedByComposefiles(t *testing.T) {
	baseDir := filepath.Join(rewriteComposeBaseDir, "dockerfile")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	tOs := []rewriteTestObject{
		{
			wantPaths: []string{filepath.Join(baseDir, "dockerfile", "Dockerfile-want")},
			gotPaths:  []string{filepath.Join(baseDir, "dockerfile", "Dockerfile-got")},
			testFn:    checkRewriteDockerfile,
		},
	}
	testRewrite(t, lPath, tOs, false)
}

// TestRewriteDockerfilesMultipleMultiStageReferencedByComposefiles ensures that
// multiple Dockerfiles with multi-stage builds, referenced by docker-compose
// files, are rewritten in the proper order.
func TestRewriteDockerfilesMultipleMultiStageReferencedByComposefiles(t *testing.T) {
	baseDir := filepath.Join(rewriteComposeBaseDir, "sort")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	tOs := []rewriteTestObject{
		{
			wantPaths: []string{filepath.Join(baseDir, "sort", "Dockerfile-one-want")},
			gotPaths:  []string{filepath.Join(baseDir, "sort", "Dockerfile-one-got")},
			testFn:    checkRewriteDockerfile,
		},
		{
			wantPaths: []string{filepath.Join(baseDir, "sort", "Dockerfile-two-want")},
			gotPaths:  []string{filepath.Join(baseDir, "sort", "Dockerfile-two-got")},
			testFn:    checkRewriteDockerfile,
		},
	}
	testRewrite(t, lPath, tOs, false)
}

// TestRewriteAssortment tests rewrite for a collection of arbitrary
// docker-compose files and Dockerfiles.
func TestRewriteAssortment(t *testing.T) {
	baseDir := filepath.Join(rewriteComposeBaseDir, "assortment")
	lPath := filepath.Join(baseDir, "docker-lock.json")
	tOs := []rewriteTestObject{
		{
			wantPaths: []string{filepath.Join(baseDir, "build", "Dockerfile-want")},
			gotPaths:  []string{filepath.Join(baseDir, "build", "Dockerfile-got")},
			testFn:    checkRewriteDockerfile,
		},
		{
			wantPaths: []string{filepath.Join(baseDir, "context", "Dockerfile-want")},
			gotPaths:  []string{filepath.Join(baseDir, "context", "Dockerfile-got")},
			testFn:    checkRewriteDockerfile,
		},
		{
			wantPaths: []string{filepath.Join(baseDir, "dockerfile", "Dockerfile-want")},
			gotPaths:  []string{filepath.Join(baseDir, "dockerfile", "Dockerfile-got")},
			testFn:    checkRewriteDockerfile,
		},
		{
			wantPaths: []string{filepath.Join(baseDir, "docker-compose-want.yml")},
			gotPaths:  []string{filepath.Join(baseDir, "docker-compose-got.yml")},
			testFn:    checkRewriteComposefile,
		},
		{
			wantPaths: []string{filepath.Join(baseDir, "docker-compose-want.yaml")},
			gotPaths:  []string{filepath.Join(baseDir, "docker-compose-got.yaml")},
			testFn:    checkRewriteComposefile,
		},
	}
	testRewrite(t, lPath, tOs, false)
}

func testRewrite(t *testing.T, lPath string, tOs []rewriteTestObject, shouldErr bool) {
	rewriteCmd := cmd.NewRewriteCmd()
	rewriteCmd.SilenceUsage = true
	rewriteCmd.SilenceErrors = true
	tmpDir := filepath.Join("testdata", "rewrite", "tmp")
	rewriteArgs := []string{
		"lock",
		"rewrite",
		fmt.Sprintf("--lockfile-path=%s", lPath),
		fmt.Sprintf("--tempdir=%s", tmpDir), "--suffix=got",
	}
	rewriteCmd.SetArgs(rewriteArgs)
	if err := rewriteCmd.Execute(); err != nil {
		if shouldErr {
			return
		}
		t.Fatal(err)
	}
	for _, tO := range tOs {
		for _, gotPath := range tO.gotPaths {
			defer os.Remove(gotPath)
		}
		tO.testFn(t, tO.wantPaths, tO.gotPaths)
	}
}

func checkRewriteDockerfile(t *testing.T, wantPaths []string, gotPaths []string) {
	for i := range gotPaths {
		gotByt, err := ioutil.ReadFile(gotPaths[i])
		if err != nil {
			t.Fatal(err)
		}
		wantByt, err := ioutil.ReadFile(wantPaths[i])
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(gotByt, wantByt) {
			t.Fatalf("Files %s and %s differ.", gotPaths[i], wantPaths[i])
		}
		gotLines := strings.Split(string(gotByt), "\n")
		wantLines := strings.Split(string(wantByt), "\n")
		if len(gotLines) != len(wantLines) {
			t.Fatalf("%s and %s have a different number of lines.", gotPaths[i], wantPaths[i])
		}
		for j := range gotLines {
			if gotLines[j] != wantLines[j] {
				t.Fatalf("Got %s, want %s.", gotLines[j], wantLines[j])
			}
		}
	}
}

func checkRewriteComposefile(t *testing.T, wantPaths []string, gotPaths []string) {
	for i := range gotPaths {
		gotByt, err := ioutil.ReadFile(gotPaths[i])
		if err != nil {
			t.Fatal(err)
		}
		wantByt, err := ioutil.ReadFile(wantPaths[i])
		if err != nil {
			t.Fatal(err)
		}
		var gotComp compose.Compose
		if err := yaml.Unmarshal(gotByt, &gotComp); err != nil {
			t.Fatal(err)
		}
		var wantComp compose.Compose
		if err := yaml.Unmarshal(wantByt, &wantComp); err != nil {
			t.Fatal(err)
		}
		if len(wantComp.Services) != len(gotComp.Services) {
			t.Fatalf("%s and %s have a different number of services.", gotPaths[i], wantPaths[i])
		}
		for serviceName := range gotComp.Services {
			gotImage := gotComp.Services[serviceName].ImageName
			wantImage := wantComp.Services[serviceName].ImageName
			if gotImage != wantImage {
				t.Fatalf("Got %s. Want %s.", gotImage, wantImage)
			}
		}
	}
}
