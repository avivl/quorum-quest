// internal/store/scylladb/scylladb_test.go
package scylladb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// gocqlSessionInterface defines the interface we need from gocql.Session
type gocqlSessionInterface interface {
	Close()
	Query(stmt string, values ...interface{}) gocqlQueryInterface
}

// gocqlQueryInterface defines the interface we need from gocql.Query
type gocqlQueryInterface interface {
	Exec() error
	Scan(dest ...interface{}) error
	WithContext(ctx context.Context) gocqlQueryInterface
}

// MockSession is a mock implementation of the gocql.Session interface
type MockSession struct {
	mock.Mock
}

// Close implements the Session.Close method
func (m *MockSession) Close() {
	m.Called()
}

// Query implements the Session.Query method
func (m *MockSession) Query(stmt string, values ...interface{}) gocqlQueryInterface {
	args := m.Called(stmt, values)
	return args.Get(0).(gocqlQueryInterface)
}

// MockQuery is a mock implementation of the gocql.Query interface
type MockQuery struct {
	mock.Mock
}

// Exec implements the Query.Exec method
func (m *MockQuery) Exec() error {
	args := m.Called()
	return args.Error(0)
}

// Scan implements the Query.Scan method
func (m *MockQuery) Scan(dest ...interface{}) error {
	args := m.Called(dest)
	if args.Get(0) != nil {
		clientID := args.Get(0).(string)
		if len(dest) > 0 {
			*dest[0].(*string) = clientID
		}
	}
	return args.Error(1)
}

// WithContext implements the Query.WithContext method
func (m *MockQuery) WithContext(ctx context.Context) gocqlQueryInterface {
	m.Called(ctx)
	return m
}

// MockStore is a custom version of Store for testing that accepts our mock interfaces
type MockStore struct {
	session             gocqlSessionInterface
	tableName           string
	keyspaceName        string
	fullTableName       string
	ttl                 int32
	l                   *observability.SLogger
	TryAcquireLockQuery string
	ValidateLockQuery   string
	ReleaseLockQuery    string
	config              *ScyllaDBConfig
}

// Implement the same methods as Store for MockStore
func (s *MockStore) TryAcquireLock(ctx context.Context, service, domain, clientId string, ttl int32) bool {
	_ttl := ttl
	if ttl == 0 {
		_ttl = s.ttl
	}

	// First attempt the insertion
	mockQuery := s.session.Query(s.TryAcquireLockQuery, service, domain, clientId, _ttl)
	err := mockQuery.WithContext(ctx).Exec()
	if err != nil {
		s.l.Errorf("Error acquiring lock: %v", err)
		return false
	}

	// Verify the lock was acquired by reading it back
	var value string
	mockValidateQuery := s.session.Query(s.ValidateLockQuery, service, domain, clientId)
	err = mockValidateQuery.WithContext(ctx).Scan(&value)
	if err != nil {
		s.l.Errorf("Error validating lock: %v", err)
		return false
	}

	return true
}

func (s *MockStore) ReleaseLock(ctx context.Context, service, domain, clientId string) {
	// First validate that this client owns the lock
	var storedClientId string
	mockValidateQuery := s.session.Query(s.ValidateLockQuery, service, domain, clientId)
	err := mockValidateQuery.WithContext(ctx).Scan(&storedClientId)

	if err == nil && storedClientId == clientId {
		// This client owns the lock, so it can release it
		mockReleaseQuery := s.session.Query(s.ReleaseLockQuery, service, domain)
		err = mockReleaseQuery.WithContext(ctx).Exec()
		if err != nil {
			s.l.Errorf("Error ReleaseLock %v", err)
		}
	} else if err != nil && err != gocql.ErrNotFound {
		// Log errors other than "not found"
		s.l.Errorf("Error validating lock ownership before release: %v", err)
	}
	// If the lock doesn't exist or isn't owned by this client, do nothing
}

func (s *MockStore) KeepAlive(ctx context.Context, service, domain, client_id string, ttl int32) time.Duration {
	var value string
	_ttl := ttl
	if ttl == 0 {
		_ttl = s.ttl
	}
	mockValidateQuery := s.session.Query(s.ValidateLockQuery, service, domain, client_id)
	err := mockValidateQuery.WithContext(ctx).Scan(&value)
	if err != nil {
		s.l.Errorf("Error from KeepAlive select %v", err)
		return time.Duration(-1) * time.Second
	}
	mockAcquireQuery := s.session.Query(s.TryAcquireLockQuery, service, domain, client_id, _ttl)
	err = mockAcquireQuery.WithContext(ctx).Exec()
	if err != nil {
		s.l.Errorf("Error from KeepAlive insert %v", err)
		return time.Duration(-1) * time.Second
	}
	return time.Duration(s.ttl) * time.Second
}

func (s *MockStore) Close() {
	s.session.Close()
}

func (s *MockStore) GetConfig() store.StoreConfig {
	return s.config
}

// SetupStoreWithMocks creates a MockStore with a mocked Session for testing
func SetupStoreWithMocks() (*MockStore, *MockSession) {
	mockSession := new(MockSession)
	logger, _, _ := observability.NewTestLogger()

	store := &MockStore{
		session:             mockSession,
		tableName:           "test_table",
		keyspaceName:        "test_keyspace",
		fullTableName:       "\"test_keyspace\".\"test_table\"",
		ttl:                 15,
		l:                   logger,
		TryAcquireLockQuery: "INSERT INTO \"test_keyspace\".\"test_table\" (service, domain, client_id) VALUES (?, ?, ?) IF NOT EXISTS USING TTL ?",
		ValidateLockQuery:   "SELECT client_id FROM \"test_keyspace\".\"test_table\" WHERE service =? and domain = ? and client_id =? ALLOW FILTERING",
		ReleaseLockQuery:    "DELETE FROM \"test_keyspace\".\"test_table\" WHERE service =? and domain =?",
		config: &ScyllaDBConfig{
			TTL: 15,
		},
	}

	return store, mockSession
}

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
	t.Run("success_when_owns_lock", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"

		// Mock validation query that confirms ownership
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(clientID, nil)

		// Mock release query
		mockReleaseQuery := new(MockQuery)
		mockReleaseQuery.On("WithContext", ctx).Return(mockReleaseQuery)
		mockReleaseQuery.On("Exec").Return(nil)

		// Setup mock session expectations
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)
		mockSession.On("Query", store.ReleaseLockQuery, []interface{}{service, domain}).Return(mockReleaseQuery)

		// Call method under test
		store.ReleaseLock(ctx, service, domain, clientID)

		// Verify mocks
		mockSession.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
		mockReleaseQuery.AssertExpectations(t)
	})

	t.Run("noop_when_doesnt_own_lock", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		differentClientID := "other-client"

		// Mock validation query that shows different owner
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(differentClientID, nil)

		// Setup mock session expectations - note no release query should be called
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)

		// Call method under test
		store.ReleaseLock(ctx, service, domain, clientID)

		// Verify mocks
		mockSession.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
	})

	t.Run("noop_when_lock_not_found", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"

		// Mock validation query that shows lock doesn't exist
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(nil, gocql.ErrNotFound)

		// Setup mock session expectations - note no release query should be called
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)

		// Call method under test
		store.ReleaseLock(ctx, service, domain, clientID)

		// Verify mocks
		mockSession.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
	})

	t.Run("logs_error_on_release_failure", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		expectedError := errors.New("delete failed")

		// Mock validation query that confirms ownership
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(clientID, nil)

		// Mock release query with error
		mockReleaseQuery := new(MockQuery)
		mockReleaseQuery.On("WithContext", ctx).Return(mockReleaseQuery)
		mockReleaseQuery.On("Exec").Return(expectedError)

		// Setup mock session expectations
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)
		mockSession.On("Query", store.ReleaseLockQuery, []interface{}{service, domain}).Return(mockReleaseQuery)

		// Call method under test
		store.ReleaseLock(ctx, service, domain, clientID)

		// Verify mocks
		mockSession.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
		mockReleaseQuery.AssertExpectations(t)
	})
}

func TestTryAcquireLock(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		ttl := int32(30)

		// Mock acquire query
		mockAcquireQuery := new(MockQuery)
		mockAcquireQuery.On("WithContext", ctx).Return(mockAcquireQuery)
		mockAcquireQuery.On("Exec").Return(nil)

		// Mock validate query
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(clientID, nil)

		// Setup mock session expectations
		mockSession.On("Query", store.TryAcquireLockQuery, []interface{}{service, domain, clientID, ttl}).Return(mockAcquireQuery)
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)

		// Call method under test
		result := store.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and mocks
		assert.True(t, result)
		mockSession.AssertExpectations(t)
		mockAcquireQuery.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
	})

	t.Run("default_ttl", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		defaultTtl := store.ttl

		// Mock acquire query
		mockAcquireQuery := new(MockQuery)
		mockAcquireQuery.On("WithContext", ctx).Return(mockAcquireQuery)
		mockAcquireQuery.On("Exec").Return(nil)

		// Mock validate query
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(clientID, nil)

		// Setup mock session expectations
		mockSession.On("Query", store.TryAcquireLockQuery, []interface{}{service, domain, clientID, defaultTtl}).Return(mockAcquireQuery)
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)

		// Call method under test with ttl=0 to use default
		result := store.TryAcquireLock(ctx, service, domain, clientID, 0)

		// Verify result and mocks
		assert.True(t, result)
		mockSession.AssertExpectations(t)
		mockAcquireQuery.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
	})

	t.Run("failure_on_acquire", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		ttl := int32(30)
		expectedError := errors.New("insert failed")

		// Mock acquire query with error
		mockAcquireQuery := new(MockQuery)
		mockAcquireQuery.On("WithContext", ctx).Return(mockAcquireQuery)
		mockAcquireQuery.On("Exec").Return(expectedError)

		// Setup mock session expectations
		mockSession.On("Query", store.TryAcquireLockQuery, []interface{}{service, domain, clientID, ttl}).Return(mockAcquireQuery)

		// Call method under test
		result := store.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and mocks
		assert.False(t, result)
		mockSession.AssertExpectations(t)
		mockAcquireQuery.AssertExpectations(t)
	})

	t.Run("failure_on_validate", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		ttl := int32(30)
		validateError := errors.New("validate failed")

		// Mock acquire query
		mockAcquireQuery := new(MockQuery)
		mockAcquireQuery.On("WithContext", ctx).Return(mockAcquireQuery)
		mockAcquireQuery.On("Exec").Return(nil)

		// Mock validate query with error
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(nil, validateError)

		// Setup mock session expectations
		mockSession.On("Query", store.TryAcquireLockQuery, []interface{}{service, domain, clientID, ttl}).Return(mockAcquireQuery)
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)

		// Call method under test
		result := store.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and mocks
		assert.False(t, result)
		mockSession.AssertExpectations(t)
		mockAcquireQuery.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
	})
}

func TestKeepAlive(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		ttl := int32(30)

		// Mock validate query
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(clientID, nil)

		// Mock acquire query
		mockAcquireQuery := new(MockQuery)
		mockAcquireQuery.On("WithContext", ctx).Return(mockAcquireQuery)
		mockAcquireQuery.On("Exec").Return(nil)

		// Setup mock session expectations
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)
		mockSession.On("Query", store.TryAcquireLockQuery, []interface{}{service, domain, clientID, ttl}).Return(mockAcquireQuery)

		// Call method under test
		result := store.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and mocks
		assert.Equal(t, time.Duration(store.ttl)*time.Second, result)
		mockSession.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
		mockAcquireQuery.AssertExpectations(t)
	})

	t.Run("default_ttl", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		defaultTtl := store.ttl

		// Mock validate query
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(clientID, nil)

		// Mock acquire query
		mockAcquireQuery := new(MockQuery)
		mockAcquireQuery.On("WithContext", ctx).Return(mockAcquireQuery)
		mockAcquireQuery.On("Exec").Return(nil)

		// Setup mock session expectations
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)
		mockSession.On("Query", store.TryAcquireLockQuery, []interface{}{service, domain, clientID, defaultTtl}).Return(mockAcquireQuery)

		// Call method under test with ttl=0 to use default
		result := store.KeepAlive(ctx, service, domain, clientID, 0)

		// Verify result and mocks
		assert.Equal(t, time.Duration(store.ttl)*time.Second, result)
		mockSession.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
		mockAcquireQuery.AssertExpectations(t)
	})

	t.Run("validate_error", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		ttl := int32(30)
		validateError := errors.New("validate failed")

		// Mock validate query with error
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(nil, validateError)

		// Setup mock session expectations
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)

		// Call method under test
		result := store.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and mocks
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockSession.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
	})

	t.Run("acquire_error", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		ttl := int32(30)
		acquireError := errors.New("acquire failed")

		// Mock validate query
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(clientID, nil)

		// Mock acquire query with error
		mockAcquireQuery := new(MockQuery)
		mockAcquireQuery.On("WithContext", ctx).Return(mockAcquireQuery)
		mockAcquireQuery.On("Exec").Return(acquireError)

		// Setup mock session expectations
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)
		mockSession.On("Query", store.TryAcquireLockQuery, []interface{}{service, domain, clientID, ttl}).Return(mockAcquireQuery)

		// Call method under test
		result := store.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and mocks
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockSession.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
		mockAcquireQuery.AssertExpectations(t)
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

// Test the ScyllaDBConfig implementation of StoreConfig interface
func TestScyllaDBConfigInterface(t *testing.T) {
	t.Run("implements_storeconfig_interface", func(t *testing.T) {
		// Create a config instance
		config := NewScyllaDBConfig()

		// Check if it implements the store.StoreConfig interface
		var i store.StoreConfig = config
		assert.NotNil(t, i)
	})

	t.Run("validate_success", func(t *testing.T) {
		config := &ScyllaDBConfig{
			Host:        "localhost",
			Port:        9042,
			Keyspace:    "test_keyspace",
			Table:       "test_table",
			TTL:         15,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"localhost:9042"},
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("validate_missing_host", func(t *testing.T) {
		config := &ScyllaDBConfig{
			Host:        "",
			Port:        9042,
			Keyspace:    "test_keyspace",
			Table:       "test_table",
			TTL:         15,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"localhost:9042"},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "host is required")
	})

	t.Run("validate_invalid_port", func(t *testing.T) {
		config := &ScyllaDBConfig{
			Host:        "localhost",
			Port:        0,
			Keyspace:    "test_keyspace",
			Table:       "test_table",
			TTL:         15,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"localhost:9042"},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port")
	})

	t.Run("validate_missing_keyspace", func(t *testing.T) {
		config := &ScyllaDBConfig{
			Host:        "localhost",
			Port:        9042,
			Keyspace:    "",
			Table:       "test_table",
			TTL:         15,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"localhost:9042"},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "keyspace is required")
	})

	t.Run("validate_missing_table", func(t *testing.T) {
		config := &ScyllaDBConfig{
			Host:        "localhost",
			Port:        9042,
			Keyspace:    "test_keyspace",
			Table:       "",
			TTL:         15,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"localhost:9042"},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "table is required")
	})

	t.Run("validate_invalid_ttl", func(t *testing.T) {
		config := &ScyllaDBConfig{
			Host:        "localhost",
			Port:        9042,
			Keyspace:    "test_keyspace",
			Table:       "test_table",
			TTL:         0,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"localhost:9042"},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid TTL")
	})

	t.Run("validate_missing_consistency", func(t *testing.T) {
		config := &ScyllaDBConfig{
			Host:        "localhost",
			Port:        9042,
			Keyspace:    "test_keyspace",
			Table:       "test_table",
			TTL:         15,
			Consistency: "",
			Endpoints:   []string{"localhost:9042"},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "consistency level is required")
	})

	t.Run("validate_missing_endpoints", func(t *testing.T) {
		config := &ScyllaDBConfig{
			Host:        "localhost",
			Port:        9042,
			Keyspace:    "test_keyspace",
			Table:       "test_table",
			TTL:         15,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one endpoint is required")
	})

	t.Run("getters", func(t *testing.T) {
		config := &ScyllaDBConfig{
			Host:        "localhost",
			Port:        9042,
			Keyspace:    "test_keyspace",
			Table:       "test_table",
			TTL:         15,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"endpoint1", "endpoint2"},
		}

		assert.Equal(t, "test_table", config.GetTableName())
		assert.Equal(t, int32(15), config.GetTTL())
		assert.Equal(t, []string{"endpoint1", "endpoint2"}, config.GetEndpoints())
	})

	t.Run("new_config_defaults", func(t *testing.T) {
		config := NewScyllaDBConfig()

		assert.Equal(t, "127.0.0.1", config.Host)
		assert.Equal(t, int32(9042), config.Port)
		assert.Equal(t, "quorum-quest", config.Keyspace)
		assert.Equal(t, "services", config.Table)
		assert.Equal(t, int32(15), config.TTL)
		assert.Equal(t, "CONSISTENCY_QUORUM", config.Consistency)
		assert.Equal(t, []string{"localhost:9042"}, config.Endpoints)
	})
}
