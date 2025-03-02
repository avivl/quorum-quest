// internal/lockservice/callbacks.go
package lockservice

// Callbacks defines the interface for leadership change notifications
type Callbacks interface {
	// OnLeaderElected is called when leadership status changes
	// isLeader is true if this node is the new leader
	OnLeaderElected(isLeader bool)

	// OnLeaderLost is called when this node was the leader but lost leadership
	OnLeaderLost()
}

// NoOpCallbacks implements Callbacks with empty methods
// Useful as a default when no callbacks are provided
type NoOpCallbacks struct{}

// OnLeaderElected implements Callbacks.OnLeaderElected with an empty method
func (c *NoOpCallbacks) OnLeaderElected(isLeader bool) {}

// OnLeaderLost implements Callbacks.OnLeaderLost with an empty method
func (c *NoOpCallbacks) OnLeaderLost() {}
