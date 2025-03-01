// internal/store/dynamodb/newstore_test.go
package dynamodb

import (
	"context"
	"testing"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestNewStore(t *testing.T) {
	logger, _, _ := observability.NewTestLogger()

	t.Run("nil_config", func(t *testing.T) {
		store, err := NewStore(context.Background(), nil, logger)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "config cannot be nil")
	})

	t.Run("nil_logger", func(t *testing.T) {
		config := &DynamoDBConfig{
			Region:    "us-east-1",
			Table:     "test_table",
			TTL:       15,
			Endpoints: []string{"http://localhost:8000"},
		}
		store, err := NewStore(context.Background(), config, nil)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "logger cannot be nil")
	})

	t.Run("validates_config", func(t *testing.T) {
		config := &DynamoDBConfig{
			Region: "us-east-1",
			// Missing required fields
		}

		store, err := NewStore(context.Background(), config, logger)
		assert.Error(t, err)
		assert.Nil(t, store)
	})

	t.Run("config_validation_missing_region", func(t *testing.T) {
		config := &DynamoDBConfig{
			Table:     "test_table",
			TTL:       15,
			Endpoints: []string{"http://localhost:8000"},
		}

		store, err := NewStore(context.Background(), config, logger)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "region is required")
	})

	t.Run("config_validation_missing_table", func(t *testing.T) {
		config := &DynamoDBConfig{
			Region:    "us-east-1",
			TTL:       15,
			Endpoints: []string{"http://localhost:8000"},
		}

		store, err := NewStore(context.Background(), config, logger)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "table is required")
	})

	t.Run("config_validation_invalid_ttl", func(t *testing.T) {
		config := &DynamoDBConfig{
			Region:    "us-east-1",
			Table:     "test_table",
			TTL:       0, // Invalid TTL
			Endpoints: []string{"http://localhost:8000"},
		}

		store, err := NewStore(context.Background(), config, logger)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "invalid TTL")
	})

	t.Run("config_validation_no_endpoints", func(t *testing.T) {
		config := &DynamoDBConfig{
			Region:    "us-east-1",
			Table:     "test_table",
			TTL:       15,
			Endpoints: []string{}, // Empty endpoints
		}

		store, err := NewStore(context.Background(), config, logger)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "at least one endpoint is required")
	})

	t.Run("config_validation_inconsistent_credentials", func(t *testing.T) {
		config := &DynamoDBConfig{
			Region:          "us-east-1",
			Table:           "test_table",
			TTL:             15,
			Endpoints:       []string{"http://localhost:8000"},
			AccessKeyID:     "access_key", // Only providing AccessKeyID without SecretAccessKey
			SecretAccessKey: "",
		}

		store, err := NewStore(context.Background(), config, logger)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "both access key and secret key must be provided together")
	})
}

func TestStoreInterfaceCompliance(t *testing.T) {
	t.Run("implements_store_interface", func(t *testing.T) {
		// Create a store instance
		mockStore, _, _ := SetupMockStore()

		// Check if it implements the store.Store interface
		var i interface{} = mockStore
		_, ok := i.(store.Store)
		assert.True(t, ok, "MockStore should implement store.Store interface")
	})
}

func TestStoreConfig(t *testing.T) {
	t.Run("returns_config", func(t *testing.T) {
		// Create a store
		mockStore, _, _ := SetupMockStore()

		// Call method under test
		result := mockStore.GetConfig()

		// Verify the result is the same config
		assert.Equal(t, mockStore.config, result)
	})
}

func TestCloseStore(t *testing.T) {
	t.Run("close_succeeds", func(t *testing.T) {
		// Create a store
		mockStore, _, _ := SetupMockStore()

		// Call method under test - should not panic
		mockStore.Close()
	})
}
