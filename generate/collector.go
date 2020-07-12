package generate

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// collector finds and returns Dockerfiles and docker-compose files,
// according to flags.
type collector struct {
	*Flags
	dBaseSet map[string]struct{}
	cBaseSet map[string]struct{}
}

// pathResult is used to collect paths concurrently.
type pathResult struct {
	path string
	err  error
}

// collectPaths collects Dockerfile and docker-compose paths.
func (c *collector) collectPaths() ([]string, []string, error) {
	doneCh := make(chan struct{})

	dPathCh := c.collectNonDefaultDPaths(doneCh)
	cPathCh := c.collectNonDefaultCPaths(doneCh)

	dPaths, cPaths, err := c.convertPathChsToSls(dPathCh, cPathCh)
	if err != nil {
		close(doneCh)
		return nil, nil, err
	}

	if len(dPaths) == 0 && len(cPaths) == 0 {
		log.Printf("No files found from flags, looking for defaults: " +
			"'Dockerfile', 'docker-compose.yml', and 'docker-compose.yaml'.",
		)

		doneCh = make(chan struct{})

		dPathCh = c.collectDefaultDPaths(doneCh)
		cPathCh = c.collectDefaultCPaths(doneCh)

		dPaths, cPaths, err = c.convertPathChsToSls(dPathCh, cPathCh)
		if err != nil {
			close(doneCh)
			return nil, nil, err
		}
	}

	log.Printf("Found Dockerfile paths: '%s' and docker-compose paths: '%s'.",
		dPaths, cPaths,
	)

	if err := c.validatePaths(dPaths, cPaths); err != nil {
		return nil, nil, err
	}

	return dPaths, cPaths, nil
}

// collectNonDefaultDPaths collects Dockerfile paths other than "Dockerfile".
func (c *collector) collectNonDefaultDPaths(
	doneCh <-chan struct{},
) chan *pathResult {
	return c.collectNonDefaultPaths(
		c.DockerfileFlags.SpecificFlags, c.SharedFlags, c.dBaseSet, doneCh,
	)
}

// collectNonDefaultCPaths collects docker-compose paths other than
// "docker-compose.yml" and "docker-compose.yaml".
func (c *collector) collectNonDefaultCPaths(
	doneCh <-chan struct{},
) chan *pathResult {
	return c.collectNonDefaultPaths(
		c.ComposefileFlags.SpecificFlags, c.SharedFlags, c.cBaseSet, doneCh,
	)
}

// collectNonDefaultPaths collects paths other than those that would be
// collected if no paths were specified (Dockerfile, docker-compose.yaml,
// and docker-compose.yml).
func (c *collector) collectNonDefaultPaths(
	specificFlags *SpecificFlags,
	sharedFlags *SharedFlags,
	bSet map[string]struct{},
	doneCh <-chan struct{},
) chan *pathResult {
	pathCh := make(chan *pathResult)
	wg := sync.WaitGroup{}

	wg.Add(1)

	go func() {
		defer wg.Done()

		if len(specificFlags.Paths) != 0 {
			wg.Add(1)

			go c.collectSuppliedPaths(
				sharedFlags.BaseDir, specificFlags.Paths, pathCh, doneCh, &wg,
			)
		}

		if len(specificFlags.Globs) != 0 {
			wg.Add(1)

			go c.collectGlobPaths(
				sharedFlags.BaseDir, specificFlags.Globs, pathCh, doneCh, &wg,
			)
		}

		if specificFlags.Recursive {
			wg.Add(1)

			go c.collectRecursivePaths(
				sharedFlags.BaseDir, bSet, pathCh, doneCh, &wg,
			)
		}
	}()

	go func() {
		wg.Wait()
		close(pathCh)
	}()

	return pathCh
}

// collectSuppliedPaths collects paths from the flags
// --dockerfiles and --composefiles.
func (c *collector) collectSuppliedPaths(
	bDir string,
	paths []string,
	pathCh chan<- *pathResult,
	doneCh <-chan struct{},
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for _, p := range paths {
		p = filepath.ToSlash(filepath.Join(bDir, p))
		c.addPathToPathCh(p, pathCh, doneCh)
	}
}

// collectGlobPaths collects paths from the flags
// --dockerfile-globs and --composefile-globs.
func (c *collector) collectGlobPaths(
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
			c.addErrToPathCh(err, pathCh, doneCh)
			return
		}

		for _, p := range paths {
			p = filepath.ToSlash(filepath.Join(bDir, filepath.ToSlash(p)))
			c.addPathToPathCh(p, pathCh, doneCh)
		}
	}
}

// collectRecursivePaths collects paths from the flags
// --dockerfile-recursive and --composefile-recursive.
func (c *collector) collectRecursivePaths(
	bDir string,
	bSet map[string]struct{},
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
			if _, ok := bSet[filepath.Base(p)]; ok {
				c.addPathToPathCh(p, pathCh, doneCh)
			}

			return nil
		},
	); err != nil {
		c.addErrToPathCh(err, pathCh, doneCh)
	}
}

// collectDefaultDPaths collects the path "Dockerfile".
func (c *collector) collectDefaultDPaths(
	doneCh <-chan struct{},
) chan *pathResult {
	return c.collectDefaultPaths(c.BaseDir, c.dBaseSet, doneCh)
}

// collectDefaultCPaths collects the paths "docker-compose.yml" and
// "docker-compose.yaml".
func (c *collector) collectDefaultCPaths(
	doneCh <-chan struct{},
) chan *pathResult {
	return c.collectDefaultPaths(c.BaseDir, c.cBaseSet, doneCh)
}

// collectDefaultPaths collects the paths Dockerfile, docker-compose.yml, and
// docker-compose.yaml.
func (c *collector) collectDefaultPaths(
	bDir string,
	bSet map[string]struct{},
	doneCh <-chan struct{},
) chan *pathResult {
	wg := sync.WaitGroup{}
	pathCh := make(chan *pathResult)

	wg.Add(1)

	go func() {
		defer wg.Done()

		for p := range bSet {
			p = filepath.ToSlash(filepath.Join(bDir, p))
			if c.isRegularFile(p) {
				c.addPathToPathCh(p, pathCh, doneCh)
			}
		}
	}()

	go func() {
		wg.Wait()
		close(pathCh)
	}()

	return pathCh
}

// isRegularFile checks that a file exists and no mode type bits are set.
func (c *collector) isRegularFile(p string) bool {
	fi, err := os.Stat(p)
	if err == nil {
		if mode := fi.Mode(); mode.IsRegular() {
			return true
		}
	}

	log.Printf("%s is not a regular file.", p)

	return false
}

// addPathToPathCh adds a path to the path channel, ensuring not to block
// the calling goroutine.
func (c *collector) addPathToPathCh(
	p string,
	pathCh chan<- *pathResult,
	doneCh <-chan struct{},
) {
	select {
	case <-doneCh:
	case pathCh <- &pathResult{path: p}:
	}
}

// addErrToPathCh adds an error to the path channel, ensuring not to block
// the calling goroutine.
func (c *collector) addErrToPathCh(
	err error,
	pathCh chan<- *pathResult,
	doneCh <-chan struct{},
) {
	select {
	case <-doneCh:
	case pathCh <- &pathResult{err: err}:
	}
}

// validatePaths ensures that paths are not outside of the current working
// directory.
func (c *collector) validatePaths(dPaths, cPaths []string) error {
	for _, paths := range [][]string{dPaths, cPaths} {
		for _, p := range paths {
			if strings.HasPrefix(p, "..") {
				return fmt.Errorf(
					"'%s' is outside the current working directory", p,
				)
			}
		}
	}

	return nil
}

// convertPathChsToSls converts the paths channels to slices, deduplicating
// the paths while checking for errors.
func (c *collector) convertPathChsToSls(
	dPathCh,
	cPathCh <-chan *pathResult,
) ([]string, []string, error) {
	dPathSet := map[string]struct{}{}
	cPathSet := map[string]struct{}{}

	for {
		select {
		case pathRes, ok := <-dPathCh:
			if err := c.handlePathResult(
				&dPathCh, pathRes, dPathSet, ok,
			); err != nil {
				return nil, nil, err
			}
		case pathRes, ok := <-cPathCh:
			if err := c.handlePathResult(
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

	i := 0

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

// handlePathResult adds the path result to its appropriate set, checking
// for error.
func (c *collector) handlePathResult(
	pathCh *<-chan *pathResult,
	pathRes *pathResult,
	pathSet map[string]struct{},
	ok bool,
) error {
	switch ok {
	case true:
		if pathRes.err != nil {
			return pathRes.err
		}

		pathSet[pathRes.path] = struct{}{}
	case false:
		*pathCh = nil
	}

	return nil
}
