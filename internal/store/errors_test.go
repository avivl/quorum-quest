// internal/store/errors_test.go
package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInvalidConfigurationError(t *testing.T) {
	err := &InvalidConfigurationError{
		Store:  "test-store",
		Config: "test-config",
	}

	expectedMsg := "test-store: invalid configuration type: string"
	assert.Equal(t, expectedMsg, err.Error())
}

func TestUnknownConstructorError(t *testing.T) {
	err := UnknownConstructorError{
		Store: "test-store",
	}

	expectedMsg := `unknown constructor "test-store" (forgotten import?)`
	assert.Equal(t, expectedMsg, err.Error())
}
