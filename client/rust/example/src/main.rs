// client/rust/example/src/main.rs

use std::sync::{Arc, Mutex};
use std::time::Duration;
use async_trait::async_trait;
use clap::Parser;
use tokio::sync::mpsc;
use tokio::time::sleep;

use quorum_quest_client::{QuorumQuestClient, Callbacks, Result};

/// Example callbacks implementation
struct ExampleCallbacks {
    leader_task_stop_tx: Arc<Mutex<Option<mpsc::Sender<()>>>>,
}

impl ExampleCallbacks {
    fn new() -> Self {
        Self {
            leader_task_stop_tx: Arc::new(Mutex::new(None)),
        }
    }
    
    async fn run_leader_tasks(stop_rx: &mut mpsc::Receiver<()>) {
        println!("🚀 Started leader tasks");
        
        loop {
            tokio::select! {
                _ = sleep(Duration::from_secs(3)) => {
                    println!("⚙️  Performing leader-specific task...");
                }
                _ = stop_rx.recv() => {
                    println!("⏹️  Leader tasks stopped");
                    return;
                }
            }
        }
    }
}

#[async_trait]
impl Callbacks for ExampleCallbacks {
    async fn on_leader_elected(&self, is_leader: bool) {
        if is_leader {
            println!("🎉 This node is now the LEADER!");
            // Start leader-specific tasks
            let tx_option = {
                let mut lock = self.leader_task_stop_tx.lock().unwrap();
                
                if lock.is_some() {
                    // Already running, don't start again
                    return;
                }
                
                let (tx, mut rx) = mpsc::channel(1);
                *lock = Some(tx.clone());
                Some((tx, rx))
            };
            
            // Only spawn the task if we actually created a channel
            if let Some((_, mut rx)) = tx_option {
                // Spawn a task to run leader-specific work
                tokio::spawn(async move {
                    Self::run_leader_tasks(&mut rx).await;
                });
            }
        } else {
            println!("⭐ Another node is the leader. Running as follower.");
        }
    }
    
    async fn on_leader_lost(&self) {
        println!("❌ Leadership LOST! Stopping leader tasks...");
        
        // Clone Arc for use in the async block to avoid holding the mutex across an await
        let tx_arc = self.leader_task_stop_tx.clone();
        
        // Tokio spawn to handle the async operation separately
        tokio::spawn(async move {
            // Take ownership of the sender inside the new task
            let tx_option = {
                let mut lock = tx_arc.lock().unwrap();
                lock.take()
            };
            
            // Send the stop signal if we have a sender
            if let Some(tx) = tx_option {
                let _ = tx.send(()).await;
            }
        });
    }
}

/// Command-line arguments
#[derive(Parser, Debug)]
#[clap(author, version, about = "Quorum Quest Example Client")]
struct Args {
    /// Server address
    #[clap(long, default_value = "http://localhost:5050")]
    server: String,
    
    /// Service name
    #[clap(long, default_value = "example-service")]
    service: String,
    
    /// Domain
    #[clap(long, default_value = "production")]
    domain: String,
    
    /// TTL in seconds
    #[clap(long, default_value = "15")]
    ttl: u64,
}

/// Initialization error
#[derive(Debug)]
enum InitError {
    Client(quorum_quest_client::Error),
    Setup(String),
}

impl std::fmt::Display for InitError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            InitError::Client(err) => write!(f, "Client error: {}", err),
            InitError::Setup(msg) => write!(f, "Setup error: {}", msg),
        }
    }
}

impl std::error::Error for InitError {}

impl From<quorum_quest_client::Error> for InitError {
    fn from(err: quorum_quest_client::Error) -> Self {
        InitError::Client(err)
    }
}

/// Run the example application
async fn run() -> Result<()> {
    // Parse command-line arguments
    let args = Args::parse();
    
    println!("🔄 Connecting to {} for service {}", args.server, args.service);
    
    // Create callbacks
    let callbacks = Arc::new(ExampleCallbacks::new());
    
    // Create client
    let mut client = QuorumQuestClient::new(
        args.service,
        args.domain,
        Duration::from_secs(args.ttl),
        args.server,
        None,
        Some(callbacks.clone()),
    ).await?;
    
    // Try to acquire the lock
    let is_leader = client.try_acquire_lock().await?;
    
    if is_leader {
        println!("🔒 Successfully acquired leadership lock!");
        
        // Start the keepalive process to maintain leadership
        client.start_keep_alive().await?;
    } else {
        println!("🔄 Could not acquire leadership lock. Running as follower.");
    }
    
    // Set up a channel for shutdown signal
    let (shutdown_tx, mut shutdown_rx) = mpsc::channel::<()>(1);
    
    // Set up Ctrl+C handler
    let shutdown_tx_clone = shutdown_tx.clone();
    ctrlc::set_handler(move || {
        let tx = shutdown_tx_clone.clone();
        tokio::spawn(async move {
            let _ = tx.send(()).await;
        });
    }).expect("Error setting Ctrl+C handler");
    
    println!("Press Ctrl+C to exit");
    
    // Status ticker
    let mut status_interval = tokio::time::interval(Duration::from_secs(5));
    
    // Main loop
    loop {
        tokio::select! {
            _ = status_interval.tick() => {
                if client.is_leader() {
                    if let Some(remaining) = client.get_remaining_lease() {
                        println!("✅ Still the leader. Lease remaining: {:?}", remaining);
                    }
                } else {
                    println!("🔄 Running as follower, monitoring for leadership changes...");
                    
                    // Attempt to acquire leadership if not already leader
                    match client.try_acquire_lock().await {
                        Ok(true) => {
                            println!("🎉 Successfully acquired leadership!");
                            if let Err(e) = client.start_keep_alive().await {
                                println!("Warning: Failed to start keep-alive: {:?}", e);
                            }
                        },
                        Ok(false) => {
                            // Still not a leader, continue
                        },
                        Err(e) => {
                            println!("Warning: Failed to try acquire lock: {:?}", e);
                        }
                    }
                }
            }
            _ = shutdown_rx.recv() => {
                println!("\n📣 Received shutdown signal. Cleaning up...");
                
                if client.is_leader() {
                    println!("🔓 Releasing leadership lock...");
                    if let Err(e) = client.release_lock().await {
                        println!("Failed to release lock: {:?}", e);
                    }
                }
                
                // Stop the keep-alive process
                client.stop_keep_alive().await;
                
                break;
            }
        }
    }
    
    println!("Goodbye!");
    Ok(())
}

#[tokio::main]
async fn main() {
    // Initialize logger
    env_logger::init();
    
    // Run the example and handle any errors
    if let Err(e) = run().await {
        eprintln!("Error: {:?}", e);
        std::process::exit(1);
    }
}