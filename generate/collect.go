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

type collectedPathsResult struct {
	paths []string
	err   error
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
	bDir, err := cmd.Flags().GetString("base-dir")
	if err != nil {
		return nil, nil, err
	}
	bDir = filepath.ToSlash(bDir)
	var (
		doneCh         = make(chan struct{})
		dPaths, cPaths []string
		dPathsCh       = collectDockerfilePaths(bDir, cmd, doneCh)
		cPathsCh       = collectComposefilePaths(bDir, cmd, doneCh)
	)
	for i := 0; i < 2; i++ {
		select {
		case res := <-dPathsCh:
			if res.err != nil {
				close(doneCh)
				return nil, nil, res.err
			}
			dPaths = res.paths
			dPathsCh = nil
		case res := <-cPathsCh:
			if res.err != nil {
				close(doneCh)
				return nil, nil, res.err
			}
			cPaths = res.paths
			cPathsCh = nil
		}
	}
	if len(dPaths) == 0 && len(cPaths) == 0 {
		var err error
		dPaths, cPaths, err = collectDefaultPaths(bDir)
		if err != nil {
			return nil, nil, err
		}
	}
	return dPaths, cPaths, nil
}

func collectDockerfilePaths(
	bDir string,
	cmd *cobra.Command,
	doneCh <-chan struct{},
) <-chan *collectedPathsResult {

	dPathCh := make(chan *collectedPathResult)
	go func() {
		cliArgs, err := getCliArgs(
			cmd,
			"dockerfiles",
			"dockerfile-globs",
			"dockerfile-recursive",
		)
		if err != nil {
			select {
			case <-doneCh:
			case dPathCh <- &collectedPathResult{err: err}:
			}
			return
		}
		var wg sync.WaitGroup
		wg.Add(1)
		go collectPathsFromArgs(
			bDir,
			cliArgs,
			defaultDPaths,
			dPathCh,
			&wg,
			doneCh,
		)
		go func() {
			wg.Wait()
			close(dPathCh)
		}()
	}()
	return dedupeAndValidatePaths(dPathCh, doneCh)
}

func collectComposefilePaths(
	bDir string,
	cmd *cobra.Command,
	doneCh <-chan struct{},
) <-chan *collectedPathsResult {

	cPathCh := make(chan *collectedPathResult)
	go func() {
		cliArgs, err := getCliArgs(
			cmd,
			"compose-files",
			"compose-file-globs",
			"compose-file-recursive",
		)
		if err != nil {
			select {
			case <-doneCh:
			case cPathCh <- &collectedPathResult{err: err}:
			}
			return
		}
		var wg sync.WaitGroup
		wg.Add(1)
		go collectPathsFromArgs(
			bDir,
			cliArgs,
			defaultCPaths,
			cPathCh,
			&wg,
			doneCh,
		)
		go func() {
			wg.Wait()
			close(cPathCh)
		}()
	}()
	return dedupeAndValidatePaths(cPathCh, doneCh)
}

func collectPathsFromArgs(
	bDir string,
	cliArgs *collectedCliArgs,
	defaultPaths []string,
	pathCh chan<- *collectedPathResult,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {

	defer wg.Done()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, p := range cliArgs.paths {
			wg.Add(1)
			go collectPathFromPath(bDir, p, pathCh, wg, doneCh)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if cliArgs.rec {
			filepath.Walk(bDir,
				func(p string, info os.FileInfo, err error) error {
					if err != nil {
						select {
						case <-doneCh:
						case pathCh <- &collectedPathResult{err: err}:
						}
						return nil
					}
					wg.Add(1)
					go collectRecursivePath(
						p,
						info,
						defaultPaths,
						pathCh,
						wg,
						doneCh,
					)
					return nil
				})
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, p := range cliArgs.globs {
			wg.Add(1)
			go collectGlobPath(bDir, p, pathCh, wg, doneCh)
		}
	}()
}

func collectPathFromPath(
	bDir string,
	p string,
	pathCh chan<- *collectedPathResult,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {

	defer wg.Done()
	p = filepath.ToSlash(filepath.Join(bDir, p))
	select {
	case <-doneCh:
	case pathCh <- &collectedPathResult{path: p}:
	}
}

func collectRecursivePath(
	p string,
	info os.FileInfo,
	defaultPaths []string,
	pathCh chan<- *collectedPathResult,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {

	defer wg.Done()
	p = filepath.ToSlash(p)
	if info.Mode().IsRegular() && isDefaultPath(p, defaultPaths) {
		select {
		case <-doneCh:
		case pathCh <- &collectedPathResult{path: p}:
		}
	}
}

func collectGlobPath(
	bDir string,
	pattern string,
	pathCh chan<- *collectedPathResult,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {

	defer wg.Done()
	pattern = filepath.ToSlash(filepath.Join(bDir, pattern))
	paths, err := filepath.Glob(pattern)
	if err != nil {
		select {
		case <-doneCh:
		case pathCh <- &collectedPathResult{err: err}:
		}
		return
	}
	for _, p := range paths {
		select {
		case <-doneCh:
			return
		case pathCh <- &collectedPathResult{path: filepath.ToSlash(p)}:
		}
	}
}

func collectDefaultPaths(bDir string) ([]string, []string, error) {
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
		go collectPathFromDefaultPath(bDir, p, pathCh, &wg)
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

func collectPathFromDefaultPath(
	bDir string,
	p string,
	pathCh chan<- *collectedPathResult,
	wg *sync.WaitGroup,
) {

	defer wg.Done()
	p = filepath.ToSlash(filepath.Join(bDir, p))
	fi, err := os.Stat(p)
	if err == nil {
		if mode := fi.Mode(); mode.IsRegular() {
			pathCh <- &collectedPathResult{path: p}
		}
	}
}

func getCliArgs(
	cmd *cobra.Command,
	pathsKey string,
	globsKey string,
	recKey string,
) (*collectedCliArgs, error) {

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

func dedupeAndValidatePaths(
	pathCh <-chan *collectedPathResult,
	doneCh <-chan struct{},
) <-chan *collectedPathsResult {

	var (
		pathsCh   = make(chan *collectedPathsResult)
		uniqPaths []string
		pathSet   = map[string]bool{}
	)
	go func() {
		for res := range pathCh {
			if res.err != nil {
				select {
				case <-doneCh:
				case pathsCh <- &collectedPathsResult{err: res.err}:
				}
				return
			}
			if strings.HasPrefix(res.path, "..") {
				select {
				case <-doneCh:
				case pathsCh <- &collectedPathsResult{
					err: fmt.Errorf(
						"%s is outside the current working directory",
						res.path,
					),
				}:
				}
				return
			}
			if !pathSet[res.path] {
				uniqPaths = append(uniqPaths, res.path)
			}
			pathSet[res.path] = true
		}
		select {
		case <-doneCh:
		case pathsCh <- &collectedPathsResult{paths: uniqPaths}:
		}
		close(pathsCh)
	}()
	return pathsCh
}
