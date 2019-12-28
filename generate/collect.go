package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

type collectedPathResult struct {
	path string
	err  error
}

type collectedCliArgs struct {
	paths []string
	globs []string
	rec   bool
	err   error
}

func collectPaths(cmd *cobra.Command) ([]string, []string, error) {
	bDir, err := cmd.Flags().GetString("base-dir")
	if err != nil {
		return nil, nil, err
	}
	bDir = filepath.ToSlash(bDir)
	var (
		defaults = []struct {
			pathsKey string
			globsKey string
			recKey   string
			paths    []string
		}{
			{
				pathsKey: "dockerfiles",
				globsKey: "dockerfile-globs",
				recKey:   "dockerfile-recursive",
				paths:    []string{"Dockerfile"},
			},
			{
				pathsKey: "compose-files",
				globsKey: "compose-file-globs",
				recKey:   "compose-file-recursive",
				paths:    []string{"docker-compose.yml", "docker-compose.yaml"},
			},
		}
		doneCh                = make(chan struct{})
		errCh                 = make(chan error)
		paths      [][]string = make([][]string, len(defaults))
		pathsFound            = make(chan bool, len(defaults))
		wg         sync.WaitGroup
	)
	for i, args := range defaults {
		paths[i] = nil
		args := args
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var err error
			cliArgsCh := getCliArgs(
				cmd, args.pathsKey, args.globsKey, args.recKey, doneCh,
			)
			if paths[i], err = collectPathsFromCliArgs(
				bDir, cliArgsCh, args.paths, doneCh,
			); paths[i] != nil {
				pathsFound <- true
			}
			if err != nil {
				errCh <- err
				return
			}
		}(i)
	}
	go func() {
		wg.Wait()
		close(pathsFound)
		close(errCh)
	}()
	for err := range errCh {
		close(doneCh)
		return nil, nil, err
	}
	if !<-pathsFound {
		for i, args := range defaults {
			args := args
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				paths[i] = collectDefaultPaths(bDir, args.paths)
			}(i)
		}
		wg.Wait()
	}
	return paths[0], paths[1], nil
}

func collectPathsFromCliArgs(
	bDir string,
	cliArgsCh <-chan *collectedCliArgs,
	defaultPaths []string,
	doneCh <-chan struct{},
) ([]string, error) {

	var wg sync.WaitGroup
	pathCh := make(chan *collectedPathResult)
	wg.Add(1)
	go func() {
		defer wg.Done()
		cliArgs := <-cliArgsCh
		if cliArgs.err != nil {
			select {
			case <-doneCh:
			case pathCh <- &collectedPathResult{err: cliArgs.err}:
			}
			return
		}
		wg.Add(1)
		go collectSuppliedPaths(bDir, cliArgs.paths, pathCh, &wg, doneCh)
		if cliArgs.rec {
			wg.Add(1)
			go collectRecursivePaths(bDir, defaultPaths, pathCh, &wg, doneCh)
		}
		if len(cliArgs.globs) != 0 {
			wg.Add(1)
			go collectGlobPaths(bDir, cliArgs.globs, pathCh, &wg, doneCh)
		}
	}()
	go func() {
		wg.Wait()
		close(pathCh)
	}()
	return dedupeAndValidatePaths(pathCh)
}

func collectSuppliedPaths(
	bDir string,
	paths []string,
	pathCh chan<- *collectedPathResult,
	wg *sync.WaitGroup,
	doneCh <-chan struct{}) {

	defer wg.Done()
	for _, p := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			select {
			case <-doneCh:
			case pathCh <- &collectedPathResult{
				path: filepath.ToSlash(filepath.Join(bDir, p)),
			}:
			}
		}(p)
	}
}

func collectRecursivePaths(
	bDir string,
	defaultPaths []string,
	pathCh chan<- *collectedPathResult,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {

	defer wg.Done()
	filepath.Walk(bDir, func(p string, info os.FileInfo, err error) error {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err != nil {
				select {
				case <-doneCh:
				case pathCh <- &collectedPathResult{err: err}:
				}
				return
			}
			p = filepath.ToSlash(p)
			if info.Mode().IsRegular() && isDefaultPath(p, defaultPaths) {
				select {
				case <-doneCh:
				case pathCh <- &collectedPathResult{path: p}:
				}
			}
		}()
		return nil
	})
}

func collectGlobPaths(
	bDir string,
	globs []string,
	pathCh chan<- *collectedPathResult,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {

	defer wg.Done()
	for _, g := range globs {
		wg.Add(1)
		go func(g string) {
			defer wg.Done()
			g = filepath.ToSlash(filepath.Join(bDir, g))
			paths, err := filepath.Glob(g)
			if err != nil {
				select {
				case <-doneCh:
				case pathCh <- &collectedPathResult{err: err}:
				}
				return
			}
			for _, p := range paths {
				wg.Add(1)
				go func(p string) {
					defer wg.Done()
					select {
					case <-doneCh:
					case pathCh <- &collectedPathResult{
						path: filepath.ToSlash(p),
					}:
					}
				}(p)
			}
		}(g)
	}
}

func collectDefaultPaths(bDir string, defaultPaths []string) []string {
	var (
		pathCh = make(chan *collectedPathResult)
		paths  []string
		wg     sync.WaitGroup
	)
	for i, p := range defaultPaths {
		wg.Add(1)
		go func(p string, i int) {
			defer wg.Done()
			p = filepath.ToSlash(filepath.Join(bDir, p))
			fi, err := os.Stat(p)
			if err == nil {
				if mode := fi.Mode(); mode.IsRegular() {
					pathCh <- &collectedPathResult{path: p}
				}
			}
		}(p, i)
	}
	go func() {
		wg.Wait()
		close(pathCh)
	}()
	for p := range pathCh {
		paths = append(paths, p.path)
	}
	return paths
}

func getCliArgs(
	cmd *cobra.Command,
	pathsKey string,
	globsKey string,
	recKey string,
	doneCh <-chan struct{},
) chan *collectedCliArgs {

	cliArgsCh := make(chan *collectedCliArgs)
	go func() {
		paths, err := cmd.Flags().GetStringSlice(pathsKey)
		if err != nil {
			select {
			case <-doneCh:
			case cliArgsCh <- &collectedCliArgs{err: err}:
			}
			return
		}
		for i, p := range paths {
			paths[i] = filepath.ToSlash(p)
		}
		globs, err := cmd.Flags().GetStringSlice(globsKey)
		if err != nil {
			select {
			case <-doneCh:
			case cliArgsCh <- &collectedCliArgs{err: err}:
			}
			return
		}
		for i, g := range globs {
			globs[i] = filepath.ToSlash(g)
		}
		rec, err := cmd.Flags().GetBool(recKey)
		if err != nil {
			select {
			case <-doneCh:
			case cliArgsCh <- &collectedCliArgs{err: err}:
			}
			return
		}
		select {
		case <-doneCh:
		case cliArgsCh <- &collectedCliArgs{
			paths: paths,
			globs: globs,
			rec:   rec,
		}:
		}
		close(cliArgsCh)
	}()
	return cliArgsCh
}

func isDefaultPath(pathToCheck string, defaultPaths []string) bool {
	for _, p := range defaultPaths {
		if filepath.Base(pathToCheck) == p {
			return true
		}
	}
	return false
}

func dedupeAndValidatePaths(
	pathCh <-chan *collectedPathResult,
) ([]string, error) {

	var (
		uniqPaths []string
		pathSet   = map[string]bool{}
	)
	for res := range pathCh {
		if res.err != nil {
			return nil, res.err
		}
		if strings.HasPrefix(res.path, "..") {
			return nil, fmt.Errorf(
				"%s is outside the current working directory",
				res.path,
			)
		}
		if !pathSet[res.path] {
			uniqPaths = append(uniqPaths, res.path)
		}
		pathSet[res.path] = true
	}
	return uniqPaths, nil
}
