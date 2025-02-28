package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/avivl/quorum-quest/internal/config"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store/scylladb"
)

type App struct {
	logger        *observability.SLogger
	metrics       *observability.OTelMetrics
	store         *scylladb.Store
	otelShutdown  func()
	configLoader  *config.ConfigLoader
	currentConfig *config.GlobalConfig[*scylladb.ScyllaDBConfig]
}

func NewApp(configPath string) (*App, error) {
	// Load initial configuration
	loader, cfg, err := config.LoadConfig[*scylladb.ScyllaDBConfig](configPath, config.ScyllaConfigLoader)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger
	logger, err := observability.NewLogger(cfg.Logger.Level.GetZapLevel())
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// Initialize OpenTelemetry
	otelShutdown, err := observability.InitProvider(context.Background(), cfg.Observability)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OpenTelemetry: %w", err)
	}

	// Initialize metrics
	metrics, err := observability.NewMetricsClient(cfg.Observability, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics client: %w", err)
	}

	// Initialize store
	store, err := scylladb.New(context.Background(), cfg.Store, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	app := &App{
		logger:        logger,
		metrics:       metrics,
		store:         store,
		otelShutdown:  otelShutdown,
		configLoader:  loader,
		currentConfig: cfg,
	}

	// Setup configuration watcher
	app.setupConfigWatcher()

	return app, nil
}

func (a *App) setupConfigWatcher() {
	a.configLoader.AddWatcher(func(newConfig interface{}) {
		cfg, ok := newConfig.(*config.GlobalConfig[*scylladb.ScyllaDBConfig])
		if !ok {
			a.logger.Error("Invalid configuration type received")
			return
		}

		// Update current configuration
		a.currentConfig = cfg

		// Handle logger changes
		if err := a.updateLogger(cfg.Logger); err != nil {
			a.logger.Errorf("Failed to update logger: %v", err)
		}

		// Handle observability changes
		if err := a.updateObservability(cfg.Observability); err != nil {
			a.logger.Errorf("Failed to update observability: %v", err)
		}

		// Handle store changes
		if err := a.updateStore(cfg.Store); err != nil {
			a.logger.Errorf("Failed to update store: %v", err)
		}

		a.logger.Info("Configuration updated successfully")
	})
}

func (a *App) updateLogger(cfg observability.LoggerConfig) error {
	// Create new logger with updated level
	newLogger, err := observability.NewLogger(cfg.Level.GetZapLevel())
	if err != nil {
		return fmt.Errorf("failed to create new logger: %w", err)
	}

	// Update logger
	a.logger = newLogger
	return nil
}

func (a *App) updateObservability(cfg observability.Config) error {
	// Shutdown existing OpenTelemetry provider
	if a.otelShutdown != nil {
		a.otelShutdown()
	}

	// Initialize new OpenTelemetry provider
	shutdown, err := observability.InitProvider(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize OpenTelemetry: %w", err)
	}

	// Update metrics client
	metrics, err := observability.NewMetricsClient(cfg, a.logger)
	if err != nil {
		return fmt.Errorf("failed to create metrics client: %w", err)
	}

	// Update app state
	a.otelShutdown = shutdown
	a.metrics = metrics
	return nil
}

func (a *App) updateStore(cfg *scylladb.ScyllaDBConfig) error {
	// Close existing store
	if a.store != nil {
		a.store.Close()
	}

	// Create new store with updated config
	store, err := scylladb.New(context.Background(), cfg, a.logger)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}

	// Update app state
	a.store = store
	return nil
}

func (a *App) Run() error {
	// Setup signal handling
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	a.logger.Info("Application started")

	// Wait for shutdown signal
	<-signals

	return a.Shutdown()
}

func (a *App) Shutdown() error {
	a.logger.Info("Shutting down application")

	// Close store
	if a.store != nil {
		a.store.Close()
	}

	// Shutdown OpenTelemetry
	if a.otelShutdown != nil {
		a.otelShutdown()
	}

	return nil
}

func main() {
	app, err := NewApp("/etc/myapp")
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
