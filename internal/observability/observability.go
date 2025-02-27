// internal/observability/observability.go
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
)

// MetricsClient interface for metrics operations
type MetricsClient interface {
	// Increment increments a counter by the given amount
	Increment(ctx context.Context, name string, value int64, attributes ...string)
	// RecordLatency records a latency metric with tags
	RecordLatency(ctx context.Context, duration time.Duration, tags ...string) error
}

// OTelMetrics implements MetricsClient using OpenTelemetry
type OTelMetrics struct {
	meter            metric.Meter
	logger           *SLogger
	latencyHistogram metric.Float64Histogram
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
func NewMetricsClient(cfg Config, logger *SLogger) (*OTelMetrics, error) {
	meter := otel.GetMeterProvider().Meter(cfg.ServiceName)
	latencyHistogram, err := meter.Float64Histogram("operation.latency")
	if err != nil {
		return nil, fmt.Errorf("failed to create latency histogram: %w", err)
	}

	return &OTelMetrics{
		meter:            meter,
		logger:           logger,
		latencyHistogram: latencyHistogram,
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

// RecordLatency records a latency metric with tags
func (m *OTelMetrics) RecordLatency(ctx context.Context, duration time.Duration, tags ...string) error {
	attrs := attributesFromTags(tags)
	m.latencyHistogram.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))
	return nil
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
