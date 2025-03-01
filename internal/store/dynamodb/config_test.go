// internal/store/dynamodb/config_test.go
package dynamodb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDynamoDBConfig(t *testing.T) {
	t.Run("new_config_with_defaults", func(t *testing.T) {
		config := NewDynamoDBConfig()

		// Verify default values
		assert.Equal(t, "us-west-2", config.Region)
		assert.Equal(t, "quorum-quest", config.Table)
		assert.Equal(t, int32(15), config.TTL)
		assert.Equal(t, []string{"dynamodb.us-west-2.amazonaws.com"}, config.Endpoints)
		assert.Empty(t, config.AccessKeyID)
		assert.Empty(t, config.SecretAccessKey)
		assert.Empty(t, config.Profile)
	})

	t.Run("validate_valid_config", func(t *testing.T) {
		config := &DynamoDBConfig{
			Region:    "us-east-1",
			Table:     "test-table",
			TTL:       30,
			Endpoints: []string{"http://localhost:8000"},
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("validate_missing_region", func(t *testing.T) {
		config := &DynamoDBConfig{
			Table:     "test-table",
			TTL:       30,
			Endpoints: []string{"http://localhost:8000"},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "region is required")
	})

	t.Run("validate_missing_table", func(t *testing.T) {
		config := &DynamoDBConfig{
			Region:    "us-east-1",
			TTL:       30,
			Endpoints: []string{"http://localhost:8000"},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "table is required")
	})

	t.Run("validate_invalid_ttl", func(t *testing.T) {
		config := &DynamoDBConfig{
			Region:    "us-east-1",
			Table:     "test-table",
			TTL:       0,
			Endpoints: []string{"http://localhost:8000"},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid TTL")

		config.TTL = -1
		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid TTL")
	})

	t.Run("validate_missing_endpoints", func(t *testing.T) {
		config := &DynamoDBConfig{
			Region: "us-east-1",
			Table:  "test-table",
			TTL:    30,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one endpoint is required")
	})

	t.Run("validate_inconsistent_credentials", func(t *testing.T) {
		// Only access key provided
		config := &DynamoDBConfig{
			Region:      "us-east-1",
			Table:       "test-table",
			TTL:         30,
			Endpoints:   []string{"http://localhost:8000"},
			AccessKeyID: "test-key",
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "both access key and secret key must be provided together")

		// Only secret key provided
		config = &DynamoDBConfig{
			Region:          "us-east-1",
			Table:           "test-table",
			TTL:             30,
			Endpoints:       []string{"http://localhost:8000"},
			SecretAccessKey: "test-secret",
		}

		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "both access key and secret key must be provided together")
	})

	t.Run("get_table_name", func(t *testing.T) {
		config := &DynamoDBConfig{
			Table: "test-table",
		}

		assert.Equal(t, "test-table", config.GetTableName())
	})

	t.Run("get_ttl", func(t *testing.T) {
		config := &DynamoDBConfig{
			TTL: 42,
		}

		assert.Equal(t, int32(42), config.GetTTL())
	})

	t.Run("get_endpoints", func(t *testing.T) {
		endpoints := []string{"endpoint1", "endpoint2"}
		config := &DynamoDBConfig{
			Endpoints: endpoints,
		}

		assert.Equal(t, endpoints, config.GetEndpoints())
	})
}
