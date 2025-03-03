# Quorum Quest Rust Client

A Rust client library for the Quorum Quest distributed leader election service.

## Prerequisites

- Rust (with Cargo)
- Protocol Buffer compiler (protoc) - required for building the gRPC code

## Building

The client is organized as a Cargo workspace that includes both the client library and an example application:

```bash
cd client/rust
cargo build
```

## Running the Example

The example application demonstrates leadership acquisition, callbacks, and maintaining leadership with keep-alive:

```bash
cd client/rust
cargo run --bin quorum-quest-example -- --server http://localhost:5050 --service my-service
```

Command line options:
- `--server` - The server address (default: http://localhost:5050)
- `--service` - The service name (default: example-service)
- `--domain` - The domain name (default: production)
- `--ttl` - The lock TTL in seconds (default: 15)

## Usage

### Basic Usage

```rust
use std::time::Duration;
use quorum_quest_client::QuorumQuestClient;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Create a client
    let mut client = QuorumQuestClient::new(
        "my-service",                // Service name
        "production",                // Domain
        Duration::from_secs(15),     // TTL
        "http://localhost:5050",     // Server address
        None,                        // Client ID (generated if None)
        None,                        // Callbacks (NoOpCallbacks if None)
    ).await?;
    
    // Try to acquire leadership
    let is_leader = client.try_acquire_lock().await?;
    
    if is_leader {
        println!("This node is now the leader!");
        
        // Start keep-alive to maintain leadership
        client.start_keep_alive().await?;
        
        // Perform leader-specific tasks...
        
    } else {
        println!("Another node is the leader. Running as follower.");
    }
    
    // Check status
    if client.is_leader() {
        if let Some(remaining) = client.get_remaining_lease() {
            println!("Remaining lease time: {:?}", remaining);
        }
    }
    
    // To voluntarily give up leadership:
    if client.is_leader() {
        client.release_lock().await?;
    }
    
    // Always stop the keep-alive process when done
    client.stop_keep_alive().await;
    
    Ok(())
}
```

### Using Callbacks

You can provide callback handlers to react to leadership changes:

```rust
use std::sync::Arc;
use std::time::Duration;
use async_trait::async_trait;
use quorum_quest_client::{QuorumQuestClient, Callbacks};

struct MyCallbacks;

#[async_trait]
impl Callbacks for MyCallbacks {
    async fn on_leader_elected(&self, is_leader: bool) {
        if is_leader {
            println!("Became leader! Starting leader tasks...");
            // Start leader-specific tasks
        } else {
            println!("Another node is leader");
        }
    }
    
    async fn on_leader_lost(&self) {
        println!("Lost leadership! Stopping leader tasks...");
        // Stop leader-specific tasks
    }
}

// Create client with callbacks
let callbacks = Arc::new(MyCallbacks);
let mut client = QuorumQuestClient::new(
    "my-service",
    "production",
    Duration::from_secs(15),
    "http://localhost:5050",
    None,
    Some(callbacks),
).await?;
```

## API Reference

### QuorumQuestClient

The main client class for the Quorum Quest service.

#### Constructor

```rust
QuorumQuestClient::new(
    service: impl Into<String>,     // Service identifier
    domain: impl Into<String>,      // Domain scope
    ttl: Duration,                  // Time-to-live for the lock
    address: impl AsRef<str>,       // Server address
    client_id: Option<String>,      // Optional client ID
    callbacks: Option<Arc<dyn Callbacks>>, // Optional callbacks
).await?
```

#### Methods

- `try_acquire_lock()` - Attempt to acquire leadership
- `release_lock()` - Voluntarily release leadership
- `start_keep_alive()` - Start a background task to automatically refresh the lock
- `stop_keep_alive()` - Stop the background refresh task
- `get_remaining_lease()` - Get the remaining time on the current lease
- `is_leader()` - Check if this client is currently the leader

### Callbacks Trait

The callbacks trait defines methods that will be called when leadership status changes:

```rust
#[async_trait]
pub trait Callbacks: Send + Sync {
    /// Called when leadership status changes
    /// is_leader is true if this node is the new leader
    async fn on_leader_elected(&self, is_leader: bool);
    
    /// Called when this node was the leader but lost leadership
    async fn on_leader_lost(&self);
}
```

## Error Handling

The library provides a custom `Error` type that encompasses various error conditions:

```rust
pub enum Error {
    Connection(tonic::transport::Error),
    Rpc(tonic::Status),
    KeepAliveAlreadyRunning,
    InvalidParameter(String),
}
```

All API methods return a `Result<T, Error>` which should be properly handled.

## Thread Safety

The client is designed to be thread-safe and can be shared between threads when wrapped in an `Arc<Mutex<>>`:

```rust
use std::sync::{Arc, Mutex};

let client = Arc::new(Mutex::new(
    QuorumQuestClient::new(
        "my-service", 
        "production", 
        Duration::from_secs(15), 
        "http://localhost:5050", 
        None, 
        None
    ).await?
));

// Clone the Arc to share the client between threads
let client_clone = client.clone();
tokio::spawn(async move {
    let mut locked_client = client_clone.lock().unwrap();
    // Use the client...
});
```

## Design Considerations

The design of this library follows these core principles:

1. **Asynchronous First** - All operations that interact with the network are async, leveraging Tokio's async runtime.
2. **Thread Safety** - All components are designed to be safely used across thread boundaries.
3. **Robust Error Handling** - Comprehensive error types and propagation to make failure modes clear.
4. **Automatic Keep-Alive** - Background tasks to maintain leadership without manual intervention.
5. **Event-Driven Architecture** - Callbacks to notify applications of leadership changes.

## License

MIT