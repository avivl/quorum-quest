# Quorum Quest Python Client

A Python client library for the Quorum Quest distributed leader election service.

## Installation

```bash
# From the client/python directory
chmod +x setup-all.sh
./setup-all.sh
```

The setup script will:
1. Copy the necessary protobuf files into the package
2. Install the package in development mode
3. Set everything up for you to run the examples

## Usage

### Basic Usage

```python
from quorum_quest_client import QuorumQuestClient

# Create a client
client = QuorumQuestClient(
    service="my-service",
    domain="production",
    ttl=15,  # TTL in seconds
    address="localhost:5050"
)

try:
    # Try to acquire leadership
    is_leader = client.try_acquire_lock()
    
    if is_leader:
        print("This node is now the leader!")
        
        # Start keep-alive to maintain leadership
        client.start_keep_alive()
        
        # Perform leader-specific tasks...
        
    else:
        print("Another node is the leader. Running as follower.")
    
    # Check status
    if client.is_leader():
        remaining = client.get_remaining_lease()
        print(f"Remaining lease time: {remaining}s")
    
    # To voluntarily give up leadership:
    if client.is_leader():
        client.release_lock()
        
finally:
    # Always close the client when done
    client.close()
```

### Using Callbacks

You can provide callback handlers to react to leadership changes:

```python
from quorum_quest_client import QuorumQuestClient

class MyCallbacks:
    def on_leader_elected(self, is_leader):
        if is_leader:
            print("Became leader! Starting leader tasks...")
            # Start leader-specific tasks
        else:
            print("Another node is leader")
    
    def on_leader_lost(self):
        print("Lost leadership! Stopping leader tasks...")
        # Stop leader-specific tasks

# Create client with callbacks
client = QuorumQuestClient(
    service="my-service",
    domain="production",
    ttl=15,
    address="localhost:5050",
    callbacks=MyCallbacks()
)
```

## Example Application

See the `example/main.py` script for a complete example application that demonstrates:

- Leadership acquisition and monitoring
- Using callbacks to respond to leadership changes
- Running leader-specific tasks
- Proper cleanup on shutdown

After running the setup script, you can run the example with:

```bash
python example/main.py --server localhost:5050 --service my-service --domain production
```

Command line options:
- `--server` - The server address (default: localhost:5050)
- `--service` - The service name (default: example-service)
- `--domain` - The domain name (default: production)
- `--ttl` - The lock TTL in seconds (default: 15)

## Troubleshooting

If you encounter import errors or other issues:

1. Make sure you've run the `setup-all.sh` script
2. Check that the protobuf files exist in `quorum_quest_client/proto/`
3. Ensure your Python environment has all dependencies installed:
   ```bash
   pip install grpcio protobuf
   ```

## API Reference

### QuorumQuestClient

The main client class for the Quorum Quest service.

#### Constructor

```python
QuorumQuestClient(
    service: str,              # Service identifier
    domain: str,               # Domain scope 
    ttl: float,                # Time-to-live for the lock in seconds
    address: str,              # Server address (e.g., "localhost:5050")
    client_id: Optional[str],  # Optional client ID (generated if not provided)
    callbacks: Optional[Any]   # Optional callback object
)
```

#### Methods

- `try_acquire_lock()` - Attempt to acquire leadership
- `release_lock()` - Voluntarily release leadership
- `keep_alive()` - Manually refresh the lock TTL
- `start_keep_alive()` - Start a background thread to automatically refresh the lock
- `stop_keep_alive()` - Stop the background refresh thread
- `get_remaining_lease()` - Get the remaining time on the current lease
- `is_leader()` - Check if this client is currently the leader
- `close()` - Release resources held by the client

### Callbacks Interface

The callbacks object should implement these methods:

- `on_leader_elected(is_leader: bool)` - Called when leadership status changes
- `on_leader_lost()` - Called when this node loses leadership

## License

MIT