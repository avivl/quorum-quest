// cmd/quorum-quest-service/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/avivl/quorum-quest/internal/config"
	"github.com/avivl/quorum-quest/internal/lockservice"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/server"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/avivl/quorum-quest/internal/store/dynamodb"
	"github.com/avivl/quorum-quest/internal/store/redis"
	"github.com/avivl/quorum-quest/internal/store/scylladb"
	"gopkg.in/yaml.v3"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "./config", "Path to configuration file or directory")
	flag.Parse()

	// Create context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Detect backend type from config
	backendType, err := config.DetectBackendType(*configPath)
	if err != nil {
		log.Fatalf("Failed to detect backend type: %v", err)
	}

	// Initialize logger
	logger, err := observability.NewLogger(observability.LogLevelInfo.GetZapLevel())
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	logger.Infof("Starting quorum-quest service with %s backend", backendType)

	// Initialize server and store initializer
	var srv interface{}
	var storeInit interface{}

	switch backendType {
	case "scylladb":
		configLoader, configObj, err := config.LoadConfig[*scylladb.ScyllaDBConfig](*configPath, config.ScyllaConfigLoader)
		if err != nil {
			logger.Fatalf("Failed to load ScyllaDB config: %v", err)
		}
		defer configLoader.Close()

		// Initialize observability
		cleanup, err := observability.InitProvider(ctx, configObj.Observability)
		if err != nil {
			logger.Fatalf("Failed to initialize observability: %v", err)
		}
		defer cleanup()

		// Create metrics client
		metrics, err := observability.NewMetricsClient(configObj.Observability, logger)
		if err != nil {
			logger.Fatalf("Failed to create metrics client: %v", err)
		}

		// Create server with ScyllaDB store
		srv, err = server.NewServer(
			configObj,
			logger,
			metrics,
			func(ctx context.Context, cfg *config.GlobalConfig[*scylladb.ScyllaDBConfig], logger *observability.SLogger) (store.Store, error) {
				// Create ScyllaDB store
				lockStore, err := lockservice.NewStore(ctx, "scylladb", cfg.Store, logger)
				if err != nil {
					return nil, fmt.Errorf("failed to initialize ScyllaDB store: %w", err)
				}
				// Since our server expects store.Store but we have store.LockStore, we need to cast if possible
				storeImpl, ok := lockStore.(store.Store)
				if !ok {
					lockStore.Close() // clean up resources
					return nil, fmt.Errorf("lockStore does not implement store.Store interface")
				}
				return storeImpl, nil
			},
		)
		if err != nil {
			logger.Fatalf("Failed to create server: %v", err)
		}

		// Set store initialization function for later use
		storeInit = func(ctx context.Context, cfg *config.GlobalConfig[*scylladb.ScyllaDBConfig], logger *observability.SLogger) (store.Store, error) {
			// Create ScyllaDB store
			lockStore, err := lockservice.NewStore(ctx, "scylladb", cfg.Store, logger)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize ScyllaDB store: %w", err)
			}
			storeImpl, ok := lockStore.(store.Store)
			if !ok {
				lockStore.Close()
				return nil, fmt.Errorf("lockStore does not implement store.Store interface")
			}
			return storeImpl, nil
		}

	case "dynamodb":
		configLoader, configObj, err := config.LoadConfig[*dynamodb.DynamoDBConfig](*configPath, config.DynamoConfigLoader)
		if err != nil {
			logger.Fatalf("Failed to load DynamoDB config: %v", err)
		}
		defer configLoader.Close()

		// Initialize observability
		cleanup, err := observability.InitProvider(ctx, configObj.Observability)
		if err != nil {
			logger.Fatalf("Failed to initialize observability: %v", err)
		}
		defer cleanup()

		// Create metrics client
		metrics, err := observability.NewMetricsClient(configObj.Observability, logger)
		if err != nil {
			logger.Fatalf("Failed to create metrics client: %v", err)
		}

		// Create server with DynamoDB store
		srv, err = server.NewServer(
			configObj,
			logger,
			metrics,
			func(ctx context.Context, cfg *config.GlobalConfig[*dynamodb.DynamoDBConfig], logger *observability.SLogger) (store.Store, error) {
				// Create DynamoDB store
				storeImpl, err := dynamodb.NewStore(ctx, cfg.Store, logger)
				if err != nil {
					return nil, fmt.Errorf("failed to initialize DynamoDB store: %w", err)
				}
				return storeImpl, nil
			},
		)
		if err != nil {
			logger.Fatalf("Failed to create server: %v", err)
		}

		// Store storeInit function for later use
		storeInit = func(ctx context.Context, cfg *config.GlobalConfig[*dynamodb.DynamoDBConfig], logger *observability.SLogger) (store.Store, error) {
			storeImpl, err := dynamodb.NewStore(ctx, cfg.Store, logger)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize DynamoDB store: %w", err)
			}
			return storeImpl, nil
		}

	case "redis":
		configLoader, configObj, err := config.LoadConfig[*redis.RedisConfig](*configPath, func(data []byte) (interface{}, error) {
			// Create a default config
			defaultConfig := &config.GlobalConfig[*redis.RedisConfig]{
				Store:         redis.NewRedisConfig(),
				ServerAddress: "localhost:5050",
				Logger: observability.LoggerConfig{
					Level: observability.LogLevelInfo,
				},
				Observability: observability.Config{
					ServiceName:    "quorum-quest",
					ServiceVersion: "0.1.0",
					Environment:    "development",
					OTelEndpoint:   "localhost:4317",
				},
				Backend: config.BackendConfig{
					Type: "redis",
				},
			}

			// If we have data, unmarshal it
			if len(data) > 0 {
				if err := yaml.Unmarshal(data, defaultConfig); err != nil {
					return nil, fmt.Errorf("failed to unmarshal Redis config: %w", err)
				}
			}

			return defaultConfig, nil
		})
		if err != nil {
			logger.Fatalf("Failed to load Redis config: %v", err)
		}
		defer configLoader.Close()

		// Initialize observability
		cleanup, err := observability.InitProvider(ctx, configObj.Observability)
		if err != nil {
			logger.Fatalf("Failed to initialize observability: %v", err)
		}
		defer cleanup()

		// Create metrics client
		metrics, err := observability.NewMetricsClient(configObj.Observability, logger)
		if err != nil {
			logger.Fatalf("Failed to create metrics client: %v", err)
		}

		// Create server with Redis store
		srv, err = server.NewServer(
			configObj,
			logger,
			metrics,
			func(ctx context.Context, cfg *config.GlobalConfig[*redis.RedisConfig], logger *observability.SLogger) (store.Store, error) {
				// Create Redis store
				lockStore, err := lockservice.NewStore(ctx, "redis", cfg.Store, logger)
				if err != nil {
					return nil, fmt.Errorf("failed to initialize Redis store: %w", err)
				}
				storeImpl, ok := lockStore.(store.Store)
				if !ok {
					lockStore.Close()
					return nil, fmt.Errorf("lockStore does not implement store.Store interface")
				}
				return storeImpl, nil
			},
		)
		if err != nil {
			logger.Fatalf("Failed to create server: %v", err)
		}

		// Store storeInit function for later use
		storeInit = func(ctx context.Context, cfg *config.GlobalConfig[*redis.RedisConfig], logger *observability.SLogger) (store.Store, error) {
			lockStore, err := lockservice.NewStore(ctx, "redis", cfg.Store, logger)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize Redis store: %w", err)
			}
			storeImpl, ok := lockStore.(store.Store)
			if !ok {
				lockStore.Close()
				return nil, fmt.Errorf("lockStore does not implement store.Store interface")
			}
			return storeImpl, nil
		}

	default:
		logger.Fatalf("Unsupported backend type: %s", backendType)
	}

	// Set up signal handling for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the server based on its type
	switch s := srv.(type) {
	case *server.Server[*scylladb.ScyllaDBConfig]:
		initFunc := storeInit.(func(context.Context, *config.GlobalConfig[*scylladb.ScyllaDBConfig], *observability.SLogger) (store.Store, error))
		go func() {
			if err := s.Start(ctx, initFunc); err != nil {
				logger.Fatalf("Failed to start server: %v", err)
			}
		}()
		logger.Info("Server started")

	case *server.Server[*dynamodb.DynamoDBConfig]:
		initFunc := storeInit.(func(context.Context, *config.GlobalConfig[*dynamodb.DynamoDBConfig], *observability.SLogger) (store.Store, error))
		go func() {
			if err := s.Start(ctx, initFunc); err != nil {
				logger.Fatalf("Failed to start server: %v", err)
			}
		}()
		logger.Info("Server started")

	case *server.Server[*redis.RedisConfig]:
		initFunc := storeInit.(func(context.Context, *config.GlobalConfig[*redis.RedisConfig], *observability.SLogger) (store.Store, error))
		go func() {
			if err := s.Start(ctx, initFunc); err != nil {
				logger.Fatalf("Failed to start server: %v", err)
			}
		}()
		logger.Info("Server started")

	default:
		logger.Fatalf("Unsupported server type")
	}

	// Wait for shutdown signal
	sig := <-signalChan
	logger.Infof("Received signal: %v, initiating shutdown", sig)

	// Shutdown server gracefully
	switch s := srv.(type) {
	case *server.Server[*scylladb.ScyllaDBConfig]:
		if err := s.Stop(); err != nil {
			logger.Errorf("Error shutting down server: %v", err)
		}
	case *server.Server[*dynamodb.DynamoDBConfig]:
		if err := s.Stop(); err != nil {
			logger.Errorf("Error shutting down server: %v", err)
		}
	case *server.Server[*redis.RedisConfig]:
		if err := s.Stop(); err != nil {
			logger.Errorf("Error shutting down server: %v", err)
		}
	}

	logger.Info("Server shutdown complete")
}
