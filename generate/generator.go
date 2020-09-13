// Package generate provides functionality to generate a Lockfile.
package generate

import (
	"errors"
	"io"
	"reflect"
)

// Generator creates a Lockfile.
type Generator struct {
	DockerfileCollector  ICollector
	ComposefileCollector ICollector
	DockerfileParser     IDockerfileParser
	ComposefileParser    IComposefileParser
	Updater              IUpdater
}

// IGenerator provides an interface for Generator's exported
// methods, which are used by docker-lock's cli as well as Verifier.
type IGenerator interface {
	GenerateLockfile(writer io.Writer) error
}

// NewGenerator returns a Generator after validating its fields.
func NewGenerator(
	dockerfileCollector ICollector,
	composefileCollector ICollector,
	dockerfileParser IDockerfileParser,
	composefileParser IComposefileParser,
	updater IUpdater,
) (*Generator, error) {
	if dockerfileParser == nil || reflect.ValueOf(dockerfileParser).IsNil() {
		return nil, errors.New("dockerfileParser may not be nil")
	}

	if composefileParser == nil || reflect.ValueOf(composefileParser).IsNil() {
		return nil, errors.New("composefileParser may not be nil")
	}

	if updater == nil || reflect.ValueOf(updater).IsNil() {
		return nil, errors.New("updater may not be nil")
	}

	return &Generator{
		DockerfileCollector:  dockerfileCollector,
		ComposefileCollector: composefileCollector,
		DockerfileParser:     dockerfileParser,
		ComposefileParser:    composefileParser,
		Updater:              updater,
	}, nil
}

// GenerateLockfile creates a Lockfile and writes it to an io.Writer.
func (g *Generator) GenerateLockfile(writer io.Writer) error {
	if writer == nil || reflect.ValueOf(writer).IsNil() {
		return errors.New("writer cannot be nil")
	}

	done := make(chan struct{})

	var dockerfilePaths <-chan *PathResult

	if g.DockerfileCollector != nil &&
		!reflect.ValueOf(g.DockerfileCollector).IsNil() {
		dockerfilePaths = g.DockerfileCollector.Paths(done)
	}

	var composefilePaths <-chan *PathResult

	if g.ComposefileCollector != nil &&
		!reflect.ValueOf(g.ComposefileCollector).IsNil() {
		composefilePaths = g.ComposefileCollector.Paths(done)
	}

	dockerfileImages := g.DockerfileParser.ParseFiles(dockerfilePaths, done)
	composefileImages := g.ComposefileParser.ParseFiles(composefilePaths, done)

	dockerfileImages, composefileImages = g.Updater.UpdateDigests(
		dockerfileImages, composefileImages, done,
	)

	lockfile, err := NewLockfile(dockerfileImages, composefileImages, done)
	if err != nil {
		close(done)
		return err
	}

	return lockfile.Write(writer)
}

// DockerfileCollector provides an easy to use Collector for
// collecting Dockerfiles.
func DockerfileCollector(
	flags *Flags,
) (*Collector, error) {
	if flags == nil {
		return nil, errors.New("flags cannot be nil")
	}

	return NewCollector(
		flags.FlagsWithSharedValues.BaseDir, []string{"Dockerfile"},
		flags.DockerfileFlags.ManualPaths, flags.DockerfileFlags.Globs,
		flags.DockerfileFlags.Recursive,
	)
}

// ComposefileCollector provides an easy to use Collector for
// collecting docker-compose files.
func ComposefileCollector(
	flags *Flags,
) (*Collector, error) {
	if flags == nil {
		return nil, errors.New("flags cannot be nil")
	}

	return NewCollector(
		flags.FlagsWithSharedValues.BaseDir,
		[]string{"docker-compose.yml", "docker-compose.yaml"},
		flags.ComposefileFlags.ManualPaths, flags.ComposefileFlags.Globs,
		flags.ComposefileFlags.Recursive,
	)
}
