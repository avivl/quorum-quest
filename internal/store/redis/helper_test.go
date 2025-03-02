// internal/store/redis/helper_test.go
package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestDefaultTTL tests the TTL fallback logic indirectly through TryAcquireLock
func TestDefaultTTL(t *testing.T) {
	// Save the original function for restoration
	original := newRedisClientFn
	defer func() { newRedisClientFn = original }()

	logger, _, _ := observability.NewTestLogger()
	ctx := context.Background()

	t.Run("use_provided_ttl", func(t *testing.T) {
		// Mock Redis client
		mockClient := new(MockRedisClient)

		// Setup Ping to succeed for New()
		statusCmd := redis.NewStatusCmd(ctx)
		statusCmd.SetVal("PONG")
		mockClient.On("Ping", mock.Anything).Return(statusCmd)

		// Setup SetNX to succeed
		boolCmd := redis.NewBoolCmd(ctx)
		boolCmd.SetVal(true)
		mockClient.On("SetNX", ctx, "custom-prefix:service1:domain1", "client1",
			time.Duration(30)*time.Second).Return(boolCmd)

		// Override with test version
		newRedisClientFn = func(addr string, password string, db int) redisClientMock {
			return mockClient
		}

		// Create a store with custom config
		config := &RedisConfig{
			Host:      "localhost",
			Port:      6379,
			TTL:       15, // Default TTL
			KeyPrefix: "custom-prefix",
		}

		store, err := New(ctx, config, logger)
		assert.NoError(t, err)

		// Call with explicit TTL (30)
		result := store.TryAcquireLock(ctx, "service1", "domain1", "client1", 30)
		assert.True(t, result)

		// Clean up
		mockClient.On("Close").Return(nil)
		store.Close()
		mockClient.AssertExpectations(t)
	})

	t.Run("use_default_ttl", func(t *testing.T) {
		// Mock Redis client
		mockClient := new(MockRedisClient)

		// Setup Ping to succeed for New()
		statusCmd := redis.NewStatusCmd(ctx)
		statusCmd.SetVal("PONG")
		mockClient.On("Ping", mock.Anything).Return(statusCmd)

		// Setup SetNX to succeed with default TTL
		boolCmd := redis.NewBoolCmd(ctx)
		boolCmd.SetVal(true)
		mockClient.On("SetNX", ctx, "custom-prefix:service1:domain1", "client1",
			time.Duration(25)*time.Second).Return(boolCmd)

		// Override with test version
		newRedisClientFn = func(addr string, password string, db int) redisClientMock {
			return mockClient
		}

		// Create a store with custom config
		config := &RedisConfig{
			Host:      "localhost",
			Port:      6379,
			TTL:       25, // Default TTL
			KeyPrefix: "custom-prefix",
		}

		store, err := New(ctx, config, logger)
		assert.NoError(t, err)

		// Call with zero TTL (should use default)
		result := store.TryAcquireLock(ctx, "service1", "domain1", "client1", 0)
		assert.True(t, result)

		// Clean up
		mockClient.On("Close").Return(nil)
		store.Close()
		mockClient.AssertExpectations(t)
	})

	t.Run("negative_ttl_uses_default", func(t *testing.T) {
		// Mock Redis client
		mockClient := new(MockRedisClient)

		// Setup Ping to succeed for New()
		statusCmd := redis.NewStatusCmd(ctx)
		statusCmd.SetVal("PONG")
		mockClient.On("Ping", mock.Anything).Return(statusCmd)

		// Setup SetNX to succeed with default TTL
		boolCmd := redis.NewBoolCmd(ctx)
		boolCmd.SetVal(true)
		mockClient.On("SetNX", ctx, "custom-prefix:service1:domain1", "client1",
			time.Duration(25)*time.Second).Return(boolCmd)

		// Override with test version
		newRedisClientFn = func(addr string, password string, db int) redisClientMock {
			return mockClient
		}

		// Create a store with custom config
		config := &RedisConfig{
			Host:      "localhost",
			Port:      6379,
			TTL:       25, // Default TTL
			KeyPrefix: "custom-prefix",
		}

		store, err := New(ctx, config, logger)
		assert.NoError(t, err)

		// Call with negative TTL (should use default)
		result := store.TryAcquireLock(ctx, "service1", "domain1", "client1", -5)
		assert.True(t, result)

		// Clean up
		mockClient.On("Close").Return(nil)
		store.Close()
		mockClient.AssertExpectations(t)
	})
}

// TestCheckLockOwnership tests the refreshLockIfOwned logic indirectly through TryAcquireLock
func TestCheckLockOwnership(t *testing.T) {
	// Save the original function for restoration
	original := newRedisClientFn
	defer func() { newRedisClientFn = original }()

	logger, _, _ := observability.NewTestLogger()
	ctx := context.Background()

	t.Run("refresh_owned_lock", func(t *testing.T) {
		// Mock Redis client
		mockClient := new(MockRedisClient)

		// Setup Ping to succeed for New()
		statusCmd := redis.NewStatusCmd(ctx)
		statusCmd.SetVal("PONG")
		mockClient.On("Ping", mock.Anything).Return(statusCmd)

		// Setup SetNX to fail (lock exists)
		boolCmd := redis.NewBoolCmd(ctx)
		boolCmd.SetVal(false)
		mockClient.On("SetNX", ctx, "lock:service1:domain1", "client1",
			time.Duration(30)*time.Second).Return(boolCmd)

		// Setup Get to return client1 (client owns the lock)
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetVal("client1")
		mockClient.On("Get", ctx, "lock:service1:domain1").Return(stringCmd)

		// Setup Expire to succeed
		expireBoolCmd := redis.NewBoolCmd(ctx)
		expireBoolCmd.SetVal(true)
		mockClient.On("Expire", ctx, "lock:service1:domain1",
			time.Duration(30)*time.Second).Return(expireBoolCmd)

		// Override with test version
		newRedisClientFn = func(addr string, password string, db int) redisClientMock {
			return mockClient
		}

		// Create a store
		config := &RedisConfig{
			Host:      "localhost",
			Port:      6379,
			TTL:       15,
			KeyPrefix: "lock",
		}

		store, err := New(ctx, config, logger)
		assert.NoError(t, err)

		// Call TryAcquireLock - should succeed because client owns the lock
		result := store.TryAcquireLock(ctx, "service1", "domain1", "client1", 30)
		assert.True(t, result)

		// Clean up
		mockClient.On("Close").Return(nil)
		store.Close()
		mockClient.AssertExpectations(t)
	})

	t.Run("different_owner", func(t *testing.T) {
		// Mock Redis client
		mockClient := new(MockRedisClient)

		// Setup Ping to succeed for New()
		statusCmd := redis.NewStatusCmd(ctx)
		statusCmd.SetVal("PONG")
		mockClient.On("Ping", mock.Anything).Return(statusCmd)

		// Setup SetNX to fail (lock exists)
		boolCmd := redis.NewBoolCmd(ctx)
		boolCmd.SetVal(false)
		mockClient.On("SetNX", ctx, "lock:service1:domain1", "client1",
			time.Duration(30)*time.Second).Return(boolCmd)

		// Setup Get to return client2 (another client owns the lock)
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetVal("client2")
		mockClient.On("Get", ctx, "lock:service1:domain1").Return(stringCmd)

		// Override with test version
		newRedisClientFn = func(addr string, password string, db int) redisClientMock {
			return mockClient
		}

		// Create a store
		config := &RedisConfig{
			Host:      "localhost",
			Port:      6379,
			TTL:       15,
			KeyPrefix: "lock",
		}

		store, err := New(ctx, config, logger)
		assert.NoError(t, err)

		// Call TryAcquireLock - should fail because another client owns the lock
		result := store.TryAcquireLock(ctx, "service1", "domain1", "client1", 30)
		assert.False(t, result)

		// Clean up
		mockClient.On("Close").Return(nil)
		store.Close()
		mockClient.AssertExpectations(t)
	})

	t.Run("key_not_found", func(t *testing.T) {
		// Mock Redis client
		mockClient := new(MockRedisClient)

		// Setup Ping to succeed for New()
		statusCmd := redis.NewStatusCmd(ctx)
		statusCmd.SetVal("PONG")
		mockClient.On("Ping", mock.Anything).Return(statusCmd)

		// Setup SetNX to fail (for some reason)
		boolCmd := redis.NewBoolCmd(ctx)
		boolCmd.SetVal(false)
		mockClient.On("SetNX", ctx, "lock:service1:domain1", "client1",
			time.Duration(30)*time.Second).Return(boolCmd)

		// Setup Get to return nil (key not found)
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetErr(redis.Nil)
		mockClient.On("Get", ctx, "lock:service1:domain1").Return(stringCmd)

		// Override with test version
		newRedisClientFn = func(addr string, password string, db int) redisClientMock {
			return mockClient
		}

		// Create a store
		config := &RedisConfig{
			Host:      "localhost",
			Port:      6379,
			TTL:       15,
			KeyPrefix: "lock",
		}

		store, err := New(ctx, config, logger)
		assert.NoError(t, err)

		// Call TryAcquireLock - should fail because key not found
		result := store.TryAcquireLock(ctx, "service1", "domain1", "client1", 30)
		assert.False(t, result)

		// Clean up
		mockClient.On("Close").Return(nil)
		store.Close()
		mockClient.AssertExpectations(t)
	})

	t.Run("get_error", func(t *testing.T) {
		// Mock Redis client
		mockClient := new(MockRedisClient)

		// Setup Ping to succeed for New()
		statusCmd := redis.NewStatusCmd(ctx)
		statusCmd.SetVal("PONG")
		mockClient.On("Ping", mock.Anything).Return(statusCmd)

		// Setup SetNX to fail (for some reason)
		boolCmd := redis.NewBoolCmd(ctx)
		boolCmd.SetVal(false)
		mockClient.On("SetNX", ctx, "lock:service1:domain1", "client1",
			time.Duration(30)*time.Second).Return(boolCmd)

		// Setup Get to return error
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetErr(errors.New("connection error"))
		mockClient.On("Get", ctx, "lock:service1:domain1").Return(stringCmd)

		// Override with test version
		newRedisClientFn = func(addr string, password string, db int) redisClientMock {
			return mockClient
		}

		// Create a store
		config := &RedisConfig{
			Host:      "localhost",
			Port:      6379,
			TTL:       15,
			KeyPrefix: "lock",
		}

		store, err := New(ctx, config, logger)
		assert.NoError(t, err)

		// Call TryAcquireLock - should fail because of connection error
		result := store.TryAcquireLock(ctx, "service1", "domain1", "client1", 30)
		assert.False(t, result)

		// Clean up
		mockClient.On("Close").Return(nil)
		store.Close()
		mockClient.AssertExpectations(t)
	})

	t.Run("expire_error", func(t *testing.T) {
		// Mock Redis client
		mockClient := new(MockRedisClient)

		// Setup Ping to succeed for New()
		statusCmd := redis.NewStatusCmd(ctx)
		statusCmd.SetVal("PONG")
		mockClient.On("Ping", mock.Anything).Return(statusCmd)

		// Setup SetNX to fail (lock exists)
		boolCmd := redis.NewBoolCmd(ctx)
		boolCmd.SetVal(false)
		mockClient.On("SetNX", ctx, "lock:service1:domain1", "client1",
			time.Duration(30)*time.Second).Return(boolCmd)

		// Setup Get to return client1 (client owns the lock)
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetVal("client1")
		mockClient.On("Get", ctx, "lock:service1:domain1").Return(stringCmd)

		// Setup Expire to fail
		expireBoolCmd := redis.NewBoolCmd(ctx)
		expireBoolCmd.SetErr(errors.New("connection error"))
		mockClient.On("Expire", ctx, "lock:service1:domain1",
			time.Duration(30)*time.Second).Return(expireBoolCmd)

		// Override with test version
		newRedisClientFn = func(addr string, password string, db int) redisClientMock {
			return mockClient
		}

		// Create a store
		config := &RedisConfig{
			Host:      "localhost",
			Port:      6379,
			TTL:       15,
			KeyPrefix: "lock",
		}

		store, err := New(ctx, config, logger)
		assert.NoError(t, err)

		// Call TryAcquireLock - should fail because expire failed
		result := store.TryAcquireLock(ctx, "service1", "domain1", "client1", 30)
		assert.False(t, result)

		// Clean up
		mockClient.On("Close").Return(nil)
		store.Close()
		mockClient.AssertExpectations(t)
	})
}
