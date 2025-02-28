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
