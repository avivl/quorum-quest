// internal/store/scylladb/scylladbconfig_test.go
package scylladb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScyllaDBConfig(t *testing.T) {
	t.Run("GetTableName", func(t *testing.T) {
		cfg := &ScyllaDBConfig{Table: "test-table"}
		assert.Equal(t, "test-table", cfg.GetTableName())
	})

	t.Run("GetTTL", func(t *testing.T) {
		cfg := &ScyllaDBConfig{TTL: 30}
		assert.Equal(t, int32(30), cfg.GetTTL())
	})

	t.Run("GetEndpoints", func(t *testing.T) {
		endpoints := []string{"endpoint1", "endpoint2"}
		cfg := &ScyllaDBConfig{Endpoints: endpoints}
		assert.Equal(t, endpoints, cfg.GetEndpoints())
	})

	t.Run("Validate_Success", func(t *testing.T) {
		cfg := &ScyllaDBConfig{
			Host:        "localhost",
			Port:        9042,
			Keyspace:    "test-keyspace",
			Table:       "test-table",
			TTL:         30,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"endpoint1"},
		}
		assert.NoError(t, cfg.Validate())
	})

	t.Run("Validate_Missing_Host", func(t *testing.T) {
		cfg := &ScyllaDBConfig{
			Port:        9042,
			Keyspace:    "test-keyspace",
			Table:       "test-table",
			TTL:         30,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"endpoint1"},
		}
		assert.Error(t, cfg.Validate())
	})

	t.Run("Validate_Invalid_Port", func(t *testing.T) {
		cfg := &ScyllaDBConfig{
			Host:        "localhost",
			Port:        0,
			Keyspace:    "test-keyspace",
			Table:       "test-table",
			TTL:         30,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"endpoint1"},
		}
		assert.Error(t, cfg.Validate())
	})

	t.Run("Validate_Missing_Keyspace", func(t *testing.T) {
		cfg := &ScyllaDBConfig{
			Host:        "localhost",
			Port:        9042,
			Table:       "test-table",
			TTL:         30,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"endpoint1"},
		}
		assert.Error(t, cfg.Validate())
	})

	t.Run("Validate_Missing_Table", func(t *testing.T) {
		cfg := &ScyllaDBConfig{
			Host:        "localhost",
			Port:        9042,
			Keyspace:    "test-keyspace",
			TTL:         30,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"endpoint1"},
		}
		assert.Error(t, cfg.Validate())
	})

	t.Run("Validate_Invalid_TTL", func(t *testing.T) {
		cfg := &ScyllaDBConfig{
			Host:        "localhost",
			Port:        9042,
			Keyspace:    "test-keyspace",
			Table:       "test-table",
			TTL:         0,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"endpoint1"},
		}
		assert.Error(t, cfg.Validate())
	})

	t.Run("Validate_Missing_Consistency", func(t *testing.T) {
		cfg := &ScyllaDBConfig{
			Host:      "localhost",
			Port:      9042,
			Keyspace:  "test-keyspace",
			Table:     "test-table",
			TTL:       30,
			Endpoints: []string{"endpoint1"},
		}
		assert.Error(t, cfg.Validate())
	})

	t.Run("Validate_No_Endpoints", func(t *testing.T) {
		cfg := &ScyllaDBConfig{
			Host:        "localhost",
			Port:        9042,
			Keyspace:    "test-keyspace",
			Table:       "test-table",
			TTL:         30,
			Consistency: "CONSISTENCY_QUORUM",
		}
		assert.Error(t, cfg.Validate())
	})

	t.Run("NewScyllaDBConfig", func(t *testing.T) {
		cfg := NewScyllaDBConfig()
		assert.Equal(t, "127.0.0.1", cfg.Host)
		assert.Equal(t, int32(9042), cfg.Port)
		assert.Equal(t, "quorum-quest", cfg.Keyspace)
		assert.Equal(t, "services", cfg.Table)
		assert.Equal(t, int32(15), cfg.TTL)
		assert.Equal(t, "CONSISTENCY_QUORUM", cfg.Consistency)
		assert.Equal(t, []string{"localhost:9042"}, cfg.Endpoints)
	})
}
