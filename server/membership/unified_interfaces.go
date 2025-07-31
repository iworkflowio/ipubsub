package membership

type (
	// StreamMembership is the interface for managing stream membership 
	// Each instance of the service will have its own StreamMembership instance
	// The manager is responsible for managing the stream membership:
	// 1. On startup, find the current peers/nodes of the cluster if exists, or start from zero
	// 2. Join the cluster if exists 
	// 3. Keep track of the streams that the instance owns, and that other nodes own
	// 4. Handle events from other nodes, e.g. node joins, node leaves, stream ownership changes
	// 5. Handle shutdown of the instance to gracefully release the streams
	StreamMembership interface {
		// Start the membership manager
		Start() error
		// Stop the membership manager
		Stop() error
		// Get all nodes in the cluster
		GetAllNodes() ([]NodeInfo, error)
	}

	NodeInfo struct {
		IsSelf bool
		NodeName string
		NodeAddr string
	}
)