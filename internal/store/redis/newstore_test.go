// internal/store/redis/newstore_test.go
package redis

import (
	"context"
	"errors"
	"testing"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Define a variable to store the original function so we can restore it
var originalNewClient func(addr string, password string, db int) redisClientMock

func init() {
	// Save the original function before we override it
	originalNewClient = func(addr string, password string, db int) redisClientMock {
		return redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		})
	}
}

func TestNew(t *testing.T) {
	logger, _, _ := observability.NewTestLogger()

	t.Run("nil_config", func(t *testing.T) {
		store, err := New(context.Background(), nil, logger)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Equal(t, ErrConfigOptionMissing, err)
	})

	t.Run("connection_error", func(t *testing.T) {
		// Mock Redis client
		mockClient := new(MockRedisClient)

		// Setup Ping to return an error
		statusCmd := redis.NewStatusCmd(context.Background())
		statusCmd.SetErr(errors.New("connection refused"))
		mockClient.On("Ping", mock.Anything).Return(statusCmd)

		// Create a config
		config := &RedisConfig{
			Host: "localhost",
			Port: 6379,
		}

		// Call New() with a mock function that returns our mock client
		// Store original and restore after test
		original := newRedisClientFn
		defer func() { newRedisClientFn = original }()

		// Override with our test version
		newRedisClientFn = func(addr string, password string, db int) redisClientMock {
			return mockClient
		}

		store, err := New(context.Background(), config, logger)

		// Verify result
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "failed to connect to Redis")
		mockClient.AssertExpectations(t)
	})

	t.Run("successful_connection", func(t *testing.T) {
		// Mock Redis client
		mockClient := new(MockRedisClient)

		// Setup Ping to succeed
		statusCmd := redis.NewStatusCmd(context.Background())
		statusCmd.SetVal("PONG")
		mockClient.On("Ping", mock.Anything).Return(statusCmd)

		// Create a config
		config := &RedisConfig{
			Host:      "localhost",
			Port:      6379,
			TTL:       30,
			KeyPrefix: "test-lock",
		}

		// Store original and restore after test
		original := newRedisClientFn
		defer func() { newRedisClientFn = original }()

		// Override with our test version
		newRedisClientFn = func(addr string, password string, db int) redisClientMock {
			return mockClient
		}

		// Call the method under test
		store, err := New(context.Background(), config, logger)

		// Verify result
		assert.NoError(t, err)
		assert.NotNil(t, store)

		// Verify store is configured correctly
		assert.Equal(t, config, store.config)
		assert.Equal(t, int32(30), store.ttl)
		assert.Equal(t, "test-lock", store.keyPrefix)

		// Clean up
		mockClient.On("Close").Return(nil)
		store.Close()
		mockClient.AssertExpectations(t)
	})
}

func TestStoreInterfaceCompliance(t *testing.T) {
	t.Run("implements_store_interface", func(t *testing.T) {
		// Create a store instance
		mockStore, _ := SetupMockStore()

		// Check if it implements the store.Store interface
		var i store.Store = mockStore
		assert.NotNil(t, i)
	})
}

func TestStoreConfig(t *testing.T) {
	t.Run("returns_config", func(t *testing.T) {
		// Create a store
		mockStore, _ := SetupMockStore()

		// Call method under test
		result := mockStore.GetConfig()

		// Verify the result is the same config
		assert.Equal(t, mockStore.config, result)
	})
}

func TestClose(t *testing.T) {
	t.Run("close_succeeds", func(t *testing.T) {
		// Create a store
		mockStore, mockClient := SetupMockStore()

		// Setup expectations
		mockClient.On("Close").Return(nil)

		// Call method under test
		mockStore.Close()

		// Verify expectations
		mockClient.AssertExpectations(t)
	})

	t.Run("close_logs_error", func(t *testing.T) {
		// Create a store
		mockStore, mockClient := SetupMockStore()

		// Setup expectations
		mockClient.On("Close").Return(errors.New("close error"))

		// Call method under test - should not panic despite error
		mockStore.Close()

		// Verify expectations
		mockClient.AssertExpectations(t)
	})
}
