package unittests

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/michaelperel/docker-lock/cmd"
)

var (
	verifyComposeBaseDir = filepath.Join("testdata", "verify", "compose")
	verifyDockerBaseDir  = filepath.Join("testdata", "verify", "docker")
)

// TestVerifyComposefileDiffNumImages ensures an error occurs if
// there are a different number of images in the docker-compose file
// referenced in the Lockfile.
func TestVerifyComposefileDiffNumImages(t *testing.T) {
	t.Parallel()
	lPath := filepath.Join(verifyComposeBaseDir, "diffnumimages", "docker-lock.json")
	flags := []string{fmt.Sprintf("--lockfile-path=%s", lPath)}
	shouldFail := true
	testVerify(t, flags, shouldFail)
}

// TestVerifyComposeDiffDigests ensures an error occurs if
// a digest found in the Lockfile differs from the generated
// digest from the docker-compose file.
func TestVerifyComposefileDiffDigests(t *testing.T) {
	t.Parallel()
	lPath := filepath.Join(verifyComposeBaseDir, "diffdigests", "docker-lock.json")
	flags := []string{fmt.Sprintf("--lockfile-path=%s", lPath)}
	shouldFail := true
	testVerify(t, flags, shouldFail)
}

// TestVerifyDockerfileDiffNumImages ensures an error occurs if
// there are a different number of images in the Dockerfile
// referenced in the Lockfile.
func TestVerifyDockerfileDiffNumImages(t *testing.T) {
	t.Parallel()
	lPath := filepath.Join(verifyDockerBaseDir, "diffnumimages", "docker-lock.json")
	flags := []string{fmt.Sprintf("--lockfile-path=%s", lPath)}
	shouldFail := true
	testVerify(t, flags, shouldFail)
}

// TestVerifyDockerfileDiffDigests ensures an error occurs if
// a digest found in the Lockfile differs from the generated
// digest from the Dockerfile.
func TestVerifyDockerfileDiffDigests(t *testing.T) {
	t.Parallel()
	lPath := filepath.Join(verifyDockerBaseDir, "diffdigests", "docker-lock.json")
	flags := []string{fmt.Sprintf("--lockfile-path=%s", lPath)}
	shouldFail := true
	testVerify(t, flags, shouldFail)
}

// TestVerifyDockerfileEnvBuildArgs ensures environment variables are used as
// build args.
func TestVerifyDockerfileEnvBuildArgs(t *testing.T) {
	t.Parallel()
	lPath := filepath.Join(verifyDockerBaseDir, "buildargs", "docker-lock.json")
	envPath := filepath.Join(verifyDockerBaseDir, "buildargs", ".env")
	flags := []string{fmt.Sprintf("--env-file=%s", envPath), fmt.Sprintf("--dockerfile-env-build-args"), fmt.Sprintf("--lockfile-path=%s", lPath)}
	var shouldFail bool
	testVerify(t, flags, shouldFail)
}

func testVerify(t *testing.T, flags []string, shouldFail bool) {
	verifyCmd := cmd.NewVerifyCmd(client)
	verifyCmd.SilenceUsage = true
	verifyCmd.SilenceErrors = true
	args := append([]string{"lock", "verify"}, flags...)
	verifyCmd.SetArgs(args)
	err := verifyCmd.Execute()
	switch {
	case shouldFail && err == nil:
		t.Fatalf("Got pass. Want fail.")
	case !shouldFail && err != nil:
		t.Fatal(err)
	}
}
