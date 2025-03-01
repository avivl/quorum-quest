// internal/server/server_dynamodb_test.go
package server

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	pb "github.com/avivl/quorum-quest/api/gen/go/v1"
	"github.com/avivl/quorum-quest/internal/config"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/avivl/quorum-quest/internal/store/dynamodb"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// CustomDynamoDBMockStore is a custom implementation that we can fully control
// for testing without accessing unexported fields of dynamodb.MockStore
type CustomDynamoDBMockStore struct {
	mockClient dynamodb.DynamoDBClientInterface
	mockWaiter dynamodb.TableExistsWaiterAPI
	tableName  string
	ttl        int32
	logger     *observability.SLogger
	config     *dynamodb.DynamoDBConfig
}

func (s *CustomDynamoDBMockStore) GetConfig() store.StoreConfig {
	return s.config
}

func (s *CustomDynamoDBMockStore) TryAcquireLock(ctx context.Context, service, domain, clientId string, ttl int32) bool {
	// Create composite primary key
	pk := fmt.Sprintf("%s:%s", service, domain)

	// Set TTL
	_ttl := ttl
	if _ttl == 0 {
		_ttl = s.ttl
	}

	now := time.Now()
	expiryTime := now.Add(time.Duration(_ttl) * time.Second).Unix()

	// Try to insert the item with a condition that it doesn't exist or has expired
	_, err := s.mockClient.PutItem(ctx, &awsdynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item: map[string]types.AttributeValue{
			"PK":        &types.AttributeValueMemberS{Value: pk},
			"ClientID":  &types.AttributeValueMemberS{Value: clientId},
			"ExpiresAt": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expiryTime)},
		},
		ConditionExpression: aws.String("attribute_not_exists(PK) OR ExpiresAt < :now"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", now.Unix())},
		},
	})

	return err == nil
}

func (s *CustomDynamoDBMockStore) ReleaseLock(ctx context.Context, service, domain, clientId string) {
	// Create composite primary key
	pk := fmt.Sprintf("%s:%s", service, domain)

	// First check if this client owns the lock
	result, err := s.mockClient.GetItem(ctx, &awsdynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
		},
	})

	if err != nil {
		s.logger.Errorf("Error checking lock ownership: %v", err)
		return
	}

	// If item exists and client ID matches, delete it
	if result.Item != nil {
		if clientIDAttr, ok := result.Item["ClientID"]; ok {
			if clientIDAttr.(*types.AttributeValueMemberS).Value == clientId {
				_, err = s.mockClient.DeleteItem(ctx, &awsdynamodb.DeleteItemInput{
					TableName: aws.String(s.tableName),
					Key: map[string]types.AttributeValue{
						"PK": &types.AttributeValueMemberS{Value: pk},
					},
				})

				if err != nil {
					s.logger.Errorf("Error releasing lock: %v", err)
				}
			}
		}
	}
}

func (s *CustomDynamoDBMockStore) KeepAlive(ctx context.Context, service, domain, clientId string, ttl int32) time.Duration {
	// Create composite primary key
	pk := fmt.Sprintf("%s:%s", service, domain)

	// Set TTL
	_ttl := ttl
	if _ttl == 0 {
		_ttl = s.ttl
	}
	expiryTime := time.Now().Add(time.Duration(_ttl) * time.Second).Unix()

	// Update the item with a condition that it exists and client ID matches
	_, err := s.mockClient.UpdateItem(ctx, &awsdynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
		},
		UpdateExpression:    aws.String("SET ExpiresAt = :expiry"),
		ConditionExpression: aws.String("ClientID = :clientId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":expiry":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expiryTime)},
			":clientId": &types.AttributeValueMemberS{Value: clientId},
		},
	})

	if err != nil {
		s.logger.Debugf("Failed to keep lock alive: %v", err)
		return time.Duration(-1) * time.Second
	}

	return time.Duration(_ttl) * time.Second
}

func (s *CustomDynamoDBMockStore) EnsureTableExists(ctx context.Context) error {
	// Check if table exists
	_, err := s.mockClient.DescribeTable(ctx, &awsdynamodb.DescribeTableInput{
		TableName: aws.String(s.tableName),
	})

	if err == nil {
		// Table exists
		return nil
	}

	// Create table if it doesn't exist
	_, err = s.mockClient.CreateTable(ctx, &awsdynamodb.CreateTableInput{
		TableName: aws.String(s.tableName),
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
		s.logger.Errorf("Failed to create table: %v", err)
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Wait for table to be active
	if s.mockWaiter != nil {
		err = s.mockWaiter.Wait(ctx, &awsdynamodb.DescribeTableInput{
			TableName: aws.String(s.tableName),
		}, 5*time.Minute)

		if err != nil {
			s.logger.Errorf("Failed to wait for table creation: %v", err)
			return fmt.Errorf("failed to wait for table creation: %w", err)
		}
	}

	return nil
}

func (s *CustomDynamoDBMockStore) Close() {
	// DynamoDB client doesn't need explicit closing
}

func setupDynamoDBMockServer(t *testing.T) (*Server[*dynamodb.DynamoDBConfig], *dynamodb.MockDynamoDBClient, *dynamodb.MockTableExistsWaiter, context.Context) {
	ctx := context.Background()

	// Create DynamoDB config
	dynamoConfig := &dynamodb.DynamoDBConfig{
		Region:          "us-east-1",
		Table:           "test_table",
		TTL:             15,
		Endpoints:       []string{"http://localhost:8000"},
		AccessKeyID:     "dummy",
		SecretAccessKey: "dummy",
	}

	// Create global config with DynamoDB config
	cfg := &config.GlobalConfig[*dynamodb.DynamoDBConfig]{
		ServerAddress: "localhost:50053",
		Store:         dynamoConfig,
	}

	// Setup logger and metrics
	logger := setupTestLogger(t)
	serverMetrics := setupTestMetrics(t, logger)

	// Setup mock DynamoDB client and waiter
	mockDynamoClient := new(dynamodb.MockDynamoDBClient)
	mockWaiter := new(dynamodb.MockTableExistsWaiter)

	// Create the store initializer that returns our mock store
	storeInitializer := func(ctx context.Context, cfg *config.GlobalConfig[*dynamodb.DynamoDBConfig], logger *observability.SLogger) (store.Store, error) {
		// Create a custom mock store that uses our mock client
		// We need to create a custom implementation since we can't access unexported fields
		mockStore := &CustomDynamoDBMockStore{
			mockClient: mockDynamoClient,
			mockWaiter: mockWaiter,
			tableName:  cfg.Store.Table,
			ttl:        cfg.Store.TTL,
			logger:     logger,
			config:     cfg.Store,
		}
		return mockStore, nil
	}

	// Initialize server
	server, err := NewServer(cfg, logger, serverMetrics, storeInitializer)
	require.NoError(t, err)

	// Initialize store
	err = server.initStore(ctx, storeInitializer)
	require.NoError(t, err)

	return server, mockDynamoClient, mockWaiter, ctx
}

func TestTryAcquireLockWithDynamoDBMock(t *testing.T) {
	server, mockDynamoClient, _, ctx := setupDynamoDBMockServer(t)
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
				// Mock successful PutItem (no error = condition passed, lock acquired)
				mockDynamoClient.On("PutItem", mock.Anything, mock.MatchedBy(func(input *awsdynamodb.PutItemInput) bool {
					// Verify it's trying to insert with the correct key
					pk := input.Item["PK"].(*types.AttributeValueMemberS).Value
					return pk == "test-service-1:test-domain-1"
				})).Return(&awsdynamodb.PutItemOutput{}, nil).Once()
			},
		},
		{
			name:           "failed lock acquisition - lock already exists",
			service:        "test-service-2",
			domain:         "test-domain-2",
			clientID:       "client-2",
			ttl:            30,
			expectedResult: false,
			mockSetup: func() {
				// Mock failed PutItem (condition not met = lock exists and not expired)
				mockDynamoClient.On("PutItem", mock.Anything, mock.MatchedBy(func(input *awsdynamodb.PutItemInput) bool {
					// Verify it's trying to insert with the correct key
					pk := input.Item["PK"].(*types.AttributeValueMemberS).Value
					return pk == "test-service-2:test-domain-2"
				})).Return(nil, &types.ConditionalCheckFailedException{}).Once()
			},
		},
		{
			name:           "failed lock acquisition - error occurred",
			service:        "test-service-3",
			domain:         "test-domain-3",
			clientID:       "client-3",
			ttl:            30,
			expectedResult: false,
			mockSetup: func() {
				// Mock PutItem with a general error
				mockDynamoClient.On("PutItem", mock.Anything, mock.MatchedBy(func(input *awsdynamodb.PutItemInput) bool {
					// Verify it's trying to insert with the correct key
					pk := input.Item["PK"].(*types.AttributeValueMemberS).Value
					return pk == "test-service-3:test-domain-3"
				})).Return(nil, errors.New("internal error")).Once()
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
			mockDynamoClient.AssertExpectations(t)
		})
	}
}

func TestReleaseLockWithDynamoDBMock(t *testing.T) {
	server, mockDynamoClient, _, ctx := setupDynamoDBMockServer(t)
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
				// Mock GetItem to check if client owns the lock
				clientIDAttr := &types.AttributeValueMemberS{Value: "client-1"}
				mockDynamoClient.On("GetItem", mock.Anything, mock.MatchedBy(func(input *awsdynamodb.GetItemInput) bool {
					// Verify it's checking the correct key
					return *input.TableName == "test_table" &&
						input.Key["PK"].(*types.AttributeValueMemberS).Value == "test-service-1:test-domain-1"
				})).Return(&awsdynamodb.GetItemOutput{
					Item: map[string]types.AttributeValue{
						"PK":       &types.AttributeValueMemberS{Value: "test-service-1:test-domain-1"},
						"ClientID": clientIDAttr,
					},
				}, nil).Once()

				// Mock DeleteItem for deleting the lock
				mockDynamoClient.On("DeleteItem", mock.Anything, mock.MatchedBy(func(input *awsdynamodb.DeleteItemInput) bool {
					// Verify it's deleting the correct key
					return *input.TableName == "test_table" &&
						input.Key["PK"].(*types.AttributeValueMemberS).Value == "test-service-1:test-domain-1"
				})).Return(&awsdynamodb.DeleteItemOutput{}, nil).Once()
			},
		},
		{
			name:     "lock release no-op - client doesn't own lock",
			service:  "test-service-2",
			domain:   "test-domain-2",
			clientID: "wrong-client",
			mockSetup: func() {
				// Mock GetItem - lock exists but owned by a different client
				clientIDAttr := &types.AttributeValueMemberS{Value: "correct-client"}
				mockDynamoClient.On("GetItem", mock.Anything, mock.MatchedBy(func(input *awsdynamodb.GetItemInput) bool {
					// Verify it's checking the correct key
					return *input.TableName == "test_table" &&
						input.Key["PK"].(*types.AttributeValueMemberS).Value == "test-service-2:test-domain-2"
				})).Return(&awsdynamodb.GetItemOutput{
					Item: map[string]types.AttributeValue{
						"PK":       &types.AttributeValueMemberS{Value: "test-service-2:test-domain-2"},
						"ClientID": clientIDAttr,
					},
				}, nil).Once()

				// No DeleteItem should be called
			},
		},
		{
			name:     "lock release no-op - lock doesn't exist",
			service:  "test-service-3",
			domain:   "test-domain-3",
			clientID: "client-3",
			mockSetup: func() {
				// Mock GetItem - lock doesn't exist
				mockDynamoClient.On("GetItem", mock.Anything, mock.MatchedBy(func(input *awsdynamodb.GetItemInput) bool {
					// Verify it's checking the correct key
					return *input.TableName == "test_table" &&
						input.Key["PK"].(*types.AttributeValueMemberS).Value == "test-service-3:test-domain-3"
				})).Return(&awsdynamodb.GetItemOutput{
					Item: map[string]types.AttributeValue{}, // Empty Item means no record found
				}, nil).Once()

				// No DeleteItem should be called
			},
		},
		{
			name:     "lock release error - error checking ownership",
			service:  "test-service-4",
			domain:   "test-domain-4",
			clientID: "client-4",
			mockSetup: func() {
				// Mock GetItem with error
				mockDynamoClient.On("GetItem", mock.Anything, mock.MatchedBy(func(input *awsdynamodb.GetItemInput) bool {
					// Verify it's checking the correct key
					return *input.TableName == "test_table" &&
						input.Key["PK"].(*types.AttributeValueMemberS).Value == "test-service-4:test-domain-4"
				})).Return(nil, errors.New("network error")).Once()

				// No DeleteItem should be called
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
			mockDynamoClient.AssertExpectations(t)
		})
	}
}

func TestKeepAliveWithDynamoDBMock(t *testing.T) {
	server, mockDynamoClient, _, ctx := setupDynamoDBMockServer(t)
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
				// Mock UpdateItem for refreshing TTL - succeeds
				mockDynamoClient.On("UpdateItem", mock.Anything, mock.MatchedBy(func(input *awsdynamodb.UpdateItemInput) bool {
					// Verify it's updating the correct key
					return *input.TableName == "test_table" &&
						input.Key["PK"].(*types.AttributeValueMemberS).Value == "test-service-1:test-domain-1" &&
						*input.ConditionExpression == "ClientID = :clientId"
				})).Return(&awsdynamodb.UpdateItemOutput{}, nil).Once()
			},
		},
		{
			name:           "failed keep-alive - client doesn't own lock",
			service:        "test-service-2",
			domain:         "test-domain-2",
			clientID:       "wrong-client",
			ttl:            30,
			expectedStatus: -1, // Failure
			mockSetup: func() {
				// Mock UpdateItem with condition check failure (client doesn't own lock)
				mockDynamoClient.On("UpdateItem", mock.Anything, mock.MatchedBy(func(input *awsdynamodb.UpdateItemInput) bool {
					// Verify it's updating the correct key
					return *input.TableName == "test_table" &&
						input.Key["PK"].(*types.AttributeValueMemberS).Value == "test-service-2:test-domain-2" &&
						*input.ConditionExpression == "ClientID = :clientId"
				})).Return(nil, &types.ConditionalCheckFailedException{}).Once()
			},
		},
		{
			name:           "failed keep-alive - error occurred",
			service:        "test-service-3",
			domain:         "test-domain-3",
			clientID:       "client-3",
			ttl:            30,
			expectedStatus: -1, // Failure
			mockSetup: func() {
				// Mock UpdateItem with general error
				mockDynamoClient.On("UpdateItem", mock.Anything, mock.MatchedBy(func(input *awsdynamodb.UpdateItemInput) bool {
					// Verify it's updating the correct key
					return *input.TableName == "test_table" &&
						input.Key["PK"].(*types.AttributeValueMemberS).Value == "test-service-3:test-domain-3" &&
						*input.ConditionExpression == "ClientID = :clientId"
				})).Return(nil, errors.New("network error")).Once()
			},
		},
		{
			name:           "failed keep-alive - lock doesn't exist",
			service:        "test-service-4",
			domain:         "test-domain-4",
			clientID:       "client-4",
			ttl:            30,
			expectedStatus: -1, // Failure
			mockSetup: func() {
				// Mock UpdateItem with resource not found error
				mockDynamoClient.On("UpdateItem", mock.Anything, mock.MatchedBy(func(input *awsdynamodb.UpdateItemInput) bool {
					// Verify it's updating the correct key
					return *input.TableName == "test_table" &&
						input.Key["PK"].(*types.AttributeValueMemberS).Value == "test-service-4:test-domain-4" &&
						*input.ConditionExpression == "ClientID = :clientId"
				})).Return(nil, &types.ResourceNotFoundException{}).Once()
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
			mockDynamoClient.AssertExpectations(t)
		})
	}
}

func TestTableInitializationWithDynamoDBMock(t *testing.T) {
	_, mockDynamoClient, mockWaiter, ctx := setupDynamoDBMockServer(t)

	// Setup mock for table existence check
	mockDynamoClient.On("DescribeTable", mock.Anything, mock.MatchedBy(func(input *awsdynamodb.DescribeTableInput) bool {
		return *input.TableName == "test_table"
	})).Return(nil, &types.ResourceNotFoundException{}).Once()

	// Setup mock for table creation
	mockDynamoClient.On("CreateTable", mock.Anything, mock.MatchedBy(func(input *awsdynamodb.CreateTableInput) bool {
		return *input.TableName == "test_table"
	})).Return(&awsdynamodb.CreateTableOutput{}, nil).Once()

	// Setup mock for waiter
	mockWaiter.On("Wait", mock.Anything, mock.MatchedBy(func(input *awsdynamodb.DescribeTableInput) bool {
		return *input.TableName == "test_table"
	}), mock.Anything).Return(nil).Once()

	// Create a custom mock store
	customStore := &CustomDynamoDBMockStore{
		mockClient: mockDynamoClient,
		mockWaiter: mockWaiter,
		tableName:  "test_table",
		ttl:        15,
		logger:     setupTestLogger(t),
	}

	// Initialize the table
	err := customStore.EnsureTableExists(ctx)

	// Assertions
	assert.NoError(t, err)
	mockDynamoClient.AssertExpectations(t)
	mockWaiter.AssertExpectations(t)

	// Test the case where table already exists
	mockDynamoClient = new(dynamodb.MockDynamoDBClient)
	mockDynamoClient.On("DescribeTable", mock.Anything, mock.MatchedBy(func(input *awsdynamodb.DescribeTableInput) bool {
		return *input.TableName == "test_table"
	})).Return(&awsdynamodb.DescribeTableOutput{}, nil).Once()

	customStore.mockClient = mockDynamoClient
	err = customStore.EnsureTableExists(ctx)

	// Assertions
	assert.NoError(t, err)
	mockDynamoClient.AssertExpectations(t)
}

func TestServerWithDynamoDBConfigValidation(t *testing.T) {
	ctx := context.Background()
	logger := setupTestLogger(t)
	serverMetrics := setupTestMetrics(t, logger)

	// Test with invalid config (empty region)
	invalidConfig := &dynamodb.DynamoDBConfig{
		Region:          "", // Invalid: empty region
		Table:           "test_table",
		TTL:             15,
		Endpoints:       []string{"http://localhost:8000"},
		AccessKeyID:     "dummy",
		SecretAccessKey: "dummy",
	}

	cfg := &config.GlobalConfig[*dynamodb.DynamoDBConfig]{
		ServerAddress: "localhost:50053",
		Store:         invalidConfig,
	}

	storeInitializer := func(ctx context.Context, cfg *config.GlobalConfig[*dynamodb.DynamoDBConfig], logger *observability.SLogger) (store.Store, error) {
		// Return error when config validation fails
		err := cfg.Store.Validate()
		if err != nil {
			return nil, fmt.Errorf("invalid configuration: %w", err)
		}

		mockStore, _, _ := dynamodb.SetupMockStore()
		return mockStore, nil
	}

	// Initialize server with invalid config
	server, err := NewServer(cfg, logger, serverMetrics, storeInitializer)
	assert.NoError(t, err, "Server creation should succeed even with invalid store config")

	// Try to initialize store - should fail due to validation
	err = server.initStore(ctx, storeInitializer)
	assert.Error(t, err, "Store initialization should fail due to invalid config")
	assert.Contains(t, err.Error(), "region is required")
}
