package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
)

// SLogger is a wrapper for a zap sugared logger with OpenTelemetry integration
type SLogger struct {
	*zap.SugaredLogger
}

const (
	traceIDKey = "trace_id"
	spanIDKey  = "span_id"
)

// MetricsClient interface for metrics operations
type MetricsClient interface {
	// Increment increments a counter by the given amount
	Increment(ctx context.Context, name string, value int64, attributes ...string)
}

// OTelMetrics implements MetricsClient using OpenTelemetry
type OTelMetrics struct {
	meter  metric.Meter
	logger *SLogger
}

// Config holds the configuration for observability components
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTelEndpoint   string
}

// InitProvider initializes OpenTelemetry with the given configuration
func InitProvider(ctx context.Context, cfg Config) (func(), error) {
	if cfg.OTelEndpoint == "" {
		cfg.OTelEndpoint = "localhost:4317"
	}

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize trace provider
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTelEndpoint),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithDialOption(grpc.WithBlock()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Initialize metrics provider
	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(cfg.OTelEndpoint),
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithDialOption(grpc.WithBlock()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(10*time.Second)),
		),
	)
	otel.SetMeterProvider(meterProvider)

	// Return cleanup function
	return func() {
		if err := tracerProvider.Shutdown(ctx); err != nil {
			fmt.Printf("Error shutting down tracer provider: %v", err)
		}
		if err := meterProvider.Shutdown(ctx); err != nil {
			fmt.Printf("Error shutting down meter provider: %v", err)
		}
	}, nil
}

// NewMetricsClient creates a new OpenTelemetry metrics client
func NewMetricsClient(cfg Config, l *SLogger) (*OTelMetrics, error) {
	meter := otel.GetMeterProvider().Meter(
		cfg.ServiceName,
		metric.WithInstrumentationVersion(cfg.ServiceVersion),
	)

	return &OTelMetrics{
		meter:  meter,
		logger: l,
	}, nil
}

// Increment increments a counter metric
func (m *OTelMetrics) Increment(ctx context.Context, name string, value int64, attributes ...string) {
	counter, err := m.meter.Int64Counter(name)
	if err != nil {
		m.logger.Errorf("Failed to create counter metric '%s': %v", name, err)
		return
	}

	attrs := attributesFromTags(attributes)
	counter.Add(ctx, value, metric.WithAttributes(attrs...))
}

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

// Helper function to convert string tags to OpenTelemetry attributes
func attributesFromTags(tags []string) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(tags)/2)
	for i := 0; i < len(tags); i += 2 {
		if i+1 < len(tags) {
			attrs = append(attrs, attribute.String(tags[i], tags[i+1]))
		}
	}
	return attrs
}

// Add this function near the other SLogger-related code
func newSLogger(logger *zap.Logger) *SLogger {
	return &SLogger{
		SugaredLogger: logger.Sugar(),
	}
}
