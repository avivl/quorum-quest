// internal/config/utils.go
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func isConfigFile(path string) bool {
	fileName := filepath.Base(path)
	patterns := []string{"config.yaml", "config.yml", "quorum-quest.yaml", "quorum-quest.yml"}
	for _, pattern := range patterns {
		if fileName == pattern {
			return true
		}
	}
	return false
}

func getFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return -1
	}
	return info.Size()
}

func (l *ConfigLoader) resolveConfigPath(eventPath string) string {
	fileInfo, err := os.Stat(l.configPath)
	if err == nil && fileInfo.IsDir() {
		return eventPath
	}
	return l.configPath
}

func (l *ConfigLoader) handleError(err error) {
	fmt.Printf("%v\n", err)
	l.mu.Lock()
	l.lastError = err
	l.mu.Unlock()
}

func (l *ConfigLoader) updateConfig(newConfig interface{}) {
	l.mu.Lock()
	l.currentConfig = newConfig
	l.lastError = nil
	l.mu.Unlock()
	l.notifyWatchers(newConfig)
}

func (l *ConfigLoader) notifyWatchers(newConfig interface{}) {
	l.watchersMutex.RLock()
	defer l.watchersMutex.RUnlock()
	for _, watcher := range l.watchers {
		watcher(newConfig)
	}
}

// resolveConfigFilePath determines the actual configuration file path
func resolveConfigFilePath(configPath string) (string, error) {
	if configPath == "" {
		return "", fmt.Errorf("config path cannot be empty")
	}

	fileInfo, err := os.Stat(configPath)
	if err != nil {
		return "", err
	}

	if !fileInfo.IsDir() {
		return configPath, nil
	}

	// Try to find a config file in the directory
	candidates := []string{
		filepath.Join(configPath, "config.yaml"),
		filepath.Join(configPath, "config.yml"),
		filepath.Join(configPath, "quorum-quest.yaml"),
		filepath.Join(configPath, "quorum-quest.yml"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no config file found in directory")
}
