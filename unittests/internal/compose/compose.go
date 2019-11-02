package compose

// Compose represents the services section of a docker-compose file.
type Compose struct {
	Services map[string]*Service `yaml:"services"`
}

// Service represents a service in the service section of a docker-compose file.
type Service struct {
	ImageName string `yaml:"image"`
}
