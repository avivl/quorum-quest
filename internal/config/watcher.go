// internal/config/wathcer.go
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
)

// watchConfig watches for configuration file changes
func (l *ConfigLoader) watchConfig() {
	for {
		select {
		case <-l.stopChan:
			return

		case event, ok := <-l.watcher.Events:
			if !ok {
				return
			}

			if isConfigFile(event.Name) && (event.Op&fsnotify.Write == fsnotify.Write) {
				l.handleFileModification(event.Name)
			}

		case err, ok := <-l.watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("Config watcher error: %v\n", err)
			l.mu.Lock()
			l.lastError = err
			l.mu.Unlock()
		}
	}
}

func (l *ConfigLoader) handleFileModification(filePath string) {
	l.waitForFileStability(filePath)

	configFilePath := l.resolveConfigPath(filePath)
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		l.handleError(fmt.Errorf("error reading config file: %w", err))
		return
	}

	newConfig, err := l.loaderFunc(data)
	if err != nil {
		l.handleError(fmt.Errorf("error parsing config: %w", err))
		return
	}

	applyEnvironmentOverrides(newConfig)

	l.updateConfig(newConfig)
}

func (l *ConfigLoader) waitForFileStability(filePath string) {
	if os.Getenv("GO_TEST_MODE") == "1" {
		return
	}

	var lastSize int64 = -1
	backoff := 50 * time.Millisecond
	for retries := 0; retries < 3; retries++ {
		if size := getFileSize(filePath); size != lastSize {
			lastSize = size
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		break
	}
}
