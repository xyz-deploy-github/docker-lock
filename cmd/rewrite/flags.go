package rewrite

// Flags are all possible flags to initialize a Rewriter.
type Flags struct {
	LockfilePath string
	TempDir      string
	ExcludeTags  bool
}
