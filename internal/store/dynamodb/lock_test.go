// internal/store/dynamodb/lock_test.go
package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTryAcquireLock(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
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
		mockStore, mockClient, _ := SetupMockStore()
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
		mockStore, mockClient, _ := SetupMockStore()
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
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		defaultTtl := mockStore.ttl

		// Capture the input to validate TTL usage
		var capturedInput *dynamodb.PutItemInput

		// Mock successful PutItem and capture input
		mockClient.On("PutItem", ctx, mock.MatchedBy(func(input *dynamodb.PutItemInput) bool {
			capturedInput = input
			return true
		})).Return(&dynamodb.PutItemOutput{}, nil)

		// Call method under test with ttl=0 to use default
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, 0)

		// Verify result
		assert.True(t, result)

		// Verify that ExpiresAt was set using the default TTL
		assert.NotNil(t, capturedInput)
		assert.NotNil(t, capturedInput.Item)
		expiresAtAttr, ok := capturedInput.Item["ExpiresAt"]
		assert.True(t, ok, "ExpiresAt attribute should exist")

		// Verify the expiry time is within expected range
		expiresAtValue, ok := expiresAtAttr.(*types.AttributeValueMemberN)
		assert.True(t, ok, "ExpiresAt should be a numeric attribute")

		// Parse the expiry timestamp
		expiryTimestamp, err := strconv.ParseInt(expiresAtValue.Value, 10, 64)
		assert.NoError(t, err, "Should be able to parse expiry timestamp")

		// Check that the expiry is roughly now + defaultTtl seconds (allow 5 second margin)
		expectedExpiry := time.Now().Unix() + int64(defaultTtl)
		assert.InDelta(t, expectedExpiry, expiryTimestamp, 5, "Expiry time should be roughly now + TTL")

		mockClient.AssertExpectations(t)
	})
}

func TestReleaseLock(t *testing.T) {
	t.Run("success_when_owns_lock", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
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
		mockStore, mockClient, _ := SetupMockStore()
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
		mockClient.AssertNotCalled(t, "DeleteItem")
	})

	t.Run("noop_when_lock_not_found", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
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
		mockClient.AssertNotCalled(t, "DeleteItem")
	})

	t.Run("handles_missing_client_id_field", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		pk := fmt.Sprintf("%s:%s", service, domain)

		// Mock GetItem response without ClientID field
		getItemOutput := &dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: pk},
				// ClientID field missing
			},
		}
		mockClient.On("GetItem", ctx, mock.Anything).Return(getItemOutput, nil)

		// No DeleteItem call expected since ClientID field is missing

		// Call method under test
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations
		mockClient.AssertExpectations(t)
		mockClient.AssertNotCalled(t, "DeleteItem")
	})

	t.Run("logs_error_on_getitem_failure", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
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
		mockClient.AssertNotCalled(t, "DeleteItem")
	})

	t.Run("logs_error_on_deleteitem_failure", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
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
		mockStore, mockClient, _ := SetupMockStore()
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
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		defaultTtl := mockStore.ttl

		// Capture the input to validate TTL usage
		var capturedInput *dynamodb.UpdateItemInput

		// Mock successful UpdateItem and capture input
		mockClient.On("UpdateItem", ctx, mock.MatchedBy(func(input *dynamodb.UpdateItemInput) bool {
			capturedInput = input
			return true
		})).Return(&dynamodb.UpdateItemOutput{}, nil)

		// Call method under test with ttl=0 to use default
		result := mockStore.KeepAlive(ctx, service, domain, clientID, 0)

		// Verify result
		assert.Equal(t, time.Duration(defaultTtl)*time.Second, result)

		// Verify expiry value in the update expression
		assert.NotNil(t, capturedInput)
		expiryAttr, ok := capturedInput.ExpressionAttributeValues[":expiry"]
		assert.True(t, ok, "Should have :expiry attribute value")

		expiryValue, ok := expiryAttr.(*types.AttributeValueMemberN)
		assert.True(t, ok, "Expiry should be a numeric attribute")

		// Parse the expiry timestamp
		expiryTimestamp, err := strconv.ParseInt(expiryValue.Value, 10, 64)
		assert.NoError(t, err, "Should be able to parse expiry timestamp")

		// Check that the expiry is roughly now + defaultTtl seconds (allow 5 second margin)
		expectedExpiry := time.Now().Unix() + int64(defaultTtl)
		assert.InDelta(t, expectedExpiry, expiryTimestamp, 5, "Expiry time should be roughly now + TTL")

		mockClient.AssertExpectations(t)
	})

	t.Run("conditional_check_failure", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
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
		mockStore, mockClient, _ := SetupMockStore()
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
