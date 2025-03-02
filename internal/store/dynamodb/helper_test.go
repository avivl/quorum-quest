// internal/store/dynamodb/helper_test.go
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

// TestResolveTTL tests the TTL fallback logic indirectly through TryAcquireLock
func TestResolveTTL(t *testing.T) {
	t.Run("use_provided_ttl", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		explicitTTL := int32(30)

		// Capture the input to validate TTL usage
		var capturedInput *dynamodb.PutItemInput

		// Mock successful PutItem and capture input
		mockClient.On("PutItem", ctx, mock.MatchedBy(func(input *dynamodb.PutItemInput) bool {
			capturedInput = input
			return true
		})).Return(&dynamodb.PutItemOutput{}, nil)

		// Call method under test with explicit TTL
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, explicitTTL)

		// Verify result
		assert.True(t, result)

		// Verify that ExpiresAt was set using the explicit TTL
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

		// Check that the expiry is roughly now + explicitTTL seconds (allow 5 second margin)
		expectedExpiry := time.Now().Unix() + int64(explicitTTL)
		assert.InDelta(t, expectedExpiry, expiryTimestamp, 5, "Expiry time should be roughly now + explicit TTL")

		mockClient.AssertExpectations(t)
	})

	t.Run("use_default_ttl", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		defaultTTL := mockStore.ttl

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

		// Check that the expiry is roughly now + defaultTTL seconds (allow 5 second margin)
		expectedExpiry := time.Now().Unix() + int64(defaultTTL)
		assert.InDelta(t, expectedExpiry, expiryTimestamp, 5, "Expiry time should be roughly now + default TTL")

		mockClient.AssertExpectations(t)
	})

	t.Run("negative_ttl_uses_default", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		defaultTTL := mockStore.ttl
		negativeTTL := int32(-5)

		// Capture the input to validate TTL usage
		var capturedInput *dynamodb.PutItemInput

		// Mock successful PutItem and capture input
		mockClient.On("PutItem", ctx, mock.MatchedBy(func(input *dynamodb.PutItemInput) bool {
			capturedInput = input
			return true
		})).Return(&dynamodb.PutItemOutput{}, nil)

		// Call method under test with negative TTL (should use default)
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, negativeTTL)

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

		// Check that the expiry is roughly now + defaultTTL seconds (allow 5 second margin)
		expectedExpiry := time.Now().Unix() + int64(defaultTTL)
		assert.InDelta(t, expectedExpiry, expiryTimestamp, 5, "Expiry time should be roughly now + default TTL")

		mockClient.AssertExpectations(t)
	})
}

// TestCheckLockOwnership tests the ownsLock function directly
func TestCheckLockOwnership(t *testing.T) {
	t.Run("owns_lock", func(t *testing.T) {
		mockStore, _, _ := SetupMockStore()
		clientID := "client-1"

		// Create a mock item representing a lock owned by clientID
		item := map[string]types.AttributeValue{
			"PK":        &types.AttributeValueMemberS{Value: "service:domain"},
			"ClientID":  &types.AttributeValueMemberS{Value: clientID},
			"ExpiresAt": &types.AttributeValueMemberN{Value: "12345"},
		}

		// Check if the client owns the lock
		result := mockStore.ownsLock(item, clientID)

		// Verify result
		assert.True(t, result, "Client should own the lock")
	})

	t.Run("doesnt_own_lock", func(t *testing.T) {
		mockStore, _, _ := SetupMockStore()
		clientID := "client-1"
		otherClientID := "client-2"

		// Create a mock item representing a lock owned by otherClientID
		item := map[string]types.AttributeValue{
			"PK":        &types.AttributeValueMemberS{Value: "service:domain"},
			"ClientID":  &types.AttributeValueMemberS{Value: otherClientID},
			"ExpiresAt": &types.AttributeValueMemberN{Value: "12345"},
		}

		// Check if the client owns the lock
		result := mockStore.ownsLock(item, clientID)

		// Verify result
		assert.False(t, result, "Client should not own the lock")
	})

	t.Run("null_item", func(t *testing.T) {
		mockStore, _, _ := SetupMockStore()
		clientID := "client-1"

		// Check with nil item
		result := mockStore.ownsLock(nil, clientID)

		// Verify result
		assert.False(t, result, "Should return false for nil item")
	})

	t.Run("missing_clientid_field", func(t *testing.T) {
		mockStore, _, _ := SetupMockStore()
		clientID := "client-1"

		// Create a mock item without ClientID field
		item := map[string]types.AttributeValue{
			"PK":        &types.AttributeValueMemberS{Value: "service:domain"},
			"ExpiresAt": &types.AttributeValueMemberN{Value: "12345"},
		}

		// Check if the client owns the lock
		result := mockStore.ownsLock(item, clientID)

		// Verify result
		assert.False(t, result, "Should return false when ClientID field is missing")
	})

	t.Run("wrong_type_clientid", func(t *testing.T) {
		mockStore, _, _ := SetupMockStore()
		clientID := "client-1"

		// Create a mock item with ClientID of wrong type
		item := map[string]types.AttributeValue{
			"PK":        &types.AttributeValueMemberS{Value: "service:domain"},
			"ClientID":  &types.AttributeValueMemberN{Value: "12345"}, // Wrong type (N instead of S)
			"ExpiresAt": &types.AttributeValueMemberN{Value: "12345"},
		}

		// Check if the client owns the lock
		result := mockStore.ownsLock(item, clientID)

		// Verify result
		assert.False(t, result, "Should return false when ClientID is wrong type")
	})
}

// TestGetLockItem tests the getLockItem function through ReleaseLock flow
func TestGetLockItem(t *testing.T) {
	t.Run("get_item_success", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		pk := fmt.Sprintf("%s:%s", service, domain)

		// Mock successful GetItem for ReleaseLock
		mockClient.On("GetItem", ctx, mock.MatchedBy(func(input *dynamodb.GetItemInput) bool {
			// Verify the key used in GetItem
			assert.NotNil(t, input.Key)
			pkAttr, ok := input.Key["PK"]
			assert.True(t, ok, "PK should exist in key")
			pkValue, ok := pkAttr.(*types.AttributeValueMemberS)
			assert.True(t, ok, "PK should be string type")
			assert.Equal(t, pk, pkValue.Value, "PK should match expected format")
			return true
		})).Return(&dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"PK":       &types.AttributeValueMemberS{Value: pk},
				"ClientID": &types.AttributeValueMemberS{Value: clientID},
			},
		}, nil).Once()

		// Mock DeleteItem for ReleaseLock
		mockClient.On("DeleteItem", ctx, mock.Anything).Return(&dynamodb.DeleteItemOutput{}, nil).Once()

		// Call ReleaseLock which uses getLockItem
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations
		mockClient.AssertExpectations(t)
	})

	t.Run("get_item_error", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"

		// Mock GetItem error for ReleaseLock
		mockClient.On("GetItem", ctx, mock.Anything).Return(nil, fmt.Errorf("connection error")).Once()

		// Call ReleaseLock which uses getLockItem
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations - DeleteItem should not be called
		mockClient.AssertExpectations(t)
		mockClient.AssertNotCalled(t, "DeleteItem")
	})

	t.Run("get_item_empty_result", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"

		// Mock empty GetItem result for ReleaseLock
		mockClient.On("GetItem", ctx, mock.Anything).Return(&dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{}, // Empty item
		}, nil).Once()

		// Call ReleaseLock which uses getLockItem
		mockStore.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations - DeleteItem should not be called
		mockClient.AssertExpectations(t)
		mockClient.AssertNotCalled(t, "DeleteItem")
	})
}

// TestDirectGetLockItem tests the getLockItem function directly
func TestDirectGetLockItem(t *testing.T) {
	t.Run("direct_call_success", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		pk := "service:domain"
		clientID := "client-1"

		// Mock GetItem to return a valid item
		mockClient.On("GetItem", ctx, mock.MatchedBy(func(input *dynamodb.GetItemInput) bool {
			assert.Equal(t, pk, input.Key["PK"].(*types.AttributeValueMemberS).Value)
			return true
		})).Return(&dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"PK":       &types.AttributeValueMemberS{Value: pk},
				"ClientID": &types.AttributeValueMemberS{Value: clientID},
			},
		}, nil).Once()

		// Call getLockItem directly
		item, err := mockStore.getLockItem(ctx, pk)

		// Verify results
		assert.NoError(t, err)
		assert.NotNil(t, item)
		assert.Equal(t, pk, item["PK"].(*types.AttributeValueMemberS).Value)
		assert.Equal(t, clientID, item["ClientID"].(*types.AttributeValueMemberS).Value)
		mockClient.AssertExpectations(t)
	})

	t.Run("direct_call_error", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		pk := "service:domain"

		// Mock GetItem to return an error
		mockErr := errors.New("connection error")
		mockClient.On("GetItem", ctx, mock.Anything).Return(nil, mockErr).Once()

		// Call getLockItem directly
		item, err := mockStore.getLockItem(ctx, pk)

		// Verify results
		assert.Error(t, err)
		assert.Nil(t, item)
		assert.Equal(t, mockErr, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("direct_call_nil_item", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		pk := "service:domain"

		// Mock GetItem to return a nil item (not found)
		mockClient.On("GetItem", ctx, mock.Anything).Return(&dynamodb.GetItemOutput{
			Item: nil,
		}, nil).Once()

		// Call getLockItem directly
		item, err := mockStore.getLockItem(ctx, pk)

		// Verify results
		assert.NoError(t, err)
		assert.Nil(t, item)
		mockClient.AssertExpectations(t)
	})
}

// TestLockExpiration tests behavior related to lock expiration
func TestLockExpiration(t *testing.T) {
	t.Run("expired_lock_acquisition", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Mock PutItem with specific condition matching for expired locks
		mockClient.On("PutItem", ctx, mock.MatchedBy(func(input *dynamodb.PutItemInput) bool {
			// Verify condition expression includes expiration check
			return *input.ConditionExpression == "attribute_not_exists(PK) OR ExpiresAt < :now"
		})).Return(&dynamodb.PutItemOutput{}, nil)

		// Call TryAcquireLock
		result := mockStore.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify success
		assert.True(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("keep_alive_with_expired_lock", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Mock UpdateItem failure due to condition check (lock expired or owned by different client)
		conditionFailedErr := fmt.Errorf("ConditionalCheckFailedException: The conditional request failed")
		mockClient.On("UpdateItem", ctx, mock.Anything).Return(nil, conditionFailedErr)

		// Call KeepAlive on an expired lock
		result := mockStore.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify failure
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockClient.AssertExpectations(t)
	})
}
