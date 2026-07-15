package config

// Config is the parsed atlas.yaml for a repository.
type Config struct {
	Version      int                   `yaml:"version"`
	App          AppConfig             `yaml:"app"`
	Dependencies map[string]Dependency `yaml:"dependencies"`
}

// AppConfig describes the primary application workload.
type AppConfig struct {
	Port int32 `yaml:"port"`
}

// Dependency is a managed infrastructure dependency declared in atlas.yaml.
type Dependency struct {
	Type    DependencyType `yaml:"type"`
	Storage string         `yaml:"storage,omitempty"`
}

// DependencyType identifies a supported (or recognized) dependency kind.
type DependencyType string

const (
	DependencyRedis    DependencyType = "redis"
	DependencyPostgres DependencyType = "postgres"
	DependencyNATS     DependencyType = "nats"
)

// SupportedVersion is the only atlas.yaml version Atlas accepts.
const SupportedVersion = 1
