// internal/observability/logger_enhanced_test.go
package observability

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// Helper function to create a logger with an observer for testing
func createTestLogger() (*SLogger, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.DebugLevel)
	return &SLogger{SugaredLogger: zap.New(core).Sugar()}, logs
}

func TestNewLogger(t *testing.T) {
	t.Run("default_logger", func(t *testing.T) {
		// This just tests that NewLogger doesn't panic or return an error
		logger, err := NewLogger(zapcore.InfoLevel)
		assert.NoError(t, err)
		assert.NotNil(t, logger)
	})
}

func TestWrapperFunctions(t *testing.T) {
	t.Run("wrap_logger", func(t *testing.T) {
		base, err := zap.NewDevelopment()
		require.NoError(t, err)

		slogger := wrapLogger(base)
		assert.NotNil(t, slogger)
		assert.Equal(t, base.Sugar(), slogger.SugaredLogger)
	})

	t.Run("new_slogger", func(t *testing.T) {
		base, err := zap.NewDevelopment()
		require.NoError(t, err)

		slogger := newSLogger(base)
		assert.NotNil(t, slogger)
		assert.Equal(t, base.Sugar(), slogger.SugaredLogger)
	})
}

func TestNewTestLogger(t *testing.T) {
	logger, logs, err := NewTestLogger()
	require.NoError(t, err)
	require.NotNil(t, logger)
	require.NotNil(t, logs)

	// Test that the logger works
	logger.Info("test message")

	entries := logs.All()
	require.Equal(t, 1, len(entries))
	assert.Equal(t, "test message", entries[0].Message)
}

func TestBasicLogging(t *testing.T) {
	logger, logs := createTestLogger()

	// Test different log levels
	testCases := []struct {
		name    string
		message string
		level   zapcore.Level
	}{
		{"debug", "debug message", zapcore.DebugLevel},
		{"info", "info message", zapcore.InfoLevel},
		{"warn", "warn message", zapcore.WarnLevel},
		{"error", "error message", zapcore.ErrorLevel},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logs.TakeAll() // Clear previous logs

			// Call the appropriate log function based on level
			switch tc.level {
			case zapcore.DebugLevel:
				logger.Debug(tc.message)
			case zapcore.InfoLevel:
				logger.Info(tc.message)
			case zapcore.WarnLevel:
				logger.Warn(tc.message)
			case zapcore.ErrorLevel:
				logger.Error(tc.message)
			}

			entries := logs.All()
			require.Equal(t, 1, len(entries))
			assert.Equal(t, tc.message, entries[0].Message)
			assert.Equal(t, tc.level, entries[0].Level)
		})
	}
}

// Simple mock that skips the trace checks
func TestLogWithContext_NoTrace(t *testing.T) {
	logger, logs := createTestLogger()
	ctx := context.Background()

	// Test log with context but no trace info
	logger.LogWithContext(ctx, zapcore.InfoLevel, "test message")

	// Should log "No trace context found" and then the message
	entries := logs.AllUntimed()
	require.Equal(t, 2, len(entries))
	assert.Equal(t, "No trace context found", entries[0].Message)
	assert.Equal(t, zapcore.InfoLevel, entries[0].Level)
	assert.Equal(t, "test message", entries[1].Message)
	assert.Equal(t, zapcore.InfoLevel, entries[1].Level)
}

func TestInfoCtx_NoTrace(t *testing.T) {
	logger, logs := createTestLogger()
	ctx := context.Background()

	// Test InfoCtx with no trace
	message := "info message"
	logger.InfoCtx(ctx, message)

	// Should log "No trace context found" and then the message
	entries := logs.AllUntimed()
	require.Equal(t, 2, len(entries))
	assert.Equal(t, "No trace context found", entries[0].Message)
	assert.Equal(t, message, entries[1].Message)
}

func TestErrorCtx_NoTrace(t *testing.T) {
	logger, logs := createTestLogger()
	ctx := context.Background()

	// Test ErrorCtx with no trace
	err := errors.New("test error")
	logger.ErrorCtx(ctx, err)

	// Should log "No trace context found" and then the error
	entries := logs.AllUntimed()
	require.Equal(t, 2, len(entries))
	assert.Equal(t, "No trace context found", entries[0].Message)
	assert.Equal(t, zapcore.InfoLevel, entries[0].Level)
	assert.Equal(t, err.Error(), entries[1].Message)
	assert.Equal(t, zapcore.ErrorLevel, entries[1].Level)
}

func TestGetTraceID_NoTrace(t *testing.T) {
	// Test with no valid trace
	traceID, ok := GetTraceID(context.Background())
	assert.False(t, ok)
	assert.Equal(t, "", traceID)
}
