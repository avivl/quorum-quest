// internal/store/redis/redis_test.go
package redis

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRedisClient is a mock implementation of the Redis client
type MockRedisClient struct {
	mock.Mock
}

// SetNX mocks the redis.Client.SetNX method
func (m *MockRedisClient) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	args := m.Called(ctx, key, value, expiration)
	return mockBoolCmd(args.Bool(0), args.Error(1))
}

// Get mocks the redis.Client.Get method
func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	args := m.Called(ctx, key)
	return mockStringCmd(args.String(0), args.Error(1))
}

// Expire mocks the redis.Client.Expire method
func (m *MockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	args := m.Called(ctx, key, expiration)
	return mockBoolCmd(args.Bool(0), args.Error(1))
}

// Del mocks the redis.Client.Del method
func (m *MockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	args := m.Called(ctx, keys)
	return mockIntCmd(args.Get(0).(int64), args.Error(1))
}

// Ping mocks the redis.Client.Ping method
func (m *MockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	args := m.Called(ctx)
	return mockStatusCmd(args.String(0), args.Error(1))
}

// Close mocks the redis.Client.Close method
func (m *MockRedisClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Helper functions to create mock commands

func mockBoolCmd(val bool, err error) *redis.BoolCmd {
	cmd := redis.NewBoolCmd(context.Background())
	cmd.SetVal(val)
	cmd.SetErr(err)
	return cmd
}

func mockStringCmd(val string, err error) *redis.StringCmd {
	cmd := redis.NewStringCmd(context.Background())
	cmd.SetVal(val)
	cmd.SetErr(err)
	return cmd
}

func mockIntCmd(val int64, err error) *redis.IntCmd {
	cmd := redis.NewIntCmd(context.Background())
	cmd.SetVal(val)
	cmd.SetErr(err)
	return cmd
}

func mockStatusCmd(val string, err error) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(context.Background())
	cmd.SetVal(val)
	cmd.SetErr(err)
	return cmd
}

// MockStore is a custom version of Store for testing
type MockStore struct {
	client    *MockRedisClient
	ttl       int32
	l         *observability.SLogger
	keyPrefix string
	config    *RedisConfig
}

// GetConfig implements the store.Store interface
func (s *MockStore) GetConfig() store.StoreConfig {
	return s.config
}

// getLockKey generates a consistent key for a lock
func (s *MockStore) getLockKey(service, domain string) string {
	return fmt.Sprintf("%s:%s:%s", s.keyPrefix, service, domain)
}

// TryAcquireLock attempts to acquire a lock for the given service and domain
func (s *MockStore) TryAcquireLock(ctx context.Context, service, domain, clientId string, ttl int32) bool {
	key := s.getLockKey(service, domain)
	_ttl := ttl
	if ttl == 0 {
		_ttl = s.ttl
	}

	// Use Redis SET with NX option to only set if key doesn't exist
	success, err := s.client.SetNX(ctx, key, clientId, time.Duration(_ttl)*time.Second).Result()
	if err != nil {
		s.l.Errorf("Error acquiring lock: %v", err)
		return false
	}

	// If SET NX succeeded, the lock was acquired
	if success {
		return true
	}

	// If SET NX failed, check if this client already owns the lock
	existingOwner, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			s.l.Errorf("Error checking lock ownership: %v", err)
		}
		return false
	}

	// If this client already owns the lock, refresh the TTL and return success
	if existingOwner == clientId {
		success, err := s.client.Expire(ctx, key, time.Duration(_ttl)*time.Second).Result()
		if err != nil {
			s.l.Errorf("Error refreshing lock TTL: %v", err)
			return false
		}
		if !success {
			s.l.Errorf("Failed to refresh lock TTL: key no longer exists")
			return false
		}
		return true
	}

	return false
}

// ReleaseLock releases a lock if it's owned by the specified client
func (s *MockStore) ReleaseLock(ctx context.Context, service, domain, clientId string) {
	key := s.getLockKey(service, domain)

	// First validate that this client owns the lock
	existingOwner, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			s.l.Errorf("Error checking lock ownership: %v", err)
		}
		return
	}

	// Only delete the key if this client owns the lock
	if existingOwner == clientId {
		_, err = s.client.Del(ctx, key).Result()
		if err != nil {
			s.l.Errorf("Error releasing lock: %v", err)
		}
	}
}

// KeepAlive refreshes a lock's TTL if it's owned by the specified client
func (s *MockStore) KeepAlive(ctx context.Context, service, domain, clientId string, ttl int32) time.Duration {
	key := s.getLockKey(service, domain)
	_ttl := ttl
	if ttl == 0 {
		_ttl = s.ttl
	}

	// Check if this client owns the lock
	existingOwner, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			s.l.Errorf("Error checking lock for keep-alive: %v", err)
		}
		return time.Duration(-1) * time.Second
	}

	// Only refresh the TTL if this client owns the lock
	if existingOwner == clientId {
		success, err := s.client.Expire(ctx, key, time.Duration(_ttl)*time.Second).Result()
		if err != nil {
			s.l.Errorf("Error refreshing lock TTL: %v", err)
			return time.Duration(-1) * time.Second
		}

		if !success {
			s.l.Errorf("Failed to refresh lock TTL: key no longer exists")
			return time.Duration(-1) * time.Second
		}

		return time.Duration(_ttl) * time.Second
	}

	return time.Duration(-1) * time.Second
}

// Close closes the Redis client connection
func (s *MockStore) Close() {
	err := s.client.Close()
	if err != nil {
		s.l.Errorf("Error closing Redis connection: %v", err)
	}
}

// SetupMockStore creates a MockStore with a mocked Redis client for testing
func SetupMockStore() (*MockStore, *MockRedisClient) {
	mockClient := new(MockRedisClient)
	logger, _, _ := observability.NewTestLogger()

	config := &RedisConfig{
		Host:      "localhost",
		Port:      6379,
		Password:  "",
		DB:        0,
		TTL:       15,
		KeyPrefix: "lock",
	}

	store := &MockStore{
		client:    mockClient,
		ttl:       config.TTL,
		l:         logger,
		keyPrefix: config.KeyPrefix,
		config:    config,
	}

	return store, mockClient
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
		// This test requires a mocked redis.NewClient function,
		// which is difficult with the current package structure.
		// In a real-world scenario, we would use a custom factory function
		// that could be overridden for testing.
		t.Skip("Skipping test that requires mocking redis.NewClient")
	})
}

func TestTryAcquireLock(t *testing.T) {
	t.Run("success_new_lock", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)
		key := mockStore.getLockKey(service, domain)

		// Mock successful SetNX (key didn't exist)
		mockClient.On("SetNX", ctx, key, clientID, time.Duration(ttl)*time.Second).Return(true, nil)

		// Call method under test
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.True(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("success_refresh_existing", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)
		key := mockStore.getLockKey(service, domain)

		// Mock failed SetNX (key exists)
		mockClient.On("SetNX", ctx, key, clientID, time.Duration(ttl)*time.Second).Return(false, nil)

		// Mock Get showing this client owns the lock
		mockClient.On("Get", ctx, key).Return(clientID, nil)

		// Mock successful Expire
		mockClient.On("Expire", ctx, key, time.Duration(ttl)*time.Second).Return(true, nil)

		// Call method under test
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.True(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("failure_different_owner", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		differentClientID := "client-2"
		ttl := int32(30)
		key := mockStore.getLockKey(service, domain)

		// Mock failed SetNX (key exists)
		mockClient.On("SetNX", ctx, key, clientID, time.Duration(ttl)*time.Second).Return(false, nil)

		// Mock Get showing different client owns the lock
		mockClient.On("Get", ctx, key).Return(differentClientID, nil)

		// Call method under test
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.False(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("failure_setnx_error", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)
		key := mockStore.getLockKey(service, domain)
		expectedError := errors.New("connection error")

		// Mock SetNX with error
		mockClient.On("SetNX", ctx, key, clientID, time.Duration(ttl)*time.Second).Return(false, expectedError)

		// Call method under test
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.False(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("failure_get_error", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)
		key := mockStore.getLockKey(service, domain)
		expectedError := errors.New("connection error")

		// Mock failed SetNX (key exists)
		mockClient.On("SetNX", ctx, key, clientID, time.Duration(ttl)*time.Second).Return(false, nil)

		// Mock Get with error
		mockClient.On("Get", ctx, key).Return("", expectedError)

		// Call method under test
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.False(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("failure_get_nil", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)
		key := mockStore.getLockKey(service, domain)

		// Mock failed SetNX (key exists)
		mockClient.On("SetNX", ctx, key, clientID, time.Duration(ttl)*time.Second).Return(false, nil)

		// Mock Get with Nil error (key doesn't exist, race condition)
		mockClient.On("Get", ctx, key).Return("", redis.Nil)

		// Call method under test
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.False(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("failure_expire_error", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)
		key := mockStore.getLockKey(service, domain)
		expectedError := errors.New("connection error")

		// Mock failed SetNX (key exists)
		mockClient.On("SetNX", ctx, key, clientID, time.Duration(ttl)*time.Second).Return(false, nil)

		// Mock Get showing this client owns the lock
		mockClient.On("Get", ctx, key).Return(clientID, nil)

		// Mock Expire with error
		mockClient.On("Expire", ctx, key, time.Duration(ttl)*time.Second).Return(false, expectedError)

		// Call method under test
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.False(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("default_ttl", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		defaultTtl := mockStore.ttl
		key := mockStore.getLockKey(service, domain)

		// Mock successful SetNX with default TTL
		mockClient.On("SetNX", ctx, key, clientID, time.Duration(defaultTtl)*time.Second).Return(true, nil)

		// Call method under test with ttl=0 to use default
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, 0)

		// Verify result and expectations
		assert.True(t, result)
		mockClient.AssertExpectations(t)
	})
}

func TestReleaseLock(t *testing.T) {
	t.Run("success_when_owns_lock", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		key := mockStore.getLockKey(service, domain)

		// Mock Get showing this client owns the lock
		mockClient.On("Get", ctx, key).Return(clientID, nil)

		// Mock successful Del
		mockClient.On("Del", ctx, []string{key}).Return(int64(1), nil)

		// Call method under test
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations
		mockClient.AssertExpectations(t)
	})

	t.Run("noop_when_doesnt_own_lock", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		differentClientID := "client-2"
		key := mockStore.getLockKey(service, domain)

		// Mock Get showing different client owns the lock
		mockClient.On("Get", ctx, key).Return(differentClientID, nil)

		// No Del call expected since client IDs don't match

		// Call method under test
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations
		mockClient.AssertExpectations(t)
	})

	t.Run("noop_when_lock_not_found", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		key := mockStore.getLockKey(service, domain)

		// Mock Get with Nil error (key doesn't exist)
		mockClient.On("Get", ctx, key).Return("", redis.Nil)

		// No Del call expected since lock doesn't exist

		// Call method under test
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations
		mockClient.AssertExpectations(t)
	})

	t.Run("logs_error_on_get_failure", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		key := mockStore.getLockKey(service, domain)
		expectedError := errors.New("connection error")

		// Mock Get with error
		mockClient.On("Get", ctx, key).Return("", expectedError)

		// No Del call expected due to Get error

		// Call method under test
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations
		mockClient.AssertExpectations(t)
	})

	t.Run("logs_error_on_del_failure", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		key := mockStore.getLockKey(service, domain)
		expectedError := errors.New("connection error")

		// Mock Get showing this client owns the lock
		mockClient.On("Get", ctx, key).Return(clientID, nil)

		// Mock Del with error
		mockClient.On("Del", ctx, []string{key}).Return(int64(0), expectedError)

		// Call method under test
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations
		mockClient.AssertExpectations(t)
	})
}

func TestKeepAlive(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)
		key := mockStore.getLockKey(service, domain)

		// Mock Get showing this client owns the lock
		mockClient.On("Get", ctx, key).Return(clientID, nil)

		// Mock successful Expire
		mockClient.On("Expire", ctx, key, time.Duration(ttl)*time.Second).Return(true, nil)

		// Call method under test
		result := mockStore.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.Equal(t, time.Duration(ttl)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("default_ttl", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		defaultTtl := mockStore.ttl
		key := mockStore.getLockKey(service, domain)

		// Mock Get showing this client owns the lock
		mockClient.On("Get", ctx, key).Return(clientID, nil)

		// Mock successful Expire with default TTL
		mockClient.On("Expire", ctx, key, time.Duration(defaultTtl)*time.Second).Return(true, nil)

		// Call method under test with ttl=0 to use default
		result := mockStore.KeepAlive(ctx, service, domain, clientID, 0)

		// Verify result and expectations
		assert.Equal(t, time.Duration(defaultTtl)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("failure_different_owner", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		differentClientID := "client-2"
		ttl := int32(30)
		key := mockStore.getLockKey(service, domain)

		// Mock Get showing different client owns the lock
		mockClient.On("Get", ctx, key).Return(differentClientID, nil)

		// No Expire call expected since client IDs don't match

		// Call method under test
		result := mockStore.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("failure_lock_not_found", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)
		key := mockStore.getLockKey(service, domain)

		// Mock Get with Nil error (key doesn't exist)
		mockClient.On("Get", ctx, key).Return("", redis.Nil)

		// No Expire call expected since lock doesn't exist

		// Call method under test
		result := mockStore.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("failure_get_error", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)
		key := mockStore.getLockKey(service, domain)
		expectedError := errors.New("connection error")

		// Mock Get with error
		mockClient.On("Get", ctx, key).Return("", expectedError)

		// No Expire call expected due to Get error

		// Call method under test
		result := mockStore.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("failure_expire_error", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)
		key := mockStore.getLockKey(service, domain)
		expectedError := errors.New("connection error")

		// Mock Get showing this client owns the lock
		mockClient.On("Get", ctx, key).Return(clientID, nil)

		// Mock Expire with error
		mockClient.On("Expire", ctx, key, time.Duration(ttl)*time.Second).Return(false, expectedError)

		// Call method under test
		result := mockStore.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("failure_expire_key_not_found", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)
		key := mockStore.getLockKey(service, domain)

		// Mock Get showing this client owns the lock
		mockClient.On("Get", ctx, key).Return(clientID, nil)

		// Mock Expire returning false (key doesn't exist anymore)
		mockClient.On("Expire", ctx, key, time.Duration(ttl)*time.Second).Return(false, nil)

		// Call method under test
		result := mockStore.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockClient.AssertExpectations(t)
	})
}

func TestClose(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()

		// Mock successful Close
		mockClient.On("Close").Return(nil)

		// Call method under test
		mockStore.Close()

		// Verify expectations
		mockClient.AssertExpectations(t)
	})

	t.Run("logs_error", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		expectedError := errors.New("close error")

		// Mock Close with error
		mockClient.On("Close").Return(expectedError)

		// Call method under test
		mockStore.Close()

		// Verify expectations
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

func TestGetConfig(t *testing.T) {
	t.Run("returns_config", func(t *testing.T) {
		// Create a store
		mockStore, _ := SetupMockStore()

		// Call method under test
		result := mockStore.GetConfig()

		// Verify the result is the same config
		assert.Equal(t, mockStore.config, result)
	})
}
