// internal/config/loader.go
// Package config handles configuration loading and watching
package config

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/avivl/quorum-quest/internal/store"
	"github.com/fsnotify/fsnotify"
)

// ConfigLoader watches configuration file changes and notifies watchers
type ConfigLoader struct {
	configPath    string
	actualFile    string
	watcher       *fsnotify.Watcher
	watchers      []func(interface{})
	watchersMutex sync.RWMutex
	loaderFunc    ConfigLoaderFunc
	mu            sync.RWMutex
	currentConfig interface{}
	lastError     error
	stopChan      chan struct{}
}

// NewConfigLoader creates a new config loader with the given path
func NewConfigLoader(configPath string) *ConfigLoader {
	return &ConfigLoader{
		configPath: configPath,
		watchers:   make([]func(interface{}), 0),
		stopChan:   make(chan struct{}),
	}
}

// AddWatcher adds a function that will be called when the configuration changes
func (l *ConfigLoader) AddWatcher(watcher func(interface{})) {
	l.watchersMutex.Lock()
	defer l.watchersMutex.Unlock()
	l.watchers = append(l.watchers, watcher)
}

// GetCurrentConfig returns the current configuration
func (l *ConfigLoader) GetCurrentConfig() interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.currentConfig
}

// GetLastError returns the last error encountered during config loading
func (l *ConfigLoader) GetLastError() error {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lastError
}

// Close closes the configuration watcher
func (l *ConfigLoader) Close() error {
	if l.stopChan != nil {
		close(l.stopChan)
	}
	if l.watcher != nil {
		return l.watcher.Close()
	}
	return nil
}

// LoadConfig loads the configuration from the given path
func LoadConfig[T store.StoreConfig](configPath string, loaderFunc ConfigLoaderFunc) (*ConfigLoader, *GlobalConfig[T], error) {
	// Create a default config first
	config := createDefaultConfig[T]()

	// Try to load from file if it exists
	if fileConfig, err := loadFromFile[T](configPath, loaderFunc); err == nil {
		config = fileConfig
	} else if !os.IsNotExist(err) && !strings.Contains(err.Error(), "no config file found") {
		// Only return error if it's not a missing file error
		return nil, nil, err
	}
	// Otherwise use the default config we created

	// Apply environment variable overrides
	applyEnvironmentOverrides(config)

	// Create file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Create config loader
	loader := &ConfigLoader{
		configPath: configPath,
		watcher:    watcher,
		loaderFunc: loaderFunc,
	}

	// Store the current config
	loader.mu.Lock()
	loader.currentConfig = config
	loader.mu.Unlock()

	// Start watching config file
	go loader.watchConfig()

	// Setup the watcher
	if err := setupWatcher(loader); err != nil {
		watcher.Close()
		return nil, nil, err
	}

	return loader, config, nil
}
