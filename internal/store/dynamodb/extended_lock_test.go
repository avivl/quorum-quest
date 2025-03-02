// internal/store/dynamodb/extended_lock_test.go
package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestGetLockKeyFormat tests that the key format is consistent
func TestGetLockKeyFormat(t *testing.T) {
	mockStore, _, _ := SetupMockStore()

	testCases := []struct {
		service  string
		domain   string
		expected string
	}{
		{"service1", "domain1", "service1:domain1"},
		{"service with space", "domain", "service with space:domain"},
		{"", "", ":"},
		{"service-123", "domain_456", "service-123:domain_456"},
		{"service:with:colons", "domain", "service:with:colons:domain"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s_%s", tc.service, tc.domain), func(t *testing.T) {
			key := mockStore.getLockKey(tc.service, tc.domain)
			assert.Equal(t, tc.expected, key)
		})
	}
}

// TestTryAcquireLockExhaustive tests more edge cases for TryAcquireLock
func TestTryAcquireLockExhaustive(t *testing.T) {
	t.Run("expired_lock_acquisition", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Mock PutItem to succeed with condition check (simulating an expired lock)
		mockClient.On("PutItem", ctx, mock.MatchedBy(func(input *dynamodb.PutItemInput) bool {
			// Verify condition expression contains expired check
			assert.Contains(t, *input.ConditionExpression, "ExpiresAt < :now")

			// Verify that now timestamp is present
			_, ok := input.ExpressionAttributeValues[":now"]
			assert.True(t, ok, "Should have :now attribute value")

			return true
		})).Return(&dynamodb.PutItemOutput{}, nil)

		// Call method under test
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result
		assert.True(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("empty_service_and_domain", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := ""
		domain := ""
		clientID := "client-1"
		ttl := int32(30)

		// Mock PutItem to succeed
		mockClient.On("PutItem", ctx, mock.MatchedBy(func(input *dynamodb.PutItemInput) bool {
			// Verify PK matches empty service and domain format
			pkAttr, ok := input.Item["PK"]
			assert.True(t, ok, "Should have PK attribute")
			pkValue, ok := pkAttr.(*types.AttributeValueMemberS)
			assert.True(t, ok, "PK should be string type")
			assert.Equal(t, ":", pkValue.Value, "PK should be : for empty service and domain")
			return true
		})).Return(&dynamodb.PutItemOutput{}, nil)

		// Call method under test
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result
		assert.True(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("conditional_check_failed_exception", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Create generic conditional check failed error
		ccfErr := fmt.Errorf("conditional check failed: the conditional request failed")

		// Mock PutItem to fail with conditional check failed
		mockClient.On("PutItem", ctx, mock.Anything).Return(nil, ccfErr)

		// Call method under test
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result
		assert.False(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("internal_service_error", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Create a generic internal server error
		intErr := fmt.Errorf("internal server error")

		// Mock PutItem to fail with internal error
		mockClient.On("PutItem", ctx, mock.Anything).Return(nil, intErr)

		// Call method under test
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result
		assert.False(t, result)
		mockClient.AssertExpectations(t)
	})
}

// TestReleaseLockExhaustive tests more edge cases for ReleaseLock
func TestReleaseLockExhaustive(t *testing.T) {
	t.Run("special_characters_in_key", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test:service"
		domain := "test-domain@123"
		clientID := "client-1"
		pk := fmt.Sprintf("%s:%s", service, domain)

		// Mock GetItem with complex key
		mockClient.On("GetItem", ctx, mock.MatchedBy(func(input *dynamodb.GetItemInput) bool {
			// Verify the key used in GetItem has special characters
			assert.NotNil(t, input.Key)
			pkAttr, ok := input.Key["PK"]
			assert.True(t, ok, "PK should exist in key")
			pkValue, ok := pkAttr.(*types.AttributeValueMemberS)
			assert.True(t, ok, "PK should be string type")
			assert.Equal(t, pk, pkValue.Value, "PK should match expected format with special chars")
			return true
		})).Return(&dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"PK":       &types.AttributeValueMemberS{Value: pk},
				"ClientID": &types.AttributeValueMemberS{Value: clientID},
			},
		}, nil)

		// Mock DeleteItem to succeed
		mockClient.On("DeleteItem", ctx, mock.Anything).Return(&dynamodb.DeleteItemOutput{}, nil)

		// Call method under test
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations
		mockClient.AssertExpectations(t)
	})

	t.Run("empty_item_response", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"

		// Mock GetItem with empty response
		mockClient.On("GetItem", ctx, mock.Anything).Return(&dynamodb.GetItemOutput{
			// Empty response, no Item
		}, nil)

		// Call method under test
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations - DeleteItem should not be called
		mockClient.AssertExpectations(t)
		mockClient.AssertNotCalled(t, "DeleteItem")
	})

	t.Run("nil_item_response", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"

		// Mock GetItem with nil Item response
		mockClient.On("GetItem", ctx, mock.Anything).Return(&dynamodb.GetItemOutput{
			Item: nil,
		}, nil)

		// Call method under test
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations - DeleteItem should not be called
		mockClient.AssertExpectations(t)
		mockClient.AssertNotCalled(t, "DeleteItem")
	})

	t.Run("delete_item_error", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		pk := fmt.Sprintf("%s:%s", service, domain)

		// Mock GetItem to return ownership to this client
		mockClient.On("GetItem", ctx, mock.Anything).Return(&dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"PK":       &types.AttributeValueMemberS{Value: pk},
				"ClientID": &types.AttributeValueMemberS{Value: clientID},
			},
		}, nil)

		// Mock DeleteItem to return error
		deleteErr := errors.New("delete operation failed")
		mockClient.On("DeleteItem", ctx, mock.Anything).Return(nil, deleteErr)

		// Call method under test
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations - errors should be handled gracefully
		mockClient.AssertExpectations(t)
	})
}

// TestKeepAliveExhaustive tests more edge cases for KeepAlive
func TestKeepAliveExhaustive(t *testing.T) {
	t.Run("update_expression_format", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Mock UpdateItem and capture input
		var capturedInput *dynamodb.UpdateItemInput
		mockClient.On("UpdateItem", ctx, mock.MatchedBy(func(input *dynamodb.UpdateItemInput) bool {
			capturedInput = input
			return true
		})).Return(&dynamodb.UpdateItemOutput{}, nil)

		// Call method under test
		result := mockStore.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result
		assert.Equal(t, time.Duration(ttl)*time.Second, result)

		// Verify update expression format
		assert.NotNil(t, capturedInput)
		assert.Equal(t, "SET ExpiresAt = :expiry", *capturedInput.UpdateExpression)
		assert.Equal(t, "ClientID = :clientId", *capturedInput.ConditionExpression)

		// Verify attribute values
		clientIdAttr, ok := capturedInput.ExpressionAttributeValues[":clientId"]
		assert.True(t, ok)
		assert.Equal(t, clientID, clientIdAttr.(*types.AttributeValueMemberS).Value)

		expiryAttr, ok := capturedInput.ExpressionAttributeValues[":expiry"]
		assert.True(t, ok)
		expiryValue, ok := expiryAttr.(*types.AttributeValueMemberN)
		assert.True(t, ok)

		// Parse expiry value
		expiryTimestamp, err := strconv.ParseInt(expiryValue.Value, 10, 64)
		assert.NoError(t, err)

		// Should be roughly now + ttl seconds
		expectedExpiry := time.Now().Unix() + int64(ttl)
		assert.InDelta(t, expectedExpiry, expiryTimestamp, 5)

		mockClient.AssertExpectations(t)
	})

	t.Run("expired_item_behavior", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Create a generic error for conditional check failure
		ccfErr := fmt.Errorf("conditional check failed: the conditional request failed")

		// Mock UpdateItem to fail with conditional check failed (expired or wrong client)
		mockClient.On("UpdateItem", ctx, mock.Anything).Return(nil, ccfErr)

		// Call method under test
		result := mockStore.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result - should indicate failure
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("connection_error", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Create a connection error
		connErr := errors.New("connection error")

		// Mock UpdateItem to fail with connection error
		mockClient.On("UpdateItem", ctx, mock.Anything).Return(nil, connErr)

		// Call method under test
		result := mockStore.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result - should indicate failure
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("throttling_error", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Create a generic error for throttling
		throttlingErr := fmt.Errorf("throttling error: rate exceeded")

		// Mock UpdateItem to fail with throttling error
		mockClient.On("UpdateItem", ctx, mock.Anything).Return(nil, throttlingErr)

		// Call method under test
		result := mockStore.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result - should indicate failure
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("resource_not_found", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Create a generic error for resource not found
		resourceErr := fmt.Errorf("resource not found error: table not found")

		// Mock UpdateItem to fail with resource not found
		mockClient.On("UpdateItem", ctx, mock.Anything).Return(nil, resourceErr)

		// Call method under test
		result := mockStore.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result - should indicate failure
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockClient.AssertExpectations(t)
	})
}
