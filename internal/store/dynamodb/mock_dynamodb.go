// internal/store/dynamodb/mock_dynamodb.go
package dynamodb

import (
	"context"
	"fmt"
	"time"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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

// TableExistsWaiterAPI defines the Wait method for the TableExistsWaiter
type TableExistsWaiterAPI interface {
	Wait(ctx context.Context, params *dynamodb.DescribeTableInput, maxWaitDur time.Duration, optFns ...func(*dynamodb.TableExistsWaiterOptions)) error
}

// MockTableExistsWaiter is a mock implementation of TableExistsWaiterAPI
type MockTableExistsWaiter struct {
	mock.Mock
}

// Wait mocks the Wait method of the TableExistsWaiter
func (m *MockTableExistsWaiter) Wait(ctx context.Context, params *dynamodb.DescribeTableInput, maxWaitDur time.Duration, optFns ...func(*dynamodb.TableExistsWaiterOptions)) error {
	args := m.Called(ctx, params, maxWaitDur)
	return args.Error(0)
}

// MockStore is a custom version of Store for testing that accepts our mock interface
type MockStore struct {
	client    DynamoDBClientInterface
	tableName string
	ttl       int32
	logger    *observability.SLogger
	config    *DynamoDBConfig
	waiter    TableExistsWaiterAPI
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

// ensureTableExists for the Mock Store
func (s *MockStore) ensureTableExists(ctx context.Context) error {
	// Check if table exists
	_, err := s.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(s.tableName),
	})

	if err == nil {
		// Table exists
		return nil
	}

	// Create table if it doesn't exist
	_, err = s.client.CreateTable(ctx, &dynamodb.CreateTableInput{
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
	if s.waiter != nil {
		err = s.waiter.Wait(ctx, &dynamodb.DescribeTableInput{
			TableName: aws.String(s.tableName),
		}, 5*time.Minute)

		if err != nil {
			s.logger.Errorf("Failed to wait for table creation: %v", err)
			return fmt.Errorf("failed to wait for table creation: %w", err)
		}
	}

	return nil
}

// Close closes the DynamoDB client
func (s *MockStore) Close() {
	// DynamoDB client doesn't need explicit closing
}

// SetupMockStore creates a MockStore with a mocked DynamoDB client for testing
func SetupMockStore() (*MockStore, *MockDynamoDBClient, *MockTableExistsWaiter) {
	mockClient := new(MockDynamoDBClient)
	mockWaiter := new(MockTableExistsWaiter)
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
		waiter:    mockWaiter,
	}

	return store, mockClient, mockWaiter
}
