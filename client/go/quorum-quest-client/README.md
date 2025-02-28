# Quorum Quest Client

A Go client library for the Quorum Quest distributed leader election service.

## Overview

This library provides a simple interface to interact with the Quorum Quest service, allowing applications to:

- Acquire leadership locks for specific service/domain pairs
- Release locks when no longer needed
- Automatically refresh locks with a keep-alive mechanism
- Monitor leadership status

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