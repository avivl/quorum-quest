// internal/config/loader_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/avivl/quorum-quest/internal/store/dynamodb"
	"github.com/avivl/quorum-quest/internal/store/scylladb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScyllaConfigLoader(t *testing.T) {
	// Test with valid ScyllaDB config
	t.Run("Valid ScyllaDB Config", func(t *testing.T) {
		validConfig := `
backend:
  type: "scylladb"

store:
  host: "test-host"
  port: 9042
  keyspace: "test-keyspace"
  table: "test-table"
  ttl: 30
  consistency: "CONSISTENCY_QUORUM"
  endpoints:
    - "test-host:9042"

serverAddress: "localhost:5050"
`
		config, err := ScyllaConfigLoader([]byte(validConfig))
		require.NoError(t, err)

		scyllaConfig, ok := config.(*GlobalConfig[*scylladb.ScyllaDBConfig])
		require.True(t, ok, "Expected GlobalConfig[*scylladb.ScyllaDBConfig]")

		assert.Equal(t, "test-host", scyllaConfig.Store.Host)
		assert.Equal(t, int32(9042), scyllaConfig.Store.Port)
		assert.Equal(t, "test-keyspace", scyllaConfig.Store.Keyspace)
		assert.Equal(t, "test-table", scyllaConfig.Store.Table)
		assert.Equal(t, int32(30), scyllaConfig.Store.TTL)
	})

	// Test with empty config data (should return defaults)
	t.Run("Empty Config Data", func(t *testing.T) {
		config, err := ScyllaConfigLoader([]byte{})
		require.NoError(t, err)

		scyllaConfig, ok := config.(*GlobalConfig[*scylladb.ScyllaDBConfig])
		require.True(t, ok, "Expected GlobalConfig[*scylladb.ScyllaDBConfig]")

		// Verify default values
		assert.Equal(t, "127.0.0.1", scyllaConfig.Store.Host)
		assert.Equal(t, int32(9042), scyllaConfig.Store.Port)
		assert.Equal(t, "quorum-quest", scyllaConfig.Store.Keyspace)
	})

	// Test with invalid YAML
	t.Run("Invalid YAML", func(t *testing.T) {
		// Testing with malformed YAML that should cause a parsing error
		invalidYAML := `
backend:
  type: "scylladb"
store:
  host: "test-host"
  port: not-a-number
`
		_, err := ScyllaConfigLoader([]byte(invalidYAML))
		require.Error(t, err, "Expected error with invalid YAML")
		assert.Contains(t, err.Error(), "failed to unmarshal")
	})

	// Test with invalid config (special test marker)
	t.Run("Invalid Config Marker", func(t *testing.T) {
		invalidConfig := `# Invalid configuration file for testing
backend:
  type: "scylladb"
`
		_, err := ScyllaConfigLoader([]byte(invalidConfig))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid configuration file for testing")
	})
}

func TestDynamoConfigLoader(t *testing.T) {
	// Test with valid DynamoDB config
	t.Run("Valid DynamoDB Config", func(t *testing.T) {
		validConfig := `
backend:
  type: "dynamodb"

store:
  region: "test-region"
  table: "test-table"
  ttl: 30
  endpoints:
    - "dynamodb.test-region.amazonaws.com"
  profile: "test-profile"
  accessKeyId: "test-key"
  secretAccessKey: "test-secret"

serverAddress: "localhost:5050"
`
		config, err := DynamoConfigLoader([]byte(validConfig))
		require.NoError(t, err)

		dynamoConfig, ok := config.(*GlobalConfig[*dynamodb.DynamoDBConfig])
		require.True(t, ok, "Expected GlobalConfig[*dynamodb.DynamoDBConfig]")

		assert.Equal(t, "test-region", dynamoConfig.Store.Region)
		assert.Equal(t, "test-table", dynamoConfig.Store.Table)
		assert.Equal(t, int32(30), dynamoConfig.Store.TTL)
		assert.Equal(t, "test-profile", dynamoConfig.Store.Profile)
		assert.Equal(t, "test-key", dynamoConfig.Store.AccessKeyID)
		assert.Equal(t, "test-secret", dynamoConfig.Store.SecretAccessKey)
	})

	// Test with empty config data (should return defaults)
	t.Run("Empty Config Data", func(t *testing.T) {
		config, err := DynamoConfigLoader([]byte{})
		require.NoError(t, err)

		dynamoConfig, ok := config.(*GlobalConfig[*dynamodb.DynamoDBConfig])
		require.True(t, ok, "Expected GlobalConfig[*dynamodb.DynamoDBConfig]")

		// Verify default values
		assert.Equal(t, "us-west-2", dynamoConfig.Store.Region)
		assert.Equal(t, "quorum-quest", dynamoConfig.Store.Table)
		assert.Equal(t, int32(15), dynamoConfig.Store.TTL)
	})

	// Test with invalid YAML
	t.Run("Invalid YAML", func(t *testing.T) {
		// Testing with malformed YAML that should cause a parsing error
		invalidYAML := `
backend:
  type: "dynamodb"
store:
  region: "us-west-2"
  ttl: not-a-number
  endpoints:
    - "dynamodb.us-west-2.amazonaws.com"
`
		_, err := DynamoConfigLoader([]byte(invalidYAML))
		require.Error(t, err, "Expected error with invalid YAML")
		assert.Contains(t, err.Error(), "failed to unmarshal")
	})
}

func TestLoadFromFile(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Test with valid config file
	t.Run("Valid Config File", func(t *testing.T) {
		// Write valid config to file
		validConfig := `
backend:
  type: "scylladb"

store:
  host: "file-host"
  port: 9043
  keyspace: "file-keyspace"
  table: "file-table"
  ttl: 25
  consistency: "CONSISTENCY_QUORUM"
  endpoints:
    - "file-host:9043"

serverAddress: "0.0.0.0:8080"
`
		configPath := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configPath, []byte(validConfig), 0644)
		require.NoError(t, err)

		// Load from file
		config, err := loadFromFile[*scylladb.ScyllaDBConfig](configPath, ScyllaConfigLoader)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Verify loaded config
		assert.Equal(t, "file-host", config.Store.Host)
		assert.Equal(t, int32(9043), config.Store.Port)
		assert.Equal(t, "file-keyspace", config.Store.Keyspace)
		assert.Equal(t, "file-table", config.Store.Table)
		assert.Equal(t, "0.0.0.0:8080", config.ServerAddress)
	})

	// Test with directory containing config file
	t.Run("Directory Path", func(t *testing.T) {
		// Create a new directory
		configDir := filepath.Join(tmpDir, "config-dir")
		err := os.Mkdir(configDir, 0755)
		require.NoError(t, err)

		// Write config to file in directory
		dirConfig := `
backend:
  type: "scylladb"

store:
  host: "dir-host"
  port: 9044
  keyspace: "dir-keyspace"
`
		configPath := filepath.Join(configDir, "config.yaml")
		err = os.WriteFile(configPath, []byte(dirConfig), 0644)
		require.NoError(t, err)

		// Load from directory path
		config, err := loadFromFile[*scylladb.ScyllaDBConfig](configDir, ScyllaConfigLoader)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Verify loaded config
		assert.Equal(t, "dir-host", config.Store.Host)
		assert.Equal(t, int32(9044), config.Store.Port)
		assert.Equal(t, "dir-keyspace", config.Store.Keyspace)
	})

	// Test with non-existent path
	t.Run("Non-existent Path", func(t *testing.T) {
		nonExistentPath := filepath.Join(tmpDir, "non-existent")
		_, err := loadFromFile[*scylladb.ScyllaDBConfig](nonExistentPath, ScyllaConfigLoader)
		assert.Error(t, err)
	})

	// Test with alternative config filenames
	t.Run("Alternative Config Filename", func(t *testing.T) {
		// Create a new directory
		altDir := filepath.Join(tmpDir, "alt-dir")
		err := os.Mkdir(altDir, 0755)
		require.NoError(t, err)

		// Write config to alternative filename
		altConfig := `
backend:
  type: "scylladb"

store:
  host: "alt-host"
`
		altConfigPath := filepath.Join(altDir, "quorum-quest.yml")
		err = os.WriteFile(altConfigPath, []byte(altConfig), 0644)
		require.NoError(t, err)

		// Load from directory path
		config, err := loadFromFile[*scylladb.ScyllaDBConfig](altDir, ScyllaConfigLoader)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Verify loaded config
		assert.Equal(t, "alt-host", config.Store.Host)
	})
}

func TestValidateConfig(t *testing.T) {
	// Test with nil config
	t.Run("Nil Config", func(t *testing.T) {
		err := validateConfig(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "configuration cannot be nil")
	})

	// Test with non-struct value
	t.Run("Non-Struct Config", func(t *testing.T) {
		err := validateConfig("string value")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "configuration must be a struct")
	})

	// Test with nil pointer
	t.Run("Nil Pointer", func(t *testing.T) {
		var config *GlobalConfig[*scylladb.ScyllaDBConfig]
		err := validateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "configuration pointer cannot be nil")
	})

	// Test with missing Store field
	t.Run("Missing Store Field", func(t *testing.T) {
		type InvalidConfig struct {
			Field string
		}
		config := InvalidConfig{Field: "value"}
		err := validateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "configuration must have a Store field")
	})

	// Test with nil Store field
	t.Run("Nil Store Field", func(t *testing.T) {
		config := &GlobalConfig[*scylladb.ScyllaDBConfig]{
			ServerAddress: "localhost:5050",
			Store:         nil,
		}
		err := validateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Store configuration cannot be nil")
	})

	// Test with valid config
	t.Run("Valid Config", func(t *testing.T) {
		config := &GlobalConfig[*scylladb.ScyllaDBConfig]{
			ServerAddress: "localhost:5050",
			Store: &scylladb.ScyllaDBConfig{
				Host:        "localhost",
				Port:        9042,
				Keyspace:    "test",
				Table:       "test",
				TTL:         15,
				Consistency: "CONSISTENCY_QUORUM",
				Endpoints:   []string{"localhost:9042"},
			},
		}
		err := validateConfig(config)
		assert.NoError(t, err)
	})
}
