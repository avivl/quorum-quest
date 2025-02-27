// internal/observability/logger.go
package observability

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// SLogger is a wrapper for a zap sugared logger with OpenTelemetry integration
type SLogger struct {
	*zap.SugaredLogger
}

const (
	traceIDKey = "trace_id"
	spanIDKey  = "span_id"
)

// NewLogger constructs a new sugared logger with OpenTelemetry integration
func NewLogger(level zapcore.Level, options ...zap.Option) (*SLogger, error) {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(level)
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	baseLogger, err := config.Build(options...)
	if err != nil {
		return nil, err
	}

	logger := wrapLogger(baseLogger)
	logger.Info("Initialized Logger level:" + config.Level.String())

	return logger, nil
}

func wrapLogger(logger *zap.Logger) *SLogger {
	return &SLogger{logger.Sugar()}
}

// getTraceInfo gets the trace and span metadata from context
func getTraceInfo(ctx context.Context) (trace.TraceID, trace.SpanID, bool) {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return trace.TraceID{}, trace.SpanID{}, false
	}

	return span.SpanContext().TraceID(), span.SpanContext().SpanID(), true
}

// LogWithContext logs a message with trace context at the specified level
func (l *SLogger) LogWithContext(ctx context.Context, level zapcore.Level, msg string, fields ...zap.Field) {
	traceID, spanID, ok := getTraceInfo(ctx)
	if !ok {
		l.Info("No trace context found")
		l.Info(msg)
		return
	}

	// Convert fields to key-value pairs for SugaredLogger
	keyValues := make([]interface{}, 0, len(fields)*2+4)
	keyValues = append(keyValues, traceIDKey, traceID.String(), spanIDKey, spanID.String())

	for _, field := range fields {
		keyValues = append(keyValues, field.Key, field.String)
	}

	switch level {
	case zapcore.InfoLevel:
		l.Infow(msg, keyValues...)
	case zapcore.ErrorLevel:
		l.Errorw(msg, keyValues...)
	case zapcore.WarnLevel:
		l.Warnw(msg, keyValues...)
	case zapcore.DebugLevel:
		l.Debugw(msg, keyValues...)
	default:
		l.Infow(msg, keyValues...)
	}
}

// InfoCtx logs a message with trace context
func (l *SLogger) InfoCtx(ctx context.Context, msg string) {
	traceID, spanID, ok := getTraceInfo(ctx)
	if !ok {
		l.Info("No trace context found")
		l.Info(msg)
		return
	}

	l.Infow(msg, traceIDKey, traceID.String(), spanIDKey, spanID.String())
}

// ErrorCtx logs an error with trace context
func (l *SLogger) ErrorCtx(ctx context.Context, err error) {
	traceID, spanID, ok := getTraceInfo(ctx)
	if !ok {
		l.Info("No trace context found")
		l.Error(err)
		return
	}

	l.Errorw(err.Error(), traceIDKey, traceID.String(), spanIDKey, spanID.String())
}

// GetTraceID returns the trace ID from context
func GetTraceID(ctx context.Context) (string, bool) {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return "", false
	}
	return span.SpanContext().TraceID().String(), true
}

// NewTestLogger creates a logger for testing
func NewTestLogger() (*SLogger, *observer.ObservedLogs, error) {
	observer, observedLogs := observer.New(zapcore.DebugLevel)
	observedOpt := zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return observer
	})

	baseLogger, err := zap.NewDevelopment(observedOpt)
	if err != nil {
		return nil, nil, err
	}

	return newSLogger(baseLogger), observedLogs, nil
}

// Add this function near the other SLogger-related code
func newSLogger(logger *zap.Logger) *SLogger {
	return &SLogger{
		SugaredLogger: logger.Sugar(),
	}
}
