package dynamodb

import (
	"context"
	"testing"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	logger, _, _ := observability.NewTestLogger()

	t.Run("success", func(t *testing.T) {
		config := &DynamoDBConfig{
			Region:          "local",
			Table:           "test_table",
			TTL:             15,
			Endpoints:       []string{"http://localhost:8000"},
			AccessKeyID:     "dummy",
			SecretAccessKey: "dummy",
		}

		store, err := New(context.Background(), config, logger)
		require.NoError(t, err)
		require.NotNil(t, store)

		// Check config is stored correctly
		assert.Equal(t, config, store.config)

		// Clean up
		store.Close()
	})

	t.Run("nil_config", func(t *testing.T) {
		store, err := New(context.Background(), nil, logger)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "config cannot be nil")
	})

	t.Run("invalid_endpoint", func(t *testing.T) {
		config := &DynamoDBConfig{
			Region:          "local",
			Table:           "test_table",
			TTL:             15,
			Endpoints:       []string{"http://invalid:8000"},
			AccessKeyID:     "dummy",
			SecretAccessKey: "dummy",
		}

		store, err := New(context.Background(), config, logger)
		assert.Error(t, err)
		assert.Nil(t, store)
	})

	t.Run("missing_table_name", func(t *testing.T) {
		config := &DynamoDBConfig{
			Region:          "local",
			TTL:             15,
			Endpoints:       []string{"http://localhost:8000"},
			AccessKeyID:     "dummy",
			SecretAccessKey: "dummy",
		}

		store, err := New(context.Background(), config, logger)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "table is required")
	})
}

func TestTryAcquireLock(t *testing.T) {
	logger, _, _ := observability.NewTestLogger()

	config := &DynamoDBConfig{
		Region:          "local",
		Table:           "test_table",
		TTL:             15,
		Endpoints:       []string{"http://localhost:8000"},
		AccessKeyID:     "dummy",
		SecretAccessKey: "dummy",
	}

	store, err := New(context.Background(), config, logger)
	require.NoError(t, err)
	require.NotNil(t, store)
	defer store.Close()

	t.Run("acquire_lock_success", func(t *testing.T) {
		success := store.TryAcquireLock(context.Background(), "test-service", "test-domain", "client-1", 15)
		assert.True(t, success)

		// Clean up
		store.ReleaseLock(context.Background(), "test-service", "test-domain", "client-1")
	})

	t.Run("acquire_lock_conflict", func(t *testing.T) {
		// First acquisition should succeed
		success1 := store.TryAcquireLock(context.Background(), "test-service", "test-domain", "client-1", 15)
		assert.True(t, success1)

		// Second acquisition should fail
		success2 := store.TryAcquireLock(context.Background(), "test-service", "test-domain", "client-2", 15)
		assert.False(t, success2)

		// Clean up
		store.ReleaseLock(context.Background(), "test-service", "test-domain", "client-1")
	})
}

func TestReleaseLock(t *testing.T) {
	logger, _, _ := observability.NewTestLogger()

	config := &DynamoDBConfig{
		Region:          "local",
		Table:           "test_table",
		TTL:             15,
		Endpoints:       []string{"http://localhost:8000"},
		AccessKeyID:     "dummy",
		SecretAccessKey: "dummy",
	}

	store, err := New(context.Background(), config, logger)
	require.NoError(t, err)
	require.NotNil(t, store)
	defer store.Close()

	t.Run("release_existing_lock", func(t *testing.T) {
		// First acquire the lock
		success := store.TryAcquireLock(context.Background(), "test-service", "test-domain", "client-1", 15)
		assert.True(t, success)

		// Release the lock
		store.ReleaseLock(context.Background(), "test-service", "test-domain", "client-1")

		// Try to acquire again - should succeed if release worked
		success = store.TryAcquireLock(context.Background(), "test-service", "test-domain", "client-1", 15)
		assert.True(t, success)

		// Clean up
		store.ReleaseLock(context.Background(), "test-service", "test-domain", "client-1")
	})

	t.Run("release_nonexistent_lock", func(t *testing.T) {
		// Should not panic or error
		store.ReleaseLock(context.Background(), "nonexistent-service", "nonexistent-domain", "client-1")
	})
}

func TestKeepAlive(t *testing.T) {
	logger, _, _ := observability.NewTestLogger()

	config := &DynamoDBConfig{
		Region:          "local",
		Table:           "test_table",
		TTL:             15,
		Endpoints:       []string{"http://localhost:8000"},
		AccessKeyID:     "dummy",
		SecretAccessKey: "dummy",
	}

	store, err := New(context.Background(), config, logger)
	require.NoError(t, err)
	require.NotNil(t, store)
	defer store.Close()

	t.Run("keep_alive_success", func(t *testing.T) {
		// First acquire the lock
		success := store.TryAcquireLock(context.Background(), "test-service", "test-domain", "client-1", 15)
		assert.True(t, success)

		// Keep alive should succeed
		duration := store.KeepAlive(context.Background(), "test-service", "test-domain", "client-1", 15)
		assert.Greater(t, duration.Seconds(), float64(0))

		// Clean up
		store.ReleaseLock(context.Background(), "test-service", "test-domain", "client-1")
	})

	t.Run("keep_alive_nonexistent_lock", func(t *testing.T) {
		duration := store.KeepAlive(context.Background(), "nonexistent-service", "nonexistent-domain", "client-1", 15)
		assert.Equal(t, float64(-1), duration.Seconds())
	})
}
