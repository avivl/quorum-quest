# Quorum Quest

A robust distributed locking service with support for multiple storage backends.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Overview

Quorum Quest is a high-performance distributed locking service designed for reliable coordination in distributed systems. It provides consistent lock semantics across multiple instances with configurable TTL and various backend storage options.

## Features

- **Multiple Storage Backends**:
  - ScyllaDB
  - DynamoDB
  - Redis
  - Easy extensibility for additional backends

- **Advanced Lock Management**:
  - Distributed lock acquisition with leader election
  - Configurable TTL and lease durations
  - Automatic lock renewal with client-side keep-alive
  - Safe lock release mechanisms

- **Robust Configuration**:
  - YAML-based configuration
  - Environment variable overrides
  - Hot-reload capability for configuration changes

- **Observability**:
  - OpenTelemetry integration for metrics and tracing
  - Structured logging
  - Health and status monitoring

- **Client Libraries**:
  - Native Go client with automatic lock renewal
  - Consistent API across storage backends

## Architecture

Quorum Quest uses a pluggable backend architecture that allows different storage technologies to be used interchangeably while maintaining the same locking semantics. The service exposes a gRPC API for lock operations and includes features like:

- Centralized configuration management with hot reload
- Graceful shutdown handling
- Comprehensive error handling and recovery
- Clear separation of concerns between service, store, and client layers

## Getting Started

### Prerequisites

- Go 1.18 or later
- One of the supported backend services (ScyllaDB, DynamoDB, or Redis)

### Installation

```bash
go get -u github.com/avivl/quorum-quest
```

### Configuration

Create a configuration file (e.g., `config.yaml`):

```yaml
serverAddress: "localhost:5050"
backend:
  type: "redis"  # Options: redis, scylladb, dynamodb

store:
  # Redis-specific configuration
  host: "localhost"
  port: 6379
  password: ""
  db: 0
  ttl: 15  # seconds
  key_prefix: "lock"

logger:
  level: "LOG_LEVELS_INFOLEVEL"  # Options: LOG_LEVELS_DEBUGLEVEL, LOG_LEVELS_INFOLEVEL, LOG_LEVELS_WARNLEVEL, LOG_LEVELS_ERRORLEVEL

observability:
  serviceName: "quorum-quest"
  serviceVersion: "0.1.0"
  environment: "development"
  otelEndpoint: "localhost:4317"
```

### Running the Service

```bash
# Run with default configuration path (./config)
quorum-quest-service

# Specify a custom configuration path
quorum-quest-service -config=/path/to/config.yaml
```

### Using the Client

```go
package main

import (
	"context"
	"log"
	"time"

	client "github.com/avivl/quorum-quest/client/go/quorum-quest-client"
)

func main() {
	// Create a new client
	qClient, err := client.NewQuorumQuestClient(
		"my-service",       // Service name
		"production",       // Domain
		30*time.Second,     // TTL
		"localhost:5050",   // Server address
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer qClient.Close()

	// Try to acquire a lock
	ctx := context.Background()
	isLeader, err := qClient.TryAcquireLock(ctx)
	if err != nil {
		log.Fatalf("Error acquiring lock: %v", err)
	}

	if isLeader {
		log.Println("Lock acquired, I am the leader")
		
		// Start automatic keep-alive
		if err := qClient.StartKeepAlive(ctx); err != nil {
			log.Fatalf("Error starting keep-alive: %v", err)
		}

		// Do leader work...
		time.Sleep(2 * time.Minute)

		// Release the lock when done
		if err := qClient.ReleaseLock(ctx); err != nil {
			log.Printf("Error releasing lock: %v", err)
		}
		qClient.StopKeepAlive()
	} else {
		log.Println("Could not acquire lock, I am a follower")
	}
}
```

## Backend Configuration

### ScyllaDB

```yaml
backend:
  type: "scylladb"

store:
  host: "127.0.0.1"
  port: 9042
  keyspace: "quorumquest"
  table: "services"
  ttl: 15
  consistency: "CONSISTENCY_QUORUM"
  endpoints: ["localhost:9042"]
```

### DynamoDB

```yaml
backend:
  type: "dynamodb"

store:
  region: "us-west-2"
  table: "quorumquest"
  ttl: 15
  endpoints: ["dynamodb.us-west-2.amazonaws.com"]
  # Optional credentials (otherwise uses AWS SDK default provider chain)
  accessKeyId: ""
  secretAccessKey: ""
```

### Redis

```yaml
backend:
  type: "redis"

store:
  host: "localhost"
  port: 6379
  password: ""
  db: 0
  ttl: 15
  key_prefix: "lock"
```

## Environment Variables

All configuration parameters can be overridden using environment variables:

```bash
# Server configuration
export QUORUMQUEST_SERVERADDRESS="0.0.0.0:5050"

# Backend type
export QUORUMQUEST_BACKEND_TYPE="redis"

# Redis-specific configuration
export QUORUMQUEST_STORE_HOST="redis.example.com"
export QUORUMQUEST_STORE_PORT="6379"
export QUORUMQUEST_STORE_TTL="30"
```

## Development

### Building from Source

```bash
# Clone the repository
git clone https://github.com/avivl/quorum-quest.git
cd quorum-quest

# Build the service
go build -o quorum-quest-service ./cmd/quorum-quest-service
```

### Running Tests

```bash
go test ./...
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

As this is a work in progress, please open an issue to discuss any major changes before submitting pull requests.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request
