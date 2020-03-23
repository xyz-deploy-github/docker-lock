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

func collectDockerfileAndComposefilePaths(
	flags *Flags,
) ([]string, []string, error) {
	doneCh := make(chan struct{})
	dBaseSet := map[string]struct{}{"Dockerfile": {}}
	dPathCh := collectNonDefaultPaths(
		flags.BaseDir, flags.Dockerfiles, dBaseSet,
		flags.DockerfileGlobs, flags.DockerfileRecursive,
		doneCh,
	)
	cBaseSet := map[string]struct{}{
		"docker-compose.yml":  {},
		"docker-compose.yaml": {},
	}
	cPathCh := collectNonDefaultPaths(
		flags.BaseDir, flags.Composefiles, cBaseSet,
		flags.ComposefileGlobs, flags.ComposefileRecursive,
		doneCh,
	)
	dPaths, cPaths, err := convertPathChsToSlices(dPathCh, cPathCh)
	if err != nil {
		close(doneCh)
		return nil, nil, err
	}
	if len(dPaths) == 0 && len(cPaths) == 0 {
		doneCh = make(chan struct{})
		dPathCh = collectDefaultPaths(flags.BaseDir, dBaseSet, doneCh)
		cPathCh = collectDefaultPaths(flags.BaseDir, cBaseSet, doneCh)
		dPaths, cPaths, err = convertPathChsToSlices(dPathCh, cPathCh)
		if err != nil {
			close(doneCh)
			return nil, nil, err
		}
	}
	if err := validatePaths(dPaths, cPaths); err != nil {
		return nil, nil, err
	}
	return dPaths, cPaths, nil
}

func collectNonDefaultPaths(
	bDir string,
	suppliedPaths []string,
	baseSet map[string]struct{},
	globs []string,
	recursive bool,
	doneCh <-chan struct{},
) chan *pathResult {
	pathCh := make(chan *pathResult)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
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
	}()
	go func() {
		wg.Wait()
		close(pathCh)
	}()
	return pathCh
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

func collectDefaultPaths(
	bDir string,
	baseSet map[string]struct{},
	doneCh <-chan struct{},
) chan *pathResult {
	var wg sync.WaitGroup
	pathCh := make(chan *pathResult)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for p := range baseSet {
			p = filepath.ToSlash(filepath.Join(bDir, p))
			addPathToPathCh(p, pathCh, doneCh)
		}
	}()
	go func() {
		wg.Wait()
		close(pathCh)
	}()
	return pathCh
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

func validatePaths(dPaths, cPaths []string) error {
	for _, paths := range [][]string{dPaths, cPaths} {
		for _, p := range paths {
			if strings.HasPrefix(p, "..") {
				return fmt.Errorf(
					"%s is outside the current working directory", p,
				)
			}
		}
	}
	return nil
}

func convertPathChsToSlices(
	dPathCh,
	cPathCh <-chan *pathResult,
) ([]string, []string, error) {
	dPathSet := map[string]struct{}{}
	cPathSet := map[string]struct{}{}
	for {
		select {
		case pathRes, ok := <-dPathCh:
			if err := handlePathResult(
				&dPathCh, pathRes, dPathSet, ok,
			); err != nil {
				return nil, nil, err
			}
		case pathRes, ok := <-cPathCh:
			if err := handlePathResult(
				&cPathCh, pathRes, cPathSet, ok,
			); err != nil {
				return nil, nil, err
			}
		}
		if dPathCh == nil && cPathCh == nil {
			break
		}
	}
	dPaths := make([]string, len(dPathSet))
	cPaths := make([]string, len(cPathSet))
	var i int
	for p := range dPathSet {
		dPaths[i] = p
		i++
	}
	i = 0
	for p := range cPathSet {
		cPaths[i] = p
		i++
	}
	return dPaths, cPaths, nil
}

func handlePathResult(
	pathCh *<-chan *pathResult,
	pathRes *pathResult,
	pathSet map[string]struct{},
	ok bool,
) error {
	if !ok {
		*pathCh = nil
	} else {
		if pathRes.err != nil {
			return pathRes.err
		}
		pathSet[pathRes.path] = struct{}{}
	}
	return nil
}
