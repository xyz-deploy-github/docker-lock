package unittests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/michaelperel/docker-lock/cmd"
	"github.com/michaelperel/docker-lock/generate"
)

type generateTestObject struct {
	filePath   string
	wantImages interface{}
	testFn     func(*testing.T, generateTestObject, generate.Lockfile)
}

var (
	generateComposeBaseDir = filepath.Join("testdata", "generate", "compose")
	generateDockerBaseDir  = filepath.Join("testdata", "generate", "docker")
	generateBothBaseDir    = filepath.Join("testdata", "generate", "both")
)

// TestGenerateComposefileImage ensures Lockfiles from docker-compose files with
// the image key are correct.
func TestGenerateComposefileImage(t *testing.T) {
	t.Parallel()
	composefile := filepath.Join(generateComposeBaseDir, "image", "docker-compose.yml")
	flags := []string{fmt.Sprintf("--compose-files=%s", composefile)}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(composefile),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}, ServiceName: "svc", DockerfilePath: ""},
			},
			testFn: checkGenerateComposefile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateComposefileBuild ensures Lockfiles from docker-compose files with
// the build key are correct.
func TestGenerateComposefileBuild(t *testing.T) {
	t.Parallel()
	composefile := filepath.Join(generateComposeBaseDir, "build", "docker-compose.yml")
	dockerfile := filepath.ToSlash(filepath.Join(generateComposeBaseDir, "build", "build", "Dockerfile"))
	flags := []string{fmt.Sprintf("--compose-files=%s", composefile)}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(composefile),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}, ServiceName: "svc", DockerfilePath: dockerfile},
			},
			testFn: checkGenerateComposefile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateComposefileDockerfile ensures Lockfiles from docker-compose files with
// the dockerfile key are correct.
func TestGenerateComposefileDockerfile(t *testing.T) {
	t.Parallel()
	composefile := filepath.Join(generateComposeBaseDir, "dockerfile", "docker-compose.yml")
	dockerfile := filepath.ToSlash(filepath.Join(generateComposeBaseDir, "dockerfile", "dockerfile", "Dockerfile"))
	flags := []string{fmt.Sprintf("--compose-files=%s", composefile)}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(composefile),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}, ServiceName: "svc", DockerfilePath: dockerfile},
			},
			testFn: checkGenerateComposefile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateComposefileContext ensures Lockfiles from docker-compose files with
// the context key are correct.
func TestGenerateComposefileContext(t *testing.T) {
	t.Parallel()
	composefile := filepath.Join(generateComposeBaseDir, "context", "docker-compose.yml")
	dockerfile := filepath.ToSlash(filepath.Join(generateComposeBaseDir, "context", "context", "Dockerfile"))
	flags := []string{fmt.Sprintf("--compose-files=%s", composefile)}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(composefile),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}, ServiceName: "svc", DockerfilePath: dockerfile},
			},
			testFn: checkGenerateComposefile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateComposefileEnv ensures Lockfiles from docker-compose files with
// environment variables replaced by values in a .env file are correct.
func TestGenerateComposefileEnv(t *testing.T) {
	t.Parallel()
	composefile := filepath.Join(generateComposeBaseDir, "env", "docker-compose.yml")
	dockerfile := filepath.ToSlash(filepath.Join(generateComposeBaseDir, "env", "env", "Dockerfile"))
	envPath := filepath.Join(generateComposeBaseDir, "env", ".env")
	flags := []string{fmt.Sprintf("--compose-files=%s", composefile), fmt.Sprintf("--env-file=%s", envPath)}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(composefile),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}, ServiceName: "svc", DockerfilePath: dockerfile},
			},
			testFn: checkGenerateComposefile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateComposefileMultipleComposefiles ensures Lockfiles from multiple
// docker-compose files are correct.
func TestGenerateComposefileMultipleComposefiles(t *testing.T) {
	t.Parallel()
	composefileOne := filepath.Join(generateComposeBaseDir, "multiple", "docker-compose-one.yml")
	composefileTwo := filepath.Join(generateComposeBaseDir, "multiple", "docker-compose-two.yml")
	dockerfilesOne := []string{filepath.ToSlash(filepath.Join(generateComposeBaseDir, "multiple", "build", "Dockerfile"))}
	dockerfilesTwo := []string{
		filepath.ToSlash(filepath.Join(generateComposeBaseDir, "multiple", "context", "Dockerfile")),
		filepath.ToSlash(filepath.Join(generateComposeBaseDir, "multiple", "dockerfile", "Dockerfile")),
	}
	flags := []string{fmt.Sprintf("--compose-files=%s,%s", composefileOne, composefileTwo)}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(composefileOne),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "ubuntu", Tag: "latest"}, ServiceName: "build-svc", DockerfilePath: dockerfilesOne[0]},
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}, ServiceName: "image-svc", DockerfilePath: ""},
			},
			testFn: checkGenerateComposefile,
		},
		{
			filePath: filepath.ToSlash(composefileTwo),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "node", Tag: "latest"}, ServiceName: "context-svc", DockerfilePath: dockerfilesTwo[0]},
				{Image: &generate.Image{Name: "golang", Tag: "latest"}, ServiceName: "dockerfile-svc", DockerfilePath: dockerfilesTwo[1]},
			},
			testFn: checkGenerateComposefile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateComposefileRecursive ensures Lockfiles from multiple docker-compose
// files in subdirectories are correct.
func TestGenerateComposefileRecursive(t *testing.T) {
	t.Parallel()
	composefileTopLevel := filepath.Join(generateComposeBaseDir, "recursive", "docker-compose.yml")
	composefileRecursiveLevel := filepath.Join(generateComposeBaseDir, "recursive", "build", "docker-compose.yml")
	dockerfileRecursiveLevel := filepath.ToSlash(filepath.Join(generateComposeBaseDir, "recursive", "build", "build", "Dockerfile"))
	recursiveBaseDir := filepath.Join(generateComposeBaseDir, "recursive")
	flags := []string{fmt.Sprintf("--base-dir=%s", recursiveBaseDir), "--compose-file-recursive"}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(composefileTopLevel),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "golang", Tag: "latest"}, ServiceName: "svc", DockerfilePath: ""},
			},
			testFn: checkGenerateComposefile,
		},
		{
			filePath: filepath.ToSlash(composefileRecursiveLevel),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}, ServiceName: "svc", DockerfilePath: dockerfileRecursiveLevel},
			},
			testFn: checkGenerateComposefile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateComposefileNoFileSpecified ensures Lockfiles include docker-compose.yml
// and docker-compose.yaml files in the base directory, if no other files are specified.
func TestGenerateComposefileNoFileSpecified(t *testing.T) {
	t.Parallel()
	baseDir := filepath.Join(generateComposeBaseDir, "nofile")
	flags := []string{fmt.Sprintf("--base-dir=%s", baseDir)}
	composefiles := []string{filepath.Join(baseDir, "docker-compose.yml"), filepath.Join(baseDir, "docker-compose.yaml")}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(composefiles[0]),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}, ServiceName: "svc", DockerfilePath: ""},
			},
			testFn: checkGenerateComposefile,
		},
		{
			filePath: filepath.ToSlash(composefiles[1]),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "golang", Tag: "latest"}, ServiceName: "svc", DockerfilePath: ""},
			},
			testFn: checkGenerateComposefile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateComposefileGlobs ensures Lockfiles include docker-compose files found
// via glob syntax.
func TestGenerateComposefileGlobs(t *testing.T) {
	t.Parallel()
	globs := strings.Join([]string{filepath.Join(generateComposeBaseDir, "globs", "**", "docker-compose.yml"),
		filepath.Join(generateComposeBaseDir, "globs", "docker-compose.yml")}, ",")
	flags := []string{fmt.Sprintf("--compose-file-globs=%s", globs)}
	composefiles := []string{filepath.Join(generateComposeBaseDir, "globs", "image", "docker-compose.yml"),
		filepath.Join(generateComposeBaseDir, "globs", "docker-compose.yml")}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(composefiles[0]),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "ubuntu", Tag: "latest"}, ServiceName: "svc", DockerfilePath: ""},
			},
			testFn: checkGenerateComposefile,
		},
		{
			filePath: filepath.ToSlash(composefiles[1]),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}, ServiceName: "svc", DockerfilePath: ""},
			},
			testFn: checkGenerateComposefile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateComposefileAssortment ensures that Lockfiles with an assortment of keys
// are correct.
func TestGenerateComposefileAssortment(t *testing.T) {
	t.Parallel()
	composefile := filepath.Join(generateComposeBaseDir, "assortment", "docker-compose.yml")
	dockerfiles := []string{
		filepath.ToSlash(filepath.Join(generateComposeBaseDir, "assortment", "build", "Dockerfile")),
		filepath.ToSlash(filepath.Join(generateComposeBaseDir, "assortment", "context", "Dockerfile")),
		filepath.ToSlash(filepath.Join(generateComposeBaseDir, "assortment", "dockerfile", "Dockerfile")),
	}
	flags := []string{fmt.Sprintf("--compose-files=%s", composefile)}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(composefile),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "golang", Tag: "latest"}, ServiceName: "build-svc", DockerfilePath: dockerfiles[0]},
				{Image: &generate.Image{Name: "node", Tag: "latest"}, ServiceName: "context-svc", DockerfilePath: dockerfiles[1]},
				{Image: &generate.Image{Name: "ubuntu", Tag: "latest"}, ServiceName: "dockerfile-svc", DockerfilePath: dockerfiles[2]},
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}, ServiceName: "image-svc", DockerfilePath: ""},
			},
			testFn: checkGenerateComposefile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateComposefileArgsDockerfileOverride ensures that build args in docker-compose
// files override args defined in Dockerfiles.
func TestGenerateComposefileArgsDockerfileOverride(t *testing.T) {
	t.Parallel()
	composefile := filepath.Join(generateComposeBaseDir, "args", "override", "docker-compose.yml")
	dockerfile := filepath.ToSlash(filepath.Join(generateComposeBaseDir, "args", "override", "Dockerfile"))
	flags := []string{fmt.Sprintf("--compose-files=%s", composefile)}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(composefile),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}, ServiceName: "svc", DockerfilePath: dockerfile},
			},
			testFn: checkGenerateComposefile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateComposefileArgsDockerfileEmpty ensures that build args in docker-compose
// files override empty args in Dockerfiles.
func TestGenerateComposefileArgsDockerfileEmpty(t *testing.T) {
	t.Parallel()
	composefile := filepath.Join(generateComposeBaseDir, "args", "empty", "docker-compose.yml")
	dockerfile := filepath.ToSlash(filepath.Join(generateComposeBaseDir, "args", "empty", "Dockerfile"))
	flags := []string{fmt.Sprintf("--compose-files=%s", composefile)}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(composefile),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}, ServiceName: "svc", DockerfilePath: dockerfile},
			},
			testFn: checkGenerateComposefile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateComposefileArgsDockerfileNoArg ensures that args defined in Dockerfiles
// but not in docker-compose files behave as though no docker-compose files exist.
func TestGenerateComposefileArgsDockerfileNoArg(t *testing.T) {
	t.Parallel()
	composefile := filepath.Join(generateComposeBaseDir, "args", "noarg", "docker-compose.yml")
	dockerfile := filepath.ToSlash(filepath.Join(generateComposeBaseDir, "args", "noarg", "Dockerfile"))
	flags := []string{fmt.Sprintf("--compose-files=%s", composefile)}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(composefile),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}, ServiceName: "svc", DockerfilePath: dockerfile},
			},
			testFn: checkGenerateComposefile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateComposefileSortDockerfiles ensures that Dockerfiles referenced
// in docker-compose files and the images in those Dockerfiles are sorted as
// required for rewriting.
func TestGenerateComposefileSortDockerfiles(t *testing.T) {
	t.Parallel()
	composefile := filepath.Join(generateComposeBaseDir, "sort", "docker-compose.yml")
	dockerfileOne := filepath.ToSlash(filepath.Join(generateComposeBaseDir, "sort", "sort", "Dockerfile-one"))
	dockerfileTwo := filepath.ToSlash(filepath.Join(generateComposeBaseDir, "sort", "sort", "Dockerfile-two"))
	flags := []string{fmt.Sprintf("--compose-files=%s", composefile)}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(composefile),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}, ServiceName: "svc-one", DockerfilePath: dockerfileOne},
				{Image: &generate.Image{Name: "golang", Tag: "latest"}, ServiceName: "svc-one", DockerfilePath: dockerfileOne},
				{Image: &generate.Image{Name: "ubuntu", Tag: "latest"}, ServiceName: "svc-two", DockerfilePath: dockerfileTwo},
				{Image: &generate.Image{Name: "java", Tag: "latest"}, ServiceName: "svc-two", DockerfilePath: dockerfileTwo},
			},
			testFn: checkGenerateComposefile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateComposefileAndDockerfileDuplicates ensures that Lockfiles do not
// include the same file twice.
func TestGenerateComposefileAndDockerfileDuplicates(t *testing.T) {
	t.Parallel()
	composefile := filepath.Join(generateBothBaseDir, "both", "docker-compose.yml")
	dockerfile := filepath.ToSlash(filepath.Join(generateBothBaseDir, "both", "both", "Dockerfile"))
	flags := []string{fmt.Sprintf("--compose-files=%s,%s", composefile, composefile),
		fmt.Sprintf("--dockerfiles=%s,%s", dockerfile, dockerfile)}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(dockerfile),
			wantImages: []generate.DockerfileImage{
				{Image: &generate.Image{Name: "ubuntu", Tag: "latest"}},
			},
			testFn: checkGenerateDockerfile,
		},
		{
			filePath: filepath.ToSlash(composefile),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "ubuntu", Tag: "latest"}, ServiceName: "both-svc", DockerfilePath: dockerfile},
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}, ServiceName: "image-svc", DockerfilePath: ""},
			},
			testFn: checkGenerateComposefile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateComposefileAbsPathDockerfile ensures that Dockerfiles referenced by
// absolute paths and relative paths in docker-compose files resolve to the same
// relative path to the current working directory in the Lockfile.
func TestGenerateComposefileAbsPathDockerfile(t *testing.T) {
	t.Parallel()
	composefile := filepath.Join(generateComposeBaseDir, "abspath", "docker-compose.yml")
	absBuildPath, err := filepath.Abs(filepath.Join(generateComposeBaseDir, "abspath", "abspath"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("TestGenerateComposefileAbsPathDockerfile_ABS_BUILD_PATH", absBuildPath); err != nil {
		t.Fatal(err)
	}
	dockerfile := filepath.ToSlash(filepath.Join(generateComposeBaseDir, "abspath", "abspath", "Dockerfile"))
	flags := []string{fmt.Sprintf("--compose-files=%s", composefile)}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(composefile),
			wantImages: []generate.ComposefileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}, ServiceName: "svc-one", DockerfilePath: dockerfile},
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}, ServiceName: "svc-two", DockerfilePath: dockerfile},
			},
			testFn: checkGenerateComposefile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateDockerfileArgsBuildStage ensures that previously defined build stages
// are not included in Lockfiles. For instance:
// # Dockerfile
// FROM busybox AS busy
// FROM busy AS anotherbusy
// should only parse the first 'busybox'.
func TestGenerateDockerfileArgsBuildStage(t *testing.T) {
	t.Parallel()
	dockerfile := filepath.Join(generateDockerBaseDir, "args", "buildstage", "Dockerfile")
	flags := []string{fmt.Sprintf("--dockerfiles=%s", dockerfile)}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(dockerfile),
			wantImages: []generate.DockerfileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}},
				{Image: &generate.Image{Name: "ubuntu", Tag: "latest"}},
			},
			testFn: checkGenerateDockerfile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateDockerfileArgsLocalArg ensures that args defined before from statements
// (aka global args) should not be overridden by args defined after from statements
// (aka local args).
func TestGenerateDockerfileArgsLocalArg(t *testing.T) {
	t.Parallel()
	dockerfile := filepath.Join(generateDockerBaseDir, "args", "localarg", "Dockerfile")
	flags := []string{fmt.Sprintf("--dockerfiles=%s", dockerfile)}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(dockerfile),
			wantImages: []generate.DockerfileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}},
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}},
			},
			testFn: checkGenerateDockerfile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateDockerfileMultipleDockerfiles ensures that Lockfiles from multiple
// Dockerfiles are correct.
func TestGenerateDockerfileMultipleDockerfiles(t *testing.T) {
	t.Parallel()
	dockerfiles := []string{filepath.Join(generateDockerBaseDir, "multiple", "DockerfileOne"),
		filepath.Join(generateDockerBaseDir, "multiple", "DockerfileTwo")}
	flags := []string{fmt.Sprintf("--dockerfiles=%s,%s", dockerfiles[0], dockerfiles[1])}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(dockerfiles[0]),
			wantImages: []generate.DockerfileImage{
				{Image: &generate.Image{Name: "ubuntu", Tag: "latest"}},
			},
			testFn: checkGenerateDockerfile,
		},
		{
			filePath: filepath.ToSlash(dockerfiles[1]),
			wantImages: []generate.DockerfileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}},
			},
			testFn: checkGenerateDockerfile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateDockerfileRecursive ensures Lockfiles from multiple Dockerfiles
// in subdirectories are correct.
func TestGenerateDockerfileRecursive(t *testing.T) {
	t.Parallel()
	recursiveBaseDir := filepath.Join(generateDockerBaseDir, "recursive")
	flags := []string{fmt.Sprintf("--base-dir=%s", recursiveBaseDir), "--dockerfile-recursive"}
	dockerfiles := []string{filepath.Join(generateDockerBaseDir, "recursive", "Dockerfile"),
		filepath.Join(generateDockerBaseDir, "recursive", "recursive", "Dockerfile")}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(dockerfiles[0]),
			wantImages: []generate.DockerfileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}},
			},
			testFn: checkGenerateDockerfile,
		},
		{
			filePath: filepath.ToSlash(dockerfiles[1]),
			wantImages: []generate.DockerfileImage{
				{Image: &generate.Image{Name: "ubuntu", Tag: "latest"}},
			},
			testFn: checkGenerateDockerfile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateDockerfileNoFileSpecified ensures Lockfiles include a Dockerfile
// in the base directory, if no other files are specified.
func TestGenerateDockerfileNoFileSpecified(t *testing.T) {
	t.Parallel()
	baseDir := filepath.Join(generateDockerBaseDir, "nofile")
	flags := []string{fmt.Sprintf("--base-dir=%s", baseDir)}
	dockerfile := filepath.Join(baseDir, "Dockerfile")
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(dockerfile),
			wantImages: []generate.DockerfileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}},
			},
			testFn: checkGenerateDockerfile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateDockerfileGlobs ensures Lockfiles include Dockerfiles files found
// via glob syntax.
func TestGenerateDockerfileGlobs(t *testing.T) {
	t.Parallel()
	globs := strings.Join([]string{filepath.Join(generateDockerBaseDir, "globs", "**", "Dockerfile"),
		filepath.Join(generateDockerBaseDir, "globs", "Dockerfile")}, ",")
	flags := []string{fmt.Sprintf("--dockerfile-globs=%s", globs)}
	dockerfiles := []string{filepath.Join(generateDockerBaseDir, "globs", "globs", "Dockerfile"),
		filepath.Join(generateDockerBaseDir, "globs", "Dockerfile")}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(dockerfiles[0]),
			wantImages: []generate.DockerfileImage{
				{Image: &generate.Image{Name: "ubuntu", Tag: "latest"}},
			},
			testFn: checkGenerateDockerfile,
		},
		{
			filePath: filepath.ToSlash(dockerfiles[1]),
			wantImages: []generate.DockerfileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}},
			},
			testFn: checkGenerateDockerfile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

// TestGenerateDockerfileEnvBuildArgs ensures environment variables are used as
// build args.
func TestGenerateDockerfileEnvBuildArgs(t *testing.T) {
	t.Parallel()
	dockerfile := filepath.Join(generateDockerBaseDir, "args", "buildargs", "Dockerfile")
	envPath := filepath.Join(generateDockerBaseDir, "args", "buildargs", ".env")
	flags := []string{fmt.Sprintf("--dockerfiles=%s", dockerfile),
		fmt.Sprintf("--env-file=%s", envPath),
		fmt.Sprintf("--dockerfile-env-build-args")}
	tOs := []generateTestObject{
		{
			filePath: filepath.ToSlash(dockerfile),
			wantImages: []generate.DockerfileImage{
				{Image: &generate.Image{Name: "busybox", Tag: "latest"}},
			},
			testFn: checkGenerateDockerfile,
		},
	}
	var shouldFail bool
	testGenerate(t, flags, tOs, shouldFail)
}

func TestGenerateInvalidInput(t *testing.T) {
	t.Parallel()
	absPath, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	absPath = filepath.FromSlash(absPath)
	flagsSlice := [][]string{
		{fmt.Sprintf("--base-dir=%s", filepath.Join("..", "Dockerfile"))},
		{fmt.Sprintf("--base-dir=%s", filepath.Join(absPath, "Dockerfile"))},
		{fmt.Sprintf("--base-dir=%s", filepath.Join(".", "not-sub-dir"))},
		{fmt.Sprintf("--dockerfiles=%s", filepath.Join("..", "Dockerfile"))},
		{fmt.Sprintf("--dockerfiles=%s", filepath.Join(absPath, "Dockerfile"))},
		{fmt.Sprintf("--compose-files=%s", filepath.Join("..", "docker-compose.yml"))},
		{fmt.Sprintf("--compose-files=%s", filepath.Join(absPath, "docker-compose.yml"))},
		{fmt.Sprintf("--compose-file-globs=%s", filepath.Join(absPath, "**/docker-compose.yml"))},
		{fmt.Sprintf("--dockerfile-globs=%s", filepath.Join(absPath, "**/Dockerfile"))},
	}
	shouldFail := true
	var tOs []generateTestObject
	for _, flags := range flagsSlice {
		testGenerate(t, flags, tOs, shouldFail)
	}
}

func testGenerate(t *testing.T, flags []string, tOs []generateTestObject, shouldFail bool) {
	lName := uuid.New().String()
	generateCmd := cmd.NewGenerateCmd(client)
	generateCmd.SilenceUsage = true
	generateCmd.SilenceErrors = true
	args := append([]string{"lock", "generate", fmt.Sprintf("--lockfile-name=%s", lName)}, flags...)
	generateCmd.SetArgs(args)
	err := generateCmd.Execute()
	defer os.Remove(lName)
	switch {
	case shouldFail && err == nil:
		t.Fatalf("Got pass. Want fail.")
	case !shouldFail && err != nil:
		t.Fatal(err)
	case shouldFail && err != nil:
		return
	}
	lByt, err := ioutil.ReadFile(lName)
	if err != nil {
		t.Fatal(err)
	}
	var lFile generate.Lockfile
	if err := json.Unmarshal(lByt, &lFile); err != nil {
		t.Fatal(err)
	}
	numFiles := len(lFile.ComposefileImages) + len(lFile.DockerfileImages)
	if numFiles != len(tOs) {
		t.Fatalf("Got '%d' files in the Lockfile. Want '%d'.", numFiles, len(tOs))
	}
	for _, tO := range tOs {
		tO.testFn(t, tO, lFile)
	}
}

func checkGenerateComposefile(t *testing.T, tO generateTestObject, lFile generate.Lockfile) {
	wantImages := tO.wantImages.([]generate.ComposefileImage)
	fImages, ok := lFile.ComposefileImages[tO.filePath]
	if !ok {
		t.Fatalf("Want '%s' filepath, but did not find it.", tO.filePath)
	}
	if len(fImages) != len(wantImages) {
		t.Fatalf("Got '%d' images for '%s'. Want '%d'.", len(fImages), tO.filePath, len(wantImages))
	}
	for i, fImage := range fImages {
		if wantImages[i].Image.Name != fImage.Image.Name ||
			wantImages[i].Image.Tag != fImage.Image.Tag {
			t.Fatalf("Got '%s:%s'. Want '%s:%s'.",
				fImage.Image.Name,
				fImage.Image.Tag,
				wantImages[i].Image.Name,
				wantImages[i].Image.Tag)
		}
		if fImage.Image.Digest == "" {
			t.Fatalf("Want digest. Got image with empty digest %+v.", fImage)
		}
		if fImage.ServiceName != wantImages[i].ServiceName {
			t.Fatalf("Got '%s' service. Want '%s'.",
				fImage.ServiceName,
				wantImages[i].ServiceName)
		}
		if fImage.DockerfilePath != wantImages[i].DockerfilePath {
			t.Fatalf("Got '%s' dockerfile. Want '%s'.",
				filepath.FromSlash(fImage.DockerfilePath),
				wantImages[i].DockerfilePath)
		}
	}
}

func checkGenerateDockerfile(t *testing.T, tO generateTestObject, lFile generate.Lockfile) {
	wantImages := tO.wantImages.([]generate.DockerfileImage)
	fImages, ok := lFile.DockerfileImages[tO.filePath]
	if !ok {
		t.Fatalf("Got '%s' filepath, but did not find it.", tO.filePath)
	}
	if len(fImages) != len(wantImages) {
		t.Fatalf("Got '%d' images for '%s'. Want '%d'.", len(fImages), tO.filePath, len(wantImages))
	}
	for i, fImage := range fImages {
		if wantImages[i].Image.Name != fImage.Image.Name ||
			wantImages[i].Image.Tag != fImage.Image.Tag {
			t.Fatalf("Got '%s:%s'. Want '%s:%s'.",
				fImage.Image.Name,
				fImage.Image.Tag,
				wantImages[i].Image.Name,
				wantImages[i].Image.Tag)
		}
		if fImage.Image.Digest == "" {
			t.Fatalf("Want digest. Got image with empty digest %+v.", fImage)
		}
	}
}
