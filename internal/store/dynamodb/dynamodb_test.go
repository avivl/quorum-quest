// internal/store/dynamodb/dynamodb_test.go
package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Define interfaces for the AWS DynamoDB SDK components we use
type DynamoDBClientInterface interface {
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	CreateTable(ctx context.Context, params *dynamodb.CreateTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error)
	DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
}

// MockDynamoDBClient is a mock implementation of DynamoDBClientInterface
type MockDynamoDBClient struct {
	mock.Mock
}

// PutItem mocks the PutItem method
func (m *MockDynamoDBClient) PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.PutItemOutput), args.Error(1)
}

// GetItem mocks the GetItem method
func (m *MockDynamoDBClient) GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.GetItemOutput), args.Error(1)
}

// DeleteItem mocks the DeleteItem method
func (m *MockDynamoDBClient) DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.DeleteItemOutput), args.Error(1)
}

// UpdateItem mocks the UpdateItem method
func (m *MockDynamoDBClient) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.UpdateItemOutput), args.Error(1)
}

// CreateTable mocks the CreateTable method
func (m *MockDynamoDBClient) CreateTable(ctx context.Context, params *dynamodb.CreateTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.CreateTableOutput), args.Error(1)
}

// DescribeTable mocks the DescribeTable method
func (m *MockDynamoDBClient) DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.DescribeTableOutput), args.Error(1)
}

// MockStore is a custom version of Store for testing that accepts our mock interface
type MockStore struct {
	client    DynamoDBClientInterface
	tableName string
	ttl       int32
	logger    *observability.SLogger
	config    *DynamoDBConfig
}

// GetConfig implements store.Store interface
func (s *MockStore) GetConfig() store.StoreConfig {
	return s.config
}

// TryAcquireLock attempts to acquire a lock
func (s *MockStore) TryAcquireLock(ctx context.Context, service, domain, clientId string, ttl int32) bool {
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
	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
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

// ReleaseLock releases a lock
func (s *MockStore) ReleaseLock(ctx context.Context, service, domain, clientId string) {
	// Create composite primary key
	pk := fmt.Sprintf("%s:%s", service, domain)

	// First check if this client owns the lock
	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
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
				_, err = s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
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

// KeepAlive refreshes a lock's TTL
func (s *MockStore) KeepAlive(ctx context.Context, service, domain, clientId string, ttl int32) time.Duration {
	// Create composite primary key
	pk := fmt.Sprintf("%s:%s", service, domain)

	// Set TTL
	_ttl := ttl
	if _ttl == 0 {
		_ttl = s.ttl
	}
	expiryTime := time.Now().Add(time.Duration(_ttl) * time.Second).Unix()

	// Update the item with a condition that it exists and client ID matches
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
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

// Close closes the DynamoDB client
func (s *MockStore) Close() {
	// DynamoDB client doesn't need explicit closing
}

// SetupMockStore creates a MockStore with a mocked DynamoDB client for testing
func SetupMockStore() (*MockStore, *MockDynamoDBClient) {
	mockClient := new(MockDynamoDBClient)
	logger, _, _ := observability.NewTestLogger()

	config := &DynamoDBConfig{
		Region:          "us-east-1",
		Table:           "test_table",
		TTL:             15,
		Endpoints:       []string{"http://localhost:8000"},
		AccessKeyID:     "dummy",
		SecretAccessKey: "dummy",
	}

	store := &MockStore{
		client:    mockClient,
		tableName: config.Table,
		ttl:       config.TTL,
		logger:    logger,
		config:    config,
	}

	return store, mockClient
}

func TestNewStore(t *testing.T) {
	logger, _, _ := observability.NewTestLogger()

	t.Run("nil_config", func(t *testing.T) {
		store, err := NewStore(context.Background(), nil, logger)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "config cannot be nil")
	})

	t.Run("validates_config", func(t *testing.T) {
		config := &DynamoDBConfig{
			Region: "us-east-1",
			// Missing required fields
		}

		store, err := NewStore(context.Background(), config, logger)
		assert.Error(t, err)
		assert.Nil(t, store)
	})
}

func TestTryAcquireLock(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Mock successful PutItem
		mockClient.On("PutItem", ctx, mock.Anything).Return(&dynamodb.PutItemOutput{}, nil)

		// Call method under test
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.True(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("failure_conditional_check", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Create ConditionalCheckFailedException
		ccfErr := &types.ConditionalCheckFailedException{Message: aws.String("The conditional request failed")}

		// Mock failed PutItem with conditional check failure
		mockClient.On("PutItem", ctx, mock.Anything).Return(nil, ccfErr)

		// Call method under test
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.False(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("failure_other_error", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Create a different error
		otherErr := errors.New("some other error")

		// Mock failed PutItem with other error
		mockClient.On("PutItem", ctx, mock.Anything).Return(nil, otherErr)

		// Call method under test
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.False(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("default_ttl", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"

		// Mock successful PutItem
		mockClient.On("PutItem", ctx, mock.Anything).Return(&dynamodb.PutItemOutput{}, nil)

		// Call method under test with ttl=0 to use default
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, 0)

		// Verify result and expectations
		assert.True(t, result)
		mockClient.AssertExpectations(t)
	})
}

func TestReleaseLock(t *testing.T) {
	t.Run("success_when_owns_lock", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		pk := fmt.Sprintf("%s:%s", service, domain)

		// Mock GetItem response with matching clientID
		getItemOutput := &dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"PK":       &types.AttributeValueMemberS{Value: pk},
				"ClientID": &types.AttributeValueMemberS{Value: clientID},
			},
		}
		mockClient.On("GetItem", ctx, mock.Anything).Return(getItemOutput, nil)

		// Mock successful DeleteItem
		mockClient.On("DeleteItem", ctx, mock.Anything).Return(&dynamodb.DeleteItemOutput{}, nil)

		// Call method under test
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations
		mockClient.AssertExpectations(t)
	})

	t.Run("noop_when_doesnt_own_lock", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		differentClientID := "client-2"
		pk := fmt.Sprintf("%s:%s", service, domain)

		// Mock GetItem response with different clientID
		getItemOutput := &dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"PK":       &types.AttributeValueMemberS{Value: pk},
				"ClientID": &types.AttributeValueMemberS{Value: differentClientID},
			},
		}
		mockClient.On("GetItem", ctx, mock.Anything).Return(getItemOutput, nil)

		// No DeleteItem call expected since client IDs don't match

		// Call method under test
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations
		mockClient.AssertExpectations(t)
	})

	t.Run("noop_when_lock_not_found", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"

		// Mock empty GetItem response (no item found)
		getItemOutput := &dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{},
		}
		mockClient.On("GetItem", ctx, mock.Anything).Return(getItemOutput, nil)

		// No DeleteItem call expected since no item found

		// Call method under test
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations
		mockClient.AssertExpectations(t)
	})

	t.Run("logs_error_on_getitem_failure", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"

		// Mock GetItem error
		getItemError := errors.New("getitem failed")
		mockClient.On("GetItem", ctx, mock.Anything).Return(nil, getItemError)

		// No DeleteItem call expected due to GetItem error

		// Call method under test
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations
		mockClient.AssertExpectations(t)
	})

	t.Run("logs_error_on_deleteitem_failure", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		pk := fmt.Sprintf("%s:%s", service, domain)

		// Mock GetItem response with matching clientID
		getItemOutput := &dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"PK":       &types.AttributeValueMemberS{Value: pk},
				"ClientID": &types.AttributeValueMemberS{Value: clientID},
			},
		}
		mockClient.On("GetItem", ctx, mock.Anything).Return(getItemOutput, nil)

		// Mock DeleteItem error
		deleteItemError := errors.New("deleteitem failed")
		mockClient.On("DeleteItem", ctx, mock.Anything).Return(nil, deleteItemError)

		// Call method under test
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations
		mockClient.AssertExpectations(t)
	})
}

func TestKeepAlive(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Mock successful UpdateItem
		mockClient.On("UpdateItem", ctx, mock.Anything).Return(&dynamodb.UpdateItemOutput{}, nil)

		// Call method under test
		result := mockStore.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.Equal(t, time.Duration(ttl)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("default_ttl", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		defaultTtl := mockStore.ttl

		// Mock successful UpdateItem
		mockClient.On("UpdateItem", ctx, mock.Anything).Return(&dynamodb.UpdateItemOutput{}, nil)

		// Call method under test with ttl=0 to use default
		result := mockStore.KeepAlive(ctx, service, domain, clientID, 0)

		// Verify result and expectations
		assert.Equal(t, time.Duration(defaultTtl)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("conditional_check_failure", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Create ConditionalCheckFailedException
		ccfErr := &types.ConditionalCheckFailedException{Message: aws.String("The conditional request failed")}

		// Mock failed UpdateItem with conditional check failure
		mockClient.On("UpdateItem", ctx, mock.Anything).Return(nil, ccfErr)

		// Call method under test
		result := mockStore.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("other_error", func(t *testing.T) {
		mockStore, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Create a different error
		otherErr := errors.New("some other error")

		// Mock failed UpdateItem with other error
		mockClient.On("UpdateItem", ctx, mock.Anything).Return(nil, otherErr)

		// Call method under test
		result := mockStore.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockClient.AssertExpectations(t)
	})
}

func TestStoreInterfaceCompliance(t *testing.T) {
	t.Run("implements_store_interface", func(t *testing.T) {
		// Create a store instance
		mockStore, _ := SetupMockStore()

		// Check if it implements the store.Store interface
		var i store.Store = mockStore
		assert.NotNil(t, i)
	})
}

func TestStoreConfig(t *testing.T) {
	t.Run("returns_config", func(t *testing.T) {
		// Create a store
		mockStore, _ := SetupMockStore()

		// Call method under test
		result := mockStore.GetConfig()

		// Verify the result is the same config
		assert.Equal(t, mockStore.config, result)
	})
}

func TestCloseStore(t *testing.T) {
	t.Run("close_succeeds", func(t *testing.T) {
		// Create a store
		mockStore, _ := SetupMockStore()

		// Call method under test - should not panic
		mockStore.Close()
	})
}
