package verify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
	"github.com/safe-waters/docker-lock/registry"
	"github.com/safe-waters/docker-lock/registry/contrib"
	"github.com/safe-waters/docker-lock/registry/firstparty"
)

var dTestDir = filepath.Join("testdata", "docker")  //nolint: gochecknoglobals
var cTestDir = filepath.Join("testdata", "compose") //nolint: gochecknoglobals

type test struct {
	flags      *Flags
	shouldFail bool
}

func TestVerifier(t *testing.T) {
	tests, err := getTests()
	if err != nil {
		t.Fatal(err)
	}

	for name, tc := range tests {
		tc := tc

		t.Run(name, func(t *testing.T) {
			v, err := NewVerifier(tc.flags)
			if err != nil {
				t.Fatal(err)
			}
			if err := verifyLockfile(v); err != nil && !tc.shouldFail {
				t.Fatal(err)
			}
		})
	}
}

// Dockerfile tests

// dDiffNumImages ensures an error occurs if
// there are a different number of images in the Dockerfile
// referenced in the Lockfile.
func dDiffNumImages() (*test, error) {
	lPath := filepath.Join(dTestDir, "diffnumimages", "docker-lock.json")

	flags, err := NewFlags(lPath, defaultConfigPath(), ".env", false, false)
	if err != nil {
		return nil, err
	}

	return &test{flags: flags, shouldFail: true}, nil
}

// dDiffDigests ensures an error occurs if
// a digest found in the Lockfile differs from the generated
// digest from the Dockerfile.
func dDiffDigests() (*test, error) {
	lPath := filepath.Join(dTestDir, "diffdigests", "docker-lock.json")

	flags, err := NewFlags(lPath, defaultConfigPath(), ".env", false, false)
	if err != nil {
		return nil, err
	}

	return &test{flags: flags, shouldFail: true}, nil
}

// dBuildArgs ensures environment variables are used as build args
func dBuildArgs() (*test, error) {
	lPath := filepath.Join(dTestDir, "buildargs", "docker-lock.json")

	envPath := filepath.Join(dTestDir, "buildargs", ".env")
	if err := godotenv.Load(envPath); err != nil {
		return nil, err
	}

	flags, err := NewFlags(lPath, defaultConfigPath(), envPath, true, false)
	if err != nil {
		return nil, err
	}

	return &test{flags: flags, shouldFail: false}, nil
}

// Composefile tests

// cDiffNumImages ensures an error occurs if
// there are a different number of images in the docker-compose file
// referenced in the Lockfile.
func cDiffNumImages() (*test, error) {
	lPath := filepath.Join(cTestDir, "diffnumimages", "docker-lock.json")

	flags, err := NewFlags(lPath, defaultConfigPath(), ".env", false, false)
	if err != nil {
		return nil, err
	}

	return &test{flags: flags, shouldFail: true}, nil
}

// cDiffDigests ensures an error occurs if
// a digest found in the Lockfile differs from the generated
// digest from the docker-compose file.
func cDiffDigests() (*test, error) {
	lPath := filepath.Join(cTestDir, "diffdigests", "docker-lock.json")

	flags, err := NewFlags(lPath, defaultConfigPath(), ".env", false, false)
	if err != nil {
		return nil, err
	}

	return &test{flags: flags, shouldFail: true}, nil
}

// Helpers

func getTests() (map[string]*test, error) {
	dBuildArgs, err := dBuildArgs()
	if err != nil {
		return nil, err
	}

	dDiffDigests, err := dDiffDigests()
	if err != nil {
		return nil, err
	}

	dDiffNumImages, err := dDiffNumImages()
	if err != nil {
		return nil, err
	}

	cDiffNumImages, err := cDiffNumImages()
	if err != nil {
		return nil, err
	}

	cDiffDigests, err := cDiffDigests()
	if err != nil {
		return nil, err
	}

	tests := map[string]*test{
		"dBuildArgs":     dBuildArgs,
		"dDiffDigests":   dDiffDigests,
		"dDiffNumImages": dDiffNumImages,
		"cDiffNumImages": cDiffNumImages,
		"cDiffDigests":   cDiffDigests,
	}

	return tests, nil
}

func verifyLockfile(v *Verifier) error {
	configPath := defaultConfigPath()

	wm, err := defaultWrapperManager(client, configPath)
	if err != nil {
		return err
	}

	return v.VerifyLockfile(wm)
}

func defaultWrapperManager(
	client *registry.HTTPClient,
	configPath string,
) (*registry.WrapperManager, error) {
	dw, err := firstparty.DefaultWrapper(client, configPath)
	if err != nil {
		return nil, err
	}

	wm := registry.NewWrapperManager(dw)
	wm.Add(firstparty.AllWrappers(client, configPath)...)
	wm.Add(contrib.AllWrappers(client, configPath)...)

	return wm, nil
}

func defaultConfigPath() string {
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
