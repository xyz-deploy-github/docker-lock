package generate

import "fmt"

type compose struct {
	Services map[string]*service `yaml:"services"`
}

type service struct {
	ImageName    string        `yaml:"image"`
	BuildWrapper *buildWrapper `yaml:"build"`
}

// buildWrapper describes the "build" section of a service. It is used
// when unmarshalling to either contain simple or verbose build sections.
type buildWrapper struct {
	Build interface{}
}

// verbose represents a "build" section with build keys specified. For instance,
// build:
//     context: ./dirWithDockerfile
//     dockerfile: Dockerfile
type verbose struct {
	Context        string   `yaml:"context"`
	DockerfilePath string   `yaml:"dockerfile"`
	Args           []string `yaml:"args"`
}

// simple represents a "build" section without build keys. For instance,
// build: dirWithDockerfile
type simple string

// UnmarshalYAML unmarshals the "build" section of a service. It first
// tries to unmarshal the bytes into a verbose type. If that fails,
// it tries to unmarshal the bytes into a simple type. If neither succeeds,
// it returns an error.
func (b *buildWrapper) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*b = buildWrapper{}

	var v verbose
	if err := unmarshal(&v); err == nil {
		b.Build = v
		return nil
	}

	var s simple
	if err := unmarshal(&s); err == nil {
		b.Build = s
		return nil
	}

	return fmt.Errorf("unable to unmarshal service")
}
