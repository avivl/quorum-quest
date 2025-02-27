// internal/lockservice/mock_test.go
package lockservice

import (
	"context"

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

// newStore creates a new store based on the provided configuration.
// It supports different types of stores, such as Mock, Redis, etc.
// The function takes a context, endpoints, and options as parameters.
// The options parameter should be of type Config.
//
// If the provided options are not of type Config, an InvalidConfigurationError is returned.
//
// The function returns a store.Store interface and an error.
// If the store creation is successful, the returned error will be nil.
//
// Example:
//
//	cfg := &Config{
//		// Set configuration options here
//	}
//	store, err := newStore(ctx, []string{"localhost:6379"}, cfg)
//	if err!= nil {
//		// Handle error
//	}
//	// Use the store
func newStore(ctx context.Context, options Config, logger *observability.SLogger) (store.Store, error) {
	cfg, ok := options.(*MockConfig)
	if !ok && cfg != nil {
		return nil, &store.InvalidConfigurationError{Store: testStoreName, Config: options}
	}

	return New(ctx, cfg)
}

type Mock struct {
	cfg *MockConfig
}

// New creates a new Mock client.
//
// New creates a new Mock client.
// The function takes a context, endpoints, and a configuration as parameters.
// The context parameter is used to control the lifetime of the operation.
// The endpoints parameter is a slice of strings representing the addresses of the store servers.
// The configuration parameter is of type *Config and holds the configuration options for the store.
//
// The function returns a pointer to a Mock struct and an error.
// If the store creation is successful, the returned error will be nil.
// If the provided configuration is not of type *Config, the function will return a store.InvalidConfigurationError.
//
// The Mock struct implements the store.Store interface, providing methods for acquiring, releasing, and keeping alive locks.
//
//nolint:gocritic
//nolint:gocritic
func New(_ context.Context, cfg *MockConfig) (*Mock, error) {
	return &Mock{cfg: cfg}, nil
}

// TryAcquireLock attempts to acquire a lock for the given service, domain, and clientId.
//
// The function takes a context, service, domain, and clientId as parameters.
// The context parameter is used to control the lifetime of the operation.
// The service parameter is a string representing the name of the service for which the lock is being acquired.
// The domain parameter is a string representing the domain within the service for which the lock is being acquired.
// The clientId parameter is a string representing the unique identifier of the client attempting to acquire the lock.
//
// The function returns a boolean value indicating whether the lock was acquired successfully.
// If the lock was acquired, the function returns true.
// If the lock was not acquired, the function returns false.
//
// If the context is canceled or expired before the lock is acquired, the function returns false.
//
// The Mock implementation of this function panics with the message "implement me".
// In a real implementation, this function should attempt to acquire the lock using the underlying store.
func (m Mock) TryAcquireLock(_ context.Context, service, domain, clientId string, ttl int32) bool {
	panic("implement me")
}

// ReleaseLock attempts to release the lock for the given service, domain, and clientId.
//
// The function takes a context, service, domain, and clientId as parameters.
// The context parameter is used to control the lifetime of the operation.
// The service parameter is a string representing th
