// internal/store/redis/config_test.go
package redis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedisConfig(t *testing.T) {
	t.Run("validate_with_defaults", func(t *testing.T) {
		config := &RedisConfig{}
		err := config.Validate()
		assert.NoError(t, err)
		assert.Equal(t, "localhost", config.Host)
		assert.Equal(t, 6379, config.Port)
		assert.Equal(t, int32(15), config.TTL)
		assert.Equal(t, "lock", config.KeyPrefix)
		assert.Equal(t, "locks", config.TableName)
	})

	t.Run("validate_with_custom_values", func(t *testing.T) {
		config := &RedisConfig{
			Host:      "redis.example.com",
			Port:      6380,
			Password:  "password123",
			DB:        1,
			TTL:       30,
			KeyPrefix: "custom-lock",
			TableName: "custom-locks",
		}
		err := config.Validate()
		assert.NoError(t, err)
		assert.Equal(t, "redis.example.com", config.Host)
		assert.Equal(t, 6380, config.Port)
		assert.Equal(t, int32(30), config.TTL)
		assert.Equal(t, "custom-lock", config.KeyPrefix)
		assert.Equal(t, "custom-locks", config.TableName)
	})

	t.Run("get_table_name", func(t *testing.T) {
		config := &RedisConfig{TableName: "test-locks"}
		assert.Equal(t, "test-locks", config.GetTableName())
	})

	t.Run("get_ttl", func(t *testing.T) {
		config := &RedisConfig{TTL: 60}
		assert.Equal(t, int32(60), config.GetTTL())
	})

	t.Run("get_endpoints_with_host", func(t *testing.T) {
		config := &RedisConfig{Host: "redis.example.com", Endpoints: []string{}}
		endpoints := config.GetEndpoints()
		assert.Equal(t, 1, len(endpoints))
		assert.Equal(t, "redis.example.com", endpoints[0])
	})

	t.Run("get_endpoints_with_explicit_endpoints", func(t *testing.T) {
		config := &RedisConfig{
			Host:      "redis.example.com",
			Endpoints: []string{"redis1.example.com", "redis2.example.com"},
		}
		endpoints := config.GetEndpoints()
		assert.Equal(t, 2, len(endpoints))
		assert.Equal(t, "redis1.example.com", endpoints[0])
		assert.Equal(t, "redis2.example.com", endpoints[1])
	})

	t.Run("clone", func(t *testing.T) {
		original := &RedisConfig{
			Host:      "redis.example.com",
			Port:      6380,
			Password:  "password123",
			DB:        1,
			TTL:       30,
			KeyPrefix: "custom-lock",
			Endpoints: []string{"redis1.example.com", "redis2.example.com"},
			TableName: "custom-locks",
		}

		clone := original.Clone()
		clonedConfig, ok := clone.(*RedisConfig)
		assert.True(t, ok)
		assert.Equal(t, original.Host, clonedConfig.Host)
		assert.Equal(t, original.Port, clonedConfig.Port)
		assert.Equal(t, original.Password, clonedConfig.Password)
		assert.Equal(t, original.DB, clonedConfig.DB)
		assert.Equal(t, original.TTL, clonedConfig.TTL)
		assert.Equal(t, original.KeyPrefix, clonedConfig.KeyPrefix)
		assert.Equal(t, original.TableName, clonedConfig.TableName)
		assert.Equal(t, len(original.Endpoints), len(clonedConfig.Endpoints))

		// Verify that the Endpoints slice was deep-copied
		assert.Equal(t, original.Endpoints, clonedConfig.Endpoints)

		// Modify the original to verify the clone is independent
		original.Endpoints[0] = "modified.example.com"
		assert.NotEqual(t, original.Endpoints[0], clonedConfig.Endpoints[0])
	})

	t.Run("get_type", func(t *testing.T) {
		config := &RedisConfig{}
		assert.Equal(t, StoreName, config.GetType())
	})

	t.Run("new_redis_config", func(t *testing.T) {
		config := NewRedisConfig()
		assert.Equal(t, "localhost", config.Host)
		assert.Equal(t, 6379, config.Port)
		assert.Equal(t, "", config.Password)
		assert.Equal(t, 0, config.DB)
		assert.Equal(t, int32(15), config.TTL)
		assert.Equal(t, "lock", config.KeyPrefix)
		assert.Equal(t, "locks", config.TableName)
		assert.Empty(t, config.Endpoints)
	})
}
