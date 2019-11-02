package parse

import "fmt"

// Compose represents the services section of a docker-compose file.
type Compose struct {
	Services map[string]*Service `yaml:"services"`
}

// Service represents a service in the service section of a docker-compose file.
type Service struct {
	ImageName    string        `yaml:"image"`
	BuildWrapper *BuildWrapper `yaml:"build"`
}

// Verbose represents a "build" section with build keys specified. For instance,
// build:
//     context: ./dirWithDockerfile
//     dockerfile: Dockerfile
type Verbose struct {
	Context        string   `yaml:"context"`
	DockerfilePath string   `yaml:"dockerfile"`
	Args           []string `yaml:"args"`
}

// Simple represents a "build" section without build keys. For instance,
// build: dirWithDockerfile
type Simple string

// BuildWrapper describes the "build" section of a service. It is used
// when unmarshalling to either contain Simple or Verbose build sections.
type BuildWrapper struct {
	Build interface{}
}

// UnmarshalYAML unmarshals the "build" section of a service. It first
// tries to unmarshal the bytes into a Verbose type. If that fails,
// it tries to unmarshal the bytes into a Simple type. If neither succeeds,
// it returns an error.
func (b *BuildWrapper) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*b = BuildWrapper{}
	var v Verbose
	if err := unmarshal(&v); err == nil {
		b.Build = v
		return nil
	}
	var s Simple
	if err := unmarshal(&s); err == nil {
		b.Build = s
		return nil
	}
	return fmt.Errorf("unable to unmarshal service")
}
