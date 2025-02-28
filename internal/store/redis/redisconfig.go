// internal/store/redis/redisconfig.go

package redis

import (
	"errors"
	"fmt"
	"strings"
)

// RedisConfig holds Redis-specific configuration
type RedisConfig struct {
	Host     string   `yaml:"host"`
	Port     int      `yaml:"port"`
	Password string   `yaml:"password"`
	DB       int      `yaml:"db"`
	TTL      int32    `yaml:"ttl"`
	Replicas []string `yaml:"replicas"`
}

// NewRedisConfig creates a new Redis configuration with default values
func NewRedisConfig() *RedisConfig {
	return &RedisConfig{
		Host:     "localhost",
		Port:     6379,
		Password: "",
		DB:       0,
		TTL:      15,
		Replicas: []string{},
	}
}

// Validate ensures the Redis configuration is valid
func (c *RedisConfig) Validate() error {
	var errs []string

	if c.Host == "" {
		errs = append(errs, "host is required")
	}

	if c.Port <= 0 || c.Port > 65535 {
		errs = append(errs, "port must be between 1 and 65535")
	}

	if c.TTL <= 0 {
		errs = append(errs, "TTL must be positive")
	}

	if c.DB < 0 {
		errs = append(errs, "DB number must be non-negative")
	}

	// Validate replicas if provided
	for i, replica := range c.Replicas {
		if replica == "" {
			errs = append(errs, fmt.Sprintf("replica %d: address cannot be empty", i))
		}
	}

	if len(errs) > 0 {
		return errors.New("store validation failed: " + strings.Join(errs, "; "))
	}

	return nil
}

// String returns a string representation of the Redis configuration
func (c *RedisConfig) String() string {
	replicasStr := "[]"
	if len(c.Replicas) > 0 {
		replicasStr = fmt.Sprintf("%v", c.Replicas)
	}

	return fmt.Sprintf(
		"RedisConfig{Host: %s, Port: %d, DB: %d, TTL: %d, Replicas: %s}",
		c.Host,
		c.Port,
		c.DB,
		c.TTL,
		replicasStr,
	)
}

// Clone creates a deep copy of the Redis configuration
func (c *RedisConfig) Clone() *RedisConfig {
	replicas := make([]string, len(c.Replicas))
	copy(replicas, c.Replicas)

	return &RedisConfig{
		Host:     c.Host,
		Port:     c.Port,
		Password: c.Password,
		DB:       c.DB,
		TTL:      c.TTL,
		Replicas: replicas,
	}
}

// GetTableName returns a placeholder table name since Redis doesn't use tables
// Implementing this method to satisfy the StoreConfig interface
func (c *RedisConfig) GetTableName() string {
	return "redis-store" // This is just a placeholder
}

// GetTTL returns the configured TTL
func (c *RedisConfig) GetTTL() int32 {
	return c.TTL
}

// GetEndpoints returns a list of Redis endpoints
func (c *RedisConfig) GetEndpoints() []string {
	endpoints := make([]string, 0, len(c.Replicas)+1)
	endpoints = append(endpoints, fmt.Sprintf("%s:%d", c.Host, c.Port))
	endpoints = append(endpoints, c.Replicas...)
	return endpoints
}
