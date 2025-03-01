// internal/store/redis/redis_store.go
package redis

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/avivl/quorum-quest/internal/lockservice"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/redis/go-redis/v9"
)

var (
	// ErrConfigOptionMissing is returned when required configuration option is missing.
	ErrConfigOptionMissing = errors.New("Redis requires a config option")
)

// StoreName the name of the store.
const StoreName string = "redis"

// Define the interface for the Redis client
type redisClient interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	Ping(ctx context.Context) *redis.StatusCmd
	Close() error
}

// Variable to hold the function for creating Redis clients, making it possible
// to replace it during tests
var newRedisClientFn = func(addr string, password string, db int) redisClientMock {
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

// init registers the Redis store with the lockservice package.
func init() {
	// Register the Redis store with the lockservice package using the StoreName and newStore function.
	lockservice.Register(StoreName, newStore)
}

// newStore creates a new Redis store instance based on the provided context, endpoints, and configuration options.
// It returns a store.Store interface and an error if any.
// If the configuration options are missing or invalid, it returns ErrConfigOptionMissing.
func newStore(ctx context.Context, options lockservice.Config, logger *observability.SLogger) (store.LockStore, error) {
	cfg, ok := options.(*RedisConfig)
	if !ok && options != nil {
		return nil, &store.InvalidConfigurationError{Store: StoreName, Config: options}
	}
	return New(ctx, cfg, logger)
}

// Store implements the store.Store interface.
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

// New creates a new Redis client.
func New(ctx context.Context, config *RedisConfig, logger *observability.SLogger) (*Store, error) {
	// Check for nil config first, before any access to config fields
	if config == nil {
		return nil, ErrConfigOptionMissing
	}

	// Apply default values if needed
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Create Redis client with the provided configuration
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	client := newRedisClientFn(addr, config.Password, config.DB)

	// Test connection
	_, err := client.Ping(ctx).Result()
	if err != nil {
		logger.Errorf("Error connecting to Redis: %v", err)
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	rdb := &Store{
		client:    client,
		ttl:       int32(math.Round((time.Duration(config.TTL).Seconds() * 1000000000))),
		l:         logger,
		keyPrefix: config.KeyPrefix,
		config:    config,
	}

	return rdb, nil
}

// getLockKey generates a consistent key for a lock
func (rdb *Store) getLockKey(service, domain string) string {
	return fmt.Sprintf("%s:%s:%s", rdb.keyPrefix, service, domain)
}

// TryAcquireLock attempts to acquire a lock for the given service and domain.
// It returns true if successful, false otherwise.
func (rdb *Store) TryAcquireLock(ctx context.Context, service, domain, clientId string, ttl int32) bool {
	key := rdb.getLockKey(service, domain)
	_ttl := ttl
	if ttl == 0 {
		_ttl = rdb.ttl
	}

	// Use Redis SET with NX option to only set if key doesn't exist
	success, err := rdb.client.SetNX(ctx, key, clientId, time.Duration(_ttl)*time.Second).Result()
	if err != nil {
		rdb.l.Errorf("Error acquiring lock: %v", err)
		return false
	}

	// If SET NX succeeded, the lock was acquired
	if success {
		return true
	}

	// If SET NX failed, check if this client already owns the lock
	existingOwner, err := rdb.client.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			rdb.l.Errorf("Error checking lock ownership: %v", err)
		}
		return false
	}

	// If this client already owns the lock, refresh the TTL and return success
	if existingOwner == clientId {
		err = rdb.client.Expire(ctx, key, time.Duration(_ttl)*time.Second).Err()
		if err != nil {
			rdb.l.Errorf("Error refreshing lock TTL: %v", err)
			return false
		}
		return true
	}

	return false
}

// ReleaseLock releases a lock if it's owned by the specified client.
func (rdb *Store) ReleaseLock(ctx context.Context, service, domain, clientId string) {
	key := rdb.getLockKey(service, domain)

	// First validate that this client owns the lock
	existingOwner, err := rdb.client.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			rdb.l.Errorf("Error checking lock ownership: %v", err)
		}
		return
	}

	// Only delete the key if this client owns the lock
	if existingOwner == clientId {
		_, err = rdb.client.Del(ctx, key).Result()
		if err != nil {
			rdb.l.Errorf("Error releasing lock: %v", err)
		}
	}
}

// KeepAlive refreshes a lock's TTL if it's owned by the specified client.
// It returns the new TTL duration if successful, or -1 second if unsuccessful.
func (rdb *Store) KeepAlive(ctx context.Context, service, domain, clientId string, ttl int32) time.Duration {
	key := rdb.getLockKey(service, domain)
	_ttl := ttl
	if ttl == 0 {
		_ttl = rdb.ttl
	}

	// Check if this client owns the lock
	existingOwner, err := rdb.client.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			rdb.l.Errorf("Error checking lock for keep-alive: %v", err)
		}
		return time.Duration(-1) * time.Second
	}

	// Only refresh the TTL if this client owns the lock
	if existingOwner == clientId {
		success, err := rdb.client.Expire(ctx, key, time.Duration(_ttl)*time.Second).Result()
		if err != nil {
			rdb.l.Errorf("Error refreshing lock TTL: %v", err)
			return time.Duration(-1) * time.Second
		}

		if !success {
			rdb.l.Errorf("Failed to refresh lock TTL: key no longer exists")
			return time.Duration(-1) * time.Second
		}

		return time.Duration(_ttl) * time.Second
	}

	return time.Duration(-1) * time.Second
}

// Close closes the Redis client connection.
func (rdb *Store) Close() {
	err := rdb.client.Close()
	if err != nil {
		rdb.l.Errorf("Error closing Redis connection: %v", err)
	}
}
