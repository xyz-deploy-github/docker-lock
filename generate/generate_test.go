package generate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
	"github.com/michaelperel/docker-lock/registry"
	"github.com/michaelperel/docker-lock/registry/contrib"
	"github.com/michaelperel/docker-lock/registry/firstparty"
)

type test struct {
	flags *Flags
	want  *Lockfile
}

var dTestDir = filepath.Join("testdata", "docker")  //nolint: gochecknoglobals
var cTestDir = filepath.Join("testdata", "compose") // nolint: gochecknoglobals
var bTestDir = filepath.Join("testdata", "both")    // nolint: gochecknoglobals

func TestGenerator(t *testing.T) {
	tests, err := getTests()
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			g, err := NewGenerator(tc.flags)
			if err != nil {
				t.Fatal(err)
			}
			got, err := writeLockfile(g)
			if err != nil {
				t.Fatal(err)
			}
			if err := compareLockfiles(got, tc.want); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Dockerfile tests

// dBuildStage ensures that previously defined build stages
// are not included in Lockfiles. For instance:
// # Dockerfile
// FROM busybox AS busy
// FROM busy AS anotherbusy
// should only parse the first 'busybox'.
func dBuildStage() (*test, error) {
	dockerfiles := []string{
		filepath.Join(dTestDir, "buildstage", "Dockerfile"),
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		dockerfiles, []string{}, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(map[string][]*DockerfileImage{
		filepath.ToSlash(dockerfiles[0]): {
			{Image: &Image{Name: "busybox", Tag: "latest"}},
			{Image: &Image{Name: "ubuntu", Tag: "latest"}},
		}}, nil,
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// dLocalArg ensures that args defined before from statements
// (aka global args) should not be overridden by args defined after
// from statements (aka local args).
func dLocalArg() (*test, error) {
	dockerfiles := []string{
		filepath.Join(dTestDir, "localarg", "Dockerfile"),
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		dockerfiles, []string{}, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(map[string][]*DockerfileImage{
		filepath.ToSlash(dockerfiles[0]): {
			{Image: &Image{Name: "busybox", Tag: "latest"}},
			{Image: &Image{Name: "busybox", Tag: "latest"}},
		}}, nil,
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// dMultiple ensures that Lockfiles from multiple Dockerfiles are correct.
func dMultiple() (*test, error) {
	dockerfiles := []string{
		filepath.Join(dTestDir, "multiple", "DockerfileOne"),
		filepath.Join(dTestDir, "multiple", "DockerfileTwo"),
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		dockerfiles, []string{}, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(map[string][]*DockerfileImage{
		filepath.ToSlash(dockerfiles[0]): {
			{Image: &Image{Name: "ubuntu", Tag: "latest"}},
		},
		filepath.ToSlash(dockerfiles[1]): {
			{Image: &Image{Name: "busybox", Tag: "latest"}},
		},
	}, nil,
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// dRecursive ensures Lockfiles from multiple Dockerfiles
// in subdirectories are correct.
func dRecursive() (*test, error) {
	dockerfiles := []string{
		filepath.Join(dTestDir, "recursive", "Dockerfile"),
		filepath.Join(dTestDir, "recursive", "recursive", "Dockerfile"),
	}
	recursiveBaseDir := filepath.Join(dTestDir, "recursive")
	flags, err := NewFlags(
		recursiveBaseDir, "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, []string{}, []string{}, []string{},
		true, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(map[string][]*DockerfileImage{
		filepath.ToSlash(dockerfiles[0]): {
			{Image: &Image{Name: "busybox", Tag: "latest"}},
		},
		filepath.ToSlash(dockerfiles[1]): {
			{Image: &Image{Name: "ubuntu", Tag: "latest"}},
		},
	}, nil,
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// dGlobs ensures Lockfiles include Dockerfiles files found via glob syntax.
func dGlobs() (*test, error) {
	globs := []string{
		filepath.Join(dTestDir, "globs", "**", "Dockerfile"),
		filepath.Join(dTestDir, "globs", "Dockerfile"),
	}
	dockerfiles := []string{
		filepath.Join(dTestDir, "globs", "globs", "Dockerfile"),
		filepath.Join(dTestDir, "globs", "Dockerfile"),
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, []string{}, globs, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(map[string][]*DockerfileImage{
		filepath.ToSlash(dockerfiles[0]): {
			{Image: &Image{Name: "ubuntu", Tag: "latest"}},
		},
		filepath.ToSlash(dockerfiles[1]): {
			{Image: &Image{Name: "busybox", Tag: "latest"}},
		},
	}, nil,
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// dBuildArgs ensures environment variables can be used as build args.
func dBuildArgs() (*test, error) {
	dockerfiles := []string{
		filepath.Join(dTestDir, "buildargs", "Dockerfile"),
	}
	envPath := filepath.ToSlash(
		filepath.Join(dTestDir, "buildargs", ".env"),
	)
	err := godotenv.Load(envPath)
	if err != nil {
		return nil, err
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), envPath,
		dockerfiles, []string{}, []string{}, []string{},
		false, false, true,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(map[string][]*DockerfileImage{
		filepath.ToSlash(dockerfiles[0]): {
			{Image: &Image{Name: "busybox", Tag: "latest"}},
		}}, nil,
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// dNoFile ensures Lockfiles include a Dockerfile in the base directory,
// if no other files are specified.
func dNoFile() (*test, error) {
	baseDir := filepath.Join(dTestDir, "nofile")
	dockerfiles := []string{
		filepath.Join(baseDir, "Dockerfile"),
	}
	flags, err := NewFlags(
		baseDir, "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, []string{}, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(map[string][]*DockerfileImage{
		filepath.ToSlash(dockerfiles[0]): {
			{Image: &Image{Name: "busybox", Tag: "latest"}},
		}}, nil,
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// Composefile tests

// cImage ensures Lockfiles from docker-compose files with
// the image key are correct.
func cImage() (*test, error) {
	composefiles := []string{
		filepath.Join(cTestDir, "image", "docker-compose.yml"),
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, composefiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(composefiles[0]): {
			{
				Image:          &Image{Name: "busybox", Tag: "latest"},
				ServiceName:    "svc",
				DockerfilePath: "",
			},
		},
	},
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// cBuild ensures Lockfiles from docker-compose files with
// the build key are correct.
func cBuild() (*test, error) {
	composefiles := []string{
		filepath.Join(cTestDir, "build", "docker-compose.yml"),
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, composefiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(composefiles[0]): {
			{
				Image:       &Image{Name: "busybox", Tag: "latest"},
				ServiceName: "svc",
				DockerfilePath: filepath.ToSlash(
					filepath.Join(cTestDir, "build", "build", "Dockerfile"),
				),
			},
		},
	},
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// cDockerfile ensures Lockfiles from docker-compose files with
// the dockerfile key are correct.
func cDockerfile() (*test, error) {
	composefiles := []string{
		filepath.Join(cTestDir, "dockerfile", "docker-compose.yml"),
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, composefiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(composefiles[0]): {
			{
				Image:       &Image{Name: "busybox", Tag: "latest"},
				ServiceName: "svc",
				DockerfilePath: filepath.ToSlash(
					filepath.Join(
						cTestDir, "dockerfile", "dockerfile", "Dockerfile",
					),
				),
			},
		},
	},
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// cContext ensures Lockfiles from docker-compose files with
// the context key are correct.
func cContext() (*test, error) {
	composefiles := []string{
		filepath.Join(cTestDir, "context", "docker-compose.yml"),
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, composefiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(composefiles[0]): {
			{
				Image:       &Image{Name: "busybox", Tag: "latest"},
				ServiceName: "svc",
				DockerfilePath: filepath.ToSlash(
					filepath.Join(
						cTestDir, "context", "context", "Dockerfile",
					),
				),
			},
		},
	},
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// cEnv ensures Lockfiles from docker-compose files with
// environment variables replaced by values in a .env file are correct.
func cEnv() (*test, error) {
	composefiles := []string{
		filepath.Join(cTestDir, "env", "docker-compose.yml"),
	}
	envPath := filepath.Join(cTestDir, "env", ".env")
	err := godotenv.Load(envPath)
	if err != nil {
		return nil, err
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), envPath,
		[]string{}, composefiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(composefiles[0]): {
			{
				Image:       &Image{Name: "busybox", Tag: "latest"},
				ServiceName: "svc",
				DockerfilePath: filepath.ToSlash(
					filepath.Join(
						cTestDir, "env", "env", "Dockerfile",
					),
				),
			},
		},
	},
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// cArgsOverride ensures that build args in docker-compose
// files override args defined in Dockerfiles.
func cArgsOverride() (*test, error) {
	composefiles := []string{
		filepath.Join(cTestDir, "args", "override", "docker-compose.yml"),
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, composefiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(composefiles[0]): {
			{
				Image:       &Image{Name: "busybox", Tag: "latest"},
				ServiceName: "svc",
				DockerfilePath: filepath.ToSlash(
					filepath.Join(cTestDir, "args", "override", "Dockerfile"),
				),
			},
		},
	},
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// cArgsEmpty ensures that build args in docker-compose
// files override empty args in Dockerfiles.
func cArgsEmpty() (*test, error) {
	composefiles := []string{
		filepath.Join(cTestDir, "args", "empty", "docker-compose.yml"),
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, composefiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(composefiles[0]): {
			{
				Image:       &Image{Name: "busybox", Tag: "latest"},
				ServiceName: "svc",
				DockerfilePath: filepath.ToSlash(
					filepath.Join(cTestDir, "args", "empty", "Dockerfile"),
				),
			},
		},
	},
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// cArgsNoArg ensures that args defined in Dockerfiles
// but not in docker-compose files behave as though no docker-compose
// files exist.
func cArgsNoArg() (*test, error) {
	composefiles := []string{
		filepath.Join(cTestDir, "args", "noarg", "docker-compose.yml"),
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, composefiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(composefiles[0]): {
			{
				Image:       &Image{Name: "busybox", Tag: "latest"},
				ServiceName: "svc",
				DockerfilePath: filepath.ToSlash(
					filepath.Join(cTestDir, "args", "noarg", "Dockerfile"),
				),
			},
		},
	},
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// cMultiple ensures Lockfiles from multiple docker-compose files are correct.
func cMultiple() (*test, error) {
	composefiles := []string{
		filepath.Join(cTestDir, "multiple", "docker-compose-one.yml"),
		filepath.Join(cTestDir, "multiple", "docker-compose-two.yml"),
	}
	dockerfiles := []string{
		filepath.ToSlash(filepath.Join(
			cTestDir, "multiple", "build", "Dockerfile"),
		),
		filepath.ToSlash(filepath.Join(
			cTestDir, "multiple", "context", "Dockerfile"),
		),
		filepath.ToSlash(filepath.Join(
			cTestDir, "multiple", "dockerfile", "Dockerfile"),
		),
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, composefiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(composefiles[0]): {
			{
				Image:          &Image{Name: "ubuntu", Tag: "latest"},
				ServiceName:    "build-svc",
				DockerfilePath: dockerfiles[0],
			},
			{
				Image:          &Image{Name: "busybox", Tag: "latest"},
				ServiceName:    "image-svc",
				DockerfilePath: "",
			},
		},
		filepath.ToSlash(composefiles[1]): {
			{
				Image:          &Image{Name: "node", Tag: "latest"},
				ServiceName:    "context-svc",
				DockerfilePath: dockerfiles[1],
			},
			{
				Image:          &Image{Name: "golang", Tag: "latest"},
				ServiceName:    "dockerfile-svc",
				DockerfilePath: dockerfiles[2],
			},
		},
	},
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// cRecursive ensures Lockfiles from multiple docker-compose
// files in subdirectories are correct.
func cRecursive() (*test, error) {
	composefiles := []string{
		filepath.ToSlash(
			filepath.Join(cTestDir, "recursive", "docker-compose.yml"),
		),
		filepath.ToSlash(
			filepath.Join(cTestDir, "recursive", "build", "docker-compose.yml"),
		),
	}
	dockerfile := filepath.ToSlash(filepath.Join(
		cTestDir, "recursive", "build", "build", "Dockerfile"),
	)
	bDir := filepath.Join(cTestDir, "recursive")
	flags, err := NewFlags(
		bDir, "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, []string{}, []string{}, []string{},
		false, true, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(nil, map[string][]*ComposefileImage{
		composefiles[0]: {
			{
				Image:          &Image{Name: "golang", Tag: "latest"},
				ServiceName:    "svc",
				DockerfilePath: "",
			},
		},
		composefiles[1]: {
			{
				Image:          &Image{Name: "busybox", Tag: "latest"},
				ServiceName:    "svc",
				DockerfilePath: dockerfile,
			},
		},
	},
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// cNoFile ensures Lockfiles include docker-compose.yml and docker-compose.yaml
// files in the base directory, if no other files are specified.
func cNoFile() (*test, error) {
	bDir := filepath.Join(cTestDir, "nofile")
	composefiles := []string{
		filepath.ToSlash(filepath.Join(bDir, "docker-compose.yml")),
		filepath.ToSlash(filepath.Join(bDir, "docker-compose.yaml")),
	}
	flags, err := NewFlags(
		bDir, "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, []string{}, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(nil, map[string][]*ComposefileImage{
		composefiles[0]: {
			{
				Image:          &Image{Name: "busybox", Tag: "latest"},
				ServiceName:    "svc",
				DockerfilePath: "",
			},
			{
				Image:          &Image{Name: "golang", Tag: "latest"},
				ServiceName:    "svc",
				DockerfilePath: "",
			},
		},
		composefiles[1]: {
			{
				Image:          &Image{Name: "golang", Tag: "latest"},
				ServiceName:    "svc",
				DockerfilePath: "",
			},
		},
	},
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// cGlobs ensures Lockfiles include docker-compose files found
// via glob syntax.
func cGlobs() (*test, error) {
	globs := []string{
		filepath.Join(cTestDir, "globs", "**", "docker-compose.yml"),
		filepath.Join(cTestDir, "globs", "docker-compose.yml"),
	}
	composefiles := []string{
		filepath.ToSlash(
			filepath.Join(cTestDir, "globs", "image", "docker-compose.yml"),
		),
		filepath.ToSlash(
			filepath.Join(cTestDir, "globs", "docker-compose.yml"),
		),
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, []string{}, []string{}, globs,
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(nil, map[string][]*ComposefileImage{
		composefiles[0]: {
			{
				Image:          &Image{Name: "ubuntu", Tag: "latest"},
				ServiceName:    "svc",
				DockerfilePath: "",
			},
		},
		composefiles[1]: {
			{
				Image:          &Image{Name: "busybox", Tag: "latest"},
				ServiceName:    "svc",
				DockerfilePath: "",
			},
		},
	},
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// cAssortment ensures that Lockfiles from an assortment of keys
// are correct.
func cAssortment() (*test, error) {
	composefiles := []string{
		filepath.Join(cTestDir, "assortment", "docker-compose.yml"),
	}
	dockerfiles := []string{
		filepath.ToSlash(
			filepath.Join(cTestDir, "assortment", "build", "Dockerfile"),
		),
		filepath.ToSlash(
			filepath.Join(cTestDir, "assortment", "context", "Dockerfile"),
		),
		filepath.ToSlash(
			filepath.Join(cTestDir, "assortment", "dockerfile", "Dockerfile"),
		),
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, composefiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(composefiles[0]): {
			{
				Image:          &Image{Name: "golang", Tag: "latest"},
				ServiceName:    "build-svc",
				DockerfilePath: dockerfiles[0],
			},
			{
				Image:          &Image{Name: "node", Tag: "latest"},
				ServiceName:    "context-svc",
				DockerfilePath: dockerfiles[1],
			},
			{
				Image:          &Image{Name: "ubuntu", Tag: "latest"},
				ServiceName:    "dockerfile-svc",
				DockerfilePath: dockerfiles[2],
			},
			{
				Image:          &Image{Name: "busybox", Tag: "latest"},
				ServiceName:    "image-svc",
				DockerfilePath: "",
			},
		},
	},
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// cSort ensures that Dockerfiles referenced in docker-compose files
// and the images in those Dockerfiles are sorted as required for rewriting.
func cSort() (*test, error) {
	composefiles := []string{
		filepath.Join(cTestDir, "sort", "docker-compose.yml"),
	}
	dockerfiles := []string{
		filepath.ToSlash(filepath.Join(
			cTestDir, "sort", "sort", "Dockerfile-one"),
		),
		filepath.ToSlash(filepath.Join(
			cTestDir, "sort", "sort", "Dockerfile-two"),
		),
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, composefiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(composefiles[0]): {
			{
				Image:          &Image{Name: "busybox", Tag: "latest"},
				ServiceName:    "svc-one",
				DockerfilePath: dockerfiles[0],
			},
			{
				Image:          &Image{Name: "golang", Tag: "latest"},
				ServiceName:    "svc-one",
				DockerfilePath: dockerfiles[0],
			},
			{
				Image:          &Image{Name: "ubuntu", Tag: "latest"},
				ServiceName:    "svc-two",
				DockerfilePath: dockerfiles[1],
			},
			{
				Image:          &Image{Name: "java", Tag: "latest"},
				ServiceName:    "svc-two",
				DockerfilePath: dockerfiles[1],
			},
		},
	},
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// cAbsPathDockerfile ensures that Dockerfiles referenced by
// absolute paths and relative paths in docker-compose files resolve to the same
// relative path to the current working directory in the Lockfile.
func cAbsPathDockerfile() (*test, error) {
	composefiles := []string{
		filepath.Join(cTestDir, "abspath", "docker-compose.yml"),
	}
	dockerfiles := []string{
		filepath.ToSlash(filepath.Join(
			cTestDir, "abspath", "abspath", "Dockerfile"),
		),
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, composefiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	absBuildPath, err := filepath.Abs(
		filepath.Join(cTestDir, "abspath", "abspath"),
	)
	if err != nil {
		return nil, err
	}
	if err := os.Setenv(
		"TestGenerateComposefileAbsPathDockerfile_ABS_BUILD_PATH",
		absBuildPath); err != nil {
		return nil, err
	}
	lockfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(composefiles[0]): {
			{
				Image:          &Image{Name: "busybox", Tag: "latest"},
				ServiceName:    "svc-one",
				DockerfilePath: dockerfiles[0],
			},
			{
				Image:          &Image{Name: "busybox", Tag: "latest"},
				ServiceName:    "svc-two",
				DockerfilePath: dockerfiles[0],
			},
		},
	},
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// Both tests

// bDuplicates ensures that Lockfiles do not include the same file twice.
func bDuplicates() (*test, error) {
	composefiles := []string{
		filepath.Join(bTestDir, "docker-compose.yml"),
		filepath.Join(bTestDir, "docker-compose.yml"),
	}
	dockerfiles := []string{
		filepath.Join(bTestDir, "both", "Dockerfile"),
		filepath.Join(bTestDir, "both", "Dockerfile"),
	}
	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		dockerfiles, composefiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}
	lockfile := NewLockfile(
		map[string][]*DockerfileImage{
			filepath.ToSlash(dockerfiles[0]): {
				{Image: &Image{Name: "ubuntu", Tag: "latest"}},
			},
		},
		map[string][]*ComposefileImage{
			filepath.ToSlash(composefiles[0]): {
				{
					Image:          &Image{Name: "ubuntu", Tag: "latest"},
					ServiceName:    "both-svc",
					DockerfilePath: filepath.ToSlash(dockerfiles[0]),
				},
				{
					Image:          &Image{Name: "busybox", Tag: "latest"},
					ServiceName:    "image-svc",
					DockerfilePath: "",
				},
			},
		},
	)
	return &test{
		flags: flags,
		want:  lockfile,
	}, nil
}

// Helpers

func getTests() (map[string]*test, error) {
	dBuildStage, err := dBuildStage()
	if err != nil {
		return nil, err
	}
	dLocalArg, err := dLocalArg()
	if err != nil {
		return nil, err
	}
	dMultiple, err := dMultiple()
	if err != nil {
		return nil, err
	}
	dRecursive, err := dRecursive()
	if err != nil {
		return nil, err
	}
	dNoFile, err := dNoFile()
	if err != nil {
		return nil, err
	}
	dGlobs, err := dGlobs()
	if err != nil {
		return nil, err
	}
	dBuildArg, err := dBuildArgs()
	if err != nil {
		return nil, err
	}
	cImage, err := cImage()
	if err != nil {
		return nil, err
	}
	cBuild, err := cBuild()
	if err != nil {
		return nil, err
	}
	cDockerfile, err := cDockerfile()
	if err != nil {
		return nil, err
	}
	cContext, err := cContext()
	if err != nil {
		return nil, err
	}
	cEnv, err := cEnv()
	if err != nil {
		return nil, err
	}
	cMultiple, err := cMultiple()
	if err != nil {
		return nil, err
	}
	cRecursive, err := cRecursive()
	if err != nil {
		return nil, err
	}
	cNoFile, err := cNoFile()
	if err != nil {
		return nil, err
	}
	cGlobs, err := cGlobs()
	if err != nil {
		return nil, err
	}
	cAssortment, err := cAssortment()
	if err != nil {
		return nil, err
	}
	cArgsOverride, err := cArgsOverride()
	if err != nil {
		return nil, err
	}
	cArgsEmpty, err := cArgsEmpty()
	if err != nil {
		return nil, err
	}
	cArgsNoArg, err := cArgsNoArg()
	if err != nil {
		return nil, err
	}
	cSort, err := cSort()
	if err != nil {
		return nil, err
	}
	bDuplicates, err := bDuplicates()
	if err != nil {
		return nil, err
	}
	cAbsPathDockerfile, err := cAbsPathDockerfile()
	if err != nil {
		return nil, err
	}
	tests := map[string]*test{
		"dBuildstage":        dBuildStage,
		"dLocalarg":          dLocalArg,
		"dMultiple":          dMultiple,
		"dRecursive":         dRecursive,
		"dNoFile":            dNoFile,
		"dGlobs":             dGlobs,
		"dBuildArg":          dBuildArg,
		"cImage":             cImage,
		"cBuild":             cBuild,
		"cDockerfile":        cDockerfile,
		"cContext":           cContext,
		"cEnv":               cEnv,
		"cMultiple":          cMultiple,
		"cRecursive":         cRecursive,
		"cNoFile":            cNoFile,
		"cGlobs":             cGlobs,
		"cAssortment":        cAssortment,
		"cArgsOverride":      cArgsOverride,
		"cArgsEmpty":         cArgsEmpty,
		"cArgsNoArg":         cArgsNoArg,
		"cSort":              cSort,
		"bDuplicates":        bDuplicates,
		"cAbsPathDockerfile": cAbsPathDockerfile,
	}
	return tests, nil
}

func compareLockfiles(got, want *Lockfile) error {
	if err := compareDockerfilePaths(
		got.DockerfileImages, want.DockerfileImages,
	); err != nil {
		return err
	}
	if err := compareComposefilePaths(
		got.ComposefileImages, want.ComposefileImages,
	); err != nil {
		return err
	}
	if err := compareDockerfileImages(
		got.DockerfileImages, want.DockerfileImages,
	); err != nil {
		return err
	}
	if err := compareComposefileImages(
		got.ComposefileImages, want.ComposefileImages,
	); err != nil {
		return err
	}
	return nil
}

func compareDockerfilePaths(got, want map[string][]*DockerfileImage) error {
	allSets := make([]map[string]struct{}, 2)
	for i, m := range []map[string][]*DockerfileImage{got, want} {
		set := map[string]struct{}{}
		for p := range m {
			set[p] = struct{}{}
		}
		allSets[i] = set
	}
	return compareFilePaths(allSets[0], allSets[1])
}

func compareComposefilePaths(got, want map[string][]*ComposefileImage) error {
	allSets := make([]map[string]struct{}, 2)
	for i, m := range []map[string][]*ComposefileImage{got, want} {
		set := map[string]struct{}{}
		for p := range m {
			set[p] = struct{}{}
		}
		allSets[i] = set
	}
	return compareFilePaths(allSets[0], allSets[1])
}

func compareFilePaths(got, want map[string]struct{}) error {
	if len(got) != len(want) {
		return fmt.Errorf(
			"got '%d' files, want '%d'", len(got), len(want),
		)
	}
	for p := range got {
		if _, ok := want[p]; !ok {
			return fmt.Errorf(
				"got '%s' files, want '%s'", got, want,
			)
		}
	}
	return nil
}

func compareDockerfileImages(got, want map[string][]*DockerfileImage) error {
	for p := range got {
		for i := range got[p] {
			if err := compareImages(
				got[p][i].Image, want[p][i].Image,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func compareComposefileImages(got, want map[string][]*ComposefileImage) error {
	for p := range got {
		for i := range got[p] {
			if err := compareImages(
				got[p][i].Image, want[p][i].Image,
			); err != nil {
				return err
			}
			if got[p][i].ServiceName != want[p][i].ServiceName {
				return fmt.Errorf(
					"got '%s' service, want '%s'",
					got[p][i].ServiceName,
					want[p][i].ServiceName,
				)
			}
			if got[p][i].DockerfilePath != want[p][i].DockerfilePath {
				return fmt.Errorf(
					"got '%s' dockerfile, want '%s'",
					filepath.FromSlash(got[p][i].DockerfilePath),
					want[p][i].DockerfilePath,
				)
			}
		}
	}
	return nil
}

func compareImages(got, want *Image) error {
	if got.Name != want.Name {
		return fmt.Errorf(
			"got '%s', want '%s'", got.Name, want.Name,
		)
	}
	if got.Tag != want.Tag {
		return fmt.Errorf(
			"got '%s', want '%s'", got.Tag, want.Tag,
		)
	}
	if got.Digest == "" {
		return fmt.Errorf(
			"got '%s', want digest", got.Digest,
		)
	}
	return nil
}

func writeLockfile(g *Generator) (*Lockfile, error) {
	configPath := getDefaultConfigPath()
	wm, err := getDefaultWrapperManager(configPath, client)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err = g.GenerateLockfile(wm, &buf); err != nil {
		return nil, err
	}
	var lFile Lockfile
	if err = json.Unmarshal(buf.Bytes(), &lFile); err != nil {
		return nil, err
	}
	return &lFile, err
}

func getDefaultWrapperManager(
	configPath string,
	client *registry.HTTPClient,
) (*registry.WrapperManager, error) {
	defaultWrapper, err := firstparty.GetDefaultWrapper(configPath, client)
	if err != nil {
		return nil, err
	}
	wrapperManager := registry.NewWrapperManager(defaultWrapper)
	firstPartyWrappers, err := firstparty.GetAllWrappers(configPath, client)
	if err != nil {
		return nil, err
	}
	contribWrappers, err := contrib.GetAllWrappers(client)
	if err != nil {
		return nil, err
	}
	wrapperManager.Add(firstPartyWrappers...)
	wrapperManager.Add(contribWrappers...)
	return wrapperManager, nil
}

func getDefaultConfigPath() string {
	if homeDir, err := os.UserHomeDir(); err == nil {
		cPath := filepath.ToSlash(
			filepath.Join(homeDir, ".docker", "config.json"),
		)
		if _, err := os.Stat(cPath); err != nil {
			return ""
		}
		return cPath
	}
	return ""
}
