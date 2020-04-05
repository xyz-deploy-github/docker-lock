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

			got, err := writeLfile(g)
			if err != nil {
				t.Fatal(err)
			}

			if err := cmpLfiles(got, tc.want); err != nil {
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
	dfiles := []string{
		filepath.Join(dTestDir, "buildstage", "Dockerfile"),
	}

	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		dfiles, []string{}, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(map[string][]*DockerfileImage{
		filepath.ToSlash(dfiles[0]): {
			{Image: &Image{Name: "busybox", Tag: "latest"}},
			{Image: &Image{Name: "ubuntu", Tag: "latest"}},
		}}, nil,
	)

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// dLocalArg ensures that args defined before from statements
// (aka global args) should not be overridden by args defined after
// from statements (aka local args).
func dLocalArg() (*test, error) {
	dfiles := []string{
		filepath.Join(dTestDir, "localarg", "Dockerfile"),
	}

	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		dfiles, []string{}, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(map[string][]*DockerfileImage{
		filepath.ToSlash(dfiles[0]): {
			{Image: &Image{Name: "busybox", Tag: "latest"}},
			{Image: &Image{Name: "busybox", Tag: "latest"}},
		}}, nil,
	)

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// dMultiple ensures that Lockfiles from multiple Dockerfiles are correct.
func dMultiple() (*test, error) {
	dfiles := []string{
		filepath.Join(dTestDir, "multiple", "DockerfileOne"),
		filepath.Join(dTestDir, "multiple", "DockerfileTwo"),
	}

	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		dfiles, []string{}, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(map[string][]*DockerfileImage{
		filepath.ToSlash(dfiles[0]): {
			{Image: &Image{Name: "ubuntu", Tag: "latest"}},
		},
		filepath.ToSlash(dfiles[1]): {
			{Image: &Image{Name: "busybox", Tag: "latest"}},
		},
	}, nil,
	)

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// dRecursive ensures Lockfiles from multiple Dockerfiles
// in subdirectories are correct.
func dRecursive() (*test, error) {
	dfiles := []string{
		filepath.Join(dTestDir, "recursive", "Dockerfile"),
		filepath.Join(dTestDir, "recursive", "recursive", "Dockerfile"),
	}
	rBDir := filepath.Join(dTestDir, "recursive")

	flags, err := NewFlags(
		rBDir, "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, []string{}, []string{}, []string{},
		true, false, false,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(map[string][]*DockerfileImage{
		filepath.ToSlash(dfiles[0]): {
			{Image: &Image{Name: "busybox", Tag: "latest"}},
		},
		filepath.ToSlash(dfiles[1]): {
			{Image: &Image{Name: "ubuntu", Tag: "latest"}},
		},
	}, nil,
	)

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// dGlobs ensures Lockfiles include Dockerfiles files found via glob syntax.
func dGlobs() (*test, error) {
	globs := []string{
		filepath.Join(dTestDir, "globs", "**", "Dockerfile"),
		filepath.Join(dTestDir, "globs", "Dockerfile"),
	}
	dfiles := []string{
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

	lfile := NewLockfile(map[string][]*DockerfileImage{
		filepath.ToSlash(dfiles[0]): {
			{Image: &Image{Name: "ubuntu", Tag: "latest"}},
		},
		filepath.ToSlash(dfiles[1]): {
			{Image: &Image{Name: "busybox", Tag: "latest"}},
		},
	}, nil,
	)

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// dBuildArgs ensures environment variables can be used as build args.
func dBuildArgs() (*test, error) {
	dfiles := []string{
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
		dfiles, []string{}, []string{}, []string{},
		false, false, true,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(map[string][]*DockerfileImage{
		filepath.ToSlash(dfiles[0]): {
			{Image: &Image{Name: "busybox", Tag: "latest"}},
		}}, nil,
	)

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// dNoFile ensures Lockfiles include a Dockerfile in the base directory,
// if no other files are specified.
func dNoFile() (*test, error) {
	bDir := filepath.Join(dTestDir, "nofile")
	dfiles := []string{
		filepath.Join(bDir, "Dockerfile"),
	}

	flags, err := NewFlags(
		bDir, "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, []string{}, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(map[string][]*DockerfileImage{
		filepath.ToSlash(dfiles[0]): {
			{Image: &Image{Name: "busybox", Tag: "latest"}},
		}}, nil,
	)

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// Composefile tests

// cImage ensures Lockfiles from docker-compose files with
// the image key are correct.
func cImage() (*test, error) {
	cfiles := []string{
		filepath.Join(cTestDir, "image", "docker-compose.yml"),
	}

	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, cfiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(cfiles[0]): {
			{
				Image:          &Image{Name: "busybox", Tag: "latest"},
				ServiceName:    "svc",
				DockerfilePath: "",
			},
		},
	})

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// cBuild ensures Lockfiles from docker-compose files with
// the build key are correct.
func cBuild() (*test, error) {
	cfiles := []string{
		filepath.Join(cTestDir, "build", "docker-compose.yml"),
	}

	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, cfiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(cfiles[0]): {
			{
				Image:       &Image{Name: "busybox", Tag: "latest"},
				ServiceName: "svc",
				DockerfilePath: filepath.ToSlash(
					filepath.Join(cTestDir, "build", "build", "Dockerfile"),
				),
			},
		},
	})

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// cDockerfile ensures Lockfiles from docker-compose files with
// the dockerfile key are correct.
func cDockerfile() (*test, error) {
	cfiles := []string{
		filepath.Join(cTestDir, "dockerfile", "docker-compose.yml"),
	}

	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, cfiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(cfiles[0]): {
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
	})

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// cContext ensures Lockfiles from docker-compose files with
// the context key are correct.
func cContext() (*test, error) {
	cfiles := []string{
		filepath.Join(cTestDir, "context", "docker-compose.yml"),
	}

	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, cfiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(cfiles[0]): {
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
	})

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// cEnv ensures Lockfiles from docker-compose files with
// environment variables replaced by values in a .env file are correct.
func cEnv() (*test, error) {
	cfiles := []string{
		filepath.Join(cTestDir, "env", "docker-compose.yml"),
	}
	envPath := filepath.Join(cTestDir, "env", ".env")

	err := godotenv.Load(envPath)
	if err != nil {
		return nil, err
	}

	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), envPath,
		[]string{}, cfiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(cfiles[0]): {
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
	})

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// cArgsOverride ensures that build args in docker-compose
// files override args defined in Dockerfiles.
func cArgsOverride() (*test, error) {
	cfiles := []string{
		filepath.Join(cTestDir, "args", "override", "docker-compose.yml"),
	}

	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, cfiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(cfiles[0]): {
			{
				Image:       &Image{Name: "busybox", Tag: "latest"},
				ServiceName: "svc",
				DockerfilePath: filepath.ToSlash(
					filepath.Join(cTestDir, "args", "override", "Dockerfile"),
				),
			},
		},
	})

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// cArgsEmpty ensures that build args in docker-compose
// files override empty args in Dockerfiles.
func cArgsEmpty() (*test, error) {
	cfiles := []string{
		filepath.Join(cTestDir, "args", "empty", "docker-compose.yml"),
	}

	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, cfiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(cfiles[0]): {
			{
				Image:       &Image{Name: "busybox", Tag: "latest"},
				ServiceName: "svc",
				DockerfilePath: filepath.ToSlash(
					filepath.Join(cTestDir, "args", "empty", "Dockerfile"),
				),
			},
		},
	})

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// cArgsNoArg ensures that args defined in Dockerfiles
// but not in docker-compose files behave as though no docker-compose
// files exist.
func cArgsNoArg() (*test, error) {
	cfiles := []string{
		filepath.Join(cTestDir, "args", "noarg", "docker-compose.yml"),
	}

	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, cfiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(cfiles[0]): {
			{
				Image:       &Image{Name: "busybox", Tag: "latest"},
				ServiceName: "svc",
				DockerfilePath: filepath.ToSlash(
					filepath.Join(cTestDir, "args", "noarg", "Dockerfile"),
				),
			},
		},
	})

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// cMultiple ensures Lockfiles from multiple docker-compose files are correct.
func cMultiple() (*test, error) {
	cfiles := []string{
		filepath.Join(cTestDir, "multiple", "docker-compose-one.yml"),
		filepath.Join(cTestDir, "multiple", "docker-compose-two.yml"),
	}
	dfiles := []string{
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
		[]string{}, cfiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(cfiles[0]): {
			{
				Image:          &Image{Name: "ubuntu", Tag: "latest"},
				ServiceName:    "build-svc",
				DockerfilePath: dfiles[0],
			},
			{
				Image:          &Image{Name: "busybox", Tag: "latest"},
				ServiceName:    "image-svc",
				DockerfilePath: "",
			},
		},
		filepath.ToSlash(cfiles[1]): {
			{
				Image:          &Image{Name: "node", Tag: "latest"},
				ServiceName:    "context-svc",
				DockerfilePath: dfiles[1],
			},
			{
				Image:          &Image{Name: "golang", Tag: "latest"},
				ServiceName:    "dockerfile-svc",
				DockerfilePath: dfiles[2],
			},
		},
	})

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// cRecursive ensures Lockfiles from multiple docker-compose
// files in subdirectories are correct.
func cRecursive() (*test, error) {
	cfiles := []string{
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

	lfile := NewLockfile(nil, map[string][]*ComposefileImage{
		cfiles[0]: {
			{
				Image:          &Image{Name: "golang", Tag: "latest"},
				ServiceName:    "svc",
				DockerfilePath: "",
			},
		},
		cfiles[1]: {
			{
				Image:          &Image{Name: "busybox", Tag: "latest"},
				ServiceName:    "svc",
				DockerfilePath: dockerfile,
			},
		},
	})

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// cNoFile ensures Lockfiles include docker-compose.yml and docker-compose.yaml
// files in the base directory, if no other files are specified.
func cNoFile() (*test, error) {
	bDir := filepath.Join(cTestDir, "nofile")
	cfiles := []string{
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

	lfile := NewLockfile(nil, map[string][]*ComposefileImage{
		cfiles[0]: {
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
		cfiles[1]: {
			{
				Image:          &Image{Name: "golang", Tag: "latest"},
				ServiceName:    "svc",
				DockerfilePath: "",
			},
		},
	})

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// cGlobs ensures Lockfiles include docker-compose files found
// via glob syntax.
func cGlobs() (*test, error) {
	globs := []string{
		filepath.Join(cTestDir, "globs", "**", "docker-compose.yml"),
		filepath.Join(cTestDir, "globs", "docker-compose.yml"),
	}
	cfiles := []string{
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

	lfile := NewLockfile(nil, map[string][]*ComposefileImage{
		cfiles[0]: {
			{
				Image:          &Image{Name: "ubuntu", Tag: "latest"},
				ServiceName:    "svc",
				DockerfilePath: "",
			},
		},
		cfiles[1]: {
			{
				Image:          &Image{Name: "busybox", Tag: "latest"},
				ServiceName:    "svc",
				DockerfilePath: "",
			},
		},
	})

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// cAssortment ensures that Lockfiles from an assortment of keys
// are correct.
func cAssortment() (*test, error) {
	cfiles := []string{
		filepath.Join(cTestDir, "assortment", "docker-compose.yml"),
	}
	dfiles := []string{
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
		[]string{}, cfiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(cfiles[0]): {
			{
				Image:          &Image{Name: "golang", Tag: "latest"},
				ServiceName:    "build-svc",
				DockerfilePath: dfiles[0],
			},
			{
				Image:          &Image{Name: "node", Tag: "latest"},
				ServiceName:    "context-svc",
				DockerfilePath: dfiles[1],
			},
			{
				Image:          &Image{Name: "ubuntu", Tag: "latest"},
				ServiceName:    "dockerfile-svc",
				DockerfilePath: dfiles[2],
			},
			{
				Image:          &Image{Name: "busybox", Tag: "latest"},
				ServiceName:    "image-svc",
				DockerfilePath: "",
			},
		},
	})

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// cSort ensures that Dockerfiles referenced in docker-compose files
// and the images in those Dockerfiles are sorted as required for rewriting.
func cSort() (*test, error) {
	cfiles := []string{
		filepath.Join(cTestDir, "sort", "docker-compose.yml"),
	}
	dfiles := []string{
		filepath.ToSlash(filepath.Join(
			cTestDir, "sort", "sort", "Dockerfile-one"),
		),
		filepath.ToSlash(filepath.Join(
			cTestDir, "sort", "sort", "Dockerfile-two"),
		),
	}

	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, cfiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(cfiles[0]): {
			{
				Image:          &Image{Name: "busybox", Tag: "latest"},
				ServiceName:    "svc-one",
				DockerfilePath: dfiles[0],
			},
			{
				Image:          &Image{Name: "golang", Tag: "latest"},
				ServiceName:    "svc-one",
				DockerfilePath: dfiles[0],
			},
			{
				Image:          &Image{Name: "ubuntu", Tag: "latest"},
				ServiceName:    "svc-two",
				DockerfilePath: dfiles[1],
			},
			{
				Image:          &Image{Name: "java", Tag: "latest"},
				ServiceName:    "svc-two",
				DockerfilePath: dfiles[1],
			},
		},
	})

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// cAbsPathDockerfile ensures that Dockerfiles referenced by
// absolute paths and relative paths in docker-compose files resolve to the same
// relative path to the current working directory in the Lockfile.
func cAbsPathDockerfile() (*test, error) {
	cfiles := []string{
		filepath.Join(cTestDir, "abspath", "docker-compose.yml"),
	}
	dfiles := []string{
		filepath.ToSlash(filepath.Join(
			cTestDir, "abspath", "abspath", "Dockerfile"),
		),
	}

	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		[]string{}, cfiles, []string{}, []string{},
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

	lfile := NewLockfile(nil, map[string][]*ComposefileImage{
		filepath.ToSlash(cfiles[0]): {
			{
				Image:          &Image{Name: "busybox", Tag: "latest"},
				ServiceName:    "svc-one",
				DockerfilePath: dfiles[0],
			},
			{
				Image:          &Image{Name: "busybox", Tag: "latest"},
				ServiceName:    "svc-two",
				DockerfilePath: dfiles[0],
			},
		},
	})

	return &test{
		flags: flags,
		want:  lfile,
	}, nil
}

// Both tests

// bDuplicates ensures that Lockfiles do not include the same file twice.
func bDuplicates() (*test, error) {
	cfiles := []string{
		filepath.Join(bTestDir, "docker-compose.yml"),
		filepath.Join(bTestDir, "docker-compose.yml"),
	}
	dfiles := []string{
		filepath.Join(bTestDir, "both", "Dockerfile"),
		filepath.Join(bTestDir, "both", "Dockerfile"),
	}

	flags, err := NewFlags(
		".", "docker-lock.json", getDefaultConfigPath(), ".env",
		dfiles, cfiles, []string{}, []string{},
		false, false, false,
	)
	if err != nil {
		return nil, err
	}

	lfile := NewLockfile(
		map[string][]*DockerfileImage{
			filepath.ToSlash(dfiles[0]): {
				{Image: &Image{Name: "ubuntu", Tag: "latest"}},
			},
		},
		map[string][]*ComposefileImage{
			filepath.ToSlash(cfiles[0]): {
				{
					Image:          &Image{Name: "ubuntu", Tag: "latest"},
					ServiceName:    "both-svc",
					DockerfilePath: filepath.ToSlash(dfiles[0]),
				},
				{
					Image:          &Image{Name: "busybox", Tag: "latest"},
					ServiceName:    "image-svc",
					DockerfilePath: "",
				},
			},
		})

	return &test{
		flags: flags,
		want:  lfile,
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

func cmpLfiles(got, want *Lockfile) error {
	if err := cmpDPaths(
		got.DockerfileImages, want.DockerfileImages,
	); err != nil {
		return err
	}

	if err := cmpCPaths(
		got.ComposefileImages, want.ComposefileImages,
	); err != nil {
		return err
	}

	if err := cmpDIms(
		got.DockerfileImages, want.DockerfileImages,
	); err != nil {
		return err
	}

	if err := cmpCIms(
		got.ComposefileImages, want.ComposefileImages,
	); err != nil {
		return err
	}

	return nil
}

func cmpDPaths(got, want map[string][]*DockerfileImage) error {
	allSets := make([]map[string]struct{}, 2)

	for i, m := range []map[string][]*DockerfileImage{got, want} {
		set := map[string]struct{}{}

		for p := range m {
			set[p] = struct{}{}
		}

		allSets[i] = set
	}

	return cmpPaths(allSets[0], allSets[1])
}

func cmpCPaths(got, want map[string][]*ComposefileImage) error {
	allSets := make([]map[string]struct{}, 2)

	for i, m := range []map[string][]*ComposefileImage{got, want} {
		set := map[string]struct{}{}

		for p := range m {
			set[p] = struct{}{}
		}

		allSets[i] = set
	}

	return cmpPaths(allSets[0], allSets[1])
}

func cmpPaths(got, want map[string]struct{}) error {
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

func cmpDIms(got, want map[string][]*DockerfileImage) error {
	for p := range got {
		for i := range got[p] {
			if err := cmpIms(
				got[p][i].Image, want[p][i].Image,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func cmpCIms(got, want map[string][]*ComposefileImage) error {
	for p := range got {
		for i := range got[p] {
			if err := cmpIms(
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

func cmpIms(got, want *Image) error {
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

func writeLfile(g *Generator) (*Lockfile, error) {
	configPath := getDefaultConfigPath()

	wm, err := getDefaultWrapperManager(configPath, client)
	if err != nil {
		return nil, err
	}

	buf := bytes.Buffer{}
	if err = g.GenerateLockfile(wm, &buf); err != nil {
		return nil, err
	}

	lfile := Lockfile{}
	if err = json.Unmarshal(buf.Bytes(), &lfile); err != nil {
		return nil, err
	}

	return &lfile, err
}

func getDefaultWrapperManager(
	configPath string,
	client *registry.HTTPClient,
) (*registry.WrapperManager, error) {
	dw, err := firstparty.GetDefaultWrapper(configPath, client)
	if err != nil {
		return nil, err
	}

	wm := registry.NewWrapperManager(dw)

	fpWrappers, err := firstparty.GetAllWrappers(configPath, client)
	if err != nil {
		return nil, err
	}

	cWrappers, err := contrib.GetAllWrappers(client)
	if err != nil {
		return nil, err
	}

	wm.Add(fpWrappers...)
	wm.Add(cWrappers...)

	return wm, nil
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
