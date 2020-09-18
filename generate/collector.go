package generate

import (
	"github.com/safe-waters/docker-lock/generate/collect"
)

// PathCollector contains PathCollectors for Dockerfiles
// and docker-compose files.
type PathCollector struct {
	DockerfileCollector  *collect.PathCollector
	ComposefileCollector *collect.PathCollector
}

// IPathCollector provides an interface for PathCollector's exported
// methods, which are used by Generator.
type IPathCollector interface {
	CollectPaths(done <-chan struct{}) (
		dockerfilePaths <-chan *collect.PathResult,
		composefilePaths <-chan *collect.PathResult,
	)
}

// CollectPaths collects Dockerfile and docker-compose file paths.
func (p *PathCollector) CollectPaths(
	done <-chan struct{},
) (
	dockerfilePaths <-chan *collect.PathResult,
	composefilePaths <-chan *collect.PathResult,
) {
	if p.DockerfileCollector != nil {
		dockerfilePaths = p.DockerfileCollector.CollectPaths(done)
	}

	if p.ComposefileCollector != nil {
		composefilePaths = p.ComposefileCollector.CollectPaths(done)
	}

	return
}
