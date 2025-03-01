// internal/store/redis/mock_redis.go
package redis

import (
	"context"
	"time"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
)

// Define a redis client interface to make mocking easier
type redisClientMock interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	Ping(ctx context.Context) *redis.StatusCmd
	Close() error
}

// MockRedisClient is a mock for the Redis client
type MockRedisClient struct {
	mock.Mock
}

// SetNX mocks the SetNX method
func (m *MockRedisClient) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	args := m.Called(ctx, key, value, expiration)
	return args.Get(0).(*redis.BoolCmd)
}

// Get mocks the Get method
func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	args := m.Called(ctx, key)
	return args.Get(0).(*redis.StringCmd)
}

// Del mocks the Del method
func (m *MockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	args := m.Called(ctx, keys)
	return args.Get(0).(*redis.IntCmd)
}

// Expire mocks the Expire method
func (m *MockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	args := m.Called(ctx, key, expiration)
	return args.Get(0).(*redis.BoolCmd)
}

// Ping mocks the Ping method
func (m *MockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	args := m.Called(ctx)
	return args.Get(0).(*redis.StatusCmd)
}

// Close mocks the Close method
func (m *MockRedisClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockStore is a mock implementation of the Redis store
type MockStore struct {
	client    *MockRedisClient
	ttl       int32
	l         *observability.SLogger
	keyPrefix string
	config    *RedisConfig
}

// GetConfig implements the Store interface
func (s *MockStore) GetConfig() store.StoreConfig {
	return s.config
}

// getLockKey generates a consistent key for a lock
func (s *MockStore) getLockKey(service, domain string) string {
	return s.keyPrefix + ":" + service + ":" + domain
}

// TryAcquireLock attempts to acquire a lock
func (s *MockStore) TryAcquireLock(ctx context.Context, service, domain, clientId string, ttl int32) bool {
	key := s.getLockKey(service, domain)
	_ttl := ttl
	if ttl == 0 {
		_ttl = s.ttl
	}

	// Use Redis SET with NX option to only set if key doesn't exist
	cmd := s.client.SetNX(ctx, key, clientId, time.Duration(_ttl)*time.Second)
	success, err := cmd.Result()
	if err != nil {
		s.l.Errorf("Error acquiring lock: %v", err)
		return false
	}

	// If SET NX succeeded, the lock was acquired
	if success {
		return true
	}

	// If SET NX failed, check if this client already owns the lock
	getCmd := s.client.Get(ctx, key)
	existingOwner, err := getCmd.Result()
	if err != nil {
		if err != redis.Nil {
			s.l.Errorf("Error checking lock ownership: %v", err)
		}
		return false
	}

	// If this client already owns the lock, refresh the TTL and return success
	if existingOwner == clientId {
		expireCmd := s.client.Expire(ctx, key, time.Duration(_ttl)*time.Second)
		err = expireCmd.Err()
		if err != nil {
			s.l.Errorf("Error refreshing lock TTL: %v", err)
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
	getCmd := s.client.Get(ctx, key)
	existingOwner, err := getCmd.Result()
	if err != nil {
		if err != redis.Nil {
			s.l.Errorf("Error checking lock ownership: %v", err)
		}
		return
	}

	// Only delete the key if this client owns the lock
	if existingOwner == clientId {
		delCmd := s.client.Del(ctx, key)
		_, err = delCmd.Result()
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
	getCmd := s.client.Get(ctx, key)
	existingOwner, err := getCmd.Result()
	if err != nil {
		if err != redis.Nil {
			s.l.Errorf("Error checking lock for keep-alive: %v", err)
		}
		return time.Duration(-1) * time.Second
	}

	// Only refresh the TTL if this client owns the lock
	if existingOwner == clientId {
		expireCmd := s.client.Expire(ctx, key, time.Duration(_ttl)*time.Second)
		success, err := expireCmd.Result()
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
		Endpoints: []string{},
		TableName: "locks",
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
