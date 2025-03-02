// internal/store/redis/redis_store.go
package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avivl/quorum-quest/internal/lockservice"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/redis/go-redis/v9"
)

// Error definitions
var (
	ErrConfigOptionMissing = errors.New("Redis requires a config option")
)

// StoreName is the registered name of the Redis store
const StoreName = "redis"

// redisClient defines the interface for Redis operations
// This allows for easier mocking in tests
type redisClient interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	Ping(ctx context.Context) *redis.StatusCmd
	Close() error
}

// Factory function for creating Redis clients
// Can be replaced during tests for mocking
var newRedisClientFn = func(addr string, password string, db int) redisClientMock {
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

// Register the Redis store with the lockservice package
func init() {
	lockservice.Register(StoreName, newStore)
}

// newStore creates a new Redis store instance from configuration
func newStore(ctx context.Context, options lockservice.Config, logger *observability.SLogger) (store.LockStore, error) {
	cfg, ok := options.(*RedisConfig)
	if !ok && options != nil {
		return nil, &store.InvalidConfigurationError{Store: StoreName, Config: options}
	}
	return New(ctx, cfg, logger)
}

// Store implements the store.Store interface for Redis
type Store struct {
	client    redisClientMock
	ttl       int32
	l         *observability.SLogger
	keyPrefix string
	config    *RedisConfig
}

// GetConfig returns the current store configuration
func (s *Store) GetConfig() store.StoreConfig {
	return s.config
}

// New creates a new Redis store with the provided configuration
func New(ctx context.Context, config *RedisConfig, logger *observability.SLogger) (*Store, error) {
	if config == nil {
		return nil, ErrConfigOptionMissing
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	client := newRedisClientFn(addr, config.Password, config.DB)

	// Test connection
	if _, err := client.Ping(ctx).Result(); err != nil {
		logger.Errorf("Error connecting to Redis: %v", err)
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Store{
		client:    client,
		ttl:       config.TTL,
		l:         logger,
		keyPrefix: config.KeyPrefix,
		config:    config,
	}, nil
}

// getLockKey generates a consistent key for a lock
func (s *Store) getLockKey(service, domain string) string {
	return fmt.Sprintf("%s:%s:%s", s.keyPrefix, service, domain)
}

// TryAcquireLock attempts to acquire a lock for the given service and domain
func (s *Store) TryAcquireLock(ctx context.Context, service, domain, clientId string, ttl int32) bool {
	key := s.getLockKey(service, domain)
	lockTTL := s.resolveTTL(ttl)

	// Try to acquire lock with SetNX
	success, err := s.client.SetNX(ctx, key, clientId, time.Duration(lockTTL)*time.Second).Result()
	if err != nil {
		s.l.Errorf("Error acquiring lock: %v", err)
		return false
	}

	if success {
		return true
	}

	// If acquisition failed, check if this client already owns the lock
	return s.refreshLockIfOwned(ctx, key, clientId, lockTTL)
}

// refreshLockIfOwned checks if the client owns the lock and refreshes it if so
func (s *Store) refreshLockIfOwned(ctx context.Context, key, clientId string, ttl int32) bool {
	existingOwner, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			s.l.Errorf("Error checking lock ownership: %v", err)
		}
		return false
	}

	if existingOwner != clientId {
		return false
	}

	// Client owns the lock, refresh TTL
	err = s.client.Expire(ctx, key, time.Duration(ttl)*time.Second).Err()
	if err != nil {
		s.l.Errorf("Error refreshing lock TTL: %v", err)
		return false
	}

	return true
}

// ReleaseLock releases a lock if it's owned by the specified client
func (s *Store) ReleaseLock(ctx context.Context, service, domain, clientId string) {
	key := s.getLockKey(service, domain)

	// Check if this client owns the lock
	existingOwner, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			s.l.Errorf("Error checking lock ownership: %v", err)
		}
		return
	}

	// Only delete if client owns the lock
	if existingOwner == clientId {
		if _, err = s.client.Del(ctx, key).Result(); err != nil {
			s.l.Errorf("Error releasing lock: %v", err)
		}
	}
}

// KeepAlive refreshes a lock's TTL if it's owned by the specified client
func (s *Store) KeepAlive(ctx context.Context, service, domain, clientId string, ttl int32) time.Duration {
	key := s.getLockKey(service, domain)
	lockTTL := s.resolveTTL(ttl)

	// Check if this client owns the lock
	existingOwner, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			s.l.Errorf("Error checking lock for keep-alive: %v", err)
		}
		return time.Duration(-1) * time.Second
	}

	if existingOwner != clientId {
		return time.Duration(-1) * time.Second
	}

	// Refresh TTL
	success, err := s.client.Expire(ctx, key, time.Duration(lockTTL)*time.Second).Result()
	if err != nil {
		s.l.Errorf("Error refreshing lock TTL: %v", err)
		return time.Duration(-1) * time.Second
	}

	if !success {
		s.l.Errorf("Failed to refresh lock TTL: key no longer exists")
		return time.Duration(-1) * time.Second
	}

	return time.Duration(lockTTL) * time.Second
}

// resolveTTL returns the provided TTL or falls back to the default
func (s *Store) resolveTTL(ttl int32) int32 {
	if ttl > 0 {
		return ttl
	}
	return s.ttl
}

// Close closes the Redis client connection
func (s *Store) Close() {
	if err := s.client.Close(); err != nil {
		s.l.Errorf("Error closing Redis connection: %v", err)
	}
}
