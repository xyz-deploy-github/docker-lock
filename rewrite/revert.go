package rewrite

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"
)

// revertRnFiles reverts all files that have been successfully renamed
// to their original content. If the file did not exist before renaming,
// it is removed. Errors writing and removing files are returned.
func (r *Rewriter) revertRnFiles(rns []*rnInfo) error {
	failedOPathsCh := make(chan string)
	wg := sync.WaitGroup{}

	for _, rn := range rns {
		wg.Add(1)

		go func(rn *rnInfo) {
			defer wg.Done()

			switch rn.origByt {
			case nil:
				if err := os.Remove(rn.oPath); err != nil {
					failedOPathsCh <- rn.oPath
				}
			default:
				if err := ioutil.WriteFile( //nolint: gosec
					rn.oPath, rn.origByt, 0644,
				); err != nil {
					failedOPathsCh <- rn.oPath
				}
			}
		}(rn)
	}

	go func() {
		wg.Wait()
		close(failedOPathsCh)
	}()

	failedOPaths := []string{}
	for oPath := range failedOPathsCh {
		failedOPaths = append(failedOPaths, oPath)
	}

	if len(failedOPaths) != 0 {
		return fmt.Errorf("failed to revert '%s'", failedOPaths)
	}

	return nil
}
