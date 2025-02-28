// internal/store/redis/redis_config.go
package redis

import (
	"github.com/avivl/quorum-quest/internal/store"
)

// RedisConfig defines the configuration for the Redis store.
type RedisConfig struct {
	Host      string   `json:"host"`
	Port      int      `json:"port"`
	Password  string   `json:"password"`
	DB        int      `json:"db"`
	TTL       int32    `json:"ttl"`
	KeyPrefix string   `json:"key_prefix"`
	Endpoints []string `json:"endpoints"`
	TableName string   `json:"table_name"` // For compatibility with StoreConfig interface
}

// Validate ensures the RedisConfig is valid.
func (c *RedisConfig) Validate() error {
	if c.Host == "" {
		c.Host = "localhost"
	}
	if c.Port == 0 {
		c.Port = 6379
	}
	if c.TTL == 0 {
		c.TTL = 15 // Default TTL in seconds
	}
	if c.KeyPrefix == "" {
		c.KeyPrefix = "lock"
	}
	if c.TableName == "" {
		c.TableName = "locks" // For StoreConfig compatibility
	}
	return nil
}

// GetTableName returns the table name (for StoreConfig compatibility).
func (c *RedisConfig) GetTableName() string {
	return c.TableName
}

// GetTTL returns the TTL value.
func (c *RedisConfig) GetTTL() int32 {
	return c.TTL
}

// GetEndpoints returns the list of endpoints.
func (c *RedisConfig) GetEndpoints() []string {
	if len(c.Endpoints) == 0 && c.Host != "" {
		// If no endpoints are explicitly set but we have a host,
		// return the host as the single endpoint
		return []string{c.Host}
	}
	return c.Endpoints
}

// Clone creates a copy of the configuration.
func (c *RedisConfig) Clone() store.StoreConfig {
	return &RedisConfig{
		Host:      c.Host,
		Port:      c.Port,
		Password:  c.Password,
		DB:        c.DB,
		TTL:       c.TTL,
		KeyPrefix: c.KeyPrefix,
		Endpoints: append([]string{}, c.Endpoints...),
		TableName: c.TableName,
	}
}

// GetType returns the type of the store.
func (c *RedisConfig) GetType() string {
	return StoreName
}

// NewConfig creates a new RedisConfig with default values.
func NewConfig() *RedisConfig {
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
	return config
}
