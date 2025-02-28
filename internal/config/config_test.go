// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/avivl/quorum-quest/internal/store/dynamodb"
	"github.com/avivl/quorum-quest/internal/store/scylladb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain sets up the test environment
func TestMain(m *testing.M) {
	// Set environment variable to indicate we're in test mode
	os.Setenv("GO_TEST_MODE", "1")

	// Run tests
	result := m.Run()

	// Exit with the test result
	os.Exit(result)
}

func TestEnvironmentOverrides(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create base config file
	baseConfig := `
backend:
  type: "scylladb"

store:
  host: "127.0.0.1"
  port: 9042
  keyspace: "quorum-quest"
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
		"QUORUMQUEST_STORE_HOST":     "env-host",
		"QUORUMQUEST_STORE_PORT":     "9043",
		"QUORUMQUEST_STORE_KEYSPACE": "env-keyspace",
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
backend:
  type: "scylladb"

store:
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
	t.Setenv("QUORUMQUEST_STORE_HOST", "env-host")

	// Load configuration
	loader, cfg, err := LoadConfig[*scylladb.ScyllaDBConfig](tmpDir, ScyllaConfigLoader)
	require.NoError(t, err)
	require.NotNil(t, loader)
	require.NotNil(t, cfg)

	// Debug output
	t.Logf("Host from env: %s", os.Getenv("QUORUMQUEST_STORE_HOST"))
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
backend:
  type: "scylladb"

store:
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
backend:
  type: "dynamodb"

store:
  region: "us-east-1"
  table: "my-dynamo-table"
  ttl: 25
  endpoints:
    - "dynamodb.us-east-1.amazonaws.com"
  profile: "prod"
  accessKeyId: "test-key"
  secretAccessKey: "test-secret"

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
		assert.Equal(t, "test-key", cfg.Store.AccessKeyID)
		assert.Equal(t, "test-secret", cfg.Store.SecretAccessKey)

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
backend:
  type: "scylladb"

store:
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
		assert.Contains(t, err.Error(), "failed to parse config: store validation failed: host is required")
	})

	t.Run("Load Non-existent File", func(t *testing.T) {
		// Create temporary directory
		tmpDir := t.TempDir()

		// Set up a default backend type environment variable to ensure test works
		t.Setenv("QUORUMQUEST_BACKEND_TYPE", "scylladb")

		// Load configuration from non-existent file
		loader, cfg, err := LoadConfig[*scylladb.ScyllaDBConfig](tmpDir, ScyllaConfigLoader)
		require.NoError(t, err, "Should not error with missing file, should use defaults")
		require.NotNil(t, loader)
		require.NotNil(t, cfg)

		// Verify default values
		assert.Equal(t, "127.0.0.1", cfg.Store.Host)
		assert.Equal(t, int32(9042), cfg.Store.Port)
		assert.Equal(t, "quorum-quest", cfg.Store.Keyspace)
		assert.Equal(t, "services", cfg.Store.Table)
	})
}

func TestDefaultValues(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Set up a default backend type environment variable to ensure test works
	t.Setenv("QUORUMQUEST_BACKEND_TYPE", "scylladb")

	// Load configuration with no file present
	loader, cfg, err := LoadConfig[*scylladb.ScyllaDBConfig](tmpDir, ScyllaConfigLoader)
	require.NoError(t, err, "Should not error with missing file, should use defaults")
	require.NotNil(t, loader)
	require.NotNil(t, cfg)

	// Verify Store defaults
	assert.Equal(t, "127.0.0.1", cfg.Store.Host, "Should use default host")
	assert.Equal(t, int32(9042), cfg.Store.Port, "Should use default port")
	assert.Equal(t, "quorum-quest", cfg.Store.Keyspace, "Should use default keyspace")
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

func TestConfigWatcher(t *testing.T) {
	// Skip this test in automated test environments since it relies on file watching
	if os.Getenv("CI") != "" {
		t.Skip("Skipping file watcher test in CI environment")
	}

	// Make sure we're not in test mode for this specific test
	previousTestMode := os.Getenv("GO_TEST_MODE")
	os.Setenv("GO_TEST_MODE", "")
	defer os.Setenv("GO_TEST_MODE", previousTestMode)

	// Create temporary directory
	tmpDir := t.TempDir()

	// Create initial config file
	initialConfig := `
backend:
  type: "scylladb"

store:
  host: "initial-host"
  port: 9042
  keyspace: "initial-keyspace"
  table: "initial-table"
  ttl: 15
  consistency: "CONSISTENCY_QUORUM"
  endpoints:
    - "initial-host:9042"

serverAddress: "localhost:5050"
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(initialConfig), 0644)
	require.NoError(t, err)

	// Load configuration
	loader, cfg, err := LoadConfig[*scylladb.ScyllaDBConfig](configPath, ScyllaConfigLoader)
	require.NoError(t, err)
	require.NotNil(t, loader)
	require.NotNil(t, cfg)

	// Verify initial config values
	assert.Equal(t, "initial-host", cfg.Store.Host)
	assert.Equal(t, "initial-keyspace", cfg.Store.Keyspace)

	// Set up watcher notification channel
	notifyCh := make(chan interface{}, 1)
	loader.AddWatcher(func(newConfig interface{}) {
		notifyCh <- newConfig
	})

	// Update config file
	updatedConfig := `
backend:
  type: "scylladb"

store:
  host: "updated-host"
  port: 9042
  keyspace: "updated-keyspace"
  table: "updated-table"
  ttl: 30
  consistency: "CONSISTENCY_QUORUM"
  endpoints:
    - "updated-host:9042"

serverAddress: "localhost:6060"
`
	// Write updated config and allow time for file watcher to detect change
	time.Sleep(100 * time.Millisecond)
	err = os.WriteFile(configPath, []byte(updatedConfig), 0644)
	require.NoError(t, err)

	// Wait for notification (with timeout)
	select {
	case newConfig := <-notifyCh:
		updatedCfg, ok := newConfig.(*GlobalConfig[*scylladb.ScyllaDBConfig])
		require.True(t, ok, "Expected GlobalConfig type")
		assert.Equal(t, "updated-host", updatedCfg.Store.Host)
		assert.Equal(t, "updated-keyspace", updatedCfg.Store.Keyspace)
		assert.Equal(t, int32(30), updatedCfg.Store.TTL)
		assert.Equal(t, "localhost:6060", updatedCfg.ServerAddress)
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for config update notification")
	}

	// Clean up
	loader.Close()
}

func TestBackendTypeOverride(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create config file with DynamoDB backend
	config := `
backend:
  type: "dynamodb"

store:
  region: "us-west-2"
  table: "quorum-quest"
  ttl: 15
  endpoints:
    - "dynamodb.us-west-2.amazonaws.com"
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(config), 0644)
	require.NoError(t, err)

	// Override backend type with environment variable
	t.Setenv("QUORUMQUEST_BACKEND_TYPE", "scylladb")

	// Detect backend type
	backendType, err := DetectBackendType(configPath)
	require.NoError(t, err)

	// Environment variable should take precedence
	assert.Equal(t, "scylladb", backendType)
}
