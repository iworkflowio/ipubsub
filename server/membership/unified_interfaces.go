package membership

type (
	// NodeMembership is the interface for managing stream membership
	// Each instance of the service will have its own NodeMembership instance
	// The manager is responsible for managing the stream membership:
	// 1. On startup, bootstrap the membership manager with the bootstrap nodes
	// 2. Keep track of all the nodes in the cluster(including self)
	// 3. Handle events from other nodes, e.g. node joins, node leaves
	// 4. Handle shutdown of the instance to gracefully shut down
	// 5. Periodically refresh the membership information based on bootstrap nodes
	NodeMembership interface {
		// Start the membership manager
		Start() error
		// Stop the membership manager
		Stop() error
		// Get all nodes in the cluster
		GetAllNodes() ([]NodeInfo, error)

		// Get the current membership version
		GetVersion() int64
	}

	NodeInfo struct {
		IsSelf bool
		Name   string
		Addr   string
		Port   int
	}
)
