// internal/store/dynamodb/dynamodb_store.go
package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Store implements the store.Store interface for DynamoDB
type Store struct {
	client    *dynamodb.Client
	tableName string
	ttl       int32
	logger    *observability.SLogger
	config    *DynamoDBConfig
}

// Add this method to the Store struct to implement the store.Store interface
func (s *Store) GetConfig() store.StoreConfig {
	return s.config
}

// NewStore creates a new DynamoDB store
func NewStore(ctx context.Context, config *DynamoDBConfig, logger *observability.SLogger) (*Store, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	if logger == nil {
		return nil, errors.New("logger cannot be nil")
	}

	// Validate config
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Create AWS configuration
	var clientOpts []func(*awsconfig.LoadOptions) error

	// Use custom endpoint if provided
	if len(config.Endpoints) > 0 {
		clientOpts = append(clientOpts, awsconfig.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{URL: config.Endpoints[0]}, nil
				},
			),
		))
	}

	// Use static credentials if provided
	if config.AccessKeyID != "" && config.SecretAccessKey != "" {
		clientOpts = append(clientOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(config.AccessKeyID, config.SecretAccessKey, ""),
		))
	}

	// Set region
	clientOpts = append(clientOpts, awsconfig.WithRegion(config.Region))

	// Create AWS config
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, clientOpts...)
	if err != nil {
		logger.Errorf("Failed to load AWS config: %v", err)
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create DynamoDB client
	client := dynamodb.NewFromConfig(awsConfig)

	// Create store
	store := &Store{
		client:    client,
		tableName: config.Table,
		ttl:       config.TTL,
		logger:    logger,
		config:    config,
	}

	// Ensure table exists
	if err := store.ensureTableExists(ctx); err != nil {
		return nil, err
	}

	return store, nil
}

// ensureTableExists checks if the DynamoDB table exists and creates it if it doesn't
func (s *Store) ensureTableExists(ctx context.Context) error {
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
	waiter := dynamodb.NewTableExistsWaiter(s.client)
	err = waiter.Wait(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(s.tableName),
	}, 5*time.Minute)

	if err != nil {
		s.logger.Errorf("Failed to wait for table creation: %v", err)
		return fmt.Errorf("failed to wait for table creation: %w", err)
	}

	return nil
}

// TryAcquireLock attempts to acquire a lock
func (s *Store) TryAcquireLock(ctx context.Context, service, domain, clientId string, ttl int32) bool {
	// Create composite primary key
	pk := fmt.Sprintf("%s:%s", service, domain)

	// Set TTL
	_ttl := ttl
	if _ttl == 0 {
		_ttl = s.ttl
	}

	now := time.Now()
	expiryTime := now.Add(time.Duration(_ttl) * time.Second).Unix()

	// Try to insert the item with a condition that:
	// 1. It doesn't exist, OR
	// 2. It has expired, OR
	// 3. The current lock is held by the same client
	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item: map[string]types.AttributeValue{
			"PK":        &types.AttributeValueMemberS{Value: pk},
			"ClientID":  &types.AttributeValueMemberS{Value: clientId},
			"ExpiresAt": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expiryTime)},
		},
		ConditionExpression: aws.String(
			"attribute_not_exists(PK) OR " + // Lock doesn't exist
				"ExpiresAt < :now OR " + // Lock has expired
				"(ClientID = :clientId AND ExpiresAt >= :now)", // Same client can renew its lock
		),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now":      &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", now.Unix())},
			":clientId": &types.AttributeValueMemberS{Value: clientId},
		},
	})

	return err == nil
}

// ReleaseLock releases a lock
func (s *Store) ReleaseLock(ctx context.Context, service, domain, clientId string) {
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
func (s *Store) KeepAlive(ctx context.Context, service, domain, clientId string, ttl int32) time.Duration {
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
func (s *Store) Close() {
	// DynamoDB client doesn't need explicit closing
}
