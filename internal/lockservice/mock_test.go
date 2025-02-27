// internal/lockservice/mock_test.go
package lockservice

import (
	"context"
	"time"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
)

const testStoreName = "mock"

// MockConfig implements store.StoreConfig
type MockConfig struct {
	TTL       int32
	Endpoints []string
	Table     string
}

// Validate validates the configuration
func (c MockConfig) Validate() error {
	return nil
}

// GetEndpoints returns the endpoints
func (c MockConfig) GetEndpoints() []string {
	return c.Endpoints
}

// GetTTL returns the TTL value
func (c MockConfig) GetTTL() int32 {
	return c.TTL
}

// GetTableName returns the table name
func (c MockConfig) GetTableName() string {
	return c.Table
}

// mockLockStoreWrapper is a wrapper that implements store.LockStore interface
// Updated to implement LockStore instead of Store
type mockLockStoreWrapper struct {
	*Mock
}

// newStore creates a new lock store based on the provided configuration.
// Updated to return LockStore instead of Store
func newStore(ctx context.Context, options Config, logger *observability.SLogger) (store.LockStore, error) {
	cfg, ok := options.(*MockConfig)
	if !ok && cfg != nil {
		return nil, &store.InvalidConfigurationError{Store: testStoreName, Config: options}
	}

	mockStore, err := New(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &mockLockStoreWrapper{mockStore}, nil
}

type Mock struct {
	cfg *MockConfig
}

// New creates a new Mock client.
func New(_ context.Context, cfg *MockConfig) (*Mock, error) {
	return &Mock{cfg: cfg}, nil
}

// TryAcquireLock attempts to acquire a lock
func (m Mock) TryAcquireLock(_ context.Context, service, domain, clientId string, ttl int32) bool {
	panic("implement me")
}

// ReleaseLock attempts to release a lock
func (m Mock) ReleaseLock(_ context.Context, service, domain, clientId string) {
	panic("implement me")
}

// KeepAlive attempts to keep a lock alive
func (m Mock) KeepAlive(_ context.Context, service, domain, clientId string, ttl int32) time.Duration {
	panic("implement me")
}

// Close closes the Mock store
func (m Mock) Close() {
	panic("implement me")
}

// GetConfig returns the current store configuration
func (m Mock) GetConfig() store.StoreConfig {
	return m.cfg
}
