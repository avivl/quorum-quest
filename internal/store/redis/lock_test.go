// internal/store/redis/lock_test.go
package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestGetLockKey(t *testing.T) {
	store, _ := SetupMockStore()

	key := store.getLockKey("service1", "domain1")
	assert.Equal(t, "lock:service1:domain1", key)

	// Change the prefix
	store.keyPrefix = "test-prefix"
	key = store.getLockKey("service1", "domain1")
	assert.Equal(t, "test-prefix:service1:domain1", key)
}

func TestTryAcquireLock(t *testing.T) {
	t.Run("success_new_lock", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Setup expectations for SetNX
		boolCmd := redis.NewBoolCmd(ctx)
		boolCmd.SetVal(true) // Lock acquired
		mockClient.On("SetNX", ctx, "lock:test-service:test-domain", clientID, time.Duration(ttl)*time.Second).Return(boolCmd)

		// Call the method under test
		result := store.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.True(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("success_already_owns_lock", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Setup expectations for SetNX (fails because lock exists)
		boolCmd := redis.NewBoolCmd(ctx)
		boolCmd.SetVal(false) // Lock couldn't be acquired with SetNX
		mockClient.On("SetNX", ctx, "lock:test-service:test-domain", clientID, time.Duration(ttl)*time.Second).Return(boolCmd)

		// Setup expectations for Get (client owns the lock)
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetVal(clientID) // Client already owns the lock
		mockClient.On("Get", ctx, "lock:test-service:test-domain").Return(stringCmd)

		// Setup expectations for Expire
		expireBoolCmd := redis.NewBoolCmd(ctx)
		expireBoolCmd.SetVal(true) // TTL refresh successful
		mockClient.On("Expire", ctx, "lock:test-service:test-domain", time.Duration(ttl)*time.Second).Return(expireBoolCmd)

		// Call the method under test
		result := store.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.True(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("failure_another_client_owns_lock", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		otherClientID := "client-2"
		ttl := int32(30)

		// Setup expectations for SetNX (fails because lock exists)
		boolCmd := redis.NewBoolCmd(ctx)
		boolCmd.SetVal(false) // Lock couldn't be acquired with SetNX
		mockClient.On("SetNX", ctx, "lock:test-service:test-domain", clientID, time.Duration(ttl)*time.Second).Return(boolCmd)

		// Setup expectations for Get (other client owns the lock)
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetVal(otherClientID) // Another client owns the lock
		mockClient.On("Get", ctx, "lock:test-service:test-domain").Return(stringCmd)

		// Call the method under test
		result := store.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.False(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("default_ttl", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		defaultTTL := store.ttl

		// Setup expectations for SetNX
		boolCmd := redis.NewBoolCmd(ctx)
		boolCmd.SetVal(true) // Lock acquired
		mockClient.On("SetNX", ctx, "lock:test-service:test-domain", clientID, time.Duration(defaultTTL)*time.Second).Return(boolCmd)

		// Call the method under test with ttl=0 to use default
		result := store.TryAcquireLock(ctx, service, domain, clientID, 0)

		// Verify result and expectations
		assert.True(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("error_on_setnx", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Setup expectations for SetNX with error
		boolCmd := redis.NewBoolCmd(ctx)
		boolCmd.SetErr(errors.New("connection error"))
		mockClient.On("SetNX", ctx, "lock:test-service:test-domain", clientID, time.Duration(ttl)*time.Second).Return(boolCmd)

		// Call the method under test
		result := store.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.False(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("error_on_get", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Setup expectations for SetNX (fails because lock exists)
		boolCmd := redis.NewBoolCmd(ctx)
		boolCmd.SetVal(false) // Lock couldn't be acquired with SetNX
		mockClient.On("SetNX", ctx, "lock:test-service:test-domain", clientID, time.Duration(ttl)*time.Second).Return(boolCmd)

		// Setup expectations for Get with error
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetErr(errors.New("connection error"))
		mockClient.On("Get", ctx, "lock:test-service:test-domain").Return(stringCmd)

		// Call the method under test
		result := store.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.False(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("error_on_nil_get", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Setup expectations for SetNX (fails because lock exists)
		boolCmd := redis.NewBoolCmd(ctx)
		boolCmd.SetVal(false) // Lock couldn't be acquired with SetNX
		mockClient.On("SetNX", ctx, "lock:test-service:test-domain", clientID, time.Duration(ttl)*time.Second).Return(boolCmd)

		// Setup expectations for Get with nil error
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetErr(redis.Nil)
		mockClient.On("Get", ctx, "lock:test-service:test-domain").Return(stringCmd)

		// Call the method under test
		result := store.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.False(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("error_on_expire", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Setup expectations for SetNX (fails because lock exists)
		boolCmd := redis.NewBoolCmd(ctx)
		boolCmd.SetVal(false) // Lock couldn't be acquired with SetNX
		mockClient.On("SetNX", ctx, "lock:test-service:test-domain", clientID, time.Duration(ttl)*time.Second).Return(boolCmd)

		// Setup expectations for Get (client owns the lock)
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetVal(clientID) // Client already owns the lock
		mockClient.On("Get", ctx, "lock:test-service:test-domain").Return(stringCmd)

		// Setup expectations for Expire with error
		expireBoolCmd := redis.NewBoolCmd(ctx)
		expireBoolCmd.SetErr(errors.New("connection error"))
		mockClient.On("Expire", ctx, "lock:test-service:test-domain", time.Duration(ttl)*time.Second).Return(expireBoolCmd)

		// Call the method under test
		result := store.TryAcquireLock(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.False(t, result)
		mockClient.AssertExpectations(t)
	})
}

func TestReleaseLock(t *testing.T) {
	t.Run("success_when_owns_lock", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"

		// Setup expectations for Get (client owns the lock)
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetVal(clientID) // Client owns the lock
		mockClient.On("Get", ctx, "lock:test-service:test-domain").Return(stringCmd)

		// Setup expectations for Del
		intCmd := redis.NewIntCmd(ctx)
		intCmd.SetVal(1) // Key deleted
		mockClient.On("Del", ctx, []string{"lock:test-service:test-domain"}).Return(intCmd)

		// Call the method under test
		store.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations
		mockClient.AssertExpectations(t)
	})

	t.Run("noop_when_doesnt_own_lock", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		otherClientID := "client-2"

		// Setup expectations for Get (other client owns the lock)
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetVal(otherClientID) // Another client owns the lock
		mockClient.On("Get", ctx, "lock:test-service:test-domain").Return(stringCmd)

		// Call the method under test
		store.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations - Del should not be called
		mockClient.AssertExpectations(t)
	})

	t.Run("noop_when_lock_not_found", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"

		// Setup expectations for Get (lock not found)
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetErr(redis.Nil) // Key doesn't exist
		mockClient.On("Get", ctx, "lock:test-service:test-domain").Return(stringCmd)

		// Call the method under test
		store.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations - Del should not be called
		mockClient.AssertExpectations(t)
	})

	t.Run("logs_error_on_get_failure", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"

		// Setup expectations for Get with error
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetErr(errors.New("connection error"))
		mockClient.On("Get", ctx, "lock:test-service:test-domain").Return(stringCmd)

		// Call the method under test
		store.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations - Del should not be called
		mockClient.AssertExpectations(t)
	})

	t.Run("logs_error_on_del_failure", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"

		// Setup expectations for Get (client owns the lock)
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetVal(clientID) // Client owns the lock
		mockClient.On("Get", ctx, "lock:test-service:test-domain").Return(stringCmd)

		// Setup expectations for Del with error
		intCmd := redis.NewIntCmd(ctx)
		intCmd.SetErr(errors.New("connection error"))
		mockClient.On("Del", ctx, []string{"lock:test-service:test-domain"}).Return(intCmd)

		// Call the method under test
		store.ReleaseLock(ctx, service, domain, clientID)

		// Verify expectations
		mockClient.AssertExpectations(t)
	})
}

func TestKeepAlive(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Setup expectations for Get (client owns the lock)
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetVal(clientID) // Client owns the lock
		mockClient.On("Get", ctx, "lock:test-service:test-domain").Return(stringCmd)

		// Setup expectations for Expire (success)
		expireBoolCmd := redis.NewBoolCmd(ctx)
		expireBoolCmd.SetVal(true) // TTL refresh successful
		mockClient.On("Expire", ctx, "lock:test-service:test-domain", time.Duration(ttl)*time.Second).Return(expireBoolCmd)

		// Call the method under test
		result := store.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.Equal(t, time.Duration(ttl)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("default_ttl", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		defaultTTL := store.ttl

		// Setup expectations for Get (client owns the lock)
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetVal(clientID) // Client owns the lock
		mockClient.On("Get", ctx, "lock:test-service:test-domain").Return(stringCmd)

		// Setup expectations for Expire (success)
		expireBoolCmd := redis.NewBoolCmd(ctx)
		expireBoolCmd.SetVal(true) // TTL refresh successful
		mockClient.On("Expire", ctx, "lock:test-service:test-domain", time.Duration(defaultTTL)*time.Second).Return(expireBoolCmd)

		// Call the method under test with ttl=0 to use default
		result := store.KeepAlive(ctx, service, domain, clientID, 0)

		// Verify result and expectations
		assert.Equal(t, time.Duration(defaultTTL)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("failure_doesnt_own_lock", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		otherClientID := "client-2"
		ttl := int32(30)

		// Setup expectations for Get (other client owns the lock)
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetVal(otherClientID) // Another client owns the lock
		mockClient.On("Get", ctx, "lock:test-service:test-domain").Return(stringCmd)

		// Call the method under test
		result := store.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("failure_lock_not_found", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Setup expectations for Get (lock not found)
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetErr(redis.Nil) // Key doesn't exist
		mockClient.On("Get", ctx, "lock:test-service:test-domain").Return(stringCmd)

		// Call the method under test
		result := store.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("error_on_get", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Setup expectations for Get with error
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetErr(errors.New("connection error"))
		mockClient.On("Get", ctx, "lock:test-service:test-domain").Return(stringCmd)

		// Call the method under test
		result := store.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("error_on_expire", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Setup expectations for Get (client owns the lock)
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetVal(clientID) // Client owns the lock
		mockClient.On("Get", ctx, "lock:test-service:test-domain").Return(stringCmd)

		// Setup expectations for Expire with error
		expireBoolCmd := redis.NewBoolCmd(ctx)
		expireBoolCmd.SetErr(errors.New("connection error"))
		mockClient.On("Expire", ctx, "lock:test-service:test-domain", time.Duration(ttl)*time.Second).Return(expireBoolCmd)

		// Call the method under test
		result := store.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("key_no_longer_exists", func(t *testing.T) {
		store, mockClient := SetupMockStore()
		ctx := context.Background()
		service := "test-service"
		domain := "test-domain"
		clientID := "client-1"
		ttl := int32(30)

		// Setup expectations for Get (client owns the lock)
		stringCmd := redis.NewStringCmd(ctx)
		stringCmd.SetVal(clientID) // Client owns the lock
		mockClient.On("Get", ctx, "lock:test-service:test-domain").Return(stringCmd)

		// Setup expectations for Expire (key doesn't exist)
		expireBoolCmd := redis.NewBoolCmd(ctx)
		expireBoolCmd.SetVal(false) // Key no longer exists
		mockClient.On("Expire", ctx, "lock:test-service:test-domain", time.Duration(ttl)*time.Second).Return(expireBoolCmd)

		// Call the method under test
		result := store.KeepAlive(ctx, service, domain, clientID, ttl)

		// Verify result and expectations
		assert.Equal(t, time.Duration(-1)*time.Second, result)
		mockClient.AssertExpectations(t)
	})
}
