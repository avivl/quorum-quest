// internal/store/redis/constructor_test.go
package redis

import (
	"context"
	"testing"

	"github.com/avivl/quorum-quest/internal/lockservice"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewStore(t *testing.T) {
	// Save the original function for restoration
	original := newRedisClientFn
	defer func() { newRedisClientFn = original }()

	logger, _, _ := observability.NewTestLogger()
	ctx := context.Background()

	t.Run("newStore_with_valid_config", func(t *testing.T) {
		// Mock Redis client
		mockClient := new(MockRedisClient)
		statusCmd := redis.NewStatusCmd(ctx)
		statusCmd.SetVal("PONG")
		mockClient.On("Ping", mock.Anything).Return(statusCmd)

		// Override with test version
		newRedisClientFn = func(addr string, password string, db int) redisClientMock {
			return mockClient
		}

		// Create config
		config := &RedisConfig{
			Host:      "localhost",
			Port:      6379,
			TTL:       30,
			KeyPrefix: "test-lock",
		}

		// Call the function under test
		result, err := newStore(ctx, config, logger)

		// Verify results
		assert.NoError(t, err)
		assert.NotNil(t, result)

		// Clean up
		mockClient.On("Close").Return(nil)
		result.Close()
		mockClient.AssertExpectations(t)
	})

	t.Run("newStore_with_invalid_config_type", func(t *testing.T) {
		// Use a different type of config
		invalidConfig := struct {
			Host string
			Port int
		}{
			Host: "localhost",
			Port: 6379,
		}

		// Call the function under test
		result, err := newStore(ctx, invalidConfig, logger)

		// Verify results
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.IsType(t, &store.InvalidConfigurationError{}, err)
	})

	t.Run("lockservice_register_test", func(t *testing.T) {
		// Since init() already registered the Redis store, check if it's in the list
		constructors := lockservice.Constructors()

		// Verify that "redis" is in the list of registered constructors
		found := false
		for _, name := range constructors {
			if name == StoreName {
				found = true
				break
			}
		}
		assert.True(t, found, "Redis store should be registered with lockservice")

		// For additional verification, try to unregister and re-register
		lockservice.Unregister(StoreName)

		// Register should work without panic
		assert.NotPanics(t, func() { lockservice.Register(StoreName, newStore) })

		// But registering twice should panic
		assert.Panics(t, func() { lockservice.Register(StoreName, newStore) })
	})

	t.Run("register_with_nil_constructor", func(t *testing.T) {
		// First unregister the existing constructor
		testStoreName := "test-store"

		// Remove if it exists
		lockservice.Unregister(testStoreName)

		// Verify that registering nil constructor panics
		assert.Panics(t, func() { lockservice.Register(testStoreName, nil) })
	})
}
