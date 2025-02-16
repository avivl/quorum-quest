package main

import (
	"context"
	"log"
	"os"

	"github.com/avivl/quorum-quest/internal/observability"

	"go.uber.org/zap/zapcore"
)

func main() {
	ctx := context.Background()

	cfg := observability.Config{
		ServiceName:    "my-service",
		ServiceVersion: "1.0.0",
		Environment:    os.Getenv("ENV"),
		OTelEndpoint:   "localhost:4317",
	}

	// Initialize OpenTelemetry
	shutdown, err := observability.InitProvider(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown()

	// Create logger
	logger, err := observability.NewLogger(zapcore.InfoLevel)
	if err != nil {
		log.Fatal(err)
	}

	// Create metrics client
	metrics, err := observability.NewMetricsClient(cfg, logger)
	if err != nil {
		log.Fatal(err)
	}

	// Use the observability tools
	metrics.Increment(ctx, "my_counter", 1, "key", "value")
	logger.InfoCtx(ctx, "Hello with trace context")
}
