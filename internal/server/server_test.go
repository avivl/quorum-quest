// internal/server/server_test.go
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	pb "github.com/avivl/quorum-quest/api/gen/go/v1"
	"github.com/avivl/quorum-quest/internal/config"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/avivl/quorum-quest/internal/store/dynamodb"
	"github.com/avivl/quorum-quest/internal/store/redis"
	"github.com/avivl/quorum-quest/internal/store/scylladb"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	redislib "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
)

type TestConfig struct {
	Endpoints   []string
	Keyspace    string
	TTL         int32
	Table       string
	Host        string
	Port        int32
	Consistency string
}

func (c TestConfig) GetEndpoints() []string { return c.Endpoints }
func (c TestConfig) GetKeyspace() string    { return c.Keyspace }
func (c TestConfig) GetTTL() int32          { return c.TTL }
func (c TestConfig) GetTableName() string   { return c.Table }
func (c TestConfig) Validate() error {
	if len(c.Endpoints) == 0 {
		return errors.New("endpoints cannot be empty")
	}
	if c.Keyspace == "" {
		return errors.New("keyspace cannot be empty")
	}
	if c.Table == "" {
		return errors.New("table name cannot be empty")
	}
	if c.TTL <= 0 {
		return errors.New("TTL must be positive")
	}
	return nil
}

func setupTestServer(t *testing.T) (*Server[TestConfig], context.Context) {
	ctx := context.Background()

	testConfig := TestConfig{
		Endpoints:   []string{"localhost:9042"},
		Host:        "localhost",
		Port:        9042,
		Keyspace:    "leader_election", // Match the keyspace name from the store
		TTL:         30,
		Table:       "locks", // Match the table name from the store
		Consistency: "CONSISTENCY_QUORUM",
	}

	cfg := &config.GlobalConfig[TestConfig]{
		ServerAddress: "localhost:50051",
		Store:         testConfig,
	}

	logger, err := observability.NewLogger(zapcore.InfoLevel, zap.Development())
	require.NoError(t, err)

	// Initialize metrics with proper configuration
	metricsConfig := observability.Config{
		ServiceName:    "test-server",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		OTelEndpoint:   "localhost:4317",
	}

	serverMetrics, err := observability.NewMetricsClient(metricsConfig, logger)
	require.NoError(t, err)

	// Create store initializer function that uses the actual ScyllaDB store
	storeInitializer := func(ctx context.Context, cfg *config.GlobalConfig[TestConfig], logger *observability.SLogger) (store.Store, error) {
		scyllaConfig := &scylladb.ScyllaDBConfig{
			Endpoints:   cfg.Store.Endpoints,
			Host:        cfg.Store.Host,
			Port:        cfg.Store.Port,
			Keyspace:    cfg.Store.Keyspace,
			Table:       cfg.Store.Table,
			TTL:         cfg.Store.TTL,
			Consistency: cfg.Store.Consistency,
		}

		// Return a new store instance
		str, err := scylladb.New(ctx, scyllaConfig, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create store: %w", err)
		}

		// Wait for the store to be initialized
		time.Sleep(time.Second * 5)

		// Verify table exists by trying to create a test record
		if acquired := str.TryAcquireLock(ctx, "_test", "_test", "_test", 1); !acquired {
			return nil, fmt.Errorf("failed to verify table initialization")
		}
		// Clean up test record
		str.ReleaseLock(ctx, "_test", "_test", "_test")

		return str, nil
	}

	// Initialize server
	server, err := NewServer(cfg, logger, serverMetrics, storeInitializer)
	require.NoError(t, err)

	// Initialize store
	err = server.initStore(ctx, storeInitializer)
	require.NoError(t, err)

	// Verify server and store are not nil
	require.NotNil(t, server, "server should not be nil")
	require.NotNil(t, server.store, "store should not be nil")

	return server, ctx
}

func TestTryAcquireLock(t *testing.T) {
	server, ctx := setupTestServer(t)
	require.NotNil(t, server)
	require.NotNil(t, server.store)

	defer func() {
		if err := server.Stop(); err != nil {
			t.Errorf("failed to stop server: %v", err)
		}
	}()

	// Wait for store to be fully initialized
	time.Sleep(time.Second * 2)

	tests := []struct {
		name           string
		service        string
		domain         string
		clientID       string
		ttl            int32
		expectedResult bool
		setupFunc      func(context.Context)
		cleanupFunc    func(context.Context)
	}{
		{
			name:           "acquire new lock",
			service:        "test-service-1",
			domain:         "test-domain-1",
			clientID:       "client-1",
			ttl:            30,
			expectedResult: true,
			setupFunc:      func(ctx context.Context) {}, // No setup needed for fresh lock
			cleanupFunc: func(ctx context.Context) {
				// Release the lock after test
				server.store.ReleaseLock(ctx, "test-service-1", "test-domain-1", "client-1")
			},
		},
		{
			name:           "acquire existing lock",
			service:        "test-service-2",
			domain:         "test-domain-2",
			clientID:       "client-2",
			ttl:            30,
			expectedResult: false,
			setupFunc: func(ctx context.Context) {
				// Pre-acquire lock with different client
				server.store.TryAcquireLock(ctx, "test-service-2", "test-domain-2", "existing-client", 30)
			},
			cleanupFunc: func(ctx context.Context) {
				// Release the pre-acquired lock
				server.store.ReleaseLock(ctx, "test-service-2", "test-domain-2", "existing-client")
			},
		},
		{
			name:           "acquire expired lock",
			service:        "test-service-3",
			domain:         "test-domain-3",
			clientID:       "client-3",
			ttl:            30,
			expectedResult: true,
			setupFunc: func(ctx context.Context) {
				// Pre-acquire lock with short TTL
				server.store.TryAcquireLock(ctx, "test-service-3", "test-domain-3", "expired-client", 1)
				// Wait for lock to expire
				time.Sleep(2 * time.Second)
			},
			cleanupFunc: func(ctx context.Context) {
				server.store.ReleaseLock(ctx, "test-service-3", "test-domain-3", "client-3")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test case
			tt.setupFunc(ctx)
			defer tt.cleanupFunc(ctx)

			// Create request
			req := &pb.TryAcquireLockRequest{
				Service:  tt.service,
				Domain:   tt.domain,
				ClientId: tt.clientID,
				Ttl:      tt.ttl,
			}

			// Execute the method
			resp, err := server.TryAcquireLock(ctx, req)

			// Assertions
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.expectedResult, resp.IsLeader)

			// Verify lock state if acquired
			if tt.expectedResult {
				// Try to acquire same lock with different client - should fail
				verificationReq := &pb.TryAcquireLockRequest{
					Service:  tt.service,
					Domain:   tt.domain,
					ClientId: "verification-client",
					Ttl:      tt.ttl,
				}
				verificationResp, err := server.TryAcquireLock(ctx, verificationReq)
				assert.NoError(t, err)
				assert.False(t, verificationResp.IsLeader)
			}
		})
	}
}

func TestReleaseLock(t *testing.T) {
	server, ctx := setupTestServer(t)
	require.NotNil(t, server)
	require.NotNil(t, server.store)

	defer func() {
		if err := server.Stop(); err != nil {
			t.Errorf("failed to stop server: %v", err)
		}
	}()

	tests := []struct {
		name       string
		service    string
		domain     string
		clientID   string
		setupFunc  func(context.Context) bool
		verifyFunc func(context.Context) bool
	}{
		{
			name:     "release existing lock",
			service:  "test-service-1",
			domain:   "test-domain-1",
			clientID: "client-1",
			setupFunc: func(ctx context.Context) bool {
				// Try to acquire lock
				return server.store.TryAcquireLock(ctx, "test-service-1", "test-domain-1", "client-1", 30)
			},
			verifyFunc: func(ctx context.Context) bool {
				// After releasing, a new client should be able to acquire the lock
				return server.store.TryAcquireLock(ctx, "test-service-1", "test-domain-1", "new-client", 30)
			},
		},
		{
			name:     "release non-existent lock",
			service:  "test-service-2",
			domain:   "test-domain-2",
			clientID: "client-2",
			setupFunc: func(ctx context.Context) bool {
				return true // No setup needed
			},
			verifyFunc: func(ctx context.Context) bool {
				// Should be able to acquire the lock after trying to release non-existent
				return server.store.TryAcquireLock(ctx, "test-service-2", "test-domain-2", "new-client", 30)
			},
		},
		{
			name:     "release with wrong client ID",
			service:  "test-service-3",
			domain:   "test-domain-3",
			clientID: "wrong-client",
			setupFunc: func(ctx context.Context) bool {
				// Acquire lock with correct client
				return server.store.TryAcquireLock(ctx, "test-service-3", "test-domain-3", "correct-client", 30)
			},
			verifyFunc: func(ctx context.Context) bool {
				// Lock should still be held by original client
				result := server.store.TryAcquireLock(ctx, "test-service-3", "test-domain-3", "new-client", 30)
				return !result // Should fail to acquire
			},
		},
		{
			name:     "release expired lock",
			service:  "test-service-4",
			domain:   "test-domain-4",
			clientID: "client-4",
			setupFunc: func(ctx context.Context) bool {
				// Acquire lock with short TTL
				if !server.store.TryAcquireLock(ctx, "test-service-4", "test-domain-4", "client-4", 1) {
					return false
				}
				// Wait for lock to expire
				time.Sleep(2 * time.Second)
				return true
			},
			verifyFunc: func(ctx context.Context) bool {
				// Should be able to acquire after release of expired lock
				return server.store.TryAcquireLock(ctx, "test-service-4", "test-domain-4", "new-client", 30)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run setup and check if it succeeded
			setupSuccess := tt.setupFunc(ctx)
			require.True(t, setupSuccess, "setup failed")

			// Create request
			req := &pb.ReleaseLockRequest{
				Service:  tt.service,
				Domain:   tt.domain,
				ClientId: tt.clientID,
			}

			// Execute the release
			resp, err := server.ReleaseLock(ctx, req)
			assert.NoError(t, err)
			assert.NotNil(t, resp)

			// Wait a bit for the release to complete
			time.Sleep(time.Second)

			// Verify the results
			verifyResult := tt.verifyFunc(ctx)
			assert.True(t, verifyResult, "verification failed after lock release")
		})
	}
}

func TestUnaryServerInterceptor(t *testing.T) {
	// Create a minimal server instance
	logger, _, _ := observability.NewTestLogger()
	metrics, _ := observability.NewMetricsClient(observability.Config{ServiceName: "test"}, logger)

	server := &Server[*scylladb.ScyllaDBConfig]{
		logger:  logger,
		metrics: metrics,
	}

	// Create the interceptor
	interceptor := server.unaryServerInterceptor()

	// Mock handler function
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	}

	// Create server info
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/TestMethod",
	}

	// Call interceptor
	resp, err := interceptor(context.Background(), "request", info, handler)

	// Validate
	assert.NoError(t, err)
	assert.Equal(t, "response", resp)
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

// MockStore for testing
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

// Helper function to check if DynamoDB is accessible
func isDynamoDBAccessible(t *testing.T) bool {
	// Create a simple AWS config
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{URL: "http://localhost:8000"}, nil
				},
			),
		),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("dummy", "dummy", ""),
		),
	)

	if err != nil {
		t.Logf("Failed to create AWS config: %v", err)
		return false
	}

	// Create DynamoDB client
	client := awsdynamodb.NewFromConfig(cfg)

	// Try to list tables
	_, err = client.ListTables(context.Background(), &awsdynamodb.ListTablesInput{})
	if err != nil {
		t.Logf("DynamoDB is not accessible: %v", err)
		return false
	}

	return true
}

func setupTestServerWithDynamoDB(t *testing.T) (*Server[*dynamodb.DynamoDBConfig], context.Context) {
	// Check if DynamoDB is accessible
	if !isDynamoDBAccessible(t) {
		t.Skip("Skipping test because DynamoDB is not accessible")
		return nil, nil
	}

	ctx := context.Background()

	// Create DynamoDB config directly
	dynamoConfig := &dynamodb.DynamoDBConfig{
		Region:          "us-east-1",
		Table:           "locks_test",
		TTL:             30,
		Endpoints:       []string{"http://localhost:8000"},
		AccessKeyID:     "dummy",
		SecretAccessKey: "dummy",
	}

	cfg := &config.GlobalConfig[*dynamodb.DynamoDBConfig]{
		ServerAddress: "localhost:50052", // Use a different port to avoid conflicts
		Store:         dynamoConfig,
	}

	logger, err := observability.NewLogger(zapcore.InfoLevel, zap.Development())
	require.NoError(t, err)

	// Initialize metrics with proper configuration
	metricsConfig := observability.Config{
		ServiceName:    "test-server-dynamodb",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		OTelEndpoint:   "localhost:4317",
	}

	serverMetrics, err := observability.NewMetricsClient(metricsConfig, logger)
	require.NoError(t, err)

	// Create store initializer function that uses DynamoDB store
	storeInitializer := func(ctx context.Context, cfg *config.GlobalConfig[*dynamodb.DynamoDBConfig], logger *observability.SLogger) (store.Store, error) {
		// Return a new store instance
		str, err := dynamodb.NewStore(ctx, cfg.Store, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create DynamoDB store: %w", err)
		}

		// Wait for the store to be initialized
		time.Sleep(time.Second * 2)

		return str, nil
	}

	// Initialize server
	server, err := NewServer(cfg, logger, serverMetrics, storeInitializer)
	require.NoError(t, err)

	// Initialize store
	err = server.initStore(ctx, storeInitializer)
	require.NoError(t, err)

	// Verify server and store are not nil
	require.NotNil(t, server, "server should not be nil")
	require.NotNil(t, server.store, "store should not be nil")

	// Create a clean table for testing
	cleanupTable(t, ctx, server)

	return server, ctx
}

// Helper to clean up the test table
func cleanupTable(t *testing.T, ctx context.Context, server *Server[*dynamodb.DynamoDBConfig]) {
	// Create AWS config
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{URL: "http://localhost:8000"}, nil
				},
			),
		),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("dummy", "dummy", ""),
		),
	)
	require.NoError(t, err)

	// Create DynamoDB client
	client := awsdynamodb.NewFromConfig(cfg)

	// Delete and recreate the table
	tableName := server.config.Store.Table

	// Try to delete the table if it exists
	_, err = client.DeleteTable(ctx, &awsdynamodb.DeleteTableInput{
		TableName: aws.String(tableName),
	})
	// Ignore errors - table might not exist

	// Wait a moment for deletion to complete
	time.Sleep(2 * time.Second)

	// Create the table
	_, err = client.CreateTable(ctx, &awsdynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("PK"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("PK"),
				KeyType:       types.KeyTypeHash,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Logf("Error creating table: %v", err)
	}

	// Wait for table to be active
	waiter := awsdynamodb.NewTableExistsWaiter(client)
	err = waiter.Wait(ctx, &awsdynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	}, 30*time.Second)
	if err != nil {
		t.Logf("Error waiting for table: %v", err)
	}

	// Wait a bit more to ensure table is ready
	time.Sleep(2 * time.Second)
}

func TestTryAcquireLockWithDynamoDB(t *testing.T) {
	server, ctx := setupTestServerWithDynamoDB(t)
	if server == nil {
		return // Test was skipped
	}

	defer func() {
		if err := server.Stop(); err != nil {
			t.Errorf("failed to stop server: %v", err)
		}
	}()

	// Test case 1: Acquire a new lock
	t.Run("acquire_new_lock", func(t *testing.T) {
		// Create request
		req := &pb.TryAcquireLockRequest{
			Service:  "test-service-1",
			Domain:   "test-domain-1",
			ClientId: "client-1",
			Ttl:      30,
		}

		// Execute the method
		resp, err := server.TryAcquireLock(ctx, req)

		// Log the response
		t.Logf("TryAcquireLock response: %+v", resp)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.IsLeader, "Expected to acquire lock")

		// Clean up
		server.store.ReleaseLock(ctx, "test-service-1", "test-domain-1", "client-1")
	})

	// Test case 2: Try to acquire an already held lock
	t.Run("acquire_existing_lock", func(t *testing.T) {
		// First acquire the lock
		success := server.store.TryAcquireLock(ctx, "test-service-2", "test-domain-2", "existing-client", 30)
		if !success {
			t.Skip("Could not acquire initial lock, skipping test")
			return
		}

		// Create request for second client
		req := &pb.TryAcquireLockRequest{
			Service:  "test-service-2",
			Domain:   "test-domain-2",
			ClientId: "client-2",
			Ttl:      30,
		}

		// Execute the method
		resp, err := server.TryAcquireLock(ctx, req)

		// Log the response
		t.Logf("TryAcquireLock response: %+v", resp)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.IsLeader, "Expected not to acquire lock")

		// Clean up
		server.store.ReleaseLock(ctx, "test-service-2", "test-domain-2", "existing-client")
	})

	// Test case 3: Acquire a lock after it expires
	t.Run("acquire_expired_lock", func(t *testing.T) {
		// First acquire the lock with a short TTL
		success := server.store.TryAcquireLock(ctx, "test-service-3", "test-domain-3", "existing-client", 1)
		if !success {
			t.Skip("Could not acquire initial lock, skipping test")
			return
		}

		// Wait for the lock to expire
		time.Sleep(3 * time.Second)

		// Create request for second client
		req := &pb.TryAcquireLockRequest{
			Service:  "test-service-3",
			Domain:   "test-domain-3",
			ClientId: "client-3",
			Ttl:      30,
		}

		// Execute the method
		resp, err := server.TryAcquireLock(ctx, req)

		// Log the response
		t.Logf("TryAcquireLock response: %+v", resp)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.IsLeader, "Expected to acquire lock after expiration")

		// Clean up
		server.store.ReleaseLock(ctx, "test-service-3", "test-domain-3", "client-3")
	})
}

func TestReleaseLockWithDynamoDB(t *testing.T) {
	server, ctx := setupTestServerWithDynamoDB(t)
	if server == nil {
		return // Test was skipped
	}

	defer func() {
		if err := server.Stop(); err != nil {
			t.Errorf("failed to stop server: %v", err)
		}
	}()

	// Test case: Release an existing lock
	t.Run("release_existing_lock", func(t *testing.T) {
		// First acquire the lock
		success := server.store.TryAcquireLock(ctx, "test-service-1", "test-domain-1", "client-1", 30)
		if !success {
			t.Skip("Could not acquire initial lock, skipping test")
			return
		}

		// Create release request
		req := &pb.ReleaseLockRequest{
			Service:  "test-service-1",
			Domain:   "test-domain-1",
			ClientId: "client-1",
		}

		// Execute the release
		resp, err := server.ReleaseLock(ctx, req)

		// Log the response
		t.Logf("ReleaseLock response: %+v", resp)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		// Verify the lock was released by trying to acquire it again
		time.Sleep(time.Second)
		success = server.store.TryAcquireLock(ctx, "test-service-1", "test-domain-1", "client-1", 30)
		assert.True(t, success, "Should be able to acquire the lock after release")

		// Clean up
		server.store.ReleaseLock(ctx, "test-service-1", "test-domain-1", "client-1")
	})
}

func TestKeepAliveWithDynamoDB(t *testing.T) {
	server, ctx := setupTestServerWithDynamoDB(t)
	if server == nil {
		return // Test was skipped
	}

	defer func() {
		if err := server.Stop(); err != nil {
			t.Errorf("failed to stop server: %v", err)
		}
	}()

	// Test case: Keep alive an existing lock
	t.Run("keep_alive_existing_lock", func(t *testing.T) {
		// First acquire the lock
		success := server.store.TryAcquireLock(ctx, "test-service-1", "test-domain-1", "client-1", 5)
		if !success {
			t.Skip("Could not acquire initial lock, skipping test")
			return
		}

		// Create keep alive request
		req := &pb.KeepAliveRequest{
			Service:  "test-service-1",
			Domain:   "test-domain-1",
			ClientId: "client-1",
			Ttl:      30,
		}

		// Execute the keep alive
		resp, err := server.KeepAlive(ctx, req)

		// Log the response
		t.Logf("KeepAlive response: %+v", resp)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.LeaseLength)
		assert.Greater(t, resp.LeaseLength.Seconds, int64(0), "Expected positive lease length")

		// Verify the lock is still held
		time.Sleep(time.Second)
		success = server.store.TryAcquireLock(ctx, "test-service-1", "test-domain-1", "other-client", 30)
		assert.False(t, success, "Lock should still be held after keep-alive")

		// Clean up
		server.store.ReleaseLock(ctx, "test-service-1", "test-domain-1", "client-1")
	})
}

// Helper function to check if Redis is accessible
func isRedisAccessible(t *testing.T) bool {
	// Create Redis client with default settings
	client := redislib.NewClient(&redislib.Options{
		Addr:     "localhost:6379",
		Password: "", // No password by default
		DB:       0,  // Default DB
	})

	// Try to ping the Redis server
	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		t.Logf("Redis is not accessible: %v", err)
		return false
	}

	// Close the client
	client.Close()
	return true
}

func setupTestServerWithRedis(t *testing.T) (*Server[*redis.RedisConfig], context.Context) {
	// Check if Redis is accessible
	if !isRedisAccessible(t) {
		t.Skip("Skipping test because Redis is not accessible")
		return nil, nil
	}

	ctx := context.Background()

	// Create Redis config
	redisConfig := &redis.RedisConfig{
		Host:      "localhost",
		Port:      6379,
		Password:  "",
		DB:        0,
		TTL:       30,
		KeyPrefix: "test-lock",
		Endpoints: []string{"localhost:6379"},
		TableName: "locks_test",
	}

	cfg := &config.GlobalConfig[*redis.RedisConfig]{
		ServerAddress: "localhost:50053", // Use a different port to avoid conflicts
		Store:         redisConfig,
	}

	logger, err := observability.NewLogger(zapcore.InfoLevel, zap.Development())
	require.NoError(t, err)

	// Initialize metrics with proper configuration
	metricsConfig := observability.Config{
		ServiceName:    "test-server-redis",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		OTelEndpoint:   "localhost:4317",
	}

	serverMetrics, err := observability.NewMetricsClient(metricsConfig, logger)
	require.NoError(t, err)

	// Create store initializer function that uses Redis store
	storeInitializer := func(ctx context.Context, cfg *config.GlobalConfig[*redis.RedisConfig], logger *observability.SLogger) (store.Store, error) {
		// Return a new store instance
		str, err := redis.New(ctx, cfg.Store, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create Redis store: %w", err)
		}

		// Wait for the store to be initialized
		time.Sleep(time.Second * 2)

		return str, nil
	}

	// Initialize server
	server, err := NewServer(cfg, logger, serverMetrics, storeInitializer)
	require.NoError(t, err)

	// Initialize store
	err = server.initStore(ctx, storeInitializer)
	require.NoError(t, err)

	// Verify server and store are not nil
	require.NotNil(t, server, "server should not be nil")
	require.NotNil(t, server.store, "store should not be nil")

	// Clean Redis test keys
	cleanupRedisTestKeys(t, ctx)

	return server, ctx
}

// Helper to clean up test keys in Redis
func cleanupRedisTestKeys(t *testing.T, ctx context.Context) {
	client := redislib.NewClient(&redislib.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	defer client.Close()

	// Delete all keys with the test-lock prefix
	keys, err := client.Keys(ctx, "test-lock:*").Result()
	if err != nil {
		t.Logf("Error listing Redis keys: %v", err)
		return
	}

	if len(keys) > 0 {
		_, err = client.Del(ctx, keys...).Result()
		if err != nil {
			t.Logf("Error deleting Redis keys: %v", err)
		}
	}

	// Verify all test keys were deleted
	keys, _ = client.Keys(ctx, "test-lock:*").Result()
	if len(keys) > 0 {
		t.Logf("Warning: Some test keys were not deleted: %v", keys)
	}
}

func TestTryAcquireLockWithRedis(t *testing.T) {
	server, ctx := setupTestServerWithRedis(t)
	if server == nil {
		return // Test was skipped
	}

	defer func() {
		if err := server.Stop(); err != nil {
			t.Errorf("failed to stop server: %v", err)
		}
	}()

	// Test case 1: Acquire a new lock
	t.Run("acquire_new_lock", func(t *testing.T) {
		// Create request
		req := &pb.TryAcquireLockRequest{
			Service:  "test-service-1",
			Domain:   "test-domain-1",
			ClientId: "client-1",
			Ttl:      30,
		}

		// Execute the method
		resp, err := server.TryAcquireLock(ctx, req)

		// Log the response
		t.Logf("TryAcquireLock response: %+v", resp)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.IsLeader, "Expected to acquire lock")

		// Clean up
		server.store.ReleaseLock(ctx, "test-service-1", "test-domain-1", "client-1")
	})

	// Test case 2: Try to acquire an already held lock
	t.Run("acquire_existing_lock", func(t *testing.T) {
		// First acquire the lock
		success := server.store.TryAcquireLock(ctx, "test-service-2", "test-domain-2", "existing-client", 30)
		if !success {
			t.Skip("Could not acquire initial lock, skipping test")
			return
		}

		// Create request for second client
		req := &pb.TryAcquireLockRequest{
			Service:  "test-service-2",
			Domain:   "test-domain-2",
			ClientId: "client-2",
			Ttl:      30,
		}

		// Execute the method
		resp, err := server.TryAcquireLock(ctx, req)

		// Log the response
		t.Logf("TryAcquireLock response: %+v", resp)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.IsLeader, "Expected not to acquire lock")

		// Clean up
		server.store.ReleaseLock(ctx, "test-service-2", "test-domain-2", "existing-client")
	})

	// Test case 3: Acquire a lock after it expires
	t.Run("acquire_expired_lock", func(t *testing.T) {
		// First acquire the lock with a short TTL
		success := server.store.TryAcquireLock(ctx, "test-service-3", "test-domain-3", "existing-client", 1)
		if !success {
			t.Skip("Could not acquire initial lock, skipping test")
			return
		}

		// Wait for the lock to expire
		time.Sleep(3 * time.Second)

		// Create request for second client
		req := &pb.TryAcquireLockRequest{
			Service:  "test-service-3",
			Domain:   "test-domain-3",
			ClientId: "client-3",
			Ttl:      30,
		}

		// Execute the method
		resp, err := server.TryAcquireLock(ctx, req)

		// Log the response
		t.Logf("TryAcquireLock response: %+v", resp)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.IsLeader, "Expected to acquire lock after expiration")

		// Clean up
		server.store.ReleaseLock(ctx, "test-service-3", "test-domain-3", "client-3")
	})
}

func TestReleaseLockWithRedis(t *testing.T) {
	server, ctx := setupTestServerWithRedis(t)
	if server == nil {
		return // Test was skipped
	}

	defer func() {
		if err := server.Stop(); err != nil {
			t.Errorf("failed to stop server: %v", err)
		}
	}()

	// Test case: Release an existing lock
	t.Run("release_existing_lock", func(t *testing.T) {
		// First acquire the lock
		success := server.store.TryAcquireLock(ctx, "test-service-1", "test-domain-1", "client-1", 30)
		if !success {
			t.Skip("Could not acquire initial lock, skipping test")
			return
		}

		// Create release request
		req := &pb.ReleaseLockRequest{
			Service:  "test-service-1",
			Domain:   "test-domain-1",
			ClientId: "client-1",
		}

		// Execute the release
		resp, err := server.ReleaseLock(ctx, req)

		// Log the response
		t.Logf("ReleaseLock response: %+v", resp)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		// Verify the lock was released by trying to acquire it again
		time.Sleep(time.Second)
		success = server.store.TryAcquireLock(ctx, "test-service-1", "test-domain-1", "client-1", 30)
		assert.True(t, success, "Should be able to acquire the lock after release")

		// Clean up
		server.store.ReleaseLock(ctx, "test-service-1", "test-domain-1", "client-1")
	})

	// Test case: Release with wrong client ID
	t.Run("release_wrong_client_id", func(t *testing.T) {
		// First acquire the lock with one client
		success := server.store.TryAcquireLock(ctx, "test-service-2", "test-domain-2", "correct-client", 30)
		if !success {
			t.Skip("Could not acquire initial lock, skipping test")
			return
		}

		// Try to release with wrong client ID
		req := &pb.ReleaseLockRequest{
			Service:  "test-service-2",
			Domain:   "test-domain-2",
			ClientId: "wrong-client",
		}

		// Execute the release
		resp, err := server.ReleaseLock(ctx, req)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		// Verify the lock is still held by trying to acquire it with a new client
		success = server.store.TryAcquireLock(ctx, "test-service-2", "test-domain-2", "new-client", 30)
		assert.False(t, success, "Lock should still be held after release attempt with wrong client ID")

		// Clean up
		server.store.ReleaseLock(ctx, "test-service-2", "test-domain-2", "correct-client")
	})
}

func TestKeepAliveWithRedis(t *testing.T) {
	server, ctx := setupTestServerWithRedis(t)
	if server == nil {
		return // Test was skipped
	}

	defer func() {
		if err := server.Stop(); err != nil {
			t.Errorf("failed to stop server: %v", err)
		}
	}()

	// Test case: Keep alive an existing lock
	t.Run("keep_alive_existing_lock", func(t *testing.T) {
		// First acquire the lock
		success := server.store.TryAcquireLock(ctx, "test-service-1", "test-domain-1", "client-1", 5)
		if !success {
			t.Skip("Could not acquire initial lock, skipping test")
			return
		}

		// Create keep alive request
		req := &pb.KeepAliveRequest{
			Service:  "test-service-1",
			Domain:   "test-domain-1",
			ClientId: "client-1",
			Ttl:      30,
		}

		// Execute the keep alive
		resp, err := server.KeepAlive(ctx, req)

		// Log the response
		t.Logf("KeepAlive response: %+v", resp)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.LeaseLength)
		assert.Greater(t, resp.LeaseLength.Seconds, int64(0), "Expected positive lease length")

		// Verify the lock is still held
		time.Sleep(time.Second)
		success = server.store.TryAcquireLock(ctx, "test-service-1", "test-domain-1", "other-client", 30)
		assert.False(t, success, "Lock should still be held after keep-alive")

		// Clean up
		server.store.ReleaseLock(ctx, "test-service-1", "test-domain-1", "client-1")
	})

	// Test case: Keep alive with wrong client ID
	t.Run("keep_alive_wrong_client_id", func(t *testing.T) {
		// First acquire the lock with one client
		success := server.store.TryAcquireLock(ctx, "test-service-2", "test-domain-2", "correct-client", 5)
		if !success {
			t.Skip("Could not acquire initial lock, skipping test")
			return
		}

		// Try to keep alive with wrong client ID
		req := &pb.KeepAliveRequest{
			Service:  "test-service-2",
			Domain:   "test-domain-2",
			ClientId: "wrong-client",
			Ttl:      30,
		}

		// Execute the keep alive
		resp, err := server.KeepAlive(ctx, req)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.LeaseLength)
		assert.Equal(t, int64(-1), resp.LeaseLength.Seconds, "Expected negative lease length for wrong client ID")

		// Clean up
		server.store.ReleaseLock(ctx, "test-service-2", "test-domain-2", "correct-client")
	})
}
