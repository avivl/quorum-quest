// internal/observability/logger_test.go
package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLogWithContext(t *testing.T) {
	// Setup test logger with observer
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := &SLogger{
		SugaredLogger: zap.New(core).Sugar(),
	}

	// Create a context without trace
	ctx := context.Background()

	// Test with no trace
	t.Run("no_trace", func(t *testing.T) {
		recorded.TakeAll() // Clear previous logs

		logger.LogWithContext(ctx, zapcore.InfoLevel, "test message")

		logs := recorded.AllUntimed()
		require.Equal(t, 2, len(logs), "Expected two log entries (no trace found + message)")
		assert.Equal(t, "No trace context found", logs[0].Message)
		assert.Equal(t, "test message", logs[1].Message)
	})
}

func TestErrorCtx(t *testing.T) {
	// Setup test logger with observer
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := &SLogger{
		SugaredLogger: zap.New(core).Sugar(),
	}

	// Test with error
	t.Run("with_error", func(t *testing.T) {
		recorded.TakeAll() // Clear previous logs

		err := assert.AnError
		logger.ErrorCtx(context.Background(), err)

		logs := recorded.AllUntimed()
		require.Equal(t, 2, len(logs), "Expected two log entries (no trace found + error)")
		assert.Equal(t, "No trace context found", logs[0].Message)
		assert.Equal(t, err.Error(), logs[1].Message)
	})
}

func TestGetTraceID(t *testing.T) {
	// Test with no valid trace
	t.Run("no_trace", func(t *testing.T) {
		traceID, ok := GetTraceID(context.Background())
		assert.False(t, ok)
		assert.Equal(t, "", traceID)
	})
}
