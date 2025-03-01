// internal/server/server_redis_test.go
package server

import (
	"context"
	"fmt"
	"testing"
	"time"

	pb "github.com/avivl/quorum-quest/api/gen/go/v1"
	"github.com/avivl/quorum-quest/internal/config"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/avivl/quorum-quest/internal/store/redis"
	redislib "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// CustomRedisMockStore is a custom implementation that we can fully control
// for testing without accessing unexported fields of redis.MockStore
type CustomRedisMockStore struct {
	mockClient *redis.MockRedisClient // Changed to pointer type
	ttl        int32
	keyPrefix  string
	logger     *observability.SLogger
	config     *redis.RedisConfig
}

func (s *CustomRedisMockStore) GetConfig() store.StoreConfig {
	return s.config
}

// getLockKey generates a consistent key for a lock
func (s *CustomRedisMockStore) getLockKey(service, domain string) string {
	return fmt.Sprintf("%s:%s:%s", s.keyPrefix, service, domain)
}

// TryAcquireLock attempts to acquire a lock
func (s *CustomRedisMockStore) TryAcquireLock(ctx context.Context, service, domain, clientId string, ttl int32) bool {
	key := s.getLockKey(service, domain)
	_ttl := ttl
	if ttl == 0 {
		_ttl = s.ttl
	}

	// Use Redis SET with NX option to only set if key doesn't exist
	cmd := s.mockClient.SetNX(ctx, key, clientId, time.Duration(_ttl)*time.Second)
	success, err := cmd.Result()
	if err != nil {
		s.logger.Errorf("Error acquiring lock: %v", err)
		return false
	}

	// If SET NX succeeded, the lock was acquired
	if success {
		return true
	}

	// If SET NX failed, check if this client already owns the lock
	getCmd := s.mockClient.Get(ctx, key)
	existingOwner, err := getCmd.Result()
	if err != nil {
		if err != redislib.Nil {
			s.logger.Errorf("Error checking lock ownership: %v", err)
		}
		return false
	}

	// If this client already owns the lock, refresh the TTL and return success
	if existingOwner == clientId {
		expireCmd := s.mockClient.Expire(ctx, key, time.Duration(_ttl)*time.Second)
		err = expireCmd.Err()
		if err != nil {
			s.logger.Errorf("Error refreshing lock TTL: %v", err)
			return false
		}
		return true
	}

	return false
}

// ReleaseLock releases a lock if it's owned by the specified client
func (s *CustomRedisMockStore) ReleaseLock(ctx context.Context, service, domain, clientId string) {
	key := s.getLockKey(service, domain)

	// First validate that this client owns the lock
	getCmd := s.mockClient.Get(ctx, key)
	existingOwner, err := getCmd.Result()
	if err != nil {
		if err != redislib.Nil {
			s.logger.Errorf("Error checking lock ownership: %v", err)
		}
		return
	}

	// Only delete the key if this client owns the lock
	if existingOwner == clientId {
		delCmd := s.mockClient.Del(ctx, key) // Changed to pass a single string key
		_, err = delCmd.Result()
		if err != nil {
			s.logger.Errorf("Error releasing lock: %v", err)
		}
	}
}

// KeepAlive refreshes a lock's TTL if it's owned by the specified client
func (s *CustomRedisMockStore) KeepAlive(ctx context.Context, service, domain, clientId string, ttl int32) time.Duration {
	key := s.getLockKey(service, domain)
	_ttl := ttl
	if ttl == 0 {
		_ttl = s.ttl
	}

	// Check if this client owns the lock
	getCmd := s.mockClient.Get(ctx, key)
	existingOwner, err := getCmd.Result()
	if err != nil {
		if err != redislib.Nil {
			s.logger.Errorf("Error checking lock for keep-alive: %v", err)
		}
		return time.Duration(-1) * time.Second
	}

	// Only refresh the TTL if this client owns the lock
	if existingOwner == clientId {
		expireCmd := s.mockClient.Expire(ctx, key, time.Duration(_ttl)*time.Second)
		success, err := expireCmd.Result()
		if err != nil {
			s.logger.Errorf("Error refreshing lock TTL: %v", err)
			return time.Duration(-1) * time.Second
		}

		if !success {
			s.logger.Errorf("Failed to refresh lock TTL: key no longer exists")
			return time.Duration(-1) * time.Second
		}

		return time.Duration(_ttl) * time.Second
	}

	return time.Duration(-1) * time.Second
}

// Close closes the Redis client connection
func (s *CustomRedisMockStore) Close() {
	err := s.mockClient.Close()
	if err != nil {
		s.logger.Errorf("Error closing Redis connection: %v", err)
	}
}

func setupRedisMockServer(t *testing.T) (*Server[*redis.RedisConfig], *redis.MockRedisClient, context.Context) {
	ctx := context.Background()

	// Create Redis config
	redisConfig := &redis.RedisConfig{
		Host:      "localhost",
		Port:      6379,
		Password:  "",
		DB:        0,
		TTL:       15,
		KeyPrefix: "lock",
		Endpoints: []string{"localhost:6379"},
		TableName: "locks",
	}

	// Create global config with Redis config
	cfg := &config.GlobalConfig[*redis.RedisConfig]{
		ServerAddress: "localhost:50052",
		Store:         redisConfig,
	}

	// Setup logger and metrics
	logger := setupTestLogger(t)
	serverMetrics := setupTestMetrics(t, logger)

	// Setup mock Redis client
	mockRedisClient := new(redis.MockRedisClient)

	// Create the store initializer that returns our mock store
	storeInitializer := func(ctx context.Context, cfg *config.GlobalConfig[*redis.RedisConfig], logger *observability.SLogger) (store.Store, error) {
		// Create a custom mock store that we can fully control
		mockStore := &CustomRedisMockStore{
			mockClient: mockRedisClient, // Fixed: now using the pointer correctly
			ttl:        cfg.Store.TTL,
			keyPrefix:  cfg.Store.KeyPrefix,
			logger:     logger,
			config:     cfg.Store,
		}
		return mockStore, nil
	}

	// Initialize server
	server, err := NewServer(cfg, logger, serverMetrics, storeInitializer)
	require.NoError(t, err)

	// Initialize store
	err = server.initStore(ctx, storeInitializer)
	require.NoError(t, err)

	return server, mockRedisClient, ctx
}

func TestTryAcquireLockWithRedisMock(t *testing.T) {
	server, mockRedisClient, ctx := setupRedisMockServer(t)
	defer server.Stop()

	tests := []struct {
		name           string
		service        string
		domain         string
		clientID       string
		ttl            int32
		expectedResult bool
		mockSetup      func()
	}{
		{
			name:           "successful lock acquisition",
			service:        "test-service-1",
			domain:         "test-domain-1",
			clientID:       "client-1",
			ttl:            30,
			expectedResult: true,
			mockSetup: func() {
				// Mock successful SetNX
				boolCmd := redislib.NewBoolCmd(ctx)
				boolCmd.SetVal(true) // Key was set (lock acquired)
				mockRedisClient.On("SetNX", mock.Anything, "lock:test-service-1:test-domain-1", "client-1",
					time.Duration(30)*time.Second).Return(boolCmd).Once()
			},
		},
		{
			name:           "failed lock acquisition - lock already exists",
			service:        "test-service-2",
			domain:         "test-domain-2",
			clientID:       "client-2",
			ttl:            30,
			expectedResult: false,
			mockSetup: func() {
				// Mock failed SetNX
				boolCmd := redislib.NewBoolCmd(ctx)
				boolCmd.SetVal(false) // Key was not set (lock not acquired)
				mockRedisClient.On("SetNX", mock.Anything, "lock:test-service-2:test-domain-2", "client-2",
					time.Duration(30)*time.Second).Return(boolCmd).Once()

				// Mock Get for checking lock ownership
				stringCmd := redislib.NewStringCmd(ctx)
				stringCmd.SetVal("other-client") // Someone else owns the lock
				mockRedisClient.On("Get", mock.Anything, "lock:test-service-2:test-domain-2").Return(stringCmd).Once()
			},
		},
		{
			name:           "lock refresh - client already owns lock",
			service:        "test-service-3",
			domain:         "test-domain-3",
			clientID:       "client-3",
			ttl:            30,
			expectedResult: true,
			mockSetup: func() {
				// Mock failed SetNX
				boolCmd := redislib.NewBoolCmd(ctx)
				boolCmd.SetVal(false) // Key was not set (lock already exists)
				mockRedisClient.On("SetNX", mock.Anything, "lock:test-service-3:test-domain-3", "client-3",
					time.Duration(30)*time.Second).Return(boolCmd).Once()

				// Mock Get for checking lock ownership
				stringCmd := redislib.NewStringCmd(ctx)
				stringCmd.SetVal("client-3") // This client already owns the lock
				mockRedisClient.On("Get", mock.Anything, "lock:test-service-3:test-domain-3").Return(stringCmd).Once()

				// Mock Expire for refreshing TTL
				boolRefreshCmd := redislib.NewBoolCmd(ctx)
				boolRefreshCmd.SetVal(true) // TTL refresh successful
				mockRedisClient.On("Expire", mock.Anything, "lock:test-service-3:test-domain-3",
					time.Duration(30)*time.Second).Return(boolRefreshCmd).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks for this test case
			tt.mockSetup()

			// Create request
			req := &pb.TryAcquireLockRequest{
				Service:  tt.service,
				Domain:   tt.domain,
				ClientId: tt.clientID,
				Ttl:      tt.ttl,
			}

			// Execute method
			resp, err := server.TryAcquireLock(ctx, req)

			// Assertions
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.expectedResult, resp.IsLeader)

			// Verify all mocks were called as expected
			mockRedisClient.AssertExpectations(t)
		})
	}
}

func TestReleaseLockWithRedisMock(t *testing.T) {
	server, mockRedisClient, ctx := setupRedisMockServer(t)
	defer server.Stop()

	tests := []struct {
		name      string
		service   string
		domain    string
		clientID  string
		mockSetup func()
	}{
		{
			name:     "successful lock release - client owns lock",
			service:  "test-service-1",
			domain:   "test-domain-1",
			clientID: "client-1",
			mockSetup: func() {
				// Mock Get for checking lock ownership
				stringCmd := redislib.NewStringCmd(ctx)
				stringCmd.SetVal("client-1") // This client owns the lock
				mockRedisClient.On("Get", mock.Anything, "lock:test-service-1:test-domain-1").Return(stringCmd).Once()

				// Mock Del for releasing the lock
				intCmd := redislib.NewIntCmd(ctx)
				intCmd.SetVal(1) // 1 key deleted
				mockRedisClient.On("Del", mock.Anything, []string{"lock:test-service-1:test-domain-1"}).Return(intCmd).Once()
			},
		},
		{
			name:     "lock release no-op - client doesn't own lock",
			service:  "test-service-2",
			domain:   "test-domain-2",
			clientID: "wrong-client",
			mockSetup: func() {
				// Mock Get for checking lock ownership
				stringCmd := redislib.NewStringCmd(ctx)
				stringCmd.SetVal("correct-client") // Another client owns the lock
				mockRedisClient.On("Get", mock.Anything, "lock:test-service-2:test-domain-2").Return(stringCmd).Once()

				// No Del should be called
			},
		},
		{
			name:     "lock release no-op - lock doesn't exist",
			service:  "test-service-3",
			domain:   "test-domain-3",
			clientID: "client-3",
			mockSetup: func() {
				// Mock Get for checking lock ownership - lock doesn't exist
				stringCmd := redislib.NewStringCmd(ctx)
				stringCmd.SetErr(redislib.Nil) // Key doesn't exist
				mockRedisClient.On("Get", mock.Anything, "lock:test-service-3:test-domain-3").Return(stringCmd).Once()

				// No Del should be called
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks for this test case
			tt.mockSetup()

			// Create request
			req := &pb.ReleaseLockRequest{
				Service:  tt.service,
				Domain:   tt.domain,
				ClientId: tt.clientID,
			}

			// Execute method
			resp, err := server.ReleaseLock(ctx, req)

			// Assertions
			assert.NoError(t, err)
			assert.NotNil(t, resp)

			// Verify all mocks were called as expected
			mockRedisClient.AssertExpectations(t)
		})
	}
}

func TestKeepAliveWithRedisMock(t *testing.T) {
	server, mockRedisClient, ctx := setupRedisMockServer(t)
	defer server.Stop()

	tests := []struct {
		name           string
		service        string
		domain         string
		clientID       string
		ttl            int32
		expectedStatus int64 // seconds for the lease, -1 for failure
		mockSetup      func()
	}{
		{
			name:           "successful keep-alive",
			service:        "test-service-1",
			domain:         "test-domain-1",
			clientID:       "client-1",
			ttl:            30,
			expectedStatus: 30, // 30 seconds lease
			mockSetup: func() {
				// Mock Get for checking lock ownership
				stringCmd := redislib.NewStringCmd(ctx)
				stringCmd.SetVal("client-1") // This client owns the lock
				mockRedisClient.On("Get", mock.Anything, "lock:test-service-1:test-domain-1").Return(stringCmd).Once()

				// Mock Expire for refreshing TTL
				boolCmd := redislib.NewBoolCmd(ctx)
				boolCmd.SetVal(true) // TTL refresh successful
				mockRedisClient.On("Expire", mock.Anything, "lock:test-service-1:test-domain-1",
					time.Duration(30)*time.Second).Return(boolCmd).Once()
			},
		},
		{
			name:           "failed keep-alive - client doesn't own lock",
			service:        "test-service-2",
			domain:         "test-domain-2",
			clientID:       "wrong-client",
			ttl:            30,
			expectedStatus: -1, // Failure
			mockSetup: func() {
				// Mock Get for checking lock ownership
				stringCmd := redislib.NewStringCmd(ctx)
				stringCmd.SetVal("correct-client") // Another client owns the lock
				mockRedisClient.On("Get", mock.Anything, "lock:test-service-2:test-domain-2").Return(stringCmd).Once()

				// No Expire should be called
			},
		},
		{
			name:           "failed keep-alive - lock doesn't exist",
			service:        "test-service-3",
			domain:         "test-domain-3",
			clientID:       "client-3",
			ttl:            30,
			expectedStatus: -1, // Failure
			mockSetup: func() {
				// Mock Get for checking lock ownership - lock doesn't exist
				stringCmd := redislib.NewStringCmd(ctx)
				stringCmd.SetErr(redislib.Nil) // Key doesn't exist
				mockRedisClient.On("Get", mock.Anything, "lock:test-service-3:test-domain-3").Return(stringCmd).Once()

				// No Expire should be called
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks for this test case
			tt.mockSetup()

			// Create request
			req := &pb.KeepAliveRequest{
				Service:  tt.service,
				Domain:   tt.domain,
				ClientId: tt.clientID,
				Ttl:      tt.ttl,
			}

			// Execute method
			resp, err := server.KeepAlive(ctx, req)

			// Assertions
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.expectedStatus, resp.LeaseLength.Seconds)

			// Verify all mocks were called as expected
			mockRedisClient.AssertExpectations(t)
		})
	}
}
