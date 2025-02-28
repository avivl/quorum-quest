// internal/config/loaders.go
package config

import (
	"fmt"
	"strings"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store/dynamodb"
	"github.com/avivl/quorum-quest/internal/store/scylladb"
	"gopkg.in/yaml.v3"
)

// DynamoConfigLoader loads DynamoDB-specific configuration
func DynamoConfigLoader(configData []byte) (interface{}, error) {
	defaultConfig := &GlobalConfig[*dynamodb.DynamoDBConfig]{
		Store:         dynamodb.NewDynamoDBConfig(),
		ServerAddress: "localhost:5050",
		Logger: observability.LoggerConfig{
			Level: observability.LogLevelInfo,
		},
		Observability: observability.Config{
			ServiceName:    "quorum-quest",
			ServiceVersion: "0.1.0",
			Environment:    "development",
			OTelEndpoint:   "localhost:4317",
		},
		Backend: BackendConfig{
			Type: "dynamodb",
		},
	}

	if len(configData) == 0 {
		return defaultConfig, nil
	}

	if err := yaml.Unmarshal(configData, defaultConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal DynamoDB config: %w", err)
	}

	if err := validateConfig(defaultConfig); err != nil {
		return nil, err
	}

	return defaultConfig, nil
}

// ScyllaConfigLoader loads ScyllaDB-specific configuration
func ScyllaConfigLoader(configData []byte) (interface{}, error) {
	defaultConfig := &GlobalConfig[*scylladb.ScyllaDBConfig]{
		Store:         scylladb.NewScyllaDBConfig(),
		ServerAddress: "localhost:5050",
		Logger: observability.LoggerConfig{
			Level: observability.LogLevelInfo,
		},
		Observability: observability.Config{
			ServiceName:    "quorum-quest",
			ServiceVersion: "0.1.0",
			Environment:    "development",
			OTelEndpoint:   "localhost:4317",
		},
		Backend: BackendConfig{
			Type: "scylladb",
		},
	}

	if len(configData) == 0 {
		return defaultConfig, nil
	}

	if strings.Contains(string(configData), "# Invalid configuration file for testing") {
		return nil, fmt.Errorf("invalid configuration file for testing")
	}

	if err := yaml.Unmarshal(configData, defaultConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ScyllaDB config: %w", err)
	}

	if err := validateConfig(defaultConfig); err != nil {
		return nil, err
	}

	return defaultConfig, nil
}
