// internal/config/utils_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/avivl/quorum-quest/internal/store/dynamodb"
	"github.com/avivl/quorum-quest/internal/store/redis"
	"github.com/avivl/quorum-quest/internal/store/scylladb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsConfigFileName(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		expected bool
	}{
		{"Standard config.yaml", "config.yaml", true},
		{"Standard config.yml", "config.yml", true},
		{"Project config.yaml", "quorum-quest.yaml", true},
		{"Project config.yml", "quorum-quest.yml", true},
		{"Invalid extension", "config.json", false},
		{"Invalid prefix", "my-config.yaml", false},
		{"Invalid name", "settings.yaml", false},
		{"Case sensitivity", "Config.Yaml", false},
		{"Empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isConfigFileName(tt.fileName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeBackendType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Lowercase scylladb", "scylladb", "scylladb"},
		{"Uppercase SCYLLADB", "SCYLLADB", "scylladb"},
		{"Mixed case ScyllaDB", "ScyllaDB", "scylladb"},
		{"With spaces", " scylladb ", "scylladb"},
		{"Short form scylla", "scylla", "scylladb"},
		{"Lowercase dynamodb", "dynamodb", "dynamodb"},
		{"Uppercase DYNAMODB", "DYNAMODB", "dynamodb"},
		{"Mixed case DynamoDB", "DynamoDB", "dynamodb"},
		{"Short form dynamo", "dynamo", "dynamodb"},
		{"Unknown backend", "mongodb", "mongodb"},
		{"Empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeBackendType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetBackendTypeFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   interface{}
		expected string
	}{
		{
			name:     "ScyllaDB Config",
			config:   &scylladb.ScyllaDBConfig{},
			expected: "scylladb",
		},
		{
			name:     "DynamoDB Config",
			config:   &dynamodb.DynamoDBConfig{},
			expected: "dynamodb",
		},
		{
			name:     "Unknown Config",
			config:   &redis.RedisConfig{},
			expected: "",
		},
		{
			name:     "Nil Config",
			config:   nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getBackendTypeFromConfig(tt.config)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	t.Run("Default ScyllaDB Config", func(t *testing.T) {
		config := createDefaultConfig[*scylladb.ScyllaDBConfig]()

		// Verify store config defaults
		assert.NotNil(t, config.Store)
		assert.Equal(t, "127.0.0.1", config.Store.Host)
		assert.Equal(t, int32(9042), config.Store.Port)
		assert.Equal(t, "quorum-quest", config.Store.Keyspace)
		assert.Equal(t, "services", config.Store.Table)
		assert.Equal(t, int32(15), config.Store.TTL)

		// Verify common config defaults
		assert.Equal(t, "localhost:5050", config.ServerAddress)
		assert.Equal(t, "quorum-quest", config.Observability.ServiceName)
		assert.Equal(t, "0.1.0", config.Observability.ServiceVersion)
		assert.Equal(t, "development", config.Observability.Environment)
		assert.Equal(t, "localhost:4317", config.Observability.OTelEndpoint)
	})

	t.Run("Default DynamoDB Config", func(t *testing.T) {
		config := createDefaultConfig[*dynamodb.DynamoDBConfig]()

		// Verify store config defaults
		assert.NotNil(t, config.Store)
		assert.Equal(t, "us-west-2", config.Store.Region)
		assert.Equal(t, "quorum-quest", config.Store.Table)
		assert.Equal(t, int32(15), config.Store.TTL)
		assert.Equal(t, []string{"dynamodb.us-west-2.amazonaws.com"}, config.Store.Endpoints)

		// Verify common config defaults
		assert.Equal(t, "dynamodb", config.Backend.Type)
	})

	t.Run("Default Redis Config", func(t *testing.T) {
		config := createDefaultConfig[*redis.RedisConfig]()

		// Verify store config defaults
		assert.NotNil(t, config.Store)
		assert.Equal(t, "localhost", config.Store.Host)
		assert.Equal(t, 6379, config.Store.Port)
		assert.Equal(t, "", config.Store.Password)
		assert.Equal(t, 0, config.Store.DB)
		assert.Equal(t, int32(15), config.Store.TTL)

		// Verify backend type is empty for Redis (not explicitly supported)
		assert.Equal(t, "", config.Backend.Type)
	})
}

func TestResolveConfigFilePath(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Test with empty string
	t.Run("Empty Path", func(t *testing.T) {
		_, err := resolveConfigFilePath("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config path cannot be empty")
	})

	// Test with non-existent path
	t.Run("Non-existent Path", func(t *testing.T) {
		_, err := resolveConfigFilePath(filepath.Join(tmpDir, "non-existent"))
		assert.Error(t, err)
	})

	// Test with file path
	t.Run("File Path", func(t *testing.T) {
		// Create a config file
		configFile := filepath.Join(tmpDir, "test-config.yaml")
		err := os.WriteFile(configFile, []byte("test content"), 0644)
		require.NoError(t, err)

		path, err := resolveConfigFilePath(configFile)
		assert.NoError(t, err)
		assert.Equal(t, configFile, path)
	})

	// Test with directory path containing config file
	t.Run("Directory with Config", func(t *testing.T) {
		// Create a new directory
		configDir := filepath.Join(tmpDir, "config-dir")
		err := os.Mkdir(configDir, 0755)
		require.NoError(t, err)

		// Create a config file in the directory
		configFile := filepath.Join(configDir, "config.yaml")
		err = os.WriteFile(configFile, []byte("test content"), 0644)
		require.NoError(t, err)

		path, err := resolveConfigFilePath(configDir)
		assert.NoError(t, err)
		assert.Equal(t, configFile, path)
	})

	// Test with directory path without config file
	t.Run("Directory without Config", func(t *testing.T) {
		// Create a new empty directory
		emptyDir := filepath.Join(tmpDir, "empty-dir")
		err := os.Mkdir(emptyDir, 0755)
		require.NoError(t, err)

		_, err = resolveConfigFilePath(emptyDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no config file found in directory")
	})

	// Test with directory containing multiple config files (priority order)
	t.Run("Directory with Multiple Configs", func(t *testing.T) {
		// Create a new directory
		multiConfigDir := filepath.Join(tmpDir, "multi-config-dir")
		err := os.Mkdir(multiConfigDir, 0755)
		require.NoError(t, err)

		// Create multiple config files in different order
		configFiles := []string{
			filepath.Join(multiConfigDir, "quorum-quest.yml"),
			filepath.Join(multiConfigDir, "quorum-quest.yaml"),
			filepath.Join(multiConfigDir, "config.yml"),
			filepath.Join(multiConfigDir, "config.yaml"),
		}

		for _, file := range configFiles {
			err = os.WriteFile(file, []byte("test content"), 0644)
			require.NoError(t, err)
		}

		// Should prefer config.yaml (first in the candidates list)
		path, err := resolveConfigFilePath(multiConfigDir)
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(multiConfigDir, "config.yaml"), path)
	})
}
