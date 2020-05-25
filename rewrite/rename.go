package rewrite

import (
	"fmt"
	"log"
	"os"
	"sort"
)

// rnInfo contains information necessary to rename a file that has been
// written to a temporary directory. In case of failure during renaming,
// the bytes of the file that will be overwritten by renaming are stored
// so that the file can be reverted to its original.
type rnInfo struct {
	oPath    string
	tmpOPath string
	origByt  []byte
	err      error
}

// renameFiles renames temporary files to their desired output paths,
// overwriting existing files. For "transaction"-like qualities,
// the method will consume all values from the rename channel before renaming
// to ensure that all temporary files have successfully been written.
// If an error occurs during renaming, all files that were renamed will be
// reverted to their original content. Errors during renaming and reverting
// are returned.
func (r *Rewriter) renameFiles(rnCh <-chan *rnInfo) error {
	allRns := []*rnInfo{}

	for rn := range rnCh {
		if rn.err != nil {
			return rn.err
		}

		allRns = append(allRns, rn)
	}

	// sort by desired output path, so if there are failures, multiple
	// invocations of the method will fail in the same way.
	sort.Slice(allRns, func(i, j int) bool {
		return allRns[i].oPath < allRns[j].oPath
	})

	// upon error, revert or remove all files that were successfully renamed.
	successRns := []*rnInfo{}

	for _, rn := range allRns {
		if rnErr := os.Rename(rn.tmpOPath, rn.oPath); rnErr != nil {
			if rvErr := r.revertRnFiles(successRns); rvErr != nil {
				return fmt.Errorf("%v: %v", rvErr, rnErr)
			}

			return rnErr
		}

		log.Printf("renamed '%s' to '%s'.", rn.tmpOPath, rn.oPath)

		successRns = append(successRns, rn)
	}

	return nil
}
