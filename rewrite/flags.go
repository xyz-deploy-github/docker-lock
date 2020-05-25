package rewrite

import "path/filepath"

// Flags are all possible flags to initialize a Rewriter.
type Flags struct {
	LockfilePath string
	Suffix       string
	TempDir      string
	Verbose      bool
}

// NewFlags creates flags for a Rewriter.
func NewFlags(lPath, suffix, tmpDir string, verbose bool) (*Flags, error) {
	lPath = convertStrToSlash(lPath)
	tmpDir = convertStrToSlash(tmpDir)

	return &Flags{
		LockfilePath: lPath,
		Suffix:       suffix,
		TempDir:      tmpDir,
		Verbose:      verbose,
	}, nil
}

// convertStrToSlash converts a filepath string to use forward slashes.
func convertStrToSlash(s string) string {
	return filepath.ToSlash(s)
}
