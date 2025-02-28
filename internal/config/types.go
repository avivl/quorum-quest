// internal/config/types.go
package config

import (
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
)

// GlobalConfig represents the application configuration with generic store config
type GlobalConfig[T store.StoreConfig] struct {
	ServerAddress string                     `yaml:"serverAddress"`
	Store         T                          `yaml:"store"`
	Logger        observability.LoggerConfig `yaml:"logger"`
	Observability observability.Config       `yaml:"observability"`
	Backend       BackendConfig              `yaml:"backend"`
}

// BackendConfig represents the backend configuration section
type BackendConfig struct {
	Type string `yaml:"type"`
}

// ConfigLoaderFunc is a function type for loading configuration
type ConfigLoaderFunc func([]byte) (interface{}, error)

// Add this if it's not already in the file
type RootConfig struct {
	Backend BackendConfig `yaml:"backend"`
}
