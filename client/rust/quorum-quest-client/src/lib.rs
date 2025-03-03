// client/rust/quorum-quest-client/src/lib.rs

use std::sync::{Arc, Mutex};
use std::time::Duration;
use async_trait::async_trait;
use thiserror::Error;
use tokio::sync::mpsc;
use tokio::task::JoinHandle;
use tonic::{transport::Channel, Request};
use uuid::Uuid;

// Include the generated protocol buffer code
pub mod proto {
    tonic::include_proto!("quorum.quest.api.v1");
}

use proto::leader_election_service_client::LeaderElectionServiceClient;
use proto::{TryAcquireLockRequest, ReleaseLockRequest, KeepAliveRequest};

/// Errors that can occur during QuorumQuest client operations
#[derive(Error, Debug)]
pub enum Error {
    #[error("Connection error: {0}")]
    Connection(#[from] tonic::transport::Error),
    
    #[error("RPC error: {0}")]
    Rpc(#[from] tonic::Status),
    
    #[error("Keep-alive is already running")]
    KeepAliveAlreadyRunning,
    
    #[error("Invalid parameter: {0}")]
    InvalidParameter(String),
}

/// Result type for QuorumQuest client operations
pub type Result<T> = std::result::Result<T, Error>;

/// Callbacks for leadership changes
#[async_trait]
pub trait Callbacks: Send + Sync {
    /// Called when leadership status changes
    /// is_leader is true if this node is the new leader
    async fn on_leader_elected(&self, is_leader: bool);
    
    /// Called when this node was the leader but lost leadership
    async fn on_leader_lost(&self);
}

/// No-op implementation of Callbacks
pub struct NoOpCallbacks;

#[async_trait]
impl Callbacks for NoOpCallbacks {
    async fn on_leader_elected(&self, _is_leader: bool) {}
    async fn on_leader_lost(&self) {}
}

/// State of the keep-alive process
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum KeepAliveState {
    Stopped,
    Running,
}

/// Client state shared between threads
struct ClientState {
    is_leader: bool,
    remaining_lease: Option<Duration>,
    keep_alive_state: KeepAliveState,
}

/// Client for the Quorum Quest leader election service
pub struct QuorumQuestClient {
    service: String,
    domain: String,
    id: String,
    ttl: Duration,
    client: LeaderElectionServiceClient<Channel>,
    callbacks: Arc<dyn Callbacks>,
    state: Arc<Mutex<ClientState>>,
    keep_alive_tx: Option<mpsc::Sender<()>>,
    keep_alive_handle: Option<JoinHandle<()>>,
}

impl QuorumQuestClient {
    /// Create a new QuorumQuestClient
    pub async fn new(
        service: impl Into<String>,
        domain: impl Into<String>,
        ttl: Duration,
        address: impl AsRef<str>,
        client_id: Option<String>,
        callbacks: Option<Arc<dyn Callbacks>>,
    ) -> Result<Self> {
        let service = service.into();
        let domain = domain.into();
        
        // Validate parameters
        if service.is_empty() {
            return Err(Error::InvalidParameter("Service name cannot be empty".into()));
        }
        if domain.is_empty() {
            return Err(Error::InvalidParameter("Domain cannot be empty".into()));
        }
        if ttl.as_secs() == 0 {
            return Err(Error::InvalidParameter("TTL must be greater than zero".into()));
        }
        
        // Connect to the server
        let client = LeaderElectionServiceClient::connect(address.as_ref().to_string()).await?;
        
        Ok(Self {
            service,
            domain,
            id: client_id.unwrap_or_else(|| Uuid::new_v4().to_string()),
            ttl,
            client,
            callbacks: callbacks.unwrap_or_else(|| Arc::new(NoOpCallbacks)),
            state: Arc::new(Mutex::new(ClientState {
                is_leader: false,
                remaining_lease: None,
                keep_alive_state: KeepAliveState::Stopped,
            })),
            keep_alive_tx: None,
            keep_alive_handle: None,
        })
    }
    
    /// Try to acquire the leadership lock
    pub async fn try_acquire_lock(&mut self) -> Result<bool> {
        let request = Request::new(TryAcquireLockRequest {
            service: self.service.clone(),
            domain: self.domain.clone(),
            client_id: self.id.clone(),
            ttl: self.ttl.as_secs() as i32,
        });
        
        let response = self.client.try_acquire_lock(request).await?;
        let response = response.into_inner();
        
        // Update state
        let mut state = self.state.lock().unwrap();
        let was_leader = state.is_leader;
        state.is_leader = response.is_leader;
        
        if response.is_leader {
            state.remaining_lease = Some(self.ttl);
        } else {
            state.remaining_lease = None;
        }
        
        // Notify callbacks about leadership changes
        if was_leader != response.is_leader {
            let callbacks = self.callbacks.clone();
            if response.is_leader {
                // Became leader
                tokio::spawn(async move {
                    callbacks.on_leader_elected(true).await;
                });
            } else if was_leader {
                // Lost leadership
                tokio::spawn(async move {
                    callbacks.on_leader_lost().await;
                });
            } else {
                // Still not leader, but notify of election result
                tokio::spawn(async move {
                    callbacks.on_leader_elected(false).await;
                });
            }
        }
        
        Ok(response.is_leader)
    }
    
    /// Release the leadership lock
    pub async fn release_lock(&mut self) -> Result<()> {
        // Update state first to prevent race conditions
        let mut state = self.state.lock().unwrap();
        let was_leader = state.is_leader;
        state.is_leader = false;
        state.remaining_lease = None;
        drop(state);
        
        let request = Request::new(ReleaseLockRequest {
            service: self.service.clone(),
            domain: self.domain.clone(),
            client_id: self.id.clone(),
        });
        
        self.client.release_lock(request).await?;
        
        // Notify about leadership loss if we were the leader
        if was_leader {
            let callbacks = self.callbacks.clone();
            tokio::spawn(async move {
                callbacks.on_leader_lost().await;
            });
        }
        
        Ok(())
    }
    
    /// Start a background task to periodically refresh the lock
    pub async fn start_keep_alive(&mut self) -> Result<()> {
        // Check if keep-alive is already running
        {
            let state = self.state.lock().unwrap();
            if state.keep_alive_state == KeepAliveState::Running {
                return Err(Error::KeepAliveAlreadyRunning);
            }
        }
        
        // Create a channel to signal the keep-alive task to stop
        let (tx, mut rx) = mpsc::channel(1);
        self.keep_alive_tx = Some(tx);
        
        // Start the keep-alive task
        let mut client = self.client.clone();
        let service = self.service.clone();
        let domain = self.domain.clone();
        let client_id = self.id.clone();
        let ttl = self.ttl;
        let state = self.state.clone();
        let callbacks = self.callbacks.clone();
        
        // Calculate refresh interval (1/3 of TTL, at least 1 second)
        let refresh_interval = std::cmp::max(ttl / 3, Duration::from_secs(1));
        
        let handle = tokio::spawn(async move {
            // Create a ticker for the refresh interval
            let mut interval = tokio::time::interval(refresh_interval);
            
            // Set initial state
            {
                let mut state = state.lock().unwrap();
                state.keep_alive_state = KeepAliveState::Running;
            }
            
            loop {
                // Wait for either the interval or a stop signal
                tokio::select! {
                    _ = interval.tick() => {
                        // Time to refresh the lock
                        let request = Request::new(KeepAliveRequest {
                            service: service.clone(),
                            domain: domain.clone(),
                            client_id: client_id.clone(),
                            ttl: ttl.as_secs() as i32,
                        });
                        
                        match client.keep_alive(request).await {
                            Ok(response) => {
                                let response = response.into_inner();
                                
                                // Convert protobuf duration to Rust duration
                                let lease = if let Some(lease_length) = response.lease_length {
                                    let secs = lease_length.seconds;
                                    let nanos = lease_length.nanos as u32;
                                    
                                    if secs < 0 {
                                        // Negative duration means leadership is lost
                                        None
                                    } else {
                                        Some(Duration::new(secs as u64, nanos))
                                    }
                                } else {
                                    // No lease length returned means leadership is lost
                                    None
                                };
                                
                                // Update state
                                let mut state = state.lock().unwrap();
                                let was_leader = state.is_leader;
                                
                                if lease.is_none() {
                                    state.is_leader = false;
                                    state.remaining_lease = None;
                                    state.keep_alive_state = KeepAliveState::Stopped;
                                    
                                    // Notify about leadership loss if we were the leader
                                    if was_leader {
                                        let callbacks = callbacks.clone();
                                        tokio::spawn(async move {
                                            callbacks.on_leader_lost().await;
                                        });
                                    }
                                    
                                    // Exit the loop if leadership is lost
                                    break;
                                } else {
                                    state.remaining_lease = lease;
                                }
                            },
                            Err(err) => {
                                // Log the error
                                log::error!("Error keeping lock alive: {:?}", err);
                                
                                // Update state on error
                                let mut state = state.lock().unwrap();
                                let was_leader = state.is_leader;
                                state.is_leader = false;
                                state.remaining_lease = None;
                                state.keep_alive_state = KeepAliveState::Stopped;
                                
                                // Notify about leadership loss if we were the leader
                                if was_leader {
                                    let callbacks = callbacks.clone();
                                    tokio::spawn(async move {
                                        callbacks.on_leader_lost().await;
                                    });
                                }
                                
                                // Exit the loop on error
                                break;
                            }
                        }
                    }
                    _ = rx.recv() => {
                        // Received stop signal
                        let mut state = state.lock().unwrap();
                        state.keep_alive_state = KeepAliveState::Stopped;
                        break;
                    }
                }
            }
        });
        
        self.keep_alive_handle = Some(handle);
        
        Ok(())
    }
    
    /// Stop the background keep-alive task
    pub async fn stop_keep_alive(&mut self) {
        // Send stop signal if the channel exists
        if let Some(tx) = self.keep_alive_tx.take() {
            let _ = tx.send(()).await;
        }
        
        // Wait for the keep-alive task to finish
        if let Some(handle) = self.keep_alive_handle.take() {
            let _ = handle.await;
        }
        
        // Update state
        let mut state = self.state.lock().unwrap();
        state.keep_alive_state = KeepAliveState::Stopped;
    }
    
    /// Get the remaining time of the current lock lease
    pub fn get_remaining_lease(&self) -> Option<Duration> {
        let state = self.state.lock().unwrap();
        state.remaining_lease
    }
    
    /// Check if this client is currently the leader
    pub fn is_leader(&self) -> bool {
        let state = self.state.lock().unwrap();
        state.is_leader
    }
}