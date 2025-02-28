// cmd/quorum-quest-service/main.go
/*package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/avivl/quorum-quest/internal/config"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/server"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/avivl/quorum-quest/internal/store/dynamodb"
	"github.com/avivl/quorum-quest/internal/store/redis"
	"github.com/avivl/quorum-quest/internal/store/scylladb"
)

// BackendType defines the available backend store types
type BackendType string

const (
	BackendDynamoDB BackendType = "dynamodb"
	BackendScyllaDB BackendType = "scylladb"
)

// App represents the application state
type App struct {
	logger       *observability.SLogger
	metrics      *observability.OTelMetrics
	configLoader *config.ConfigLoader
	otelShutdown func()
	grpcServer   interface{} // Will be server.Server[T]
	backendType  BackendType
	configPath   string
}

func main() {
	ctx := context.Background()

	// Parse command-line flags
	configPath := flag.String("config", "/etc/quorum-quest/config.yaml", "Path to configuration file")
	flag.Parse()

	// Initialize the application
	app, err := NewApp(ctx, *configPath)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Setup signal handling for graceful shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Create a context that cancels on signal
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		sig := <-signals
		app.logger.Infof("Received signal: %v", sig)
		cancel()
	}()

	// Run the application
	if err := app.Run(ctx); err != nil {
		app.logger.Errorf("Application error: %v", err)
		os.Exit(1)
	}
}

// NewApp creates a new application instance based on the provided configuration
func NewApp(ctx context.Context, configPath string) (*App, error) {
	// Create basic app structure
	app := &App{
		configPath: configPath,
	}

	// Load configuration and determine backend type
	backendType, err := app.determineBackendType(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to determine backend type: %w", err)
	}
	app.backendType = backendType

	// Initialize the app with the correct backend type
	switch backendType {
	case BackendDynamoDB:
		err = app.initWithDynamoDB(ctx)
	case BackendScyllaDB:
		err = app.initWithScyllaDB(ctx)
	default:
		err = fmt.Errorf("unsupported backend type: %s", backendType)
	}

	if err != nil {
		return nil, err
	}

	return app, nil
}

// determineBackendType identifies the backend type from the configuration file
func (a *App) determineBackendType(configPath string) (BackendType, error) {
	// Read the config file to determine the backend type
	backendType, err := config.DetectBackendType(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to detect backend type: %w", err)
	}

	switch backendType {
	case "dynamodb":
		return BackendDynamoDB, nil
	case "scylladb":
		return BackendScyllaDB, nil
	default:
		return "", fmt.Errorf("unsupported backend type: %s", backendType)
	}
}

// initWithBackend initializes the application with the specified backend type and configuration
// initWithBackend initializes the application with the specified backend type and configuration
func (a *App) initWithBackend(
	ctx context.Context,
	configLoader config.ConfigLoaderFunc,
	storeInitializer func(ctx context.Context, cfg interface{}, logger *observability.SLogger) (store.Store, error),
	setupWatcher func(),
) error {
	// Load configuration
	loader, cfg, err := config.LoadConfig(a.configPath, configLoader)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	a.configLoader = loader

	// Initialize logger
	logger, err := observability.NewLogger(cfg.Logger.Level.GetZapLevel())
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	a.logger = logger

	// Initialize OpenTelemetry
	otelShutdown, err := observability.InitProvider(ctx, cfg.Observability)
	if err != nil {
		return fmt.Errorf("failed to initialize OpenTelemetry: %w", err)
	}
	a.otelShutdown = otelShutdown

	// Initialize metrics
	metrics, err := observability.NewMetricsClient(cfg.Observability, a.logger)
	if err != nil {
		return fmt.Errorf("failed to create metrics client: %w", err)
	}
	a.metrics = metrics

	// Initialize server with store initializer
	grpcServer, err := server.NewServer(cfg, a.logger, a.metrics, storeInitializer)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	a.grpcServer = grpcServer

	// Setup configuration watcher
	setupWatcher()

	return nil
}

// initWithDynamoDB initializes the application with DynamoDB backend
func (a *App) initWithDynamoDB(ctx context.Context) error {
	return a.initWithBackend(
		ctx,
		config.DynamoConfigLoader,
		func(ctx context.Context, genericCfg interface{}, logger *observability.SLogger) (store.Store, error) {
			cfg, ok := genericCfg.(*config.GlobalConfig[*dynamodb.DynamoDBConfig])
			if !ok {
				return nil, fmt.Errorf("invalid config type, expected *config.GlobalConfig[*dynamodb.DynamoDBConfig]")
			}
			return dynamodb.New(ctx, cfg.Store, logger)
		},
		a.setupDynamoDBConfigWatcher,
	)
}

// initWithScyllaDB initializes the application with ScyllaDB backend
func (a *App) initWithScyllaDB(ctx context.Context) error {
	return a.initWithBackend(
		ctx,
		config.ScyllaConfigLoader,
		func(ctx context.Context, genericCfg interface{}, logger *observability.SLogger) (store.Store, error) {
			cfg, ok := genericCfg.(*config.GlobalConfig[*scylladb.ScyllaDBConfig])
			if !ok {
				return nil, fmt.Errorf("invalid config type, expected *config.GlobalConfig[*scylladb.ScyllaDBConfig]")
			}
			return scylladb.New(ctx, cfg.Store, logger)
		},
		a.setupScyllaDBConfigWatcher,
	)
}

// initWithRedis initializes the application with Redis backend
func (a *App) initWithRedis(ctx context.Context) error {
	return a.initWithBackend(
		ctx,
		config.RedisConfigLoader,
		func(ctx context.Context, genericCfg interface{}, logger *observability.SLogger) (store.Store, error) {
			cfg, ok := genericCfg.(*config.GlobalConfig[*redis.RedisConfig])
			if !ok {
				return nil, fmt.Errorf("invalid config type, expected *config.GlobalConfig[*redis.RedisConfig]")
			}
			redisStore, err := redis.NewRedisStore(cfg.Store)
			if err != nil {
				return nil, err
			}
			redisStore.SetLogger(logger)
			return redisStore, nil
		},
		a.setupRedisConfigWatcher,
	)
}

// Run starts the application and blocks until shutdown
func (a *App) Run(ctx context.Context) error {
	a.logger.Info("Starting Quorum Quest Leader Election Service")

	// Start the server based on backend type
	var err error
	switch a.backendType {
	case BackendDynamoDB:
		err = a.runDynamoDBServer(ctx)
	case BackendScyllaDB:
		err = a.runScyllaDBServer(ctx)
	default:
		err = errors.New("unknown backend type")
	}

	if err != nil {
		return err
	}

	// Wait for context cancellation (from signal handler)
	<-ctx.Done()

	// Perform graceful shutdown
	return a.Shutdown()
}

// runDynamoDBServer starts the server with DynamoDB backend
func (a *App) runDynamoDBServer(ctx context.Context) error {
	dynServer, ok := a.grpcServer.(*server.Server[*dynamodb.DynamoDBConfig])
	if !ok {
		return errors.New("invalid server type for DynamoDB backend")
	}

	return dynServer.Start(ctx, func(ctx context.Context, cfg *config.GlobalConfig[*dynamodb.DynamoDBConfig], logger *observability.SLogger) (store.Store, error) {
		return dynamodb.New(ctx, cfg.Store, logger)
	})
}

// runScyllaDBServer starts the server with ScyllaDB backend
func (a *App) runScyllaDBServer(ctx context.Context) error {
	scyllaServer, ok := a.grpcServer.(*server.Server[*scylladb.ScyllaDBConfig])
	if !ok {
		return errors.New("invalid server type for ScyllaDB backend")
	}

	return scyllaServer.Start(ctx, func(ctx context.Context, cfg *config.GlobalConfig[*scylladb.ScyllaDBConfig], logger *observability.SLogger) (store.Store, error) {
		return scylladb.New(ctx, cfg.Store, logger)
	})
}

// setupDynamoDBConfigWatcher sets up configuration watching for DynamoDB backend
func (a *App) setupDynamoDBConfigWatcher() {
	_, ok := a.grpcServer.(*server.Server[*dynamodb.DynamoDBConfig])
	if !ok {
		a.logger.Error("Failed to setup config watcher: invalid server type")
		return
	}

	a.configLoader.AddWatcher(func(newConfig interface{}) {
		_, ok := newConfig.(*config.GlobalConfig[*dynamodb.DynamoDBConfig])
		if !ok {
			a.logger.Error("Invalid configuration type received")
			return
		}

		// Update server configuration
		// For a full implementation, you would need to update server, metrics, etc.
		a.logger.Info("Configuration updated")
	})
}

// setupScyllaDBConfigWatcher sets up configuration watching for ScyllaDB backend
func (a *App) setupScyllaDBConfigWatcher() {
	_, ok := a.grpcServer.(*server.Server[*scylladb.ScyllaDBConfig])
	if !ok {
		a.logger.Error("Failed to setup config watcher: invalid server type")
		return
	}

	a.configLoader.AddWatcher(func(newConfig interface{}) {
		_, ok := newConfig.(*config.GlobalConfig[*scylladb.ScyllaDBConfig])
		if !ok {
			a.logger.Error("Invalid configuration type received")
			return
		}

		// Update server configuration
		// For a full implementation, you would need to update server, metrics, etc.
		a.logger.Info("Configuration updated")
	})
}

// Shutdown gracefully stops the application
func (a *App) Shutdown() error {
	a.logger.Info("Shutting down application")

	// Stop the server based on backend type
	switch a.backendType {
	case BackendDynamoDB:
		if s, ok := a.grpcServer.(*server.Server[*dynamodb.DynamoDBConfig]); ok {
			if err := s.Stop(); err != nil {
				a.logger.Errorf("Error stopping server: %v", err)
			}
		}
	case BackendScyllaDB:
		if s, ok := a.grpcServer.(*server.Server[*scylladb.ScyllaDBConfig]); ok {
			if err := s.Stop(); err != nil {
				a.logger.Errorf("Error stopping server: %v", err)
			}
		}
	}

	// Shutdown OpenTelemetry
	if a.otelShutdown != nil {
		a.otelShutdown()
	}

	a.logger.Info("Application shutdown complete")
	return nil
}
*/