# client/python/quorum_quest_client/client.py

import time
import uuid
import threading
from typing import Optional, Callable, Any, Dict, Tuple
import grpc

# Import the generated protobuf code
# Using relative imports based on the project structure
import sys
import os
import importlib.util

# Dynamically import the protobuf modules
def import_proto_modules():
    """Dynamically import the protobuf modules from the correct location."""
    # Try different relative paths to find the proto files
    possible_paths = [
        # From the package directory
        os.path.abspath(os.path.join(os.path.dirname(__file__), '../../../api/gen/python/v1')),
        # From the repository root
        os.path.abspath(os.path.join(os.path.dirname(__file__), '../../api/gen/python/v1')),
    ]
    
    for proto_path in possible_paths:
        if os.path.exists(proto_path):
            if proto_path not in sys.path:
                sys.path.insert(0, os.path.dirname(proto_path))
            break
    
    # Now import the modules
    try:
        from v1 import quorum_quest_api_pb2, quorum_quest_api_pb2_grpc
        return quorum_quest_api_pb2, quorum_quest_api_pb2_grpc
    except ImportError:
        raise ImportError(
            "Could not import Quorum Quest protobuf modules. "
            "Make sure the generated protobuf files are available at "
            "api/gen/python/v1/ relative to the repository root."
        )

# Import the protobuf modules
quorum_quest_api_pb2, quorum_quest_api_pb2_grpc = import_proto_modules()

class NoOpCallbacks:
    """Default no-op implementation of leader election callbacks."""
    
    def on_leader_elected(self, is_leader: bool) -> None:
        """Called when leadership status changes.
        
        Args:
            is_leader: True if this node is the new leader
        """
        pass
    
    def on_leader_lost(self) -> None:
        """Called when this node was the leader but lost leadership."""
        pass


class QuorumQuestClient:
    """Client for the Quorum Quest leader election service."""
    
    def __init__(self, 
                 service: str, 
                 domain: str, 
                 ttl: float, 
                 address: str, 
                 client_id: Optional[str] = None,
                 callbacks: Optional[Any] = None):
        """Initialize a new QuorumQuestClient.
        
        Args:
            service: Service identifier
            domain: Domain scope
            ttl: Time-to-live for the lock in seconds
            address: Server address (e.g., "localhost:5050")
            client_id: Optional client identifier (generated if not provided)
            callbacks: Optional callback object for leadership events
        
        Raises:
            ValueError: If required parameters are invalid
        """
        if not service:
            raise ValueError("Service name cannot be empty")
        if not domain:
            raise ValueError("Domain cannot be empty")
        if ttl <= 0:
            raise ValueError("TTL must be greater than zero")
        if not address:
            raise ValueError("Server address cannot be empty")
            
        self.service = service
        self.domain = domain
        self.ttl = ttl
        self.id = client_id or str(uuid.uuid4())
        self.callbacks = callbacks or NoOpCallbacks()
        
        # State variables
        self._lock = threading.RLock()
        self._is_leader = False
        self._remaining_lease = -1
        self._keep_alive_thread = None
        self._keep_alive_stop_event = threading.Event()
        
        # Set up the gRPC connection
        self.channel = grpc.insecure_channel(
            address,
            options=[
                ('grpc.keepalive_time_ms', 10000),
                ('grpc.keepalive_timeout_ms', 3000),
                ('grpc.http2.max_pings_without_data', 0),
                ('grpc.keepalive_permit_without_calls', 1)
            ]
        )
        self.stub = quorum_quest_api_pb2_grpc.LeaderElectionServiceStub(self.channel)
    
    def try_acquire_lock(self) -> bool:
        """Attempt to acquire the leadership lock.
        
        Returns:
            bool: True if the lock was acquired, False otherwise
            
        Raises:
            grpc.RpcError: If the gRPC call fails
        """
        request = quorum_quest_api_pb2.TryAcquireLockRequest(
            service=self.service,
            domain=self.domain,
            client_id=self.id,
            ttl=int(self.ttl)
        )
        
        response = self.stub.TryAcquireLock(request)
        
        with self._lock:
            was_leader = self._is_leader
            self._is_leader = response.is_leader
            if response.is_leader:
                self._remaining_lease = self.ttl
            else:
                self._remaining_lease = -1
        
        # Notify callbacks about leadership changes
        if was_leader != response.is_leader:
            if response.is_leader:
                # Became leader
                self.callbacks.on_leader_elected(True)
            elif was_leader:
                # Lost leadership
                self.callbacks.on_leader_lost()
            else:
                # Still not leader, but notify of election result
                self.callbacks.on_leader_elected(False)
                
        return response.is_leader
    
    def release_lock(self) -> None:
        """Release a previously acquired lock.
        
        Raises:
            grpc.RpcError: If the gRPC call fails
        """
        with self._lock:
            was_leader = self._is_leader
            self._is_leader = False
            self._remaining_lease = -1
        
        request = quorum_quest_api_pb2.ReleaseLockRequest(
            service=self.service,
            domain=self.domain,
            client_id=self.id
        )
        
        self.stub.ReleaseLock(request)
        
        # Notify callbacks about leadership loss
        if was_leader:
            self.callbacks.on_leader_lost()
    
    def keep_alive(self) -> float:
        """Refresh the lock TTL.
        
        Returns:
            float: New lease duration in seconds, or -1 if the lock is lost
            
        Raises:
            grpc.RpcError: If the gRPC call fails
        """
        request = quorum_quest_api_pb2.KeepAliveRequest(
            service=self.service,
            domain=self.domain,
            client_id=self.id,
            ttl=int(self.ttl)
        )
        
        response = self.stub.KeepAlive(request)
        lease_seconds = response.lease_length.seconds + (response.lease_length.nanos / 1e9)
        
        with self._lock:
            was_leader = self._is_leader
            
            # If lease is negative, we're no longer the leader
            if lease_seconds < 0:
                self._is_leader = False
                self._remaining_lease = -1
                if was_leader:
                    self.callbacks.on_leader_lost()
            else:
                self._remaining_lease = lease_seconds
                
        return lease_seconds
    
    def get_remaining_lease(self) -> float:
        """Get the remaining time of the current lock lease.
        
        Returns:
            float: Remaining lease time in seconds, or -1 if not the leader
        """
        with self._lock:
            return self._remaining_lease
    
    def is_leader(self) -> bool:
        """Check if this client currently holds the lock.
        
        Returns:
            bool: True if this client is the leader, False otherwise
        """
        with self._lock:
            return self._is_leader
    
    def _keep_alive_worker(self) -> None:
        """Background worker that periodically refreshes the lock."""
        # Use a third of the TTL as the refresh interval to avoid expiration
        refresh_interval = max(self.ttl / 3, 3)  # At least 3 seconds
        
        while not self._keep_alive_stop_event.is_set():
            try:
                lease = self.keep_alive()
                if lease < 0:
                    # Lost the lock
                    break
                
                # Wait for the next refresh cycle or until stopped
                self._keep_alive_stop_event.wait(refresh_interval)
            except grpc.RpcError:
                # If we can't reach the server, assume the lock is lost
                with self._lock:
                    was_leader = self._is_leader
                    self._is_leader = False
                    self._remaining_lease = -1
                
                if was_leader:
                    self.callbacks.on_leader_lost()
                break
    
    def start_keep_alive(self) -> None:
        """Start a background thread to periodically refresh the lock.
        
        Raises:
            RuntimeError: If keep-alive is already running
        """
        with self._lock:
            if self._keep_alive_thread and self._keep_alive_thread.is_alive():
                raise RuntimeError("Keep-alive is already running")
            
            self._keep_alive_stop_event.clear()
            self._keep_alive_thread = threading.Thread(
                target=self._keep_alive_worker,
                daemon=True
            )
            self._keep_alive_thread.start()
    
    def stop_keep_alive(self) -> None:
        """Stop the background keep-alive thread."""
        with self._lock:
            if self._keep_alive_thread and self._keep_alive_thread.is_alive():
                self._keep_alive_stop_event.set()
                self._keep_alive_thread.join(timeout=5)
                self._keep_alive_thread = None
    
    def close(self) -> None:
        """Release resources held by the client."""
        self.stop_keep_alive()
        if hasattr(self, 'channel'):
            self.channel.close()