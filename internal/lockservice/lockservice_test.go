// internal/lockservice/lockservice_test.go
package lockservice

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegister(t *testing.T) {
	t.Cleanup(UnregisterAllConstructors)

	Register(testStoreName, newStore)

	assert.Len(t, constructors, 1)
}

func TestRegister_duplicate(t *testing.T) {
	t.Cleanup(UnregisterAllConstructors)

	Register(testStoreName, newStore)
	assert.Len(t, constructors, 1)

	assert.Panics(t, func() {
		Register(testStoreName, newStore)
	})
}

func TestRegister_nil(t *testing.T) {
	t.Cleanup(UnregisterAllConstructors)

	assert.Panics(t, func() {
		Register(testStoreName, nil)
	})
}

func TestUnregister(t *testing.T) {
	t.Cleanup(UnregisterAllConstructors)

	Register(testStoreName, newStore)
	assert.Len(t, constructors, 1)

	Unregister(testStoreName)

	constructorsMu.Lock()
	defer constructorsMu.Unlock()

	assert.Empty(t, constructors)
}

func TestConstructors(t *testing.T) {
	t.Cleanup(UnregisterAllConstructors)

	Register(testStoreName, newStore)
	assert.Len(t, constructors, 1)

	cttrs := Constructors()

	expected := []string{testStoreName}
	assert.Equal(t, expected, cttrs)
}

func TestNewStore(t *testing.T) {
	t.Cleanup(UnregisterAllConstructors)

	Register(testStoreName, newStore)

	assert.Len(t, constructors, 1)

	s, err := NewStore(context.Background(), testStoreName, nil, nil)
	require.NoError(t, err)

	assert.NotNil(t, s)
	// Updated to check for mockLockStoreWrapper instead of mockStoreWrapper
	assert.IsType(t, &mockLockStoreWrapper{}, s)

	// Verify it's wrapping a *Mock
	wrapper, ok := s.(*mockLockStoreWrapper)
	assert.True(t, ok)
	assert.IsType(t, &Mock{}, wrapper.Mock)
}
