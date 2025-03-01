// internal/store/dynamodb/integration_test.go
package dynamodb

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration contains integration tests that use a real DynamoDB instance
// These tests are skipped by default and can be run by setting the environment variable
// DYNAMODB_INTEGRATION_TEST=1
func TestIntegration(t *testing.T) {
	if os.Getenv("DYNAMODB_INTEGRATION_TEST") != "1" {
		t.Skip("Skipping integration tests. Set DYNAMODB_INTEGRATION_TEST=1 to run")
	}

	// Use a local DynamoDB instance by default
	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8000"
	}

	// Create a logger
	logger, _, err := observability.NewTestLogger()
	require.NoError(t, err)

	// Create a test table name with timestamp to avoid conflicts
	tableName := "test_locks_" + time.Now().Format("20060102150405")

	// Create a config
	config := &DynamoDBConfig{
		Region:          "us-east-1",
		Table:           tableName,
		TTL:             5,
		Endpoints:       []string{endpoint},
		AccessKeyID:     "test", // Local DynamoDB typically accepts any credentials
		SecretAccessKey: "test",
	}

	// Create the store
	store, err := NewStore(context.Background(), config, logger)
	require.NoError(t, err)
	defer store.Close()

	t.Run("acquire_and_release_lock", func(t *testing.T) {
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"

		// Acquire the lock
		acquired := store.TryAcquireLock(ctx, service, domain, clientID, 10)
		assert.True(t, acquired)

		// Try to acquire the same lock with different client ID (should fail)
		acquired = store.TryAcquireLock(ctx, service, domain, "client-2", 10)
		assert.False(t, acquired)

		// Try to acquire the same lock with the same client ID (should succeed)
		acquired = store.TryAcquireLock(ctx, service, domain, clientID, 10)
		assert.True(t, acquired)

		// Release the lock
		store.ReleaseLock(ctx, service, domain, clientID)

		// Now another client should be able to acquire the lock
		acquired = store.TryAcquireLock(ctx, service, domain, "client-2", 10)
		assert.True(t, acquired)
	})

	t.Run("keep_alive", func(t *testing.T) {
		ctx := context.Background()
		service := "test-service-2"
		domain := "test-domain-2"
		clientID := "client-1"
		ttl := int32(10)

		// Acquire the lock
		acquired := store.TryAcquireLock(ctx, service, domain, clientID, ttl)
		assert.True(t, acquired)

		// Keep alive the lock
		duration := store.KeepAlive(ctx, service, domain, clientID, ttl)
		assert.Equal(t, time.Duration(ttl)*time.Second, duration)

		// Try to keep alive with different client ID (should fail)
		duration = store.KeepAlive(ctx, service, domain, "client-2", ttl)
		assert.Equal(t, time.Duration(-1)*time.Second, duration)
	})

	t.Run("lock_expiration", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping lock expiration test in short mode")
		}

		ctx := context.Background()
		service := "test-service-3"
		domain := "test-domain-3"
		clientID := "client-1"
		shortTTL := int32(2) // 2 seconds

		// Acquire the lock with short TTL
		acquired := store.TryAcquireLock(ctx, service, domain, clientID, shortTTL)
		assert.True(t, acquired)

		// Wait for the lock to expire
		time.Sleep(time.Duration(shortTTL+1) * time.Second)

		// Now another client should be able to acquire the lock
		acquired = store.TryAcquireLock(ctx, service, domain, "client-2", 10)
		assert.True(t, acquired)
	})
}
