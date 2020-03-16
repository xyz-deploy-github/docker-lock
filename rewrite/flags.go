package rewrite

import "path/filepath"

type RewriterFlags struct {
	LockfilePath string
	Suffix       string
	TempDir      string
}

func NewRewriterFlags(
	lockfilePath, suffix, tempDir string,
) (*RewriterFlags, error) {
	lockfilePath = convertStringToSlash(lockfilePath)
	tempDir = convertStringToSlash(tempDir)
	return &RewriterFlags{
		LockfilePath: lockfilePath,
		Suffix:       suffix,
		TempDir:      tempDir,
	}, nil
}

func convertStringToSlash(s string) string {
	return filepath.ToSlash(s)
}
