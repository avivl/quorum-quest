// internal/store/scylladb/scylladb_test.go
package scylladb

import (
	"context"
	"testing"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	logger, _, _ := observability.NewTestLogger()

	t.Run("success", func(t *testing.T) {
		// Skip the actual connection test if no ScyllaDB available
		t.Skip("Skipping test that requires ScyllaDB connection")

		config := &ScyllaDBConfig{
			Host:        "localhost",
			Port:        9042,
			Keyspace:    "test_keyspace",
			Table:       "test_table",
			TTL:         15,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"localhost:9042"},
		}

		store, err := New(context.Background(), config, logger)
		require.NoError(t, err)
		require.NotNil(t, store)

		// Check config is stored correctly
		assert.Equal(t, config, store.config)

		// Clean up
		store.Close()
	})

	t.Run("nil_config", func(t *testing.T) {
		store, err := New(context.Background(), nil, logger)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Equal(t, ErrConfigOptionMissing, err)
		assert.Contains(t, err.Error(), "ScyllaDB requires a config option")
	})

	t.Run("multiple_endpoints", func(t *testing.T) {
		config := &ScyllaDBConfig{
			Host:        "localhost",
			Port:        9042,
			Keyspace:    "test_keyspace",
			Table:       "test_table",
			TTL:         15,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"endpoint1", "endpoint2"},
		}

		store, err := New(context.Background(), config, logger)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Equal(t, ErrMultipleEndpointsUnsupported, err)
	})
}

func TestParseConsistency(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected gocql.Consistency
	}{
		{"quorum", "CONSISTENCY_QUORUM", gocql.Quorum},
		{"one", "CONSISTENCY_ONE", gocql.One},
		{"all", "CONSISTENCY_ALL", gocql.All},
		{"default", "UNKNOWN", gocql.Quorum},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parseConsistency(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestReleaseLock(t *testing.T) {
	// Remove the unused logger variable
	t.Run("success_case", func(t *testing.T) {
		// Skip this test until we can properly mock gocql.Session
		t.Skip("Skipping test that requires refactoring for proper mocking")
	})
}

// Helper function to check if a string contains a substring
