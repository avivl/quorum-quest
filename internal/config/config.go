// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/avivl/quorum-quest/internal/store/dynamodb"
	"github.com/avivl/quorum-quest/internal/store/redis"
	"github.com/avivl/quorum-quest/internal/store/scylladb"
)

// setupWatcher configures the file watcher for both the file and its parent directory
func setupWatcher(loader *ConfigLoader) error {
	// Always watch the parent directory to catch file creation/deletion/rename
	var dirToWatch string

	// Lock to safely access loader.configPath and loader.actualFile
	loader.mu.Lock()
	defer loader.mu.Unlock()

	fileInfo, err := os.Stat(loader.configPath)
	if err == nil && fileInfo.IsDir() {
		dirToWatch = loader.configPath
	} else {
		dirToWatch = filepath.Dir(loader.configPath)
	}

	// Add the directory to the watcher
	if err := loader.watcher.Add(dirToWatch); err != nil {
		return fmt.Errorf("failed to watch directory %s: %w", dirToWatch, err)
	}

	// If we have an actual file, also watch it directly
	if loader.actualFile != "" {
		if err := loader.watcher.Add(loader.actualFile); err != nil {
			// Not a fatal error, we can still watch the directory
			fmt.Printf("Warning: couldn't watch config file directly: %v\n", err)
		}
	}

	return nil
}

// loadFromFile attempts to load configuration from a file
func loadFromFile[T store.StoreConfig](configPath string, loaderFunc ConfigLoaderFunc) (*GlobalConfig[T], error) {
	// Ensure the config path exists
	fileInfo, err := os.Stat(configPath)
	if err != nil {
		return nil, err
	}

	var configFile string
	if fileInfo.IsDir() {
		// Try common config filenames
		candidates := []string{
			filepath.Join(configPath, "config.yaml"),
			filepath.Join(configPath, "config.yml"),
			filepath.Join(configPath, "quorum-quest.yaml"),
			filepath.Join(configPath, "quorum-quest.yml"),
		}

		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				configFile = candidate
				break
			}
		}

		if configFile == "" {
			return nil, fmt.Errorf("no config file found in directory %s", configPath)
		}
	} else {
		configFile = configPath
	}

	// Read configuration file
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse configuration
	configObj, err := loaderFunc(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	config, ok := configObj.(*GlobalConfig[T])
	if !ok {
		return nil, fmt.Errorf("invalid config type, expected *GlobalConfig[T], got %T", configObj)
	}

	return config, nil
}

// createDefaultConfig creates a default configuration
func createDefaultConfig[T store.StoreConfig]() *GlobalConfig[T] {
	var storeConfig T

	// Create new default store config based on type
	switch any(storeConfig).(type) {
	case *scylladb.ScyllaDBConfig:
		defaultScyllaConfig := scylladb.NewScyllaDBConfig()
		storeConfig = any(defaultScyllaConfig).(T)
	case *dynamodb.DynamoDBConfig:
		defaultDynamoConfig := dynamodb.NewDynamoDBConfig()
		storeConfig = any(defaultDynamoConfig).(T)
	case *redis.RedisConfig:
		defaultRedisConfig := redis.NewRedisConfig()
		storeConfig = any(defaultRedisConfig).(T)
	}

	// Create default global config
	return &GlobalConfig[T]{
		ServerAddress: "localhost:5050",
		Store:         storeConfig,
		Logger: observability.LoggerConfig{
			Level: observability.LogLevelInfo,
		},
		Observability: observability.Config{
			ServiceName:    "quorum-quest",
			ServiceVersion: "0.1.0",
			Environment:    "development",
			OTelEndpoint:   "localhost:4317",
		},
		Backend: BackendConfig{
			Type: getBackendTypeFromConfig(storeConfig),
		},
	}
}

// getBackendTypeFromConfig detects the backend type from the store config type
func getBackendTypeFromConfig(storeConfig interface{}) string {
	switch storeConfig.(type) {
	case *scylladb.ScyllaDBConfig:
		return "scylladb"
	case *dynamodb.DynamoDBConfig:
		return "dynamodb"
	default:
		return ""
	}
}

// applyEnvironmentOverrides applies environment variable overrides to the configuration
func applyEnvironmentOverrides(config interface{}) {
	if config == nil {
		return
	}

	// Get the value that the interface points to
	v := reflect.ValueOf(config)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return
	}

	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return
	}

	// Process each field in the struct
	processStruct(v, "QUORUMQUEST")
}

// processStruct processes a struct applying environment variable overrides
func processStruct(v reflect.Value, prefix string) {
	t := v.Type()

	// Iterate through all fields in the struct
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if !fieldValue.CanSet() {
			continue
		}

		// Get the field name from yaml tag if available, otherwise use field name
		fieldName := field.Name
		if yamlTag := field.Tag.Get("yaml"); yamlTag != "" {
			parts := strings.Split(yamlTag, ",")
			if parts[0] != "" && parts[0] != "-" {
				fieldName = parts[0]
			}
		}

		// Construct environment variable name based on field name
		envName := prefix + "_" + strings.ToUpper(fieldName)

		// Handle special case for backend.type
		if fieldName == "type" && prefix == "QUORUMQUEST_BACKEND" {
			// Already handled in DetectBackendType
			continue
		}

		// If the field is a pointer to a struct and not nil, dereference and process
		if fieldValue.Kind() == reflect.Ptr && fieldValue.Type().Elem().Kind() == reflect.Struct && !fieldValue.IsNil() {
			processStruct(fieldValue.Elem(), envName)
			continue
		}

		// If the field is a struct, process it recursively
		if fieldValue.Kind() == reflect.Struct {
			processStruct(fieldValue, envName)
			continue
		}

		// Apply environment variable override
		applyEnvOverride(fieldValue, envName)
	}
}

// applyEnvOverride applies an environment variable override to a field
func applyEnvOverride(fieldValue reflect.Value, envName string) {
	// Check if environment variable exists
	envValue, exists := os.LookupEnv(envName)
	if !exists {
		return
	}

	// Apply override based on field type
	switch fieldValue.Kind() {
	case reflect.String:
		fieldValue.SetString(envValue)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if intValue, err := strconv.ParseInt(envValue, 10, 64); err == nil {
			fieldValue.SetInt(intValue)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if uintValue, err := strconv.ParseUint(envValue, 10, 64); err == nil {
			fieldValue.SetUint(uintValue)
		}
	case reflect.Float32, reflect.Float64:
		if floatValue, err := strconv.ParseFloat(envValue, 64); err == nil {
			fieldValue.SetFloat(floatValue)
		}
	case reflect.Bool:
		if boolValue, err := strconv.ParseBool(envValue); err == nil {
			fieldValue.SetBool(boolValue)
		}
	case reflect.Slice:
		// For slices of strings, split by comma
		if fieldValue.Type().Elem().Kind() == reflect.String {
			values := strings.Split(envValue, ",")
			slice := reflect.MakeSlice(fieldValue.Type(), len(values), len(values))
			for i, v := range values {
				slice.Index(i).SetString(strings.TrimSpace(v))
			}
			fieldValue.Set(slice)
		}
	}
}

// isConfigFileName checks if a filename is a recognized config file name
func isConfigFileName(fileName string) bool {
	configPatterns := []string{"config.yaml", "config.yml", "quorum-quest.yaml", "quorum-quest.yml"}
	for _, pattern := range configPatterns {
		if fileName == pattern {
			return true
		}
	}
	return false
}

// normalizeBackendType standardizes the backend type string
func normalizeBackendType(backendType string) string {
	// Convert to lowercase and trim spaces
	normalized := strings.ToLower(strings.TrimSpace(backendType))

	// Map common variations to standard names
	switch normalized {
	case "scylla", "scylladb":
		return "scylladb"
	case "dynamo", "dynamodb":
		return "dynamodb"
	default:
		return normalized
	}
}
