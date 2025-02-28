// client/go/quorum-quest-client/example/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	quorumquestclient "github.com/avivl/quorum-quest/client/go/quorum-quest-client"
)

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
		fmt.Println("\nReceived shutdown signal. Cleaning up...")
		cancel()
	}()

	// Create a new client
	client, err := quorumquestclient.NewQuorumQuestClient(
		serviceName,
		"production",
		15*time.Second,
		serverAddr,
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Try to acquire the lock
	isLeader, err := client.TryAcquireLock(ctx)
	if err != nil {
		log.Fatalf("Failed to try acquire lock: %v", err)
	}

	if isLeader {
		fmt.Println("Successfully acquired leadership lock!")

		// Start the keepalive process to maintain leadership
		if err := client.StartKeepAlive(ctx); err != nil {
			log.Fatalf("Failed to start keep-alive: %v", err)
		}

		// Run the leader process
		runLeaderProcess(ctx, client)
	} else {
		fmt.Println("Could not acquire leadership lock. Another instance is the leader.")

		// Run as follower or exit
		runFollowerProcess(ctx)
	}
}

func runLeaderProcess(ctx context.Context, client *quorumquestclient.QuorumQuestClient) {
	// Status ticker to check more frequently
	statusTicker := time.NewTicker(2 * time.Second)
	defer statusTicker.Stop()

	// Add recovery logic
	if !client.IsLeader() {
		fmt.Println("Already lost leadership before starting leader process!")
		return
	}

	fmt.Println("Running as leader! Press Ctrl+C to exit.")

	for {
		select {
		case <-ctx.Done():
			// Context canceled, clean up
			fmt.Println("Releasing leadership lock...")
			if err := client.ReleaseLock(context.Background()); err != nil {
				log.Printf("Failed to release lock: %v", err)
			}
			return

		case <-statusTicker.C:
			// Check leadership status more frequently
			if !client.IsLeader() {
				fmt.Println("Lost leadership! Trying to reacquire...")

				// Try to reacquire the lock
				isLeader, err := client.TryAcquireLock(ctx)
				if err != nil {
					log.Printf("Failed to try reacquire lock: %v", err)
					return
				}

				if isLeader {
					fmt.Println("Successfully reacquired leadership!")
					if err := client.StartKeepAlive(ctx); err != nil {
						log.Printf("Failed to restart keep-alive: %v", err)
						return
					}
				} else {
					fmt.Println("Could not reacquire leadership. Exiting leader process.")
					return
				}
			}

			fmt.Printf("Still the leader. Lease remaining: %v\n", client.GetRemainingLease())
			performLeaderTask()
		}
	}
}

func runFollowerProcess(ctx context.Context) {
	fmt.Println("Running as follower! Press Ctrl+C to exit.")

	// Wait until context is canceled
	<-ctx.Done()
	fmt.Println("Follower shutting down.")
}

func performLeaderTask() {
	// Example of a task that only the leader should perform
	fmt.Println("Performing leader-specific task...")
	// Simulate work
	time.Sleep(100 * time.Millisecond)
}
