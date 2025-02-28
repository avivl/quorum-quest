// internal/store/redis/redis.go
package redis

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/avivl/quorum-quest/internal/store"
	"github.com/redis/go-redis/v9"
)

// RedisConfig holds Redis-specific configuration

// RedisStore implements the Store interface using Redis
type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
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

// Close closes the Redis connection
func (s *RedisStore) Close() error {
	return s.client.Close()
}
