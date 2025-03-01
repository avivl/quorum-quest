// internal/server/server_test.go
package server

import (
	"context"
	"testing"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store/scylladb"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
)

// setupTestLogger creates a logger for testing
func setupTestLogger(t *testing.T) *observability.SLogger {
	logger, err := observability.NewLogger(zapcore.InfoLevel, zap.Development())
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	return logger
}

// setupTestMetrics creates metrics for testing
func setupTestMetrics(t *testing.T, logger *observability.SLogger) *observability.OTelMetrics {
	metricsConfig := observability.Config{
		ServiceName:    "test-server",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		OTelEndpoint:   "localhost:4317",
	}

	metrics, err := observability.NewMetricsClient(metricsConfig, logger)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}
	return metrics
}

// TestUnaryServerInterceptor tests the server interceptor
func TestUnaryServerInterceptor(t *testing.T) {
	// Create a minimal server instance
	logger, _, _ := observability.NewTestLogger()
	metrics, _ := observability.NewMetricsClient(observability.Config{ServiceName: "test"}, logger)

	// Use ScyllaDBConfig which implements the StoreConfig interface
	server := &Server[*scylladb.ScyllaDBConfig]{
		logger:  logger,
		metrics: metrics,
	}

	// Create the interceptor
	interceptor := server.unaryServerInterceptor()

	// Mock handler function
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	}

	// Create server info
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/TestMethod",
	}

	// Call interceptor
	resp, err := interceptor(context.Background(), "request", info, handler)

	// Validate
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if resp != "response" {
		t.Errorf("Expected 'response', got: %v", resp)
	}
}
