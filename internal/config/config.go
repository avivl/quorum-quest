package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/avivl/quorum-quest/internal/store/dynamodb"
	"github.com/avivl/quorum-quest/internal/store/scylladb"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// ConfigLoader handles loading of configurations
type ConfigLoader struct {
	v             *viper.Viper
	mu            sync.RWMutex
	watchers      []func(interface{})
	currentConfig interface{}
}

// ConfigLoadFn defines a function type for loading specific configurations
type ConfigLoadFn[T store.StoreConfig] func(*viper.Viper) (T, error)

// GlobalConfig represents the complete application configuration
type GlobalConfig[T store.StoreConfig] struct {
	Store         T                          `yaml:"-"`
	Observability observability.Config       `yaml:"observability"`
	Logger        observability.LoggerConfig `yaml:"logger"`
	ServerAddress string                     `yaml:"serverAddress"`
}

// NewConfigLoader creates a new configuration loader
func NewConfigLoader(configPath string) *ConfigLoader {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configPath)
	v.AddConfigPath(".")

	v.SetEnvPrefix("QuorumQuest")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	return &ConfigLoader{
		v:        v,
		watchers: make([]func(interface{}), 0),
	}
}

// AddWatcher adds a callback function that will be called when configuration changes
func (cl *ConfigLoader) AddWatcher(callback func(interface{})) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.watchers = append(cl.watchers, callback)
}

// GetCurrentConfig returns the current configuration
func (cl *ConfigLoader) GetCurrentConfig() interface{} {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	return cl.currentConfig
}

// notifyWatchers calls all registered watchers with the new configuration
func (cl *ConfigLoader) notifyWatchers(newConfig interface{}) {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	for _, watcher := range cl.watchers {
		watcher(newConfig)
	}
}

// LoadConfig loads the complete application configuration including store config
func LoadConfig[T store.StoreConfig](configPath string, loadFn ConfigLoadFn[T]) (*ConfigLoader, *GlobalConfig[T], error) {
	cl := NewConfigLoader(configPath)

	// Set defaults
	setDefaults(cl.v)

	// Read configuration file
	if err := cl.v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, nil, fmt.Errorf("error reading config file: %w", err)
		}
		fmt.Println("No config file found, using defaults and environment variables")
	}

	// Load initial configuration
	config, err := loadConfiguration(cl.v, loadFn)
	if err != nil {
		return nil, nil, err
	}

	// Store the current configuration
	cl.mu.Lock()
	cl.currentConfig = config
	cl.mu.Unlock()

	// Setup configuration watching
	cl.v.WatchConfig()
	cl.v.OnConfigChange(func(e fsnotify.Event) {
		fmt.Printf("Config file changed: %s\n", e.Name)

		// Reload configuration
		newConfig, err := loadConfiguration(cl.v, loadFn)
		if err != nil {
			fmt.Printf("Error reloading configuration: %v\n", err)
			return
		}

		// Update current configuration and notify watchers
		cl.mu.Lock()
		cl.currentConfig = newConfig
		cl.mu.Unlock()

		cl.notifyWatchers(newConfig)
	})

	return cl, config, nil
}

// loadConfiguration loads configuration using the provided loader function
func loadConfiguration[T store.StoreConfig](v *viper.Viper, loadFn ConfigLoadFn[T]) (*GlobalConfig[T], error) {
	// Load store config using the provided function
	storeConfig, err := loadFn(v)
	if err != nil {
		return nil, fmt.Errorf("failed to load store config: %w", err)
	}

	// Create and populate global config
	config := &GlobalConfig[T]{}
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("unable to decode global config: %w", err)
	}

	// Set the store config
	config.Store = storeConfig

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// validateConfig validates all configuration sections
func validateConfig[T any](cfg *GlobalConfig[T]) error {
	// Type assert Store to StoreConfig interface
	storeConfig, ok := any(cfg.Store).(store.StoreConfig)
	if !ok {
		return fmt.Errorf("store config does not implement StoreConfig interface")
	}

	if err := storeConfig.Validate(); err != nil {
		return fmt.Errorf("store configuration error: %w", err)
	}

	// Validate OpenTelemetry config
	if cfg.Observability.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}
	if cfg.Observability.ServiceVersion == "" {
		return fmt.Errorf("service version is required")
	}
	if cfg.Observability.Environment == "" {
		return fmt.Errorf("environment is required")
	}
	if cfg.Observability.OTelEndpoint == "" {
		return fmt.Errorf("OpenTelemetry endpoint is required")
	}

	// Validate server address
	if cfg.ServerAddress == "" {
		return fmt.Errorf("server address is required")
	}

	return nil
}

// setDefaults sets default values for configuration
func setDefaults(v *viper.Viper) {
	// OpenTelemetry defaults
	v.SetDefault("observability.serviceName", "quorum-quest")
	v.SetDefault("observability.serviceVersion", "0.1.0")
	v.SetDefault("observability.environment", "development")
	v.SetDefault("observability.otelEndpoint", "localhost:4317")

	// Logger defaults
	v.SetDefault("logger.level", "LOG_LEVELS_INFOLEVEL")

	// Server defaults
	v.SetDefault("serverAddress", "localhost:5050")
}

// ScyllaConfigLoader loads ScyllaDB configuration
func ScyllaConfigLoader(v *viper.Viper) (*scylladb.ScyllaDBConfig, error) {
	// Set ScyllaDB defaults
	v.SetDefault("scyllaDbConfig.host", "127.0.0.1")
	v.SetDefault("scyllaDbConfig.port", 9042)
	v.SetDefault("scyllaDbConfig.keyspace", "ballot")
	v.SetDefault("scyllaDbConfig.table", "services")
	v.SetDefault("scyllaDbConfig.ttl", 15)
	v.SetDefault("scyllaDbConfig.consistency", "CONSISTENCY_QUORUM")
	v.SetDefault("scyllaDbConfig.endpoints", []string{"localhost:9042"})

	config := &scylladb.ScyllaDBConfig{}
	if err := v.UnmarshalKey("scyllaDbConfig", config); err != nil {
		return nil, fmt.Errorf("unable to decode ScyllaDB config: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid ScyllaDB configuration: %w", err)
	}

	return config, nil
}

// DynamoConfigLoader loads DynamoDB configuration
func DynamoConfigLoader(v *viper.Viper) (*dynamodb.DynamoDBConfig, error) {
	// Set DynamoDB defaults
	v.SetDefault("dynamoDbConfig.region", "us-west-2")
	v.SetDefault("dynamoDbConfig.table", "services")
	v.SetDefault("dynamoDbConfig.ttl", 15)
	v.SetDefault("dynamoDbConfig.endpoints", []string{"dynamodb.us-west-2.amazonaws.com"})
	v.SetDefault("dynamoDbConfig.profile", "default")
	v.SetDefault("dynamoDbConfig.maxRetries", 3)

	config := &dynamodb.DynamoDBConfig{}
	if err := v.UnmarshalKey("dynamoDbConfig", config); err != nil {
		return nil, fmt.Errorf("unable to decode DynamoDB config: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid DynamoDB configuration: %w", err)
	}

	return config, nil
}

// Example usage:
/*
func main() {
    // For ScyllaDB
    scyllaLoader, scyllaConfig, err := LoadConfig[*scylladb.ScyllaDBConfig](
        "/etc/myapp",
        ScyllaConfigLoader,
    )
    if err != nil {
        log.Fatal(err)
    }

    // For DynamoDB
    dynamoLoader, dynamoConfig, err := LoadConfig[*dynamodb.DynamoDBConfig](
        "/etc/myapp",
        DynamoConfigLoader,
    )
    if err != nil {
        log.Fatal(err)
    }

    // Add watchers for config changes
    scyllaLoader.AddWatcher(func(newConfig interface{}) {
        cfg := newConfig.(*GlobalConfig[*scylladb.ScyllaDBConfig])
        // Handle ScyllaDB config changes
    })

    dynamoLoader.AddWatcher(func(newConfig interface{}) {
        cfg := newConfig.(*GlobalConfig[*dynamodb.DynamoDBConfig])
        // Handle DynamoDB config changes
    })
}*/
