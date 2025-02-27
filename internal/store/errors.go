// internal/store/errors.go
package store

import (
	"errors"
	"fmt"
)

var (
	// ErrNotReachable is thrown when the API cannot be reached for issuing common store operations.
	ErrNotReachable = errors.New("api not reachable")
	// ErrCannotLock is thrown when there is an error acquiring a lock on a key.
	ErrCannotLock = errors.New("error acquiring the lock")
	// ErrKeyModified is thrown during an atomic operation if the index does not match the one in the store.
	ErrKeyModified = errors.New("unable to complete atomic operation, key modified")
	// ErrKeyNotFound is thrown when the key is not found in the store during a Get operation.
	ErrKeyNotFound = errors.New("key not found in store")
)

// InvalidConfigurationError is thrown when the type of the configuration is not supported by a store.
type InvalidConfigurationError struct {
	Store  string
	Config any
}

func (e *InvalidConfigurationError) Error() string {
	return fmt.Sprintf("%s: invalid configuration type: %T", e.Store, e.Config)
}

// UnknownConstructorError is thrown when a requested store is not register.
type UnknownConstructorError struct {
	Store string
}

func (e UnknownConstructorError) Error() string {
	return fmt.Sprintf("unknown constructor %q (forgotten import?)", e.Store)
}
