# client/python/example/main.py

import os
import sys
import time
import signal
import threading
import argparse
from typing import Optional

# Add the necessary paths to sys.path to find both the client package and the protobuf files
current_dir = os.path.dirname(os.path.abspath(__file__))
client_dir = os.path.dirname(current_dir)
repo_root = os.path.dirname(os.path.dirname(client_dir))

# Add paths to sys.path
sys.path.append(client_dir)  # For the client package
sys.path.append(repo_root)   # For the api.gen.python.v1 modules

from quorum_quest_client import QuorumQuestClient

class ExampleCallbacks:
    """Example implementation of leader election callbacks."""
    
    def __init__(self):
        self._lock = threading.RLock()
        self._leader_task_stop_event = None
        self._leader_task_thread = None
    
    def on_leader_elected(self, is_leader: bool) -> None:
        """Called when leadership status changes."""
        if is_leader:
            print("🎉 This node is now the LEADER!")
            # Start leader-specific tasks
            with self._lock:
                if self._leader_task_thread and self._leader_task_thread.is_alive():
                    # Already running, don't start again
                    return
                
                self._leader_task_stop_event = threading.Event()
                self._leader_task_thread = threading.Thread(
                    target=self._run_leader_tasks,
                    args=(self._leader_task_stop_event,),
                    daemon=True
                )
                self._leader_task_thread.start()
        else:
            print("⭐ Another node is the leader. Running as follower.")
    
    def on_leader_lost(self) -> None:
        """Called when this node was the leader but lost leadership."""
        print("❌ Leadership LOST! Stopping leader tasks...")
        
        # Stop leader-specific tasks
        with self._lock:
            if self._leader_task_stop_event:
                self._leader_task_stop_event.set()
                if self._leader_task_thread:
                    self._leader_task_thread.join(timeout=2)
                self._leader_task_stop_event = None
                self._leader_task_thread = None
    
    def _run_leader_tasks(self, stop_event: threading.Event) -> None:
        """Simulate leader-specific tasks."""
        print("🚀 Started leader tasks")
        
        while not stop_event.is_set():
            print("⚙️  Performing leader-specific task...")
            # Wait for 3 seconds or until stopped
            stop_event.wait(3)
        
        print("⏹️  Leader tasks stopped")


def main():
    # Parse command line arguments
    parser = argparse.ArgumentParser(description="Quorum Quest Client Example")
    parser.add_argument("--server", default="localhost:5050", help="Server address (default: localhost:5050)")
    parser.add_argument("--service", default="example-service", help="Service name (default: example-service)")
    parser.add_argument("--domain", default="production", help="Domain name (default: production)")
    parser.add_argument("--ttl", type=int, default=15, help="Lock TTL in seconds (default: 15)")
    args = parser.parse_args()
    
    # Setup shutdown signal handling
    shutdown_event = threading.Event()
    
    def signal_handler(sig, frame):
        print("\n📣 Received shutdown signal. Cleaning up...")
        shutdown_event.set()
    
    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)
    
    # Create callbacks
    callbacks = ExampleCallbacks()
    
    # Create client
    client = QuorumQuestClient(
        service=args.service,
        domain=args.domain,
        ttl=args.ttl,
        address=args.server,
        callbacks=callbacks
    )
    
    print(f"🔄 Connecting to {args.server} for service {args.service}")
    
    try:
        # Try to acquire the lock
        is_leader = client.try_acquire_lock()
        
        if is_leader:
            print("🔒 Successfully acquired leadership lock!")
            
            # Start the keepalive process to maintain leadership
            client.start_keep_alive()
        else:
            print("🔄 Could not acquire leadership lock. Running as follower.")
        
        # Main loop that monitors status and responds to shutdown
        while not shutdown_event.is_set():
            if client.is_leader():
                remaining = client.get_remaining_lease()
                print(f"✅ Still the leader. Lease remaining: {remaining:.1f}s")
            else:
                print("🔄 Running as follower, monitoring for leadership changes...")
                
                # Attempt to acquire leadership if not already leader
                try:
                    is_leader = client.try_acquire_lock()
                    if is_leader:
                        print("🎉 Successfully acquired leadership!")
                        client.start_keep_alive()
                except Exception as e:
                    print(f"Warning: Failed to try acquire lock: {e}")
            
            # Wait before next status check
            for _ in range(50):  # Check shutdown event more frequently than full sleep
                if shutdown_event.is_set():
                    break
                time.sleep(0.1)
    
    finally:
        print("🛑 Shutting down...")
        
        if client.is_leader():
            print("🔓 Releasing leadership lock...")
            try:
                client.release_lock()
            except Exception as e:
                print(f"Failed to release lock: {e}")
        
        client.close()
        print("Goodbye!")


if __name__ == "__main__":
    main()