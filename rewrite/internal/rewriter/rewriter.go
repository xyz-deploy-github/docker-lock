package rewriter

// Compose represents the services section of a docker-compose file.
type Compose struct {
	Services map[string]*Service `yaml:"services"`
}

// Service represents a service in the service section of a docker-compose file.
type Service struct {
	Image string      `yaml:"image"`
	Build interface{} `yaml:"build"`
}
