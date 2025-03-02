# Quorum Quest Client

A Go client library for the Quorum Quest distributed leader election service.

## Overview

This library provides a simple interface to interact with the Quorum Quest service, allowing applications to:

- Acquire leadership locks for specific service/domain pairs
- Release locks when no longer needed
- Automatically refresh locks with a keep-alive mechanism
- Monitor leadership status

# Event-Driven Architecture in Quorum Quest

Quorum Quest now implements an event-driven architecture to simplify leadership management in distributed applications. This approach allows your application to focus on *what* to do when leadership changes, rather than *how* the election algorithm works.

## Key Features

- **Callbacks Interface**: React to leadership changes through a simple, well-defined interface
- **Separation of Concerns**: Election mechanics are isolated from business logic
- **Clean State Management**: Automatic handling of leadership transitions
- **Graceful Shutdown**: Proper cleanup of resources and leadership handoff

## Using the Callbacks System

### 1. Implement the Callbacks Interface

```go
// Implement the Callbacks interface for your application
type MyCallbacks struct {
    // Your application state here
}

// Called when leadership status changes
func (c *MyCallbacks) OnLeaderElected(isLeader bool) {
    if isLeader {
        // This node is now the leader
        startLeaderTasks()
    } else {
        // Another node is the leader
        runFollowerMode()
    }
}

// Called when this node was leader but lost leadership
func (c *MyCallbacks) OnLeaderLost() {
    // Stop leader-specific activities
    stopLeaderTasks()
}
```

### 2. Register Callbacks with the Client

```go
callbacks := &MyCallbacks{}

client, err := quorumquestclient.NewQuorumQuestClient(
    "my-service",
    "production",
    15*time.Second,
    "localhost:5050",
    quorumquestclient.WithCallbacks(callbacks),
)
```

### 3. Start Leadership Acquisition

Leadership events will now be handled automatically:

```go
// Try to acquire leadership
isLeader, err := client.TryAcquireLock(ctx)
if err != nil {
    log.Fatalf("Failed to try acquire lock: %v", err)
}

// Start keepalive if we're the leader
if isLeader {
    if err := client.StartKeepAlive(ctx); err != nil {
        log.Fatalf("Failed to start keep-alive: %v", err)
    }
}
```

## How It Works

1. When your node acquires leadership, `OnLeaderElected(true)` is called
2. If leadership acquisition fails, `OnLeaderElected(false)` is called
3. If your node loses leadership, `OnLeaderLost()` is called
4. The keepalive process automatically maintains leadership and handles transitions

## Best Practices

- Keep callback methods lightweight and non-blocking
- Use channels or worker pools for long-running leader tasks
- Implement proper cleanup in `OnLeaderLost()`
- Consider using a "ReadyToExit" mechanism for graceful shutdown

## Example

See the provided example in `client/go/quorum-quest-client/example/main.go` for a complete working implementation.

## Supported Backends

The event-driven architecture works with all supported storage backends:

- ScyllaDB
- DynamoDB
- Redis

Each backend implements the same leadership semantics, so your callbacks will work consistently regardless of the backend chosen.
## Installation

```bash
go get github.com/avivl/quorum-quest/client/go/quorum-quest-client
```

## Usage

### Basic Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	quorumquestclient "github.com/avivl/quorum-quest/client/go/quorum-quest-client"
)

func main() {
	// Create a client
	client, err := quorumquestclient.NewQuorumQuestClient(
		"my-service",      // Service name
		"production",      // Domain 
		15*time.Second,    // TTL (lease duration)
		"localhost:5050",  // Server address
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Create a context
	ctx := context.Background()

	// Try to acquire leadership
	isLeader, err := client.TryAcquireLock(ctx)
	if err != nil {
		log.Fatalf("Failed to try acquire lock: %v", err)
	}

	if isLeader {
		fmt.Println("We are the leader!")
		
		// Start keepalive to maintain leadership
		if err := client.StartKeepAlive(ctx); err != nil {
			log.Fatalf("Failed to start keep-alive: %v", err)
		}
		
		// Run leader-specific code
		// ...
		
		// When done, release the lock
		if err := client.ReleaseLock(ctx); err != nil {
			log.Printf("Failed to release lock: %v", err)
		}
	} else {
		fmt.Println("Another instance is the leader")
	}
}
```

### Leadership Check

```go
// Check if we're still the leader
if client.IsLeader() {
    // Perform leader-only tasks
} else {
    // Run as follower or exit
}

// Get the remaining lease time
remainingLease := client.GetRemainingLease()
fmt.Printf("Lease remaining: %v\n", remainingLease)
```

### Custom Options

```go
// Create client with custom client ID
client, err := quorumquestclient.NewQuorumQuestClient(
    "my-service",
    "production",
    15*time.Second,
    "localhost:5050",
    quorumquestclient.WithClientID("custom-id-123"),
)
```

## API Reference

### NewQuorumQuestClient

```go
func NewQuorumQuestClient(service string, domain string, ttl time.Duration, address string, opts ...Option) (*QuorumQuestClient, error)
```

Creates a new client for the Quorum Quest service.

Parameters:
- `service`: The service name to acquire a lock for
- `domain`: The domain name (e.g., "production", "staging")
- `ttl`: The time-to-live for the lock
- `address`: The gRPC server address
- `opts`: Optional configuration options

### TryAcquireLock

```go
func (q *QuorumQuestClient) TryAcquireLock(ctx context.Context) (bool, error)
```

Attempts to acquire a lock. Returns true if successful.

### ReleaseLock

```go
func (q *QuorumQuestClient) ReleaseLock(ctx context.Context) error
```

Releases a previously acquired lock.

### StartKeepAlive

```go
func (q *QuorumQuestClient) StartKeepAlive(ctx context.Context) error
```

Starts a background goroutine to periodically refresh the lock.

### StopKeepAlive

```go
func (q *QuorumQuestClient) StopKeepAlive()
```

Stops the background keep-alive goroutine.

### IsLeader

```go
func (q *QuorumQuestClient) IsLeader() bool
```

Returns true if this client currently holds the lock.

### GetRemainingLease

```go
func (q *QuorumQuestClient) GetRemainingLease() time.Duration
```

Returns the remaining time of the current lock lease.

### Close

```go
func (q *QuorumQuestClient) Close() error
```

Releases resources held by the client.

## Options

### WithClientID

```go
func WithClientID(id string) Option
```

Sets a specific client ID instead of generating a random one.

### WithServerStub

```go
func WithServerStub(stub pb.LeaderElectionServiceClient) Option
```

Allows injecting a mock client for testing.

## Error Handling

The client handles various error conditions:
- Connection failures to the gRPC server
- Failed lock acquisition attempts
- Lost connections during keep-alive
- Internal server errors

All public methods return appropriate errors that should be checked.

## Thread Safety

The client is thread-safe and can be used from multiple goroutines.