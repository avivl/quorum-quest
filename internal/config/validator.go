// internal/config/validator.go
package config

import (
	"fmt"
	"reflect"
)

// validateConfig validates the configuration
func validateConfig(config interface{}) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	v := reflect.ValueOf(config)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return fmt.Errorf("configuration pointer cannot be nil")
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return fmt.Errorf("configuration must be a struct")
	}

	// Check if Store field exists and is not nil
	storeField := v.FieldByName("Store")
	if !storeField.IsValid() {
		return fmt.Errorf("configuration must have a Store field")
	}

	if storeField.Kind() == reflect.Ptr && storeField.IsNil() {
		return fmt.Errorf("Store configuration cannot be nil")
	}

	// If Store implements Validate() method, call it
	if validator, ok := storeField.Interface().(interface{ Validate() error }); ok {
		if err := validator.Validate(); err != nil {
			return fmt.Errorf("store validation failed: %w", err)
		}
	}

	return nil
}
