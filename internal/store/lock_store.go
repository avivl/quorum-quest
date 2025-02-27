// internal/store/lock_store.go
package store

import (
	"context"
	"time"
)

// LockStore provides operations for distributed lock management
type LockStore interface {
	// TryAcquireLock attempts to acquire a lock
	// Returns true if successful, false otherwise
	TryAcquireLock(ctx context.Context, service, domain, clientID string, ttl int32) bool

	// ReleaseLock releases a previously acquired lock
	ReleaseLock(ctx context.Context, service, domain, clientID string)

	// KeepAlive refreshes the TTL of an existing lock
	// Returns the new lease duration or negative duration if failed
	KeepAlive(ctx context.Context, service, domain, clientID string, ttl int32) time.Duration

	// Close releases resources held by the store
	Close()

	// GetConfig returns the current store configuration
	GetConfig() StoreConfig
}
