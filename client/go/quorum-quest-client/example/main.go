// client/go/quorum-quest-client/example/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	quorumquestclient "github.com/avivl/quorum-quest/client/go/quorum-quest-client"
)

// ExampleCallbacks implements the lockservice.Callbacks interface
// to demonstrate the event-driven architecture
type ExampleCallbacks struct {
	tasksMutex      sync.Mutex
	leaderTasksStop chan struct{}
}

// OnLeaderElected is called when leadership status changes
func (c *ExampleCallbacks) OnLeaderElected(isLeader bool) {
	if isLeader {
		fmt.Println("🎉 This node is now the LEADER!")
		// Start leader-specific tasks
		c.tasksMutex.Lock()
		defer c.tasksMutex.Unlock()

		if c.leaderTasksStop != nil {
			// Already running, don't start again
			return
		}

		c.leaderTasksStop = make(chan struct{})
		go c.runLeaderTasks(c.leaderTasksStop)
	} else {
		fmt.Println("⭐ Another node is the leader. Running as follower.")
	}
}

// OnLeaderLost is called when this node was the leader but lost leadership
func (c *ExampleCallbacks) OnLeaderLost() {
	fmt.Println("❌ Leadership LOST! Stopping leader tasks...")

	// Stop leader-specific tasks
	c.tasksMutex.Lock()
	defer c.tasksMutex.Unlock()

	if c.leaderTasksStop != nil {
		close(c.leaderTasksStop)
		c.leaderTasksStop = nil
	}
}

// runLeaderTasks simulates leader-specific tasks
func (c *ExampleCallbacks) runLeaderTasks(stopChan chan struct{}) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	fmt.Println("🚀 Started leader tasks")

	for {
		select {
		case <-ticker.C:
			fmt.Println("⚙️  Performing leader-specific task...")
		case <-stopChan:
			fmt.Println("⏹️  Leader tasks stopped")
			return
		}
	}
}

func main() {
	// Parse command line arguments or use defaults
	serverAddr := "localhost:5050"
	if len(os.Args) > 1 {
		serverAddr = os.Args[1]
	}

	serviceName := "example-service"
	if len(os.Args) > 2 {
		serviceName = os.Args[2]
	}

	// Create a context that will be canceled on SIGINT or SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals
		fmt.Println("\n📣 Received shutdown signal. Cleaning up...")
		cancel()
	}()

	// Create callbacks
	callbacks := &ExampleCallbacks{}

	// Create a new client with callbacks
	client, err := quorumquestclient.NewQuorumQuestClient(
		serviceName,
		"production",
		15*time.Second,
		serverAddr,
		quorumquestclient.WithCallbacks(callbacks),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Printf("🔄 Connecting to %s for service %s\n", serverAddr, serviceName)

	// Try to acquire the lock
	isLeader, err := client.TryAcquireLock(ctx)
	if err != nil {
		log.Fatalf("Failed to try acquire lock: %v", err)
	}

	if isLeader {
		fmt.Println("🔒 Successfully acquired leadership lock!")

		// Start the keepalive process to maintain leadership
		if err := client.StartKeepAlive(ctx); err != nil {
			log.Fatalf("Failed to start keep-alive: %v", err)
		}
	} else {
		fmt.Println("🔄 Could not acquire leadership lock. Running as follower.")
	}

	// Setup monitor for leadership status changes
	statusTicker := time.NewTicker(5 * time.Second)
	defer statusTicker.Stop()

	// This loop will keep the process running and periodically check status
	for {
		select {
		case <-ctx.Done():
			// Context canceled, clean up and exit
			fmt.Println("🛑 Shutting down...")

			if client.IsLeader() {
				fmt.Println("🔓 Releasing leadership lock...")
				if err := client.ReleaseLock(context.Background()); err != nil {
					log.Printf("Failed to release lock: %v", err)
				}
			}
			return

		case <-statusTicker.C:
			if client.IsLeader() {
				fmt.Printf("✅ Still the leader. Lease remaining: %v\n", client.GetRemainingLease())
			} else {
				fmt.Println("🔄 Running as follower, monitoring for leadership changes...")

				// Attempt to acquire leadership if not already leader
				isLeader, err := client.TryAcquireLock(ctx)
				if err != nil {
					log.Printf("Warning: Failed to try acquire lock: %v", err)
					continue
				}

				if isLeader {
					fmt.Println("🎉 Successfully acquired leadership!")
					if err := client.StartKeepAlive(ctx); err != nil {
						log.Printf("Warning: Failed to start keep-alive: %v", err)
					}
				}
			}
		}
	}
}
