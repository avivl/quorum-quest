// internal/store/redis/redis.go
package redis

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/avivl/quorum-quest/internal/lockservice"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/redis/go-redis/v9"
)

// StoreName is the name of the store
const StoreName string = "redis"

// RedisStore implements both the Store and LockStore interfaces using Redis
type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
	config *RedisConfig
	logger *observability.SLogger
}

// init registers the Redis store with the lockservice package
func init() {
	// Register the Redis store with the lockservice package
	lockservice.Register(StoreName, newStore)
}

// newStore creates a new Redis store for lockservice
func newStore(ctx context.Context, options lockservice.Config, logger *observability.SLogger) (store.LockStore, error) {
	cfg, ok := options.(*RedisConfig)
	if !ok && options != nil {
		return nil, &store.InvalidConfigurationError{Store: StoreName, Config: options}
	}

	redisStore, err := NewRedisStore(cfg)
	if err != nil {
		return nil, err
	}

	// Set the logger
	redisStore.logger = logger

	return redisStore, nil
}

// NewRedisStore creates a new Redis store instance
func NewRedisStore(cfg *RedisConfig) (*RedisStore, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid Redis configuration: %w", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStore{
		client: client,
		ttl:    time.Duration(cfg.TTL) * time.Second,
		config: cfg,
	}, nil
}

// Get retrieves a service record by ID
func (s *RedisStore) Get(ctx context.Context, id string) (*store.ServiceRecord, error) {
	data, err := s.client.Get(ctx, id).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get service record: %w", err)
	}

	record := &store.ServiceRecord{}
	if err := record.UnmarshalJSON([]byte(data)); err != nil {
		return nil, fmt.Errorf("failed to unmarshal service record: %w", err)
	}

	return record, nil
}

// Put stores a service record
func (s *RedisStore) Put(ctx context.Context, record *store.ServiceRecord) error {
	if record == nil {
		return errors.New("record cannot be nil")
	}

	// Validate record fields
	if record.ID == "" {
		return errors.New("record ID cannot be empty")
	}

	// Validate that the record can be properly marshaled (catches invalid UTF-8)
	if !utf8.ValidString(record.ID) || !utf8.ValidString(record.Name) || !utf8.ValidString(record.Status) {
		return errors.New("record contains invalid UTF-8 characters")
	}

	data, err := record.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal service record: %w", err)
	}

	if err := s.client.Set(ctx, record.ID, data, s.ttl).Err(); err != nil {
		return fmt.Errorf("failed to store service record: %w", err)
	}

	return nil
}

// Delete removes a service record by ID
func (s *RedisStore) Delete(ctx context.Context, id string) error {
	result, err := s.client.Del(ctx, id).Result()
	if err != nil {
		return fmt.Errorf("failed to delete service record: %w", err)
	}

	if result == 0 {
		return store.ErrNotFound
	}

	return nil
}

// GetConfig returns the current store configuration
func (s *RedisStore) GetConfig() store.StoreConfig {
	return s.config
}

// TryAcquireLock attempts to acquire a lock using Redis
// Uses SET with NX option (only set if key doesn't exist)
func (s *RedisStore) TryAcquireLock(ctx context.Context, service, domain, clientID string, ttl int32) bool {
	lockKey := s.getLockKey(service, domain)
	lockTTL := s.getLockTTL(ttl)

	// Try to acquire lock with NX option (only set if not exists)
	success, err := s.client.SetNX(ctx, lockKey, clientID, lockTTL).Result()
	if err != nil {
		if s.logger != nil {
			s.logger.Errorf("Error acquiring lock: %v", err)
		}
		return false
	}

	return success
}

// ReleaseLock releases a previously acquired lock
// Only release if the current client is the lock owner
func (s *RedisStore) ReleaseLock(ctx context.Context, service, domain, clientID string) {
	lockKey := s.getLockKey(service, domain)

	// Get the lock value to verify ownership
	storedClientID, err := s.client.Get(ctx, lockKey).Result()
	if err != nil {
		if err != redis.Nil && s.logger != nil {
			s.logger.Errorf("Error validating lock ownership: %v", err)
		}
		return
	}

	// Only release if this client owns the lock
	if storedClientID == clientID {
		err = s.client.Del(ctx, lockKey).Err()
		if err != nil && s.logger != nil {
			s.logger.Errorf("Error releasing lock: %v", err)
		}
	}
}

// KeepAlive refreshes the TTL of an existing lock
func (s *RedisStore) KeepAlive(ctx context.Context, service, domain, clientID string, ttl int32) time.Duration {
	lockKey := s.getLockKey(service, domain)
	lockTTL := s.getLockTTL(ttl)

	// First verify ownership
	storedClientID, err := s.client.Get(ctx, lockKey).Result()
	if err != nil {
		if err != redis.Nil && s.logger != nil {
			s.logger.Errorf("Error in KeepAlive when getting lock: %v", err)
		}
		return -1 * time.Second
	}

	// Only refresh if this client owns the lock
	if storedClientID != clientID {
		return -1 * time.Second
	}

	// Extend the lock TTL
	success, err := s.client.Expire(ctx, lockKey, lockTTL).Result()
	if err != nil {
		if s.logger != nil {
			s.logger.Errorf("Error in KeepAlive when extending lock: %v", err)
		}
		return -1 * time.Second
	}

	if !success {
		// Lock doesn't exist anymore
		return -1 * time.Second
	}

	return lockTTL
}

func (s *RedisStore) Close() {
	// Ignore the error from Close as per the LockStore interface requirement
	_ = s.client.Close()
}

// Helper functions

// getLockKey constructs the Redis key for a lock
func (s *RedisStore) getLockKey(service, domain string) string {
	return fmt.Sprintf("lock:%s:%s", service, domain)
}

// getLockTTL returns the TTL to use for a lock
func (s *RedisStore) getLockTTL(ttl int32) time.Duration {
	if ttl > 0 {
		return time.Duration(ttl) * time.Second
	}
	return s.ttl
}
