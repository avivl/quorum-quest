// internal/store/dynamodb/redis_compatibility_test.go
package dynamodb

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRedisCompatibility tests DynamoDB implementation for compatibility with Redis interface
// These tests verify that the DynamoDB store behaves similarly to Redis for common operations
func TestRedisCompatibility(t *testing.T) {
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
	tableName := fmt.Sprintf("compat_locks_%d", time.Now().Unix())

	// Create a config
	config := &DynamoDBConfig{
		Region:          "us-east-1",
		Table:           tableName,
		TTL:             15,
		Endpoints:       []string{endpoint},
		AccessKeyID:     "test", // Local DynamoDB typically accepts any credentials
		SecretAccessKey: "test",
	}

	// Create the store
	dynamoStore, err := NewStore(context.Background(), config, logger)
	require.NoError(t, err)
	defer dynamoStore.Close()

	// Test store interface compliance
	t.Run("store_interface_compliance", func(t *testing.T) {
		var storeInterface store.Store = dynamoStore
		assert.NotNil(t, storeInterface)
	})

	// Lock acquisition test
	t.Run("simple_acquire_lock", func(t *testing.T) {
		ctx := context.Background()
		service := "compat-service"
		domain := "compat-domain"
		clientID := "client-1"

		// Acquire lock
		acquired := dynamoStore.TryAcquireLock(ctx, service, domain, clientID, 30)
		assert.True(t, acquired, "Should acquire lock on first attempt")

		// Try to acquire with different client (should fail)
		acquired = dynamoStore.TryAcquireLock(ctx, service, domain, "client-2", 30)
		assert.False(t, acquired, "Should not acquire lock with different client")

		// Release the lock
		dynamoStore.ReleaseLock(ctx, service, domain, clientID)

		// Now another client should be able to acquire
		acquired = dynamoStore.TryAcquireLock(ctx, service, domain, "client-2", 30)
		assert.True(t, acquired, "Should acquire lock after release")
	})

	// Keep alive test
	t.Run("keep_alive", func(t *testing.T) {
		ctx := context.Background()
		service := "compat-service-2"
		domain := "compat-domain-2"
		clientID := "client-1"
		ttl := int32(30)

		// Acquire the lock
		acquired := dynamoStore.TryAcquireLock(ctx, service, domain, clientID, ttl)
		assert.True(t, acquired, "Should acquire lock")

		// Keep alive with same client
		duration := dynamoStore.KeepAlive(ctx, service, domain, clientID, ttl)
		assert.Equal(t, time.Duration(ttl)*time.Second, duration, "Keep alive should return TTL duration")

		// Try to keep alive with different client
		duration = dynamoStore.KeepAlive(ctx, service, domain, "client-2", ttl)
		assert.Equal(t, time.Duration(-1)*time.Second, duration, "Keep alive should fail with different client")
	})

	// Self renewal test
	t.Run("self_renewal", func(t *testing.T) {
		ctx := context.Background()
		service := "compat-service-3"
		domain := "compat-domain-3"
		clientID := "client-1"
		ttl := int32(30)

		// Acquire the lock
		acquired := dynamoStore.TryAcquireLock(ctx, service, domain, clientID, ttl)
		assert.True(t, acquired, "Should acquire lock")

		// Try to acquire again with same client (should succeed - renew lock)
		acquired = dynamoStore.TryAcquireLock(ctx, service, domain, clientID, ttl)
		assert.True(t, acquired, "Same client should be able to renew lock")
	})

	// Lock isolation test
	t.Run("lock_isolation", func(t *testing.T) {
		ctx := context.Background()
		service1 := "compat-service-4a"
		service2 := "compat-service-4b"
		domain := "compat-domain-4"
		clientID := "client-1"

		// Acquire locks for different services
		acquired1 := dynamoStore.TryAcquireLock(ctx, service1, domain, clientID, 30)
		acquired2 := dynamoStore.TryAcquireLock(ctx, service2, domain, clientID, 30)

		assert.True(t, acquired1, "Should acquire lock for service 1")
		assert.True(t, acquired2, "Should acquire lock for service 2")

		// Different client should not be able to acquire first service's lock
		acquired3 := dynamoStore.TryAcquireLock(ctx, service1, domain, "client-2", 30)
		assert.False(t, acquired3, "Different client should not acquire lock for service 1")

		// But should be able to if we release it
		dynamoStore.ReleaseLock(ctx, service1, domain, clientID)
		acquired4 := dynamoStore.TryAcquireLock(ctx, service1, domain, "client-2", 30)
		assert.True(t, acquired4, "After release, different client should acquire lock for service 1")

		// Original client should still hold lock for service 2
		duration := dynamoStore.KeepAlive(ctx, service2, domain, clientID, 30)
		assert.Equal(t, time.Duration(30)*time.Second, duration, "Original client should still hold lock for service 2")
	})
}
