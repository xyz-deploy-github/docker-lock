package rewrite

import "path/filepath"

// Flags are all possible flags to initialize a Rewriter.
type Flags struct {
	LockfilePath string
	Suffix       string
	TempDir      string
}

// NewFlags creates flags for a Rewriter.
func NewFlags(
	lockfilePath, suffix, tempDir string,
) (*Flags, error) {
	lockfilePath = convertStringToSlash(lockfilePath)
	tempDir = convertStringToSlash(tempDir)
	return &Flags{
		LockfilePath: lockfilePath,
		Suffix:       suffix,
		TempDir:      tempDir,
	}, nil
}

func convertStringToSlash(s string) string {
	return filepath.ToSlash(s)
}
