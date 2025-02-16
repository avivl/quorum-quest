package store

import (
	"context"
	"time"
)

// Store interface with Config integration
type Store interface {
	TryAcquireLock(ctx context.Context, service, domain, clientId string, ttl int32) bool
	ReleaseLock(ctx context.Context, service, domain, clientId string)
	KeepAlive(ctx context.Context, service, domain, clientId string, ttl int32) time.Duration
	Close()
	// GetConfig returns the current store configuration
	GetConfig() StoreConfig
}
