// internal/lockservice/lockservice.go
package lockservice

import (
	"context"
	"sort"
	"sync"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
)

var (
	constructorsMu sync.RWMutex
	constructors   = make(map[string]Constructor)
)

// Config the raw type of the store configurations.
type Config any

// Constructor The signature of a store constructor.
// Updated to return LockStore instead of Store
type Constructor func(ctx context.Context, options Config, logger *observability.SLogger) (store.LockStore, error)

// Register registers a new store constructor.
// It panics if the constructor is nil or if it's called twice for the same name.
func Register(name string, cttr Constructor) {
	constructorsMu.Lock()
	defer constructorsMu.Unlock()

	if cttr == nil {
		panic("quorum-ques: Register constructor is nil")
	}

	if _, dup := constructors[name]; dup {
		panic("quorum-ques: Register called twice for constructor " + name)
	}

	constructors[name] = cttr
}

// Unregister unregisters a store constructor.
func Unregister(storeName string) {
	constructorsMu.Lock()
	defer constructorsMu.Unlock()

	delete(constructors, storeName)
}

// UnregisterAllConstructors unregisters all store constructors.
func UnregisterAllConstructors() {
	constructorsMu.Lock()
	defer constructorsMu.Unlock()

	constructors = make(map[string]Constructor)
}

// Constructors returns a sorted list of the names of the registered constructors.
func Constructors() []string {
	constructorsMu.RLock()
	defer constructorsMu.RUnlock()

	list := make([]string, 0, len(constructors))
	for name := range constructors {
		list = append(list, name)
	}

	sort.Strings(list)

	return list
}

// NewStore creates a new lock store instance using the specified constructor.
// Updated to return LockStore instead of Store
func NewStore(ctx context.Context, storeName string, options Config, logger *observability.SLogger) (store.LockStore, error) {
	constructorsMu.RLock()
	construct, ok := constructors[storeName]
	constructorsMu.RUnlock()

	if !ok {
		return nil, &store.UnknownConstructorError{Store: storeName}
	}

	if construct == nil {
		return nil, &store.UnknownConstructorError{Store: storeName}
	}

	return construct(ctx, options, logger)
}
