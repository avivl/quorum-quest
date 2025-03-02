// internal/store/dynamodb/dynamodb_store.go
package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avivl/quorum-quest/internal/lockservice"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Error definitions
var (
	ErrNilConfig = errors.New("config cannot be nil")
	ErrNilLogger = errors.New("logger cannot be nil")
	StoreName    = "dynamodb"
)

// Store implements the store.Store interface for DynamoDB
type Store struct {
	client    *dynamodb.Client
	tableName string
	ttl       int32
	logger    *observability.SLogger
	config    *DynamoDBConfig
}

// Register the DynamoDB store with the lockservice package
func init() {
	lockservice.Register(StoreName, newStore)
}

// newStore creates a new DynamoDB store instance from configuration
func newStore(ctx context.Context, options lockservice.Config, logger *observability.SLogger) (store.LockStore, error) {
	cfg, ok := options.(*DynamoDBConfig)
	if !ok && options != nil {
		return nil, &store.InvalidConfigurationError{Store: StoreName, Config: options}
	}
	return NewStore(ctx, cfg, logger)
}

// GetConfig returns the current store configuration
func (s *Store) GetConfig() store.StoreConfig {
	return s.config
}

// NewStore creates a new DynamoDB store
func NewStore(ctx context.Context, config *DynamoDBConfig, logger *observability.SLogger) (*Store, error) {
	if config == nil {
		return nil, ErrNilConfig
	}

	if logger == nil {
		return nil, ErrNilLogger
	}

	// Validate config
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Create AWS client
	client, err := createDynamoDBClient(ctx, config, logger)
	if err != nil {
		return nil, err
	}

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

// createDynamoDBClient creates a new AWS DynamoDB client from configuration
func createDynamoDBClient(ctx context.Context, config *DynamoDBConfig, logger *observability.SLogger) (*dynamodb.Client, error) {
	var clientOpts []func(*awsconfig.LoadOptions) error

	// Add custom endpoint if provided
	if len(config.Endpoints) > 0 {
		clientOpts = append(clientOpts, awsconfig.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{URL: config.Endpoints[0]}, nil
				},
			),
		))
	}

	// Add static credentials if provided
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
	return dynamodb.NewFromConfig(awsConfig), nil
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

	// Create the table
	if err := s.createTable(ctx); err != nil {
		return err
	}

	// Wait for table to be active
	return s.waitForTableActive(ctx)
}

// createTable creates a new DynamoDB table
func (s *Store) createTable(ctx context.Context) error {
	_, err := s.client.CreateTable(ctx, &dynamodb.CreateTableInput{
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

	return nil
}

// waitForTableActive waits for the table to become active
func (s *Store) waitForTableActive(ctx context.Context) error {
	waiter := dynamodb.NewTableExistsWaiter(s.client)
	err := waiter.Wait(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(s.tableName),
	}, 5*time.Minute)

	if err != nil {
		s.logger.Errorf("Failed to wait for table creation: %v", err)
		return fmt.Errorf("failed to wait for table creation: %w", err)
	}

	return nil
}

// getLockKey generates a composite primary key for a lock
func (s *Store) getLockKey(service, domain string) string {
	return fmt.Sprintf("%s:%s", service, domain)
}

// resolveTTL returns the provided TTL or falls back to the default
func (s *Store) resolveTTL(ttl int32) int32 {
	if ttl > 0 {
		return ttl
	}
	return s.ttl
}

// TryAcquireLock attempts to acquire a lock
func (s *Store) TryAcquireLock(ctx context.Context, service, domain, clientId string, ttl int32) bool {
	pk := s.getLockKey(service, domain)
	lockTTL := s.resolveTTL(ttl)
	now := time.Now()
	expiryTime := now.Add(time.Duration(lockTTL) * time.Second).Unix()

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

// ReleaseLock releases a lock if owned by this client
func (s *Store) ReleaseLock(ctx context.Context, service, domain, clientId string) {
	pk := s.getLockKey(service, domain)

	// First check if this client owns the lock
	item, err := s.getLockItem(ctx, pk)
	if err != nil {
		s.logger.Errorf("Error checking lock ownership: %v", err)
		return
	}

	// If no lock exists or client doesn't own it, return
	if !s.ownsLock(item, clientId) {
		return
	}

	// Delete the lock
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

// getLockItem retrieves a lock item from DynamoDB
func (s *Store) getLockItem(ctx context.Context, pk string) (map[string]types.AttributeValue, error) {
	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
		},
	})

	if err != nil {
		return nil, err
	}

	return result.Item, nil
}

// ownsLock checks if the provided clientId owns the lock
func (s *Store) ownsLock(item map[string]types.AttributeValue, clientId string) bool {
	if item == nil {
		return false
	}

	clientIDAttr, ok := item["ClientID"]
	if !ok {
		return false
	}

	clientIDValue, ok := clientIDAttr.(*types.AttributeValueMemberS)
	if !ok {
		return false
	}

	return clientIDValue.Value == clientId
}

// KeepAlive refreshes a lock's TTL
func (s *Store) KeepAlive(ctx context.Context, service, domain, clientId string, ttl int32) time.Duration {
	pk := s.getLockKey(service, domain)
	lockTTL := s.resolveTTL(ttl)
	expiryTime := time.Now().Add(time.Duration(lockTTL) * time.Second).Unix()

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

	return time.Duration(lockTTL) * time.Second
}

// Close is a no-op for DynamoDB as the client doesn't need explicit closing
func (s *Store) Close() {
	// DynamoDB client doesn't need explicit closing
}
