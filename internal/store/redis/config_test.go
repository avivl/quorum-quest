// internal/store/redis/config_test.go
package redis

import (
	"testing"

	"github.com/avivl/quorum-quest/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestRedisConfiguration(t *testing.T) {
	t.Run("new_config_with_defaults", func(t *testing.T) {
		config := NewRedisConfig()

		// Verify default values
		assert.Equal(t, "localhost", config.Host)
		assert.Equal(t, 6379, config.Port)
		assert.Equal(t, "", config.Password)
		assert.Equal(t, 0, config.DB)
		assert.Equal(t, int32(15), config.TTL)
		assert.Equal(t, "lock", config.KeyPrefix)
		assert.Empty(t, config.Endpoints)
		assert.Equal(t, "locks", config.TableName)
	})

	t.Run("validate_valid_config", func(t *testing.T) {
		config := &RedisConfig{
			Host:      "redis-server",
			Port:      6379,
			TTL:       30,
			KeyPrefix: "test-lock",
		}

		err := config.Validate()
		assert.NoError(t, err)

		// Should set default values for empty fields
		assert.Equal(t, "redis-server", config.Host)
		assert.Equal(t, 6379, config.Port)
		assert.Equal(t, int32(30), config.TTL)
		assert.Equal(t, "test-lock", config.KeyPrefix)
		assert.Equal(t, "locks", config.TableName)
	})

	t.Run("validate_empty_config", func(t *testing.T) {
		config := &RedisConfig{}

		err := config.Validate()
		assert.NoError(t, err)

		// Should set default values
		assert.Equal(t, "localhost", config.Host)
		assert.Equal(t, 6379, config.Port)
		assert.Equal(t, int32(15), config.TTL)
		assert.Equal(t, "lock", config.KeyPrefix)
		assert.Equal(t, "locks", config.TableName)
	})

	t.Run("clone_creates_deep_copy", func(t *testing.T) {
		original := &RedisConfig{
			Host:      "redis-server",
			Port:      6379,
			Password:  "secret",
			DB:        1,
			TTL:       30,
			KeyPrefix: "test-lock",
			Endpoints: []string{"endpoint1", "endpoint2"},
			TableName: "test-locks",
		}

		cloned := original.Clone()

		// Verify it's the same type
		clonedConfig, ok := cloned.(*RedisConfig)
		assert.True(t, ok)

		// Verify all fields are copied
		assert.Equal(t, original.Host, clonedConfig.Host)
		assert.Equal(t, original.Port, clonedConfig.Port)
		assert.Equal(t, original.Password, clonedConfig.Password)
		assert.Equal(t, original.DB, clonedConfig.DB)
		assert.Equal(t, original.TTL, clonedConfig.TTL)
		assert.Equal(t, original.KeyPrefix, clonedConfig.KeyPrefix)
		assert.Equal(t, original.TableName, clonedConfig.TableName)

		// Check that endpoints are deep copied
		assert.Equal(t, original.Endpoints, clonedConfig.Endpoints)
		assert.NotSame(t, &original.Endpoints, &clonedConfig.Endpoints)
	})

	t.Run("get_table_name", func(t *testing.T) {
		config := &RedisConfig{
			TableName: "test-table",
		}

		assert.Equal(t, "test-table", config.GetTableName())
	})

	t.Run("get_ttl", func(t *testing.T) {
		config := &RedisConfig{
			TTL: 42,
		}

		assert.Equal(t, int32(42), config.GetTTL())
	})

	t.Run("get_endpoints", func(t *testing.T) {
		t.Run("with_explicit_endpoints", func(t *testing.T) {
			endpoints := []string{"endpoint1", "endpoint2"}
			config := &RedisConfig{
				Endpoints: endpoints,
			}

			assert.Equal(t, endpoints, config.GetEndpoints())
		})

		t.Run("with_host_only", func(t *testing.T) {
			config := &RedisConfig{
				Host:      "redis-server",
				Endpoints: []string{},
			}

			endpoints := config.GetEndpoints()
			assert.Len(t, endpoints, 1)
			assert.Equal(t, "redis-server", endpoints[0])
		})
	})

	t.Run("get_type", func(t *testing.T) {
		config := &RedisConfig{}
		assert.Equal(t, StoreName, config.GetType())
	})

	t.Run("implements_storeconfig_interface", func(t *testing.T) {
		config := NewRedisConfig()
		var i store.StoreConfig = config
		assert.NotNil(t, i)
	})
}
