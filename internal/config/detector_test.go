// internal/config/detector_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectBackendType(t *testing.T) {
	t.Run("Detect DynamoDB Backend", func(t *testing.T) {
		// Create temporary directory
		tmpDir := t.TempDir()

		// Create config file with DynamoDB backend
		dynamoConfig := `
backend:
  type: "dynamodb"

dynamoDbConfig:
  region: "us-west-2"
  table: "quorum-quest"
  ttl: 15
  endpoints:
    - "dynamodb.us-west-2.amazonaws.com"

observability:
  serviceName: "quorum-quest"
  serviceVersion: "1.0.0"
`
		configPath := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configPath, []byte(dynamoConfig), 0644)
		require.NoError(t, err)

		// Detect backend type
		backendType, err := DetectBackendType(configPath)
		require.NoError(t, err)
		assert.Equal(t, "dynamodb", backendType)
	})

	t.Run("Detect ScyllaDB Backend", func(t *testing.T) {
		// Create temporary directory
		tmpDir := t.TempDir()

		// Create config file with ScyllaDB backend
		scyllaConfig := `
backend:
  type: "scylladb"

scyllaDbConfig:
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
  serviceVersion: "1.0.0"
`
		configPath := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configPath, []byte(scyllaConfig), 0644)
		require.NoError(t, err)

		// Detect backend type
		backendType, err := DetectBackendType(configPath)
		require.NoError(t, err)
		assert.Equal(t, "scylladb", backendType)
	})

	t.Run("Case Insensitive Backend Type", func(t *testing.T) {
		// Create temporary directory
		tmpDir := t.TempDir()

		// Create config file with mixed case backend type
		mixedCaseConfig := `
backend:
  type: "DynamoDB"

dynamoDbConfig:
  region: "us-west-2"
  table: "quorum-quest"
`
		configPath := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configPath, []byte(mixedCaseConfig), 0644)
		require.NoError(t, err)

		// Detect backend type (should be normalized to lowercase)
		backendType, err := DetectBackendType(configPath)
		require.NoError(t, err)
		assert.Equal(t, "dynamodb", backendType)
	})

	t.Run("Missing Backend Type", func(t *testing.T) {
		// Create temporary directory
		tmpDir := t.TempDir()

		// Create config file without backend type
		invalidConfig := `
observability:
  serviceName: "quorum-quest"
  serviceVersion: "1.0.0"

dynamoDbConfig:
  region: "us-west-2"
  table: "quorum-quest"
`
		configPath := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configPath, []byte(invalidConfig), 0644)
		require.NoError(t, err)

		// Attempt to detect backend type
		_, err = DetectBackendType(configPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "backend type not specified")
	})

	t.Run("Detect From Directory Path", func(t *testing.T) {
		// Create temporary directory
		tmpDir := t.TempDir()

		// Create config file in directory
		validConfig := `
backend:
  type: "scylladb"

scyllaDbConfig:
  host: "127.0.0.1"
`
		configPath := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configPath, []byte(validConfig), 0644)
		require.NoError(t, err)

		// Detect backend type from directory path
		backendType, err := DetectBackendType(tmpDir)
		require.NoError(t, err)
		assert.Equal(t, "scylladb", backendType)
	})

	t.Run("Non-existent Config Path", func(t *testing.T) {
		// Create a path that doesn't exist
		nonExistentPath := filepath.Join(t.TempDir(), "non-existent-config.yaml")

		// Attempt to detect backend type
		_, err := DetectBackendType(nonExistentPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Invalid YAML", func(t *testing.T) {
		// Create temporary directory
		tmpDir := t.TempDir()

		// Create invalid YAML file
		invalidYAML := `
backend:
  type: "dynamodb"
invalid:yaml:content
`
		configPath := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
		require.NoError(t, err)

		// Attempt to detect backend type
		_, err = DetectBackendType(configPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid configuration file")
	})

	t.Run("Alternative Config Filenames", func(t *testing.T) {
		// Create temporary directory
		tmpDir := t.TempDir()

		// Create config file with alternative name
		validConfig := `
backend:
  type: "dynamodb"
`
		// Test different alternative filenames
		alternativeNames := []string{
			"config.yml",
			"quorum-quest.yaml",
			"quorum-quest.yml",
		}

		for _, filename := range alternativeNames {
			configPath := filepath.Join(tmpDir, filename)
			err := os.WriteFile(configPath, []byte(validConfig), 0644)
			require.NoError(t, err)

			// Detect backend type from directory with alternative filename
			backendType, err := DetectBackendType(tmpDir)
			require.NoError(t, err)
			assert.Equal(t, "dynamodb", backendType)

			// Clean up for next iteration
			os.Remove(configPath)
		}
	})

	t.Run("Unsupported Backend Type", func(t *testing.T) {
		// Create temporary directory
		tmpDir := t.TempDir()

		// Create config file with unsupported backend
		unsupportedConfig := `
backend:
  type: "redis"
`
		configPath := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configPath, []byte(unsupportedConfig), 0644)
		require.NoError(t, err)

		// Detect backend type - this should work as we're just detecting, not validating
		backendType, err := DetectBackendType(configPath)
		require.NoError(t, err)
		assert.Equal(t, "redis", backendType)
		// The validation of supported backends happens in the NewApp function
	})

	t.Run("Environment Variable Override", func(t *testing.T) {
		// Create temporary directory
		tmpDir := t.TempDir()

		// Create config file with DynamoDB backend
		dynamoConfig := `
backend:
  type: "dynamodb"
`
		configPath := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configPath, []byte(dynamoConfig), 0644)
		require.NoError(t, err)

		// Set environment variable to override
		t.Setenv("QUORUMQUEST_BACKEND_TYPE", "scylladb")

		// Detect backend type
		backendType, err := DetectBackendType(configPath)
		require.NoError(t, err)
		assert.Equal(t, "scylladb", backendType, "Environment variable should override config file")

		// Clear environment variable
		t.Setenv("QUORUMQUEST_BACKEND_TYPE", "")
	})
}
