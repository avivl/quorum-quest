// client/go/quorum-quest-client/client.go
package quorumquestclient

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	pb "github.com/avivl/quorum-quest/api/gen/go/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// QuorumQuestClient is a client for the Quorum Quest leader election service
type QuorumQuestClient struct {
	mu              sync.Mutex
	notifyStop      chan struct{}
	notifyWaitGroup sync.WaitGroup
	ttl             time.Duration
	remainingLease  time.Duration
	service         string
	domain          string
	id              string
	ldClient        pb.LeaderElectionServiceClient
	conn            *grpc.ClientConn
}

// Option is a function that configures a QuorumQuestClient
type Option func(*QuorumQuestClient)

// WithServerStub allows injecting a mock client for testing
func WithServerStub(stub pb.LeaderElectionServiceClient) Option {
	return func(q *QuorumQuestClient) {
		q.ldClient = stub
	}
}

// WithClientID allows setting a specific client ID instead of generating a random one
func WithClientID(id string) Option {
	return func(q *QuorumQuestClient) {
		if id != "" {
			q.id = id
		}
	}
}

// NewQuorumQuestClient creates a new client for the Quorum Quest service
func NewQuorumQuestClient(service string, domain string, ttl time.Duration, address string, opts ...Option) (*QuorumQuestClient, error) {
	if service == "" {
		return nil, errors.New("service name cannot be empty")
	}
	if domain == "" {
		return nil, errors.New("domain cannot be empty")
	}
	if ttl <= 0 {
		return nil, errors.New("TTL must be greater than zero")
	}
	if address == "" {
		return nil, errors.New("server address cannot be empty")
	}

	// Create gRPC connection with appropriate parameters
	conn, err := grpc.Dial(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}))
	if err != nil {
		return nil, fmt.Errorf("could not connect to server: %w", err)
	}

	// Create the client and set default values
	ldClient := pb.NewLeaderElectionServiceClient(conn)
	client := &QuorumQuestClient{
		ttl:            ttl,
		remainingLease: time.Duration(-1),
		id:             uuid.NewString(),
		service:        service,
		domain:         domain,
		ldClient:       ldClient,
		conn:           conn,
	}

	// Apply any custom options
	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// Close releases resources held by the client
func (q *QuorumQuestClient) Close() error {
	q.StopKeepAlive()
	if q.conn != nil {
		return q.conn.Close()
	}
	return nil
}

// TryAcquireLock attempts to acquire a lock for the service/domain
// Returns true if the lock was acquired, false otherwise
func (q *QuorumQuestClient) TryAcquireLock(ctx context.Context) (bool, error) {
	req := &pb.TryAcquireLockRequest{
		ClientId: q.id,
		Service:  q.service,
		Domain:   q.domain,
		Ttl:      int32(q.ttl.Seconds()),
	}

	resp, err := q.ldClient.TryAcquireLock(ctx, req)
	if err != nil {
		return false, fmt.Errorf("failed to try acquire lock: %w", err)
	}

	if resp.IsLeader {
		// If we acquired the lock, start the keepalive timer
		q.mu.Lock()
		q.remainingLease = q.ttl
		q.mu.Unlock()
	}

	return resp.IsLeader, nil
}

// ReleaseLock releases a previously acquired lock
func (q *QuorumQuestClient) ReleaseLock(ctx context.Context) error {
	req := &pb.ReleaseLockRequest{
		ClientId: q.id,
		Service:  q.service,
		Domain:   q.domain,
	}

	_, err := q.ldClient.ReleaseLock(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	q.mu.Lock()
	q.remainingLease = time.Duration(-1)
	q.mu.Unlock()

	return nil
}

// keepAlive refreshes the lock TTL
func (q *QuorumQuestClient) keepAlive(ctx context.Context) (time.Duration, error) {
	req := &pb.KeepAliveRequest{
		ClientId: q.id,
		Service:  q.service,
		Domain:   q.domain,
		Ttl:      int32(q.ttl.Seconds()),
	}

	resp, err := q.ldClient.KeepAlive(ctx, req)
	if err != nil {
		return time.Duration(-1), fmt.Errorf("failed to keep lock alive: %w", err)
	}

	lease := resp.LeaseLength.AsDuration()
	q.mu.Lock()
	q.remainingLease = lease
	q.mu.Unlock()

	return lease, nil
}

// GetRemainingLease returns the remaining time of the current lock lease
func (q *QuorumQuestClient) GetRemainingLease() time.Duration {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.remainingLease
}

// IsLeader returns true if this client currently holds the lock
func (q *QuorumQuestClient) IsLeader() bool {
	return q.GetRemainingLease() > 0
}

// StartKeepAlive starts a background goroutine to periodically refresh the lock
func (q *QuorumQuestClient) StartKeepAlive(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.notifyStop != nil {
		return errors.New("keep-alive is already running")
	}

	q.notifyStop = make(chan struct{})
	q.notifyWaitGroup.Add(1)

	go func() {
		defer q.notifyWaitGroup.Done()

		// Start with half the TTL to avoid expiration
		refreshInterval := q.ttl / 3
		if refreshInterval < 3*time.Second {
			refreshInterval = 3 * time.Second
		}

		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-q.notifyStop:
				return
			case <-ticker.C:
				// Create a timeout context for each keepalive call
				keepAliveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				lease, err := q.keepAlive(keepAliveCtx)
				cancel()

				if err != nil {
					q.mu.Lock()
					q.remainingLease = time.Duration(-1)
					q.mu.Unlock()
					return
				}

				// If server returned a negative lease, we're no longer the leader
				if lease < 0 {
					q.mu.Lock()
					q.remainingLease = time.Duration(-1)
					q.mu.Unlock()
					return
				}
			}
		}
	}()

	return nil
}

// StopKeepAlive stops the background keep-alive goroutine
func (q *QuorumQuestClient) StopKeepAlive() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.notifyStop != nil {
		close(q.notifyStop)
		q.notifyWaitGroup.Wait()
		q.notifyStop = nil
	}
}
