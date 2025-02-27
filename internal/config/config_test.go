// internal/config/config_test.go
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

func TestEnvironmentOverrides(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create base config file
	baseConfig := `
scyllaDbConfig:
  host: "127.0.0.1"
  port: 9042
  keyspace: "quorum-ques"
  table: "services"
  ttl: 15
  consistency: "CONSISTENCY_QUORUM"
  endpoints:
    - "localhost:9042"

observability:
  serviceName: "quorum-quest"
  serviceVersion: "0.1.0"
  environment: "development"
  otelEndpoint: "localhost:4317"

logger:
  level: "LOG_LEVELS_INFOLEVEL"

serverAddress: "localhost:5050"
`
	err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(baseConfig), 0644)
	require.NoError(t, err)

	// Set environment variables
	envVars := map[string]string{
		"QUORUMQUEST_SCYLLADBCONFIG_HOST":     "env-host",
		"QUORUMQUEST_SCYLLADBCONFIG_PORT":     "9043",
		"QUORUMQUEST_SCYLLADBCONFIG_KEYSPACE": "env-keyspace",
	}

	// Set all environment variables
	for k, v := range envVars {
		t.Setenv(k, v)
	}

	// Load configuration
	loader, cfg, err := LoadConfig[*scylladb.ScyllaDBConfig](tmpDir, ScyllaConfigLoader)
	require.NoError(t, err)
	require.NotNil(t, loader)
	require.NotNil(t, cfg)

	// Debug output
	t.Logf("Environment variables:")
	for k, v := range envVars {
		t.Logf("%s = %s", k, v)
	}
	t.Logf("Loaded config: %+v", cfg.Store)

	// Verify overrides
	assert.Equal(t, "env-host", cfg.Store.Host, "Host should be overridden by environment variable")
	assert.Equal(t, int32(9043), cfg.Store.Port, "Port should be overridden by environment variable")
	assert.Equal(t, "env-keyspace", cfg.Store.Keyspace, "Keyspace should be overridden by environment variable")

	// Verify non-overridden values remain unchanged
	assert.Equal(t, "services", cfg.Store.Table, "Table should retain default value")
	assert.Equal(t, "CONSISTENCY_QUORUM", cfg.Store.Consistency, "Consistency should retain default value")
}

func TestEnvironmentPrecedence(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create config file
	configContent := `
scyllaDbConfig:
  host: "file-host"
  port: 9042
  keyspace: "file-keyspace"
  table: "services"
  ttl: 15
  consistency: "CONSISTENCY_QUORUM"
  endpoints:
    - "file-host:9042"
`
	err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(configContent), 0644)
	require.NoError(t, err)

	// Set environment variables
	t.Setenv("QUORUMQUEST_SCYLLADBCONFIG_HOST", "env-host")

	// Load configuration
	loader, cfg, err := LoadConfig[*scylladb.ScyllaDBConfig](tmpDir, ScyllaConfigLoader)
	require.NoError(t, err)
	require.NotNil(t, loader)
	require.NotNil(t, cfg)

	// Debug output
	t.Logf("Host from env: %s", os.Getenv("QUORUMQUEST_SCYLLADBCONFIG_HOST"))
	t.Logf("Host from config: %s", cfg.Store.Host)

	// Verify environment variables take precedence
	assert.Equal(t, "env-host", cfg.Store.Host, "Environment value should override file value for host")

	// Verify non-overridden values remain as file values
	assert.Equal(t, "file-keyspace", cfg.Store.Keyspace, "File value should be retained when no environment override exists")
	assert.Equal(t, int32(9042), cfg.Store.Port, "File value should be retained when no environment override exists")
}
func TestLoadFromFiles(t *testing.T) {
	t.Run("Load ScyllaDB Config", func(t *testing.T) {
		// Create temporary directory
		tmpDir := t.TempDir()

		// Create ScyllaDB config file
		scyllaConfig := `
scyllaDbConfig:
  host: "scylla-host"
  port: 9042
  keyspace: "my-keyspace"
  table: "my-table"
  ttl: 30
  consistency: "CONSISTENCY_QUORUM"
  endpoints:
    - "scylla-host:9042"
    - "scylla-host-2:9042"

observability:
  serviceName: "my-scylla-service"
  serviceVersion: "1.1.0"
  environment: "staging"
  otelEndpoint: "otel:4317"

logger:
  level: "LOG_LEVELS_DEBUGLEVEL"

serverAddress: "0.0.0.0:8080"
`
		err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(scyllaConfig), 0644)
		require.NoError(t, err)

		// Load configuration
		loader, cfg, err := LoadConfig[*scylladb.ScyllaDBConfig](tmpDir, ScyllaConfigLoader)
		require.NoError(t, err)
		require.NotNil(t, loader)
		require.NotNil(t, cfg)

		// Verify ScyllaDB config
		assert.Equal(t, "scylla-host", cfg.Store.Host)
		assert.Equal(t, int32(9042), cfg.Store.Port)
		assert.Equal(t, "my-keyspace", cfg.Store.Keyspace)
		assert.Equal(t, "my-table", cfg.Store.Table)
		assert.Equal(t, int32(30), cfg.Store.TTL)
		assert.Equal(t, "CONSISTENCY_QUORUM", cfg.Store.Consistency)
		assert.Equal(t, []string{"scylla-host:9042", "scylla-host-2:9042"}, cfg.Store.Endpoints)

		// Verify common config
		assert.Equal(t, "my-scylla-service", cfg.Observability.ServiceName)
		assert.Equal(t, "1.1.0", cfg.Observability.ServiceVersion)
		assert.Equal(t, "staging", cfg.Observability.Environment)
		assert.Equal(t, "otel:4317", cfg.Observability.OTelEndpoint)
		assert.Equal(t, "LOG_LEVELS_DEBUGLEVEL", string(cfg.Logger.Level))
		assert.Equal(t, "0.0.0.0:8080", cfg.ServerAddress)
	})

	t.Run("Load DynamoDB Config", func(t *testing.T) {
		// Create temporary directory
		tmpDir := t.TempDir()

		// Create DynamoDB config file
		dynamoConfig := `
dynamoDbConfig:
  region: "us-east-1"
  table: "my-dynamo-table"
  ttl: 25
  endpoints:
    - "dynamodb.us-east-1.amazonaws.com"
  profile: "prod"
  maxRetries: 5

observability:
  serviceName: "my-dynamo-service"
  serviceVersion: "2.0.0"
  environment: "production"
  otelEndpoint: "collector:4317"

logger:
  level: "LOG_LEVELS_INFOLEVEL"

serverAddress: "0.0.0.0:9090"
`
		err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(dynamoConfig), 0644)
		require.NoError(t, err)

		// Load configuration
		loader, cfg, err := LoadConfig[*dynamodb.DynamoDBConfig](tmpDir, DynamoConfigLoader)
		require.NoError(t, err)
		require.NotNil(t, loader)
		require.NotNil(t, cfg)

		// Verify DynamoDB config
		assert.Equal(t, "us-east-1", cfg.Store.Region)
		assert.Equal(t, "my-dynamo-table", cfg.Store.Table)
		assert.Equal(t, int32(25), cfg.Store.TTL)
		assert.Equal(t, []string{"dynamodb.us-east-1.amazonaws.com"}, cfg.Store.Endpoints)
		assert.Equal(t, "prod", cfg.Store.Profile)

		// Verify common config
		assert.Equal(t, "my-dynamo-service", cfg.Observability.ServiceName)
		assert.Equal(t, "2.0.0", cfg.Observability.ServiceVersion)
		assert.Equal(t, "production", cfg.Observability.Environment)
		assert.Equal(t, "collector:4317", cfg.Observability.OTelEndpoint)
		assert.Equal(t, "LOG_LEVELS_INFOLEVEL", string(cfg.Logger.Level))
		assert.Equal(t, "0.0.0.0:9090", cfg.ServerAddress)
	})

	t.Run("Load Invalid Config File", func(t *testing.T) {
		// Create temporary directory
		tmpDir := t.TempDir()

		// Create invalid config file
		invalidConfig := `
scyllaDbConfig:
  host: "" # Invalid: empty host
  port: -1 # Invalid: negative port
  keyspace: "my-keyspace"
  ttl: 0   # Invalid: zero TTL

observability:
  serviceName: "" # Invalid: empty service name
`
		err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(invalidConfig), 0644)
		require.NoError(t, err)

		// Attempt to load configuration
		_, _, err = LoadConfig[*scylladb.ScyllaDBConfig](tmpDir, ScyllaConfigLoader)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})

	t.Run("Load Non-existent File", func(t *testing.T) {
		// Create temporary directory
		tmpDir := t.TempDir()

		// Load configuration from non-existent file
		loader, cfg, err := LoadConfig[*scylladb.ScyllaDBConfig](tmpDir, ScyllaConfigLoader)
		require.NoError(t, err, "Should not error with missing file, should use defaults")
		require.NotNil(t, loader)
		require.NotNil(t, cfg)

		// Verify default values
		assert.Equal(t, "127.0.0.1", cfg.Store.Host)
		assert.Equal(t, int32(9042), cfg.Store.Port)
		assert.Equal(t, "quorum-ques", cfg.Store.Keyspace)
		assert.Equal(t, "services", cfg.Store.Table)
	})
}
func TestDefaultValues(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Load configuration with no file present
	loader, cfg, err := LoadConfig[*scylladb.ScyllaDBConfig](tmpDir, ScyllaConfigLoader)
	require.NoError(t, err, "Should not error with missing file, should use defaults")
	require.NotNil(t, loader)
	require.NotNil(t, cfg)

	// Verify Store defaults
	assert.Equal(t, "127.0.0.1", cfg.Store.Host, "Should use default host")
	assert.Equal(t, int32(9042), cfg.Store.Port, "Should use default port")
	assert.Equal(t, "quorum-ques", cfg.Store.Keyspace, "Should use default keyspace")
	assert.Equal(t, "services", cfg.Store.Table, "Should use default table")
	assert.Equal(t, int32(15), cfg.Store.TTL, "Should use default TTL")
	assert.Equal(t, "CONSISTENCY_QUORUM", cfg.Store.Consistency, "Should use default consistency")
	assert.Equal(t, []string{"localhost:9042"}, cfg.Store.Endpoints, "Should use default endpoints")

	// Verify Observability defaults
	assert.Equal(t, "quorum-quest", cfg.Observability.ServiceName, "Should use default service name")
	assert.Equal(t, "0.1.0", cfg.Observability.ServiceVersion, "Should use default service version")
	assert.Equal(t, "development", cfg.Observability.Environment, "Should use default environment")
	assert.Equal(t, "localhost:4317", cfg.Observability.OTelEndpoint, "Should use default OTel endpoint")

	// Verify Logger defaults
	assert.Equal(t, "LOG_LEVELS_INFOLEVEL", string(cfg.Logger.Level), "Should use default logger level")

	// Verify Server defaults
	assert.Equal(t, "localhost:5050", cfg.ServerAddress, "Should use default server address")
}

func TestNotifyWatchers(t *testing.T) {
	loader := NewConfigLoader(".")

	var called bool
	var receivedConfig interface{}

	// Add a watcher
	loader.AddWatcher(func(config interface{}) {
		called = true
		receivedConfig = config
	})

	// Create test config
	testConfig := "test-config"

	// Notify watchers
	loader.notifyWatchers(testConfig)

	// Verify watcher was called
	assert.True(t, called, "Watcher should have been called")
	assert.Equal(t, testConfig, receivedConfig, "Watcher should receive correct config")
}

func TestGetCurrentConfig(t *testing.T) {
	loader := NewConfigLoader(".")
	testConfig := "test-config"

	// Set current config
	loader.mu.Lock()
	loader.currentConfig = testConfig
	loader.mu.Unlock()

	// Get current config
	config := loader.GetCurrentConfig()

	// Verify
	assert.Equal(t, testConfig, config, "GetCurrentConfig should return correct config")
}
