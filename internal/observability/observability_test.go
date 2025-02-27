// internal/observability/observability_test.go
package observability

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLogger(t *testing.T) {
	t.Run("NewLogger", func(t *testing.T) {
		logger, err := NewLogger(zapcore.InfoLevel)
		require.NoError(t, err)
		require.NotNil(t, logger)
	})

	t.Run("LogLevels", func(t *testing.T) {
		logger, logs, err := NewTestLogger()
		require.NoError(t, err)
		logs.TakeAll() // Clear initialization logs

		tests := []struct {
			level   zapcore.Level
			logFunc func(args ...interface{})
			message string
		}{
			{zapcore.DebugLevel, logger.Debug, "debug message"},
			{zapcore.InfoLevel, logger.Info, "info message"},
			{zapcore.WarnLevel, logger.Warn, "warn message"},
			{zapcore.ErrorLevel, logger.Error, "error message"},
		}

		for _, tt := range tests {
			t.Run(tt.level.String(), func(t *testing.T) {
				tt.logFunc(tt.message)
				require.Equal(t, 1, logs.Len())
				entry := logs.All()[0]
				assert.Equal(t, tt.level, entry.Level)
				assert.Equal(t, tt.message, entry.Message)
				logs.TakeAll() // Clear logs for next test
			})
		}
	})

	t.Run("ContextualLogging", func(t *testing.T) {
		logger, logs, err := NewTestLogger()
		require.NoError(t, err)
		logs.TakeAll() // Clear initialization logs

		ctx := context.Background()
		logger.InfoCtx(ctx, "test message")
		// TODO once ther will a real provider agent the value should be 1 and not 2
		require.Equal(t, 2, logs.Len())
		entry := logs.All()[0]
		// TODO once ther will a real provider agent the value should be test message
		assert.Equal(t, "No trace context found", entry.Message)
	})
}

func TestMetrics(t *testing.T) {
	t.Run("NewMetricsClient", func(t *testing.T) {
		logger, err := NewLogger(zapcore.InfoLevel)
		require.NoError(t, err)

		cfg := Config{
			ServiceName:    "test-service",
			ServiceVersion: "1.0.0",
			Environment:    "test",
			OTelEndpoint:   "localhost:4317",
		}

		metrics, err := NewMetricsClient(cfg, logger)
		require.NoError(t, err)
		require.NotNil(t, metrics)
	})
}

func TestOpenTelemetry(t *testing.T) {
	t.Run("InitProvider", func(t *testing.T) {
		cfg := Config{
			ServiceName:    "test-service",
			ServiceVersion: "1.0.0",
			Environment:    "test",
			OTelEndpoint:   "localhost:4317",
		}

		shutdown, err := InitProvider(context.Background(), cfg)
		require.NoError(t, err)
		require.NotNil(t, shutdown)
		defer shutdown()
	})
}

func TestLogLevel(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected zapcore.Level
	}{
		{LogLevelDebug, zapcore.DebugLevel},
		{LogLevelInfo, zapcore.InfoLevel},
		{LogLevelWarn, zapcore.WarnLevel},
		{LogLevelError, zapcore.ErrorLevel},
		{"INVALID_LEVEL", zapcore.InfoLevel}, // Default to Info for invalid levels
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.GetZapLevel())
		})
	}
}

func TestTracing(t *testing.T) {
	t.Run("LogWithTrace", func(t *testing.T) {
		core, recorded := observer.New(zapcore.InfoLevel)
		logger := &SLogger{
			SugaredLogger: zap.New(core, zap.WithFatalHook(zapcore.WriteThenPanic)).Sugar(),
		}

		// Clear any initialization logs
		recorded.TakeAll()

		ctx := context.Background()
		logger.InfoCtx(ctx, "test message with trace")
		entries := recorded.All()
		// TODO once ther will a real provider agent the value should be 1 and not 2
		require.Equal(t, 2, len(entries))
		// TODO once ther will a real provider agent the value should be test message with trace and not No trace context found"
		assert.Equal(t, "No trace context found", entries[0].Message)
	})

	t.Run("MetricsWithTags", func(t *testing.T) {
		// Create a test logger that doesn't output initialization messages
		core, _ := observer.New(zapcore.InfoLevel)
		logger := &SLogger{
			SugaredLogger: zap.New(core, zap.WithFatalHook(zapcore.WriteThenPanic)).Sugar(),
		}

		cfg := Config{
			ServiceName:    "test-service",
			ServiceVersion: "1.0.0",
			Environment:    "test",
			OTelEndpoint:   "localhost:4317",
		}

		metrics, err := NewMetricsClient(cfg, logger)
		require.NoError(t, err)

		ctx := context.Background()
		err = metrics.RecordLatency(ctx, time.Millisecond*100, "operation", "test-op", "status", "success")
		require.NoError(t, err)
	})
}
