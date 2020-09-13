package generate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Collector gathers Dockerfiles and docker-compose files.
type Collector struct {
	BaseDir      string
	DefaultPaths []string
	ManualPaths  []string
	Globs        []string
	Recursive    bool
}

// ICollector provides an interface for Collector's exported
// methods, which are used by Generator.
type ICollector interface {
	Paths(done <-chan struct{}) <-chan *PathResult
}

// PathResult holds collected paths and errors that occurred
// while gathering them.
type PathResult struct {
	Path string
	Err  error
}

// NewCollector returns a Collector after validating its fields.
func NewCollector(
	baseDir string,
	defaultPaths []string,
	manualPaths []string,
	globs []string,
	recursive bool,
) (*Collector, error) {
	if recursive && len(defaultPaths) == 0 {
		return nil,
			errors.New("if recursive is true, defaultPaths must also be set")
	}

	return &Collector{
		BaseDir:      baseDir,
		DefaultPaths: defaultPaths,
		ManualPaths:  manualPaths,
		Globs:        globs,
		Recursive:    recursive,
	}, nil
}

// Paths gathers file paths specified by Collector. It removes duplicates and
// ensures that the file paths are within a subdirectory of the base directory.
func (c *Collector) Paths(done <-chan struct{}) <-chan *PathResult {
	pathResults := make(chan *PathResult)

	var waitGroup sync.WaitGroup

	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()

		intermediatePathResults := make(chan *PathResult)
		intermediateDone := make(chan struct{})

		var intermediateWaitGroup sync.WaitGroup

		if len(c.ManualPaths) != 0 {
			intermediateWaitGroup.Add(1)

			go c.collectManualPaths(
				intermediatePathResults, intermediateDone,
				&intermediateWaitGroup,
			)
		}

		if len(c.Globs) != 0 {
			intermediateWaitGroup.Add(1)

			go c.collectGlobs(
				intermediatePathResults, intermediateDone,
				&intermediateWaitGroup,
			)
		}

		if c.Recursive {
			intermediateWaitGroup.Add(1)

			go c.collectRecursive(
				intermediatePathResults, intermediateDone,
				&intermediateWaitGroup,
			)
		}

		if len(c.ManualPaths) == 0 &&
			len(c.Globs) == 0 &&
			!c.Recursive &&
			len(c.DefaultPaths) != 0 {
			intermediateWaitGroup.Add(1)

			go c.collectDefaultPaths(
				intermediatePathResults, intermediateDone,
				&intermediateWaitGroup,
			)
		}

		go func() {
			intermediateWaitGroup.Wait()
			close(intermediatePathResults)
		}()

		seenPaths := map[string]struct{}{}

		for result := range intermediatePathResults {
			if result.Err != nil {
				close(intermediateDone)

				select {
				case <-done:
				case pathResults <- result:
				}

				return
			}

			if _, ok := seenPaths[result.Path]; !ok {
				seenPaths[result.Path] = struct{}{}

				select {
				case <-done:
				case pathResults <- result:
				}
			}
		}
	}()

	go func() {
		waitGroup.Wait()
		close(pathResults)
	}()

	return pathResults
}

func (c *Collector) collectManualPaths(
	pathResults chan<- *PathResult,
	done <-chan struct{},
	waitGroup *sync.WaitGroup,
) {
	defer waitGroup.Done()

	for _, path := range c.ManualPaths {
		path = filepath.Join(c.BaseDir, path)

		if err := c.validatePath(path); err != nil {
			select {
			case <-done:
			case pathResults <- &PathResult{Err: err}:
			}

			return
		}

		select {
		case <-done:
			return
		case pathResults <- &PathResult{Path: path}:
		}
	}
}

func (c *Collector) collectDefaultPaths(
	pathResults chan<- *PathResult,
	done <-chan struct{},
	waitGroup *sync.WaitGroup,
) {
	defer waitGroup.Done()

	for _, path := range c.DefaultPaths {
		path = filepath.Join(c.BaseDir, path)

		if err := c.validatePath(path); err != nil {
			select {
			case <-done:
			case pathResults <- &PathResult{Err: err}:
			}

			return
		}

		if err := c.fileExists(path); err == nil {
			select {
			case <-done:
				return
			case pathResults <- &PathResult{Path: path}:
			}
		}
	}
}

func (c *Collector) collectGlobs(
	pathResults chan<- *PathResult,
	done <-chan struct{},
	waitGroup *sync.WaitGroup,
) {
	defer waitGroup.Done()

	for _, glob := range c.Globs {
		glob = filepath.Join(c.BaseDir, glob)

		paths, err := filepath.Glob(glob)
		if err != nil {
			select {
			case <-done:
			case pathResults <- &PathResult{Err: err}:
			}

			return
		}

		for _, path := range paths {
			if err := c.validatePath(path); err != nil {
				select {
				case <-done:
				case pathResults <- &PathResult{Err: err}:
				}

				return
			}

			select {
			case <-done:
				return
			case pathResults <- &PathResult{Path: path}:
			}
		}
	}
}

func (c *Collector) collectRecursive(
	pathResults chan<- *PathResult,
	done <-chan struct{},
	waitGroup *sync.WaitGroup,
) {
	defer waitGroup.Done()

	defaultSet := map[string]struct{}{}

	for _, p := range c.DefaultPaths {
		defaultSet[p] = struct{}{}
	}

	if err := filepath.Walk(
		c.BaseDir, func(path string, info os.FileInfo, err error,
		) error {
			if err != nil {
				return err
			}

			if _, ok := defaultSet[filepath.Base(path)]; ok {
				if err := c.validatePath(path); err != nil {
					return err
				}

				select {
				case <-done:
				case pathResults <- &PathResult{Path: path}:
				}
			}

			return nil
		},
	); err != nil {
		select {
		case <-done:
		case pathResults <- &PathResult{Err: err}:
		}
	}
}

func (c *Collector) fileExists(path string) error {
	_, err := os.Stat(path)
	return err
}

func (c *Collector) validatePath(path string) error {
	if strings.HasPrefix(path, "..") {
		return fmt.Errorf("'%s' is outside the current working directory", path)
	}

	return nil
}
