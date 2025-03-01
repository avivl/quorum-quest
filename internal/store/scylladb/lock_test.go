// internal/store/scylladb/lock_test.go
package scylladb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTryAcquireLock(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		ttl := int32(30)

		// Mock acquire query
		mockAcquireQuery := new(MockQuery)
		mockAcquireQuery.On("WithContext", ctx).Return(mockAcquireQuery)
		mockAcquireQuery.On("Exec").Return(nil)

		// Mock validate query
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(clientID, nil)

		// Setup mock session expectations
		mockSession.On("Query", store.TryAcquireLockQuery, []interface{}{service, domain, clientID, ttl}).Return(mockAcquireQuery)
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)

		// Call method under test
		result := store.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and mocks
		assert.True(t, result)
		mockSession.AssertExpectations(t)
		mockAcquireQuery.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
	})

	t.Run("default_ttl", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		defaultTtl := store.ttl

		// Mock acquire query
		mockAcquireQuery := new(MockQuery)
		mockAcquireQuery.On("WithContext", ctx).Return(mockAcquireQuery)
		mockAcquireQuery.On("Exec").Return(nil)

		// Mock validate query
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(clientID, nil)

		// Setup mock session expectations
		mockSession.On("Query", store.TryAcquireLockQuery, []interface{}{service, domain, clientID, defaultTtl}).Return(mockAcquireQuery)
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)

		// Call method under test with ttl=0 to use default
		result := store.TryAcquireLock(ctx, service, domain, clientID, 0)

		// Verify result and mocks
		assert.True(t, result)
		mockSession.AssertExpectations(t)
		mockAcquireQuery.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
	})

	t.Run("failure_on_acquire", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		ttl := int32(30)
		expectedError := errors.New("insert failed")

		// Mock acquire query with error
		mockAcquireQuery := new(MockQuery)
		mockAcquireQuery.On("WithContext", ctx).Return(mockAcquireQuery)
		mockAcquireQuery.On("Exec").Return(expectedError)

		// Setup mock session expectations
		mockSession.On("Query", store.TryAcquireLockQuery, []interface{}{service, domain, clientID, ttl}).Return(mockAcquireQuery)

		// Call method under test
		result := store.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and mocks
		assert.False(t, result)
		mockSession.AssertExpectations(t)
		mockAcquireQuery.AssertExpectations(t)
	})

	t.Run("failure_on_validate", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		ttl := int32(30)
		validateError := errors.New("validate failed")

		// Mock acquire query
		mockAcquireQuery := new(MockQuery)
		mockAcquireQuery.On("WithContext", ctx).Return(mockAcquireQuery)
		mockAcquireQuery.On("Exec").Return(nil)

		// Mock validate query with error
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(nil, validateError)

		// Setup mock session expectations
		mockSession.On("Query", store.TryAcquireLockQuery, []interface{}{service, domain, clientID, ttl}).Return(mockAcquireQuery)
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)

		// Call method under test
		result := store.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and mocks
		assert.False(t, result)
		mockSession.AssertExpectations(t)
		mockAcquireQuery.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
	})
}

func TestReleaseLock(t *testing.T) {
	t.Run("success_when_owns_lock", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"

		// Mock validation query that confirms ownership
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(clientID, nil)

		// Mock release query
		mockReleaseQuery := new(MockQuery)
		mockReleaseQuery.On("WithContext", ctx).Return(mockReleaseQuery)
		mockReleaseQuery.On("Exec").Return(nil)

		// Setup mock session expectations
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)
		mockSession.On("Query", store.ReleaseLockQuery, []interface{}{service, domain}).Return(mockReleaseQuery)

		// Call method under test
		store.ReleaseLock(ctx, service, domain, clientID)

		// Verify mocks
		mockSession.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
		mockReleaseQuery.AssertExpectations(t)
	})

	t.Run("noop_when_doesnt_own_lock", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		differentClientID := "other-client"

		// Mock validation query that shows different owner
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(differentClientID, nil)

		// Setup mock session expectations - note no release query should be called
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)

		// Call method under test
		store.ReleaseLock(ctx, service, domain, clientID)

		// Verify mocks
		mockSession.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
	})

	t.Run("noop_when_lock_not_found", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"

		// Mock validation query that shows lock doesn't exist
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(nil, gocql.ErrNotFound)

		// Setup mock session expectations - note no release query should be called
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)

		// Call method under test
		store.ReleaseLock(ctx, service, domain, clientID)

		// Verify mocks
		mockSession.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
	})

	t.Run("logs_error_on_release_failure", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		expectedError := errors.New("delete failed")

		// Mock validation query that confirms ownership
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(clientID, nil)

		// Mock release query with error
		mockReleaseQuery := new(MockQuery)
		mockReleaseQuery.On("WithContext", ctx).Return(mockReleaseQuery)
		mockReleaseQuery.On("Exec").Return(expectedError)

		// Setup mock session expectations
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)
		mockSession.On("Query", store.ReleaseLockQuery, []interface{}{service, domain}).Return(mockReleaseQuery)

		// Call method under test
		store.ReleaseLock(ctx, service, domain, clientID)

		// Verify mocks
		mockSession.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
		mockReleaseQuery.AssertExpectations(t)
	})

	t.Run("logs_other_validation_errors", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		otherError := errors.New("unexpected error")

		// Mock validation query with a different error than NotFound
		mockValidateQuery := new(MockQuery)
		mockValidateQuery.On("WithContext", ctx).Return(mockValidateQuery)
		mockValidateQuery.On("Scan", mock.Anything).Return(nil, otherError)

		// Setup mock session expectations
		mockSession.On("Query", store.ValidateLockQuery, []interface{}{service, domain, clientID}).Return(mockValidateQuery)

		// Call method under test
		store.ReleaseLock(ctx, service, domain, clientID)

		// Verify mocks
		mockSession.AssertExpectations(t)
		mockValidateQuery.AssertExpectations(t)
	})
}

func TestKeepAlive(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		ttl := int32(30)

		// Mock acquire query (keepalive uses TryAcquireLockQuery in our implementation)
		mockAcquireQuery := new(MockQuery)
		mockAcquireQuery.On("WithContext", ctx).Return(mockAcquireQuery)
		mockAcquireQuery.On("Exec").Return(nil)

		// Setup mock session expectations
		mockSession.On("Query", store.TryAcquireLockQuery, []interface{}{service, domain, clientID, ttl}).Return(mockAcquireQuery)

		// Call method under test
		result := store.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and mocks
		assert.Equal(t, time.Duration(ttl)*time.Second, result)
		mockSession.AssertExpectations(t)
		mockAcquireQuery.AssertExpectations(t)
	})

	t.Run("default_ttl", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		defaultTtl := store.ttl

		// Mock acquire query
		mockAcquireQuery := new(MockQuery)
		mockAcquireQuery.On("WithContext", ctx).Return(mockAcquireQuery)
		mockAcquireQuery.On("Exec").Return(nil)

		// Setup mock session expectations
		mockSession.On("Query", store.TryAcquireLockQuery, []interface{}{service, domain, clientID, defaultTtl}).Return(mockAcquireQuery)

		// Call method under test with ttl=0 to use default
		result := store.KeepAlive(ctx, service, domain, clientID, 0)

		// Verify result and mocks
		assert.Equal(t, time.Duration(defaultTtl)*time.Second, result)
		mockSession.AssertExpectations(t)
		mockAcquireQuery.AssertExpectations(t)
	})

	t.Run("failure_returns_negative_duration", func(t *testing.T) {
		store, mockSession := SetupStoreWithMocks()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "test-client"
		ttl := int32(30)
		acquireError := errors.New("keepalive failed")

		// Mock acquire query with error
		mockAcquireQuery := new(MockQuery)
		mockAcquireQuery.On("WithContext", ctx).Return(mockAcquireQuery)
		mockAcquireQuery.On("Exec").Return(acquireError)

		// Setup mock session expectations
		mockSession.On("Query", store.TryAcquireLockQuery, []interface{}{service, domain, clientID, ttl}).Return(mockAcquireQuery)

		// Call method under test
		result := store.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and mocks
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockSession.AssertExpectations(t)
		mockAcquireQuery.AssertExpectations(t)
	})
}
