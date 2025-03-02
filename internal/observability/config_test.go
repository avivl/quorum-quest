// internal/observability/config_test.go
package observability

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

func TestLogLevelConversion(t *testing.T) {
	testCases := []struct {
		name          string
		logLevel      LogLevel
		expectedLevel zapcore.Level
	}{
		{"debug_level", LogLevelDebug, zapcore.DebugLevel},
		{"info_level", LogLevelInfo, zapcore.InfoLevel},
		{"warn_level", LogLevelWarn, zapcore.WarnLevel},
		{"error_level", LogLevelError, zapcore.ErrorLevel},
		{"empty_level", "", zapcore.InfoLevel},
		{"invalid_level", "INVALID", zapcore.InfoLevel},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualLevel := tc.logLevel.GetZapLevel()
			assert.Equal(t, tc.expectedLevel, actualLevel)
		})
	}
}

func TestConfigStructs(t *testing.T) {
	t.Run("config_defaults", func(t *testing.T) {
		// Test that an empty Config is a valid struct
		cfg := Config{}

		// Verify fields are empty but it's a valid struct
		assert.Equal(t, "", cfg.ServiceName)
		assert.Equal(t, "", cfg.ServiceVersion)
		assert.Equal(t, "", cfg.Environment)
		assert.Equal(t, "", cfg.OTelEndpoint)
	})

	t.Run("config_values", func(t *testing.T) {
		// Test that values are assigned correctly
		cfg := Config{
			ServiceName:    "test-service",
			ServiceVersion: "1.0.0",
			Environment:    "test",
			OTelEndpoint:   "localhost:4317",
		}

		assert.Equal(t, "test-service", cfg.ServiceName)
		assert.Equal(t, "1.0.0", cfg.ServiceVersion)
		assert.Equal(t, "test", cfg.Environment)
		assert.Equal(t, "localhost:4317", cfg.OTelEndpoint)
	})

	t.Run("logger_config", func(t *testing.T) {
		// Test LoggerConfig
		logCfg := LoggerConfig{
			Level: LogLevelDebug,
		}

		assert.Equal(t, LogLevelDebug, logCfg.Level)
		assert.Equal(t, zapcore.DebugLevel, logCfg.Level.GetZapLevel())
	})
}

func TestLogLevelConstants(t *testing.T) {
	// Test that constants are defined correctly
	assert.Equal(t, LogLevel("LOG_LEVELS_DEBUGLEVEL"), LogLevelDebug)
	assert.Equal(t, LogLevel("LOG_LEVELS_INFOLEVEL"), LogLevelInfo)
	assert.Equal(t, LogLevel("LOG_LEVELS_WARNLEVEL"), LogLevelWarn)
	assert.Equal(t, LogLevel("LOG_LEVELS_ERRORLEVEL"), LogLevelError)
}
