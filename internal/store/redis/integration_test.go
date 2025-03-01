// internal/store/redis/integration_test.go
package redis

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration contains integration tests that use a real Redis instance
// These tests are skipped by default and can be run by setting the environment variable
// REDIS_INTEGRATION_TEST=1
func TestIntegration(t *testing.T) {
	if os.Getenv("REDIS_INTEGRATION_TEST") != "1" {
		t.Skip("Skipping integration tests. Set REDIS_INTEGRATION_TEST=1 to run")
	}

	// Use a local Redis instance by default
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}

	port := 6379 // Default Redis port

	// Create a logger
	logger, _, err := observability.NewTestLogger()
	require.NoError(t, err)

	// Use a unique key prefix for tests to avoid conflicts
	keyPrefix := "test_lock_" + time.Now().Format("20060102150405")

	// Create a config
	config := &RedisConfig{
		Host:      host,
		Port:      port,
		TTL:       5, // Short TTL for tests
		KeyPrefix: keyPrefix,
	}

	// Create the store
	store, err := New(context.Background(), config, logger)
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

		// Try with different TTL
		newTTL := int32(20)
		duration = store.KeepAlive(ctx, service, domain, clientID, newTTL)
		assert.Equal(t, time.Duration(newTTL)*time.Second, duration)

		// Try with default TTL (0)
		duration = store.KeepAlive(ctx, service, domain, clientID, 0)
		assert.Equal(t, time.Duration(config.TTL)*time.Second, duration)

		// Another client should not be able to keep the lock alive
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

	t.Run("multiple_domains", func(t *testing.T) {
		ctx := context.Background()
		service := "test-service-4"
		clientID := "client-1"

		// Acquire locks in different domains
		acquired1 := store.TryAcquireLock(ctx, service, "domain-1", clientID, 10)
		assert.True(t, acquired1)

		acquired2 := store.TryAcquireLock(ctx, service, "domain-2", clientID, 10)
		assert.True(t, acquired2)

		// Release one of the locks
		store.ReleaseLock(ctx, service, "domain-1", clientID)

		// Try to acquire the released lock with a different client
		acquired3 := store.TryAcquireLock(ctx, service, "domain-1", "client-2", 10)
		assert.True(t, acquired3)

		// The unreleased lock should still be held
		acquired4 := store.TryAcquireLock(ctx, service, "domain-2", "client-2", 10)
		assert.False(t, acquired4)
	})

	t.Run("same_service_different_domains", func(t *testing.T) {
		ctx := context.Background()
		service := "test-service-5"
		clientID := "client-1"

		// Acquire lock in domain1
		acquired1 := store.TryAcquireLock(ctx, service, "domain-a", clientID, 10)
		assert.True(t, acquired1)

		// Different domain should be acquirable independently
		acquired2 := store.TryAcquireLock(ctx, service, "domain-b", clientID, 10)
		assert.True(t, acquired2)

		// Different client should not be able to acquire lock in domain-a
		acquired3 := store.TryAcquireLock(ctx, service, "domain-a", "client-2", 10)
		assert.False(t, acquired3)

		// But should be able to acquire lock in domain-c
		acquired4 := store.TryAcquireLock(ctx, service, "domain-c", "client-2", 10)
		assert.True(t, acquired4)
	})
}
