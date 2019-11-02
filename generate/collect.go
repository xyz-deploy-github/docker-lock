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
}

var (
	defaultDPaths = []string{"Dockerfile"}
	defaultCPaths = []string{"docker-compose.yml", "docker-compose.yaml"}
)

func collectPaths(cmd *cobra.Command) ([]string, []string, error) {
	var (
		dPaths, cPaths []string
		dErr, cErr     error
		wg             sync.WaitGroup
	)
	bDir, err := cmd.Flags().GetString("base-dir")
	if err != nil {
		return nil, nil, err
	}
	bDir = filepath.ToSlash(bDir)
	wg.Add(1)
	go func() {
		defer wg.Done()
		dPaths, dErr = collectDockerfilePaths(cmd, bDir)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		cPaths, cErr = collectComposefilePaths(cmd, bDir)
	}()
	wg.Wait()
	if dErr != nil {
		return nil, nil, dErr
	}
	if cErr != nil {
		return nil, nil, cErr
	}
	if len(dPaths) == 0 && len(cPaths) == 0 {
		var err error
		dPaths, cPaths, err = collectDefaultPaths(cmd, bDir)
		if err != nil {
			return nil, nil, err
		}
	}
	if err := validatePaths(dPaths, cPaths); err != nil {
		return nil, nil, err
	}
	return dPaths, cPaths, nil
}

func collectDockerfilePaths(cmd *cobra.Command, bDir string) ([]string, error) {
	cliArgs, err := getCliArgs(cmd, "dockerfiles", "dockerfile-globs", "dockerfile-recursive")
	if err != nil {
		return nil, err
	}
	return collectPathsFromArgs(cliArgs, bDir, defaultDPaths)
}

func collectComposefilePaths(cmd *cobra.Command, bDir string) ([]string, error) {
	cliArgs, err := getCliArgs(cmd, "compose-files", "compose-file-globs", "compose-file-recursive")
	if err != nil {
		return nil, err
	}
	return collectPathsFromArgs(cliArgs, bDir, defaultCPaths)
}

func collectPathsFromArgs(cliArgs *collectedCliArgs, bDir string, defaultPaths []string) ([]string, error) {
	var (
		pathCh = make(chan *collectedPathResult)
		wg     sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, p := range cliArgs.paths {
			wg.Add(1)
			go func(p string) {
				defer wg.Done()
				collectPathFromPath(p, bDir, pathCh)
			}(p)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if cliArgs.rec {
			filepath.Walk(bDir, func(p string, info os.FileInfo, err error) error {
				wg.Add(1)
				go func() {
					defer wg.Done()
					collectRecursivePath(p, info, defaultPaths, pathCh, err)
				}()
				return nil
			})
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, p := range cliArgs.globs {
			wg.Add(1)
			go func(p string) {
				defer wg.Done()
				collectGlobPath(p, bDir, pathCh)
			}(p)
		}
	}()
	go func() {
		wg.Wait()
		close(pathCh)
	}()
	paths, err := dedupeAndValidatePaths(pathCh)
	if err != nil {
		return nil, err
	}
	return paths, nil
}

func collectPathFromPath(p string, bDir string, pathCh chan<- *collectedPathResult) {
	p = filepath.ToSlash(filepath.Join(bDir, p))
	pathCh <- &collectedPathResult{path: p}
}

func collectRecursivePath(p string, info os.FileInfo, defaultPaths []string, pathCh chan<- *collectedPathResult, err error) {
	if err != nil {
		pathCh <- &collectedPathResult{err: err}
		return
	}
	p = filepath.ToSlash(p)
	if info.Mode().IsRegular() && isDefaultPath(p, defaultPaths) {
		pathCh <- &collectedPathResult{path: p}
	}
}

func collectGlobPath(pattern string, bDir string, pathCh chan<- *collectedPathResult) {
	pattern = filepath.ToSlash(filepath.Join(bDir, pattern))
	paths, err := filepath.Glob(pattern)
	if err != nil {
		pathCh <- &collectedPathResult{err: err}
		return
	}
	for _, p := range paths {
		pathCh <- &collectedPathResult{path: filepath.ToSlash(p)}
	}
}

func collectDefaultPaths(cmd *cobra.Command, bDir string) ([]string, []string, error) {
	var (
		dPaths []string
		cPaths []string
		wg     sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		dPaths = collectPathsFromDefaultPaths(bDir, defaultDPaths)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		cPaths = collectPathsFromDefaultPaths(bDir, defaultCPaths)
	}()
	wg.Wait()
	return dPaths, cPaths, nil
}

func collectPathsFromDefaultPaths(bDir string, defaultPaths []string) []string {
	var (
		paths  []string
		pathCh = make(chan *collectedPathResult)
		wg     sync.WaitGroup
	)
	for _, p := range defaultPaths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			collectPathFromDefaultPath(p, bDir, pathCh)
		}(p)
	}
	go func() {
		wg.Wait()
		close(pathCh)
	}()
	for res := range pathCh {
		paths = append(paths, res.path)
	}
	return paths
}

func collectPathFromDefaultPath(p string, bDir string, pathCh chan<- *collectedPathResult) {
	p = filepath.ToSlash(filepath.Join(bDir, p))
	fi, err := os.Stat(p)
	if err == nil {
		if mode := fi.Mode(); mode.IsRegular() {
			pathCh <- &collectedPathResult{path: p}
		}
	}
}

func getCliArgs(cmd *cobra.Command, pathsKey string, globsKey string, recKey string) (*collectedCliArgs, error) {
	paths, err := cmd.Flags().GetStringSlice(pathsKey)
	if err != nil {
		return nil, err
	}
	for i, p := range paths {
		paths[i] = filepath.ToSlash(p)
	}
	globs, err := cmd.Flags().GetStringSlice(globsKey)
	if err != nil {
		return nil, err
	}
	for i, g := range globs {
		globs[i] = filepath.ToSlash(g)
	}
	rec, err := cmd.Flags().GetBool(recKey)
	if err != nil {
		return nil, err
	}
	return &collectedCliArgs{paths: paths, globs: globs, rec: rec}, nil
}

func isDefaultPath(pathToCheck string, defaultPaths []string) bool {
	for _, p := range defaultPaths {
		if filepath.Base(pathToCheck) == p {
			return true
		}
	}
	return false
}

func dedupeAndValidatePaths(pathCh <-chan *collectedPathResult) ([]string, error) {
	var (
		uniqPaths []string
		pathSet   = map[string]bool{}
	)
	for res := range pathCh {
		if res.err != nil {
			return nil, res.err
		}
		if !pathSet[res.path] {
			uniqPaths = append(uniqPaths, res.path)
		}
		pathSet[res.path] = true
	}
	return uniqPaths, nil
}

func validatePaths(dPaths, cPaths []string) error {
	for _, ps := range [][]string{dPaths, cPaths} {
		for _, p := range ps {
			if strings.HasPrefix(p, "..") {
				return fmt.Errorf("%s is outside the current working directory", p)
			}
		}
	}
	return nil
}
