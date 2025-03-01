// internal/store/scylladb/newstore_test.go
package scylladb

import (
	"context"
	"testing"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
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
		assert.Contains(t, err.Error(), "ScyllaDB only supports one endpoint")
	})
}

func TestParseConsistency(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"quorum", "CONSISTENCY_QUORUM", 4}, // gocql.Quorum = 4
		{"one", "CONSISTENCY_ONE", 1},       // gocql.One = 1
		{"all", "CONSISTENCY_ALL", 5},       // gocql.All = 5
		{"default", "UNKNOWN", 4},           // Default = gocql.Quorum = 4
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parseConsistency(tc.input)
			assert.Equal(t, tc.expected, int(result))
		})
	}
}

func TestClose(t *testing.T) {
	t.Run("closes_session", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()

		// Setup expectations
		mockSession.On("Close").Return()

		// Call method under test
		store.Close()

		// Verify expectations
		mockSession.AssertExpectations(t)
	})
}

func TestGetConfig(t *testing.T) {
	t.Run("returns_config", func(t *testing.T) {
		// Create a config
		config := &ScyllaDBConfig{
			Host:        "localhost",
			Port:        9042,
			Keyspace:    "test_keyspace",
			Table:       "test_table",
			TTL:         15,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"localhost:9042"},
		}

		// Create a mock store
		mockSession := new(MockSession)
		logger, _, _ := observability.NewTestLogger()

		store := &MockStore{
			session: mockSession,
			config:  config,
			l:       logger,
		}

		// Call method under test
		result := store.GetConfig()

		// Verify the result is the same config
		assert.Equal(t, config, result)
	})
}

// Test compliance with the Store interface
func TestStoreInterfaceCompliance(t *testing.T) {
	t.Run("implements_store_interface", func(t *testing.T) {
		// Create a store instance
		s, _ := SetupStoreWithMocks()

		// Check if it implements the store.Store interface
		var i store.Store = s
		assert.NotNil(t, i)
	})
}
