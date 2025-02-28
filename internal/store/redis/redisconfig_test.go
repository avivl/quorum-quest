// internal/store/redis/redisconfig_test.go
package redis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRedisConfig(t *testing.T) {
	config := NewRedisConfig()
	assert.NotNil(t, config)
	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 6379, config.Port)
	assert.Equal(t, "", config.Password)
	assert.Equal(t, 0, config.DB)
	assert.Equal(t, int32(15), config.TTL)
	assert.Empty(t, config.Replicas)
}

func TestRedisConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    *RedisConfig
		wantError bool
		errorMsg  string
	}{
		{
			name:      "Valid config",
			config:    NewRedisConfig(),
			wantError: false,
		},
		{
			name: "Empty host",
			config: &RedisConfig{
				Host:     "",
				Port:     6379,
				TTL:      15,
				DB:       0,
				Replicas: []string{},
			},
			wantError: true,
			errorMsg:  "host is required",
		},
		{
			name: "Invalid port (too low)",
			config: &RedisConfig{
				Host:     "localhost",
				Port:     0,
				TTL:      15,
				DB:       0,
				Replicas: []string{},
			},
			wantError: true,
			errorMsg:  "port must be between 1 and 65535",
		},
		{
			name: "Invalid port (too high)",
			config: &RedisConfig{
				Host:     "localhost",
				Port:     65536,
				TTL:      15,
				DB:       0,
				Replicas: []string{},
			},
			wantError: true,
			errorMsg:  "port must be between 1 and 65535",
		},
		{
			name: "Invalid TTL",
			config: &RedisConfig{
				Host:     "localhost",
				Port:     6379,
				TTL:      0,
				DB:       0,
				Replicas: []string{},
			},
			wantError: true,
			errorMsg:  "TTL must be positive",
		},
		{
			name: "Invalid DB number",
			config: &RedisConfig{
				Host:     "localhost",
				Port:     6379,
				TTL:      15,
				DB:       -1,
				Replicas: []string{},
			},
			wantError: true,
			errorMsg:  "DB number must be non-negative",
		},
		{
			name: "Empty replica address",
			config: &RedisConfig{
				Host:     "localhost",
				Port:     6379,
				TTL:      15,
				DB:       0,
				Replicas: []string{""},
			},
			wantError: true,
			errorMsg:  "replica 0: address cannot be empty",
		},
		{
			name: "Multiple validation errors",
			config: &RedisConfig{
				Host:     "",
				Port:     0,
				TTL:      0,
				DB:       -1,
				Replicas: []string{""},
			},
			wantError: true,
			errorMsg:  "store validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRedisConfig_String(t *testing.T) {
	tests := []struct {
		name     string
		config   *RedisConfig
		expected string
	}{
		{
			name:     "Default config",
			config:   NewRedisConfig(),
			expected: "RedisConfig{Host: localhost, Port: 6379, DB: 0, TTL: 15, Replicas: []}",
		},
		{
			name: "Config with replicas",
			config: &RedisConfig{
				Host:     "redis.example.com",
				Port:     6380,
				DB:       1,
				TTL:      30,
				Replicas: []string{"replica1:6379", "replica2:6379"},
			},
			expected: "RedisConfig{Host: redis.example.com, Port: 6380, DB: 1, TTL: 30, Replicas: [replica1:6379 replica2:6379]}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.String())
		})
	}
}

func TestRedisConfig_Clone(t *testing.T) {
	original := &RedisConfig{
		Host:     "redis.example.com",
		Port:     6380,
		Password: "secret",
		DB:       1,
		TTL:      30,
		Replicas: []string{"replica1:6379", "replica2:6379"},
	}

	clone := original.Clone()

	// Verify all fields are equal
	assert.Equal(t, original.Host, clone.Host)
	assert.Equal(t, original.Port, clone.Port)
	assert.Equal(t, original.Password, clone.Password)
	assert.Equal(t, original.DB, clone.DB)
	assert.Equal(t, original.TTL, clone.TTL)
	assert.Equal(t, original.Replicas, clone.Replicas)

	// Verify it's a deep copy by modifying the clone
	clone.Host = "modified"
	clone.Replicas[0] = "modified"
	assert.NotEqual(t, original.Host, clone.Host)
	assert.NotEqual(t, original.Replicas[0], clone.Replicas[0])
}

func TestRedisConfig_StoreConfigInterface(t *testing.T) {
	config := NewRedisConfig()

	// Test GetTableName
	tableName := config.GetTableName()
	assert.Equal(t, "redis-store", tableName)

	// Test GetTTL
	ttl := config.GetTTL()
	assert.Equal(t, int32(15), ttl)

	// Test GetEndpoints
	endpoints := config.GetEndpoints()
	assert.Len(t, endpoints, 1)
	assert.Equal(t, "localhost:6379", endpoints[0])

	// Test with replicas
	config.Replicas = []string{"replica1:6379", "replica2:6379"}
	endpointsWithReplicas := config.GetEndpoints()
	assert.Len(t, endpointsWithReplicas, 3)
	assert.Equal(t, "localhost:6379", endpointsWithReplicas[0])
	assert.Equal(t, "replica1:6379", endpointsWithReplicas[1])
	assert.Equal(t, "replica2:6379", endpointsWithReplicas[2])
}
