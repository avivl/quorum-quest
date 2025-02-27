// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/avivl/quorum-quest/internal/store/dynamodb"
	"github.com/avivl/quorum-quest/internal/store/scylladb"
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

	// Basic configuration
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configPath)
	v.AddConfigPath(".")

	// Environment variable configuration
	v.SetEnvPrefix("QUORUMQUEST")
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

	// Read configuration file (if exists)
	if err := cl.v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, nil, fmt.Errorf("error reading config file: %w", err)
		}
		fmt.Println("No config file found, using defaults and environment variables")
	}

	// Load configuration (environment variables will take precedence)
	config, err := loadConfiguration(cl.v, loadFn)
	if err != nil {
		return nil, nil, err
	}

	cl.mu.Lock()
	cl.currentConfig = config
	cl.mu.Unlock()

	return cl, config, nil
}

// loadConfiguration loads configuration using the provided loader function
func loadConfiguration[T store.StoreConfig](v *viper.Viper, loadFn ConfigLoadFn[T]) (*GlobalConfig[T], error) {
	// Load store config using the provided function
	storeConfig, err := loadFn(v)
	if err != nil {
		return nil, fmt.Errorf("failed to load store config: %w", err)
	}

	// Create the global config
	config := &GlobalConfig[T]{
		Store: storeConfig,
		Observability: observability.Config{
			ServiceName:    getEnvOrValue(v, "QUORUMQUEST_OBSERVABILITY_SERVICENAME", v.GetString("observability.serviceName")),
			ServiceVersion: getEnvOrValue(v, "QUORUMQUEST_OBSERVABILITY_SERVICEVERSION", v.GetString("observability.serviceVersion")),
			Environment:    getEnvOrValue(v, "QUORUMQUEST_OBSERVABILITY_ENVIRONMENT", v.GetString("observability.environment")),
			OTelEndpoint:   getEnvOrValue(v, "QUORUMQUEST_OBSERVABILITY_OTELENDPOINT", v.GetString("observability.otelEndpoint")),
		},
		Logger: observability.LoggerConfig{
			Level: observability.LogLevel(getEnvOrValue(v, "QUORUMQUEST_LOGGER_LEVEL", v.GetString("logger.level"))),
		},
		ServerAddress: getEnvOrValue(v, "QUORUMQUEST_SERVERADDRESS", v.GetString("serverAddress")),
	}

	return config, nil
}

// ScyllaConfigLoader loads ScyllaDB configuration
// internal/config/config.go
// Remove debug print statements
func ScyllaConfigLoader(v *viper.Viper) (*scylladb.ScyllaDBConfig, error) {
	// Set ScyllaDB specific defaults first
	setScyllaDefaults(v)

	// Create configuration with environment variables taking precedence
	config := &scylladb.ScyllaDBConfig{
		Host:        getEnvOrValue(v, "QUORUMQUEST_SCYLLADBCONFIG_HOST", v.GetString("scyllaDbConfig.host")),
		Port:        int32(getEnvIntOrValue(v, "QUORUMQUEST_SCYLLADBCONFIG_PORT", v.GetInt("scyllaDbConfig.port"))),
		Keyspace:    getEnvOrValue(v, "QUORUMQUEST_SCYLLADBCONFIG_KEYSPACE", v.GetString("scyllaDbConfig.keyspace")),
		Table:       getEnvOrValue(v, "QUORUMQUEST_SCYLLADBCONFIG_TABLE", v.GetString("scyllaDbConfig.table")),
		TTL:         int32(getEnvIntOrValue(v, "QUORUMQUEST_SCYLLADBCONFIG_TTL", v.GetInt("scyllaDbConfig.ttl"))),
		Consistency: getEnvOrValue(v, "QUORUMQUEST_SCYLLADBCONFIG_CONSISTENCY", v.GetString("scyllaDbConfig.consistency")),
		Endpoints:   getEnvSliceOrValue(v, "QUORUMQUEST_SCYLLADBCONFIG_ENDPOINTS", v.GetStringSlice("scyllaDbConfig.endpoints")),
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid ScyllaDB configuration: %w", err)
	}

	return config, nil
}

func getEnvOrValue(v *viper.Viper, envKey, defaultValue string) string {
	if value := os.Getenv(envKey); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrValue(v *viper.Viper, envKey string, defaultValue int) int {
	if value := os.Getenv(envKey); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvSliceOrValue(v *viper.Viper, envKey string, defaultValue []string) []string {
	if value := os.Getenv(envKey); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
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

// setScyllaDefaults sets default values for ScyllaDB configuration
func setScyllaDefaults(v *viper.Viper) {
	defaults := map[string]interface{}{
		"scyllaDbConfig": map[string]interface{}{
			"host":        "127.0.0.1",
			"port":        9042,
			"keyspace":    "quorum-ques",
			"table":       "services",
			"ttl":         15,
			"consistency": "CONSISTENCY_QUORUM",
			"endpoints":   []string{"localhost:9042"},
		},
		"observability": map[string]interface{}{
			"serviceName":    "quorum-quest",
			"serviceVersion": "0.1.0",
			"environment":    "development",
			"otelEndpoint":   "localhost:4317",
		},
		"logger": map[string]interface{}{
			"level": "LOG_LEVELS_INFOLEVEL",
		},
		"serverAddress": "localhost:5050",
	}

	for key, value := range defaults {
		v.SetDefault(key, value)
	}
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
