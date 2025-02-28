// internal/config/detecor.go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DetectBackendType determines the backend type from the configuration file
func DetectBackendType(configPath string) (string, error) {
	// Check for environment variable override first
	if envType := os.Getenv("QUORUMQUEST_BACKEND_TYPE"); envType != "" {
		return normalizeBackendType(envType), nil
	}

	// Also check QUORUMQUEST_BACKEND for compatibility
	if envType := os.Getenv("QUORUMQUEST_BACKEND"); envType != "" {
		return normalizeBackendType(envType), nil
	}

	// Find the actual config file path
	fileInfo, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("configuration file not found at %s", configPath)
		}
		return "", err
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
			if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
				configFile = candidate
				break
			}
		}

		if configFile == "" {
			return "", fmt.Errorf("no config file found in directory %s", configPath)
		}
	} else {
		configFile = configPath
	}

	// Read and parse the configuration file
	data, err := os.ReadFile(configFile)
	if err != nil {
		return "", fmt.Errorf("failed to read config file: %w", err)
	}

	var config RootConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("invalid configuration file: %w", err)
	}

	if config.Backend.Type == "" {
		return "", fmt.Errorf("backend type not specified in config")
	}

	return normalizeBackendType(config.Backend.Type), nil
}
