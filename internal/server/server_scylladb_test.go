// internal/server/server_scylladb_test.go
package server

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "github.com/avivl/quorum-quest/api/gen/go/v1"
	"github.com/avivl/quorum-quest/internal/config"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/avivl/quorum-quest/internal/store/scylladb"
	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// CustomScyllaDBMockStore is a custom implementation that we can fully control
// for testing without accessing unexported fields of scylladb.MockStore
type CustomScyllaDBMockStore struct {
	mockSession      *scylladb.MockSession
	mockQuery        *scylladb.MockQuery
	tableName        string
	keyspaceName     string
	fullTableName    string
	ttl              int32
	logger           *observability.SLogger
	config           *scylladb.ScyllaDBConfig
	acquireLockQuery string
	validateQuery    string
	releaseLockQuery string
}

func (s *CustomScyllaDBMockStore) GetConfig() store.StoreConfig {
	return s.config
}

func (s *CustomScyllaDBMockStore) TryAcquireLock(ctx context.Context, service, domain, clientId string, ttl int32) bool {
	_ttl := ttl
	if ttl == 0 {
		_ttl = s.ttl
	}

	// First attempt the insertion
	query := s.mockSession.Query(s.acquireLockQuery, service, domain, clientId, _ttl)
	err := query.WithContext(ctx).Exec()
	if err != nil {
		s.logger.Errorf("Error acquiring lock: %v", err)
		return false
	}

	// Verify the lock was acquired by reading it back
	var value string
	validateQuery := s.mockSession.Query(s.validateQuery, service, domain, clientId)
	err = validateQuery.WithContext(ctx).Scan(&value)
	if err != nil {
		s.logger.Errorf("Error validating lock: %v", err)
		return false
	}

	return true
}

func (s *CustomScyllaDBMockStore) ReleaseLock(ctx context.Context, service, domain, clientId string) {
	// First validate that this client owns the lock
	var storedClientId string
	validateQuery := s.mockSession.Query(s.validateQuery, service, domain, clientId)
	err := validateQuery.WithContext(ctx).Scan(&storedClientId)

	if err == nil && storedClientId == clientId {
		// This client owns the lock, so it can release it
		releaseQuery := s.mockSession.Query(s.releaseLockQuery, service, domain)
		err = releaseQuery.WithContext(ctx).Exec()
		if err != nil {
			s.logger.Errorf("Error ReleaseLock %v", err)
		}
	} else if err != nil && err != gocql.ErrNotFound {
		// Log errors other than "not found"
		s.logger.Errorf("Error validating lock ownership before release: %v", err)
	}
	// If the lock doesn't exist or isn't owned by this client, do nothing
}

func (s *CustomScyllaDBMockStore) KeepAlive(ctx context.Context, service, domain, clientId string, ttl int32) time.Duration {
	_ttl := ttl
	if ttl == 0 {
		_ttl = s.ttl
	}

	s.logger.Infof("Attempting KeepAlive for service=%s, domain=%s, client_id=%s", service, domain, clientId)

	// Rather than checking if the lock exists first, just try to reacquire it
	// This is a more robust approach than querying and then updating
	query := s.mockSession.Query(s.acquireLockQuery, service, domain, clientId, _ttl)
	err := query.WithContext(ctx).Exec()
	if err != nil {
		s.logger.Errorf("Error from KeepAlive insert %v", err)
		return time.Duration(-1) * time.Second
	}

	s.logger.Infof("Lock refreshed with TTL=%d", _ttl)
	return time.Duration(_ttl) * time.Second
}

func (s *CustomScyllaDBMockStore) Close() {
	s.mockSession.Close()
}

func setupScyllaDBMockServer(t *testing.T) (*Server[*scylladb.ScyllaDBConfig], *scylladb.MockSession, *scylladb.MockQuery, context.Context) {
	ctx := context.Background()

	// Create ScyllaDB config
	scyllaConfig := &scylladb.ScyllaDBConfig{
		Host:        "localhost",
		Port:        9042,
		Keyspace:    "quorum-quest",
		Table:       "services",
		TTL:         15,
		Consistency: "CONSISTENCY_QUORUM",
		Endpoints:   []string{"localhost:9042"},
	}

	// Create global config with ScyllaDB config
	cfg := &config.GlobalConfig[*scylladb.ScyllaDBConfig]{
		ServerAddress: "localhost:50051",
		Store:         scyllaConfig,
	}

	// Setup logger and metrics
	logger := setupTestLogger(t)
	serverMetrics := setupTestMetrics(t, logger)

	// Setup mock session and query
	mockSession := new(scylladb.MockSession)
	mockQuery := new(scylladb.MockQuery)

	// Configure mock query behavior for WithContext
	mockQuery.On("WithContext", mock.Anything).Return(mockQuery)

	// Create the store initializer that returns our mock store
	storeInitializer := func(ctx context.Context, cfg *config.GlobalConfig[*scylladb.ScyllaDBConfig], logger *observability.SLogger) (store.Store, error) {
		// Create a custom mock store that we can fully control
		mockStore := &CustomScyllaDBMockStore{
			mockSession:      mockSession,
			mockQuery:        mockQuery,
			tableName:        "test_table",
			keyspaceName:     "test_keyspace",
			fullTableName:    "\"test_keyspace\".\"test_table\"",
			ttl:              15,
			logger:           logger,
			config:           cfg.Store,
			acquireLockQuery: "INSERT INTO \"test_keyspace\".\"test_table\" (service, domain, client_id) VALUES (?, ?, ?) IF NOT EXISTS USING TTL ?",
			validateQuery:    "SELECT client_id FROM \"test_keyspace\".\"test_table\" WHERE service =? and domain = ? and client_id =? ALLOW FILTERING",
			releaseLockQuery: "DELETE FROM \"test_keyspace\".\"test_table\" WHERE service =? and domain =?",
		}
		return mockStore, nil
	}

	// Initialize server
	server, err := NewServer(cfg, logger, serverMetrics, storeInitializer)
	require.NoError(t, err)

	// Initialize store
	err = server.initStore(ctx, storeInitializer)
	require.NoError(t, err)

	return server, mockSession, mockQuery, ctx
}

func TestTryAcquireLockWithScyllaDBMock(t *testing.T) {
	server, mockSession, mockQuery, ctx := setupScyllaDBMockServer(t)
	defer server.Stop()

	tests := []struct {
		name           string
		service        string
		domain         string
		clientID       string
		ttl            int32
		expectedResult bool
		mockSetup      func()
	}{
		{
			name:           "successful lock acquisition",
			service:        "test-service-1",
			domain:         "test-domain-1",
			clientID:       "client-1",
			ttl:            30,
			expectedResult: true,
			mockSetup: func() {
				// Mock the query for TryAcquireLock
				mockSession.On("Query",
					"INSERT INTO \"test_keyspace\".\"test_table\" (service, domain, client_id) VALUES (?, ?, ?) IF NOT EXISTS USING TTL ?",
					mock.Anything).Return(mockQuery).Once()
				mockQuery.On("Exec").Return(nil).Once()

				// Mock the query for validation
				mockSession.On("Query",
					"SELECT client_id FROM \"test_keyspace\".\"test_table\" WHERE service =? and domain = ? and client_id =? ALLOW FILTERING",
					mock.Anything).Return(mockQuery).Once()
				mockQuery.On("Scan", mock.Anything).Return("client-1", nil).Once()
			},
		},
		{
			name:           "failed lock acquisition due to error",
			service:        "test-service-2",
			domain:         "test-domain-2",
			clientID:       "client-2",
			ttl:            30,
			expectedResult: false,
			mockSetup: func() {
				// Mock the query to fail
				mockSession.On("Query",
					"INSERT INTO \"test_keyspace\".\"test_table\" (service, domain, client_id) VALUES (?, ?, ?) IF NOT EXISTS USING TTL ?",
					mock.Anything).Return(mockQuery).Once()
				mockQuery.On("Exec").Return(errors.New("mock error")).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks for this test case
			tt.mockSetup()

			// Create request
			req := &pb.TryAcquireLockRequest{
				Service:  tt.service,
				Domain:   tt.domain,
				ClientId: tt.clientID,
				Ttl:      tt.ttl,
			}

			// Execute method
			resp, err := server.TryAcquireLock(ctx, req)

			// Assertions
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.expectedResult, resp.IsLeader)

			// Verify all mocks were called as expected
			mockSession.AssertExpectations(t)
			mockQuery.AssertExpectations(t)
		})
	}
}

func TestReleaseLockWithScyllaDBMock(t *testing.T) {
	server, mockSession, mockQuery, ctx := setupScyllaDBMockServer(t)
	defer server.Stop()

	tests := []struct {
		name      string
		service   string
		domain    string
		clientID  string
		mockSetup func()
	}{
		{
			name:     "successful lock release - client owns lock",
			service:  "test-service-1",
			domain:   "test-domain-1",
			clientID: "client-1",
			mockSetup: func() {
				// Mock validation query - client owns the lock
				mockSession.On("Query",
					"SELECT client_id FROM \"test_keyspace\".\"test_table\" WHERE service =? and domain = ? and client_id =? ALLOW FILTERING",
					mock.Anything).Return(mockQuery).Once()
				mockQuery.On("Scan", mock.Anything).Return("client-1", nil).Once()

				// Mock release query
				mockSession.On("Query",
					"DELETE FROM \"test_keyspace\".\"test_table\" WHERE service =? and domain =?",
					mock.Anything).Return(mockQuery).Once()
				mockQuery.On("Exec").Return(nil).Once()
			},
		},
		{
			name:     "lock release no-op - client doesn't own lock",
			service:  "test-service-2",
			domain:   "test-domain-2",
			clientID: "wrong-client",
			mockSetup: func() {
				// Mock validation query - client doesn't own the lock
				mockSession.On("Query",
					"SELECT client_id FROM \"test_keyspace\".\"test_table\" WHERE service =? and domain = ? and client_id =? ALLOW FILTERING",
					mock.Anything).Return(mockQuery).Once()
				mockQuery.On("Scan", mock.Anything).Return("correct-client", nil).Once()

				// No release query should be called
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks for this test case
			tt.mockSetup()

			// Create request
			req := &pb.ReleaseLockRequest{
				Service:  tt.service,
				Domain:   tt.domain,
				ClientId: tt.clientID,
			}

			// Execute method
			resp, err := server.ReleaseLock(ctx, req)

			// Assertions
			assert.NoError(t, err)
			assert.NotNil(t, resp)

			// Verify all mocks were called as expected
			mockSession.AssertExpectations(t)
			mockQuery.AssertExpectations(t)
		})
	}
}

func TestKeepAliveWithScyllaDBMock(t *testing.T) {
	server, mockSession, mockQuery, ctx := setupScyllaDBMockServer(t)
	defer server.Stop()

	tests := []struct {
		name           string
		service        string
		domain         string
		clientID       string
		ttl            int32
		expectedStatus int64 // seconds for the lease, -1 for failure
		mockSetup      func()
	}{
		{
			name:           "successful keep-alive",
			service:        "test-service-1",
			domain:         "test-domain-1",
			clientID:       "client-1",
			ttl:            30,
			expectedStatus: 30, // 30 seconds lease
			mockSetup: func() {
				// Mock the reacquisition query
				mockSession.On("Query",
					"INSERT INTO \"test_keyspace\".\"test_table\" (service, domain, client_id) VALUES (?, ?, ?) IF NOT EXISTS USING TTL ?",
					mock.Anything).Return(mockQuery).Once()
				mockQuery.On("Exec").Return(nil).Once()
			},
		},
		{
			name:           "failed keep-alive",
			service:        "test-service-2",
			domain:         "test-domain-2",
			clientID:       "client-2",
			ttl:            30,
			expectedStatus: -1, // Failure
			mockSetup: func() {
				// Mock the reacquisition query with failure
				mockSession.On("Query",
					"INSERT INTO \"test_keyspace\".\"test_table\" (service, domain, client_id) VALUES (?, ?, ?) IF NOT EXISTS USING TTL ?",
					mock.Anything).Return(mockQuery).Once()
				mockQuery.On("Exec").Return(errors.New("mock error")).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks for this test case
			tt.mockSetup()

			// Create request
			req := &pb.KeepAliveRequest{
				Service:  tt.service,
				Domain:   tt.domain,
				ClientId: tt.clientID,
				Ttl:      tt.ttl,
			}

			// Execute method
			resp, err := server.KeepAlive(ctx, req)

			// Assertions
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.expectedStatus, resp.LeaseLength.Seconds)

			// Verify all mocks were called as expected
			mockSession.AssertExpectations(t)
			mockQuery.AssertExpectations(t)
		})
	}
}
