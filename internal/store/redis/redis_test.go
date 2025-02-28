// internal/store/redis/redis_test.go
package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) (*RedisStore, *miniredis.Miniredis, func()) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	cfg := &RedisConfig{
		Host: mr.Host(),
		Port: mr.Server().Addr().Port,
		TTL:  1,
	}

	store, err := NewRedisStore(cfg)
	require.NoError(t, err)

	cleanup := func() {
		store.Close()
		mr.Close()
	}

	return store, mr, cleanup
}

func TestRedisStore_Operations(t *testing.T) {
	redisStore, _, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	// Test Put
	record := &store.ServiceRecord{
		ID:        "test-service",
		Name:      "Test Service",
		Status:    "healthy",
		Timestamp: time.Now().Unix(),
	}

	err := redisStore.Put(ctx, record)
	require.NoError(t, err)

	// Test Get
	retrieved, err := redisStore.Get(ctx, record.ID)
	require.NoError(t, err)
	assert.Equal(t, record.ID, retrieved.ID)
	assert.Equal(t, record.Name, retrieved.Name)
	assert.Equal(t, record.Status, retrieved.Status)
	assert.Equal(t, record.Timestamp, retrieved.Timestamp)

	// Test Get non-existent
	_, err = redisStore.Get(ctx, "non-existent")
	assert.ErrorIs(t, err, store.ErrNotFound)

	// Test Delete
	err = redisStore.Delete(ctx, record.ID)
	require.NoError(t, err)

	// Verify deletion
	_, err = redisStore.Get(ctx, record.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)

	// Test Delete non-existent
	err = redisStore.Delete(ctx, "non-existent")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestRedisStore_TTL(t *testing.T) {
	redisStore, mr, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	record := &store.ServiceRecord{
		ID:        "test-service",
		Name:      "Test Service",
		Status:    "healthy",
		Timestamp: time.Now().Unix(),
	}

	err := redisStore.Put(ctx, record)
	require.NoError(t, err)

	// Fast forward time past TTL
	mr.FastForward(2 * time.Second)

	// Record should be expired
	_, err = redisStore.Get(ctx, record.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestRedisStore_InvalidOperations(t *testing.T) {
	redisStore, _, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	// Test Put with nil record
	err := redisStore.Put(ctx, nil)
	assert.Error(t, err)

	// Test Put with invalid record (will fail JSON marshaling)
	invalidRecord := &store.ServiceRecord{
		ID: string([]byte{0xff, 0xfe, 0xfd}), // Invalid UTF-8
	}
	err = redisStore.Put(ctx, invalidRecord)
	assert.Error(t, err)
}

func TestRedisStore_LockOperations(t *testing.T) {
	redisStore, _, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	service := "test-service"
	domain := "test-domain"
	clientID := "client-1"

	// Test TryAcquireLock
	success := redisStore.TryAcquireLock(ctx, service, domain, clientID, 0)
	assert.True(t, success, "First lock acquisition should succeed")

	// Test acquiring the same lock (should fail)
	success = redisStore.TryAcquireLock(ctx, service, domain, "client-2", 0)
	assert.False(t, success, "Second lock acquisition should fail")

	// Test ReleaseLock
	redisStore.ReleaseLock(ctx, service, domain, clientID)

	// Lock should be released, so we can acquire it again
	success = redisStore.TryAcquireLock(ctx, service, domain, "client-2", 0)
	assert.True(t, success, "Lock acquisition after release should succeed")

	// Test ReleaseLock with wrong client ID (should not release the lock)
	redisStore.ReleaseLock(ctx, service, domain, "wrong-client")
	success = redisStore.TryAcquireLock(ctx, service, domain, "client-3", 0)
	assert.False(t, success, "Lock should not be released with wrong client ID")

	// Test correct release
	redisStore.ReleaseLock(ctx, service, domain, "client-2")
	success = redisStore.TryAcquireLock(ctx, service, domain, "client-3", 0)
	assert.True(t, success, "Lock should be released with correct client ID")
}

func TestRedisStore_KeepAlive(t *testing.T) {
	redisStore, miniRedis, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	service := "test-service"
	domain := "test-domain"
	clientID := "client-1"

	// Acquire lock
	success := redisStore.TryAcquireLock(ctx, service, domain, clientID, 0)
	assert.True(t, success, "Lock acquisition should succeed")

	// Keep alive with correct client ID
	duration := redisStore.KeepAlive(ctx, service, domain, clientID, 0)
	assert.Equal(t, time.Duration(redisStore.ttl), duration, "KeepAlive should return correct duration")

	// Fast forward almost to expiration
	miniRedis.FastForward(900 * time.Millisecond)

	// Keep alive again
	duration = redisStore.KeepAlive(ctx, service, domain, clientID, 0)
	assert.Equal(t, time.Duration(redisStore.ttl), duration, "KeepAlive should extend TTL")

	// Keep alive with wrong client ID
	duration = redisStore.KeepAlive(ctx, service, domain, "wrong-client", 0)
	assert.Equal(t, time.Duration(-1)*time.Second, duration, "KeepAlive should fail with wrong client ID")

	// Fast forward past TTL
	miniRedis.FastForward(2 * time.Second)

	// Keep alive after expiration
	duration = redisStore.KeepAlive(ctx, service, domain, clientID, 0)
	assert.Equal(t, time.Duration(-1)*time.Second, duration, "KeepAlive should fail after expiration")
}

func TestRedisStore_GetConfig(t *testing.T) {
	redisStore, _, cleanup := setupTestRedis(t)
	defer cleanup()

	config := redisStore.GetConfig()
	assert.NotNil(t, config, "GetConfig should return config")

	// Verify it's the correct type
	redisConfig, ok := config.(*RedisConfig)
	assert.True(t, ok, "Config should be of type *RedisConfig")
	assert.Equal(t, redisStore.config.Host, redisConfig.Host)
	assert.Equal(t, redisStore.config.Port, redisConfig.Port)
	assert.Equal(t, redisStore.config.TTL, redisConfig.TTL)
}
