// internal/store/redis/redisconfig_test.go
package redis

import (
	"testing"

	"github.com/avivl/quorum-quest/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestRedisConfig(t *testing.T) {
	t.Run("GetTableName", func(t *testing.T) {
		cfg := &RedisConfig{TableName: "test-table"}
		assert.Equal(t, "test-table", cfg.GetTableName())
	})

	t.Run("GetTTL", func(t *testing.T) {
		cfg := &RedisConfig{TTL: 30}
		assert.Equal(t, int32(30), cfg.GetTTL())
	})

	t.Run("GetEndpoints_Explicit", func(t *testing.T) {
		endpoints := []string{"endpoint1", "endpoint2"}
		cfg := &RedisConfig{Endpoints: endpoints}
		assert.Equal(t, endpoints, cfg.GetEndpoints())
	})

	t.Run("GetEndpoints_FromHost", func(t *testing.T) {
		cfg := &RedisConfig{Host: "redis-server", Endpoints: []string{}}
		assert.Equal(t, []string{"redis-server"}, cfg.GetEndpoints())
	})

	t.Run("GetEndpoints_Empty", func(t *testing.T) {
		cfg := &RedisConfig{Host: "", Endpoints: []string{}}
		assert.Empty(t, cfg.GetEndpoints())
	})

	t.Run("GetType", func(t *testing.T) {
		cfg := &RedisConfig{}
		assert.Equal(t, StoreName, cfg.GetType())
	})

	t.Run("Validate_DefaultValues", func(t *testing.T) {
		cfg := &RedisConfig{}
		err := cfg.Validate()
		assert.NoError(t, err)

		// Check that defaults were applied
		assert.Equal(t, "localhost", cfg.Host)
		assert.Equal(t, 6379, cfg.Port)
		assert.Equal(t, int32(15), cfg.TTL)
		assert.Equal(t, "lock", cfg.KeyPrefix)
		assert.Equal(t, "locks", cfg.TableName)
	})

	t.Run("Validate_CustomValues", func(t *testing.T) {
		cfg := &RedisConfig{
			Host:      "redis-server",
			Port:      6380,
			Password:  "secret",
			DB:        1,
			TTL:       30,
			KeyPrefix: "custom-lock",
			TableName: "custom-locks",
		}
		err := cfg.Validate()
		assert.NoError(t, err)

		// Check that values were preserved
		assert.Equal(t, "redis-server", cfg.Host)
		assert.Equal(t, 6380, cfg.Port)
		assert.Equal(t, "secret", cfg.Password)
		assert.Equal(t, 1, cfg.DB)
		assert.Equal(t, int32(30), cfg.TTL)
		assert.Equal(t, "custom-lock", cfg.KeyPrefix)
		assert.Equal(t, "custom-locks", cfg.TableName)
	})

	t.Run("Clone", func(t *testing.T) {
		original := &RedisConfig{
			Host:      "redis-server",
			Port:      6380,
			Password:  "secret",
			DB:        1,
			TTL:       30,
			KeyPrefix: "custom-lock",
			Endpoints: []string{"endpoint1", "endpoint2"},
			TableName: "custom-locks",
		}

		cloned := original.Clone()

		// Check that it's a different instance
		assert.NotSame(t, original, cloned)

		// Check values match
		clonedConfig, ok := cloned.(*RedisConfig)
		assert.True(t, ok)
		assert.Equal(t, original.Host, clonedConfig.Host)
		assert.Equal(t, original.Port, clonedConfig.Port)
		assert.Equal(t, original.Password, clonedConfig.Password)
		assert.Equal(t, original.DB, clonedConfig.DB)
		assert.Equal(t, original.TTL, clonedConfig.TTL)
		assert.Equal(t, original.KeyPrefix, clonedConfig.KeyPrefix)
		assert.Equal(t, original.TableName, clonedConfig.TableName)
		assert.Equal(t, original.Endpoints, clonedConfig.Endpoints)

		// Verify that modifying cloned doesn't affect original
		clonedConfig.Host = "different-host"
		clonedConfig.Endpoints = append(clonedConfig.Endpoints, "endpoint3")
		assert.NotEqual(t, original.Host, clonedConfig.Host)
		assert.NotEqual(t, len(original.Endpoints), len(clonedConfig.Endpoints))
	})

	t.Run("NewConfig", func(t *testing.T) {
		cfg := NewRedisConfig()
		assert.Equal(t, "localhost", cfg.Host)
		assert.Equal(t, 6379, cfg.Port)
		assert.Equal(t, "", cfg.Password)
		assert.Equal(t, 0, cfg.DB)
		assert.Equal(t, int32(15), cfg.TTL)
		assert.Equal(t, "lock", cfg.KeyPrefix)
		assert.Equal(t, "locks", cfg.TableName)
		assert.Empty(t, cfg.Endpoints)
	})

	t.Run("StoreConfig_Interface", func(t *testing.T) {
		var cfg store.StoreConfig = &RedisConfig{}
		assert.NotNil(t, cfg)

		// Test the interface methods
		_ = cfg.Validate()
		tableName := cfg.GetTableName()
		assert.Equal(t, "locks", tableName)
		ttl := cfg.GetTTL()
		assert.Equal(t, int32(15), ttl)
		endpoints := cfg.GetEndpoints()
		assert.Equal(t, []string{"localhost"}, endpoints)
	})
}
