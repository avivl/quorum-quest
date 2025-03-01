// internal/store/scylladb/mock_scylladb.go
package scylladb

import (
	"context"
	"fmt"
	"time"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/gocql/gocql"
	"github.com/stretchr/testify/mock"
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

// initSession initializes the query strings for testing
func (s *MockStore) initSession() {
	s.TryAcquireLockQuery = fmt.Sprintf("INSERT INTO %s (service, domain, client_id) VALUES (?, ?, ?) IF NOT EXISTS USING TTL ?", s.fullTableName)
	s.ValidateLockQuery = fmt.Sprintf("SELECT client_id FROM %s WHERE service =? and domain = ? and client_id =? ALLOW FILTERING", s.fullTableName)
	s.ReleaseLockQuery = fmt.Sprintf("DELETE FROM %s WHERE service =? and domain =?", s.fullTableName)
}

// No-op function for testing - doesn't actually run queries
func (s *MockStore) validateKeyspace() {
	// This is just a mock implementation for testing
}

// No-op function for testing - doesn't actually run queries
func (s *MockStore) validateTable() error {
	// This is just a mock implementation for testing
	return nil
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
	_ttl := ttl
	if ttl == 0 {
		_ttl = s.ttl
	}

	s.l.Infof("Attempting KeepAlive for service=%s, domain=%s, client_id=%s", service, domain, client_id)

	// Rather than checking if the lock exists first, just try to reacquire it
	// This is a more robust approach than querying and then updating
	mockAcquireQuery := s.session.Query(s.TryAcquireLockQuery, service, domain, client_id, _ttl)
	err := mockAcquireQuery.WithContext(ctx).Exec()
	if err != nil {
		s.l.Errorf("Error from KeepAlive insert %v", err)
		return time.Duration(-1) * time.Second
	}

	s.l.Infof("Lock refreshed with TTL=%d", _ttl)
	return time.Duration(_ttl) * time.Second
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
