package rewrite

import "path/filepath"

// Flags are all possible flags to initialize a Rewriter.
type Flags struct {
	LockfilePath string
	Suffix       string
	TempDir      string
}

// NewFlags creates flags for a Rewriter.
func NewFlags(lPath, suffix, tmpDir string) (*Flags, error) {
	lPath = convertStrToSlash(lPath)
	tmpDir = convertStrToSlash(tmpDir)

	return &Flags{
		LockfilePath: lPath,
		Suffix:       suffix,
		TempDir:      tmpDir,
	}, nil
}

// convertStrToSlash converts a filepath string to use forward slashes.
func convertStrToSlash(s string) string {
	return filepath.ToSlash(s)
}
