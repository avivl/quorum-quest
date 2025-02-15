package store

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

var (
	ErrInvalidConfig = errors.New("invalid configuration")
)

// Config defines the base configuration interface
type Config interface {
	// GetTableName returns the table name for the store
	GetTableName() string
	// GetTTL returns the time-to-live duration
	GetTTL() int32
	// GetEndpoints returns the list of endpoints
	GetEndpoints() []string
	// Validate checks if the configuration is valid
	Validate() error
}

// ConfigLoader handles loading of database configurations
type ConfigLoader struct {
	v *viper.Viper
}

// ConfigLoadFn defines a function type for loading specific configurations
type ConfigLoadFn[T Config] func(*viper.Viper) (T, error)

// NewConfigLoader creates a new configuration loader
func NewConfigLoader(configPath string) *ConfigLoader {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configPath)
	v.AddConfigPath(".")

	return &ConfigLoader{v: v}
}

// LoadConfig loads configuration using the provided loader function
func LoadConfig[T Config](configPath, prefix string, loadFn ConfigLoadFn[T]) (T, error) {
	cl := NewConfigLoader(configPath)

	// Enable environment variables with provided prefix
	cl.v.SetEnvPrefix(strings.ToUpper(prefix))
	cl.v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	cl.v.AutomaticEnv()

	// Read configuration file
	if err := cl.v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return *new(T), fmt.Errorf("error reading config file: %w", err)
		}
		fmt.Println("No config file found, using defaults and environment variables")
	}

	// Use the provided function to load the configuration
	return loadFn(cl.v)
}
