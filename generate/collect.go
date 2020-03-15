package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type pathResult struct {
	path string
	err  error
}
type pathsResult struct {
	paths []string
	err   error
}

func collectDockerfileAndComposefilePaths(
	flags *GeneratorFlags,
) ([]string, []string, error) {
	doneCh := make(chan struct{})
	dPathsCh := collectDockerfilePaths(flags, doneCh)
	cPathsCh := collectComposefilePaths(flags, doneCh)
	dPaths := []string{}
	cPaths := []string{}
	for {
		select {
		case pathsRes := <-dPathsCh:
			if err := handlePathsResult(
				&dPaths, pathsRes, &dPathsCh, doneCh,
			); err != nil {
				return nil, nil, err
			}
		case pathsRes := <-cPathsCh:
			if err := handlePathsResult(
				&cPaths, pathsRes, &cPathsCh, doneCh,
			); err != nil {
				return nil, nil, err
			}
		}
		if dPathsCh == nil && cPathsCh == nil {
			break
		}
	}
	return dPaths, cPaths, nil
}

func collectDockerfilePaths(
	flags *GeneratorFlags,
	doneCh <-chan struct{},
) <-chan *pathsResult {
	pathsCh := make(chan *pathsResult)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		baseSet := map[string]struct{}{"Dockerfile": {}}
		paths, err := collectPaths(
			flags.BaseDir, flags.Dockerfiles, baseSet,
			flags.DockerfileGlobs, flags.DockerfileRecursive,
		)
		addPathsAndErrToPathsCh(paths, err, pathsCh, doneCh)
		close(pathsCh)
	}()
	return pathsCh
}

func collectComposefilePaths(
	flags *GeneratorFlags,
	doneCh <-chan struct{},
) <-chan *pathsResult {
	pathsCh := make(chan *pathsResult)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		baseSet := map[string]struct{}{
			"docker-compose.yml":  {},
			"docker-compose.yaml": {},
		}
		paths, err := collectPaths(
			flags.BaseDir, flags.Composefiles, baseSet,
			flags.ComposefileGlobs, flags.ComposefileRecursive,
		)
		addPathsAndErrToPathsCh(paths, err, pathsCh, doneCh)
		close(pathsCh)
	}()
	return pathsCh
}

func collectPaths(
	bDir string,
	suppliedPaths []string,
	baseSet map[string]struct{},
	globs []string,
	recursive bool,
) ([]string, error) {
	pathCh := make(chan *pathResult)
	doneCh := make(chan struct{})
	var wg sync.WaitGroup
	if len(suppliedPaths) != 0 {
		wg.Add(1)
		go collectSuppliedPaths(bDir, suppliedPaths, pathCh, doneCh, &wg)
	}
	if len(globs) != 0 {
		wg.Add(1)
		go collectGlobPaths(bDir, globs, pathCh, doneCh, &wg)
	}
	if recursive {
		wg.Add(1)
		go collectRecursivePaths(bDir, baseSet, pathCh, doneCh, &wg)
	}
	go func() {
		wg.Wait()
		close(pathCh)
		close(doneCh)
	}()
	set := map[string]struct{}{}
	for pathRes := range pathCh {
		if pathRes.err != nil {
			return nil, pathRes.err
		}
		set[pathRes.path] = struct{}{}
	}
	if len(set) == 0 {
		collectDefaultPaths(bDir, baseSet, set)
	}
	if err := validatePaths(set); err != nil {
		return nil, err
	}
	paths := make([]string, len(set))
	var i int
	for p := range set {
		paths[i] = p
		i++
	}
	return paths, nil
}

func collectSuppliedPaths(
	bDir string,
	paths []string,
	pathCh chan<- *pathResult,
	doneCh <-chan struct{},
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for _, p := range paths {
		p = filepath.ToSlash(filepath.Join(bDir, p))
		addPathToPathCh(p, pathCh, doneCh)
	}
}

func collectGlobPaths(
	bDir string,
	globs []string,
	pathCh chan<- *pathResult,
	doneCh <-chan struct{},
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for _, g := range globs {
		paths, err := filepath.Glob(g)
		if err != nil {
			addErrToPathCh(err, pathCh, doneCh)
			return
		}
		for _, p := range paths {
			p = filepath.ToSlash(filepath.Join(bDir, filepath.ToSlash(p)))
			addPathToPathCh(p, pathCh, doneCh)
		}
	}
}

func collectRecursivePaths(
	bDir string,
	defaultNames map[string]struct{},
	pathCh chan<- *pathResult,
	doneCh <-chan struct{},
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	if err := filepath.Walk(
		bDir, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			p = filepath.ToSlash(p)
			if _, ok := defaultNames[filepath.Base(p)]; ok {
				addPathToPathCh(p, pathCh, doneCh)
			}
			return nil
		}); err != nil {
		addErrToPathCh(err, pathCh, doneCh)
	}
}

func collectDefaultPaths(bDir string, baseSet, set map[string]struct{}) {
	for p := range baseSet {
		p = filepath.ToSlash(filepath.Join(bDir, p))
		if fileIsRegular(p) {
			set[p] = struct{}{}
		}
	}
}

func fileIsRegular(p string) bool {
	fi, err := os.Stat(p)
	if err == nil {
		if mode := fi.Mode(); mode.IsRegular() {
			return true
		}
	}
	return false
}

func addPathToPathCh(
	p string,
	pathCh chan<- *pathResult,
	doneCh <-chan struct{},
) {
	if fileIsRegular(p) {
		select {
		case <-doneCh:
		case pathCh <- &pathResult{path: p}:
		}
	}
}

func addPathsAndErrToPathsCh(
	paths []string,
	err error,
	pathsCh chan<- *pathsResult,
	doneCh <-chan struct{},
) {
	select {
	case <-doneCh:
	case pathsCh <- &pathsResult{paths: paths, err: err}:
	}
}

func addErrToPathCh(
	err error,
	pathCh chan<- *pathResult,
	doneCh <-chan struct{},
) {
	select {
	case <-doneCh:
	case pathCh <- &pathResult{err: err}:
	}
}

func validatePaths(set map[string]struct{}) error {
	for p := range set {
		if strings.HasPrefix(p, "..") {
			return fmt.Errorf("%s is outside the current working directory", p)
		}
	}
	return nil
}

func handlePathsResult(
	paths *[]string,
	pathsRes *pathsResult,
	pathsCh *<-chan *pathsResult,
	doneCh chan<- struct{},
) error {
	if pathsRes.err != nil {
		close(doneCh)
		return pathsRes.err
	}
	*paths = pathsRes.paths
	*pathsCh = nil
	return nil
}
