// internal/server/server_additional_test.go
package server

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	pb "github.com/avivl/quorum-quest/api/gen/go/v1"
	"github.com/avivl/quorum-quest/internal/config"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/avivl/quorum-quest/internal/store/scylladb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockStore for general server tests
type MockStore struct {
	tryAcquireLockFunc func(ctx context.Context, service, domain, clientId string, ttl int32) bool
	releaseLockFunc    func(ctx context.Context, service, domain, clientId string)
	keepAliveFunc      func(ctx context.Context, service, domain, clientId string, ttl int32) time.Duration
	closeFunc          func()
	getConfigFunc      func() store.StoreConfig
}

func (m *MockStore) TryAcquireLock(ctx context.Context, service, domain, clientId string, ttl int32) bool {
	if m.tryAcquireLockFunc != nil {
		return m.tryAcquireLockFunc(ctx, service, domain, clientId, ttl)
	}
	return false
}

func (m *MockStore) ReleaseLock(ctx context.Context, service, domain, clientId string) {
	if m.releaseLockFunc != nil {
		m.releaseLockFunc(ctx, service, domain, clientId)
	}
}

func (m *MockStore) KeepAlive(ctx context.Context, service, domain, clientId string, ttl int32) time.Duration {
	if m.keepAliveFunc != nil {
		return m.keepAliveFunc(ctx, service, domain, clientId, ttl)
	}
	return 0
}

func (m *MockStore) Close() {
	if m.closeFunc != nil {
		m.closeFunc()
	}
}

func (m *MockStore) GetConfig() store.StoreConfig {
	if m.getConfigFunc != nil {
		return m.getConfigFunc()
	}
	return nil
}

func TestServe(t *testing.T) {
	// Create a minimal server instance
	logger, _, _ := observability.NewTestLogger()
	metrics, _ := observability.NewMetricsClient(observability.Config{ServiceName: "test"}, logger)

	server := &Server[*scylladb.ScyllaDBConfig]{
		logger:  logger,
		metrics: metrics,
	}

	// Test without listener
	err := server.serve(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "listener is required")

	// Create a test listener
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	server.listener = listener

	// Start server in goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error)
	go func() {
		errCh <- server.serve(ctx)
	}()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Stop the server
	server.Stop()

	// Check no error from serve
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("server.serve didn't return in time")
	}
}

func TestServerCreationWithNilConfig(t *testing.T) {
	logger, _, _ := observability.NewTestLogger()
	metrics, _ := observability.NewMetricsClient(observability.Config{ServiceName: "test"}, logger)

	// Test with nil config
	_, err := NewServer[*scylladb.ScyllaDBConfig](nil, logger, metrics, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config is nil")

	// Test with nil logger
	_, err = NewServer(&config.GlobalConfig[*scylladb.ScyllaDBConfig]{}, nil, metrics, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "logger is nil")

	// Test with nil metrics
	_, err = NewServer(&config.GlobalConfig[*scylladb.ScyllaDBConfig]{}, logger, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "metrics is nil")

	// Test with nil store initializer
	_, err = NewServer(&config.GlobalConfig[*scylladb.ScyllaDBConfig]{}, logger, metrics, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "store initializer is nil")
}

func TestKeepAlive(t *testing.T) {
	// Setup mock store that implements the store interface
	mockStore := &MockStore{
		keepAliveFunc: func(ctx context.Context, service, domain, clientId string, ttl int32) time.Duration {
			return 10 * time.Second
		},
	}

	// Create server with mock store
	logger, _, _ := observability.NewTestLogger()
	metrics, _ := observability.NewMetricsClient(observability.Config{ServiceName: "test"}, logger)

	server := &Server[*scylladb.ScyllaDBConfig]{
		logger:  logger,
		metrics: metrics,
		store:   mockStore,
	}

	// Test KeepAlive
	req := &pb.KeepAliveRequest{
		Service:  "test-service",
		Domain:   "test-domain",
		ClientId: "client-123",
		Ttl:      30,
	}

	resp, err := server.KeepAlive(context.Background(), req)

	// Validate
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(10), resp.LeaseLength.Seconds)
}

func TestStoreInitializationError(t *testing.T) {
	ctx := context.Background()
	logger := setupTestLogger(t)
	metrics := setupTestMetrics(t, logger)

	// Create config
	cfg := &config.GlobalConfig[*scylladb.ScyllaDBConfig]{
		ServerAddress: "localhost:0",
		Store:         &scylladb.ScyllaDBConfig{},
	}

	// Store initializer that always fails
	storeInitializer := func(ctx context.Context, cfg *config.GlobalConfig[*scylladb.ScyllaDBConfig], logger *observability.SLogger) (store.Store, error) {
		return nil, errors.New("store initialization failed")
	}

	// Create server
	server, err := NewServer(cfg, logger, metrics, storeInitializer)
	require.NoError(t, err)

	// Try to initialize store - should fail
	err = server.initStore(ctx, storeInitializer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "store initialization failed")

	// Try to start server - should fail because store initialization failed
	err = server.Start(ctx, storeInitializer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "store initialization failed")
}

func TestStartListenError(t *testing.T) {
	ctx := context.Background()
	logger := setupTestLogger(t)
	metrics := setupTestMetrics(t, logger)

	// Create config with an invalid address that will cause listener creation to fail
	cfg := &config.GlobalConfig[*scylladb.ScyllaDBConfig]{
		ServerAddress: "invalid:address:format",
		Store:         &scylladb.ScyllaDBConfig{},
	}

	// Store initializer that returns a mock store
	storeInitializer := func(ctx context.Context, cfg *config.GlobalConfig[*scylladb.ScyllaDBConfig], logger *observability.SLogger) (store.Store, error) {
		return &MockStore{}, nil
	}

	// Create server
	server, err := NewServer(cfg, logger, metrics, storeInitializer)
	require.NoError(t, err)

	// Try to start server - should fail because of invalid listener address
	err = server.Start(ctx, storeInitializer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid") // Address format error
}

func TestServerStop(t *testing.T) {
	// Setup a mock store to verify it gets closed
	storeClosed := false
	mockStore := &MockStore{
		closeFunc: func() {
			storeClosed = true
		},
	}

	// Create server with mock store
	logger, _, _ := observability.NewTestLogger()
	metrics, _ := observability.NewMetricsClient(observability.Config{ServiceName: "test"}, logger)

	server := &Server[*scylladb.ScyllaDBConfig]{
		logger:  logger,
		metrics: metrics,
		store:   mockStore,
	}

	// Create a test listener
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	server.listener = listener

	// Start server in background to initialize internal gRPC server
	// This is necessary because server.Stop() only closes the store if server.server is not nil
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		server.serve(ctx)
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	// Now stop the server
	server.Stop()
	cancel() // Cancel the context to end the goroutine

	// Verify
	assert.True(t, storeClosed, "Store should be closed")
}

func TestTryAcquireLockBasic(t *testing.T) {
	// Setup mock store for testing TryAcquireLock
	mockStore := &MockStore{
		tryAcquireLockFunc: func(ctx context.Context, service, domain, clientId string, ttl int32) bool {
			// Mock acquisition logic - succeed if service starts with "success", fail otherwise
			return service == "success-service"
		},
	}

	// Create server with mock store
	logger, _, _ := observability.NewTestLogger()
	metrics, _ := observability.NewMetricsClient(observability.Config{ServiceName: "test"}, logger)

	server := &Server[*scylladb.ScyllaDBConfig]{
		logger:  logger,
		metrics: metrics,
		store:   mockStore,
	}

	// Test successful acquisition
	successReq := &pb.TryAcquireLockRequest{
		Service:  "success-service",
		Domain:   "test-domain",
		ClientId: "client-1",
		Ttl:      30,
	}

	successResp, err := server.TryAcquireLock(context.Background(), successReq)
	assert.NoError(t, err)
	assert.NotNil(t, successResp)
	assert.True(t, successResp.IsLeader)

	// Test failed acquisition
	failReq := &pb.TryAcquireLockRequest{
		Service:  "fail-service",
		Domain:   "test-domain",
		ClientId: "client-1",
		Ttl:      30,
	}

	failResp, err := server.TryAcquireLock(context.Background(), failReq)
	assert.NoError(t, err)
	assert.NotNil(t, failResp)
	assert.False(t, failResp.IsLeader)
}

func TestReleaseLockBasic(t *testing.T) {
	// Track which locks were released
	releasedLocks := make(map[string]bool)

	// Setup mock store for testing ReleaseLock
	mockStore := &MockStore{
		releaseLockFunc: func(ctx context.Context, service, domain, clientId string) {
			// Mark this lock as released
			key := service + ":" + domain + ":" + clientId
			releasedLocks[key] = true
		},
	}

	// Create server with mock store
	logger, _, _ := observability.NewTestLogger()
	metrics, _ := observability.NewMetricsClient(observability.Config{ServiceName: "test"}, logger)

	server := &Server[*scylladb.ScyllaDBConfig]{
		logger:  logger,
		metrics: metrics,
		store:   mockStore,
	}

	// Test lock release
	req := &pb.ReleaseLockRequest{
		Service:  "test-service",
		Domain:   "test-domain",
		ClientId: "client-1",
	}

	resp, err := server.ReleaseLock(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify lock was released
	key := "test-service:test-domain:client-1"
	assert.True(t, releasedLocks[key], "Lock should have been released")
}
