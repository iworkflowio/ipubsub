package membership

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/iworkflowio/async-output-service/config"
	"github.com/iworkflowio/async-output-service/service/log"
	"github.com/iworkflowio/async-output-service/service/log/tag"
)

// MembershipImpl implements NodeMembership using HashiCorp's memberlist
type MembershipImpl struct {
	logger                log.Logger
	config                *config.Config
	memberlist            *memberlist.Memberlist
	bootstrapNodeProvider *BootstrapNodeProvider

	nodes       []NodeInfo
	nodeNameMap map[string]bool
	version     int64 // version of the cluster, incremented when the cluster is updated
	// protect the nodes list and node name map
	sync.RWMutex

	shutdownCh  chan struct{}
	refreshDone chan struct{} // Add channel to track refresh goroutine completion
}

// NewNodeMembershipImpl creates a new NodeMembership implementation
func NewNodeMembershipImpl(config *config.Config, logger log.Logger) (NodeMembership, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	bootstrapNodeProvider := NewBootstrapNodeProvider(config)

	// Create memberlist configuration
	mlConfig := memberlist.DefaultLANConfig()
	mlConfig.Name = config.NodeConfig.NodeName
	var err error
	mlConfig.BindAddr, mlConfig.BindPort, err = config.NodeConfig.GetGossipBindAddrPort()
	if err != nil {
		return nil, err
	}

	mlConfig.AdvertiseAddr, mlConfig.AdvertisePort, err = config.NodeConfig.GetGossipAdvertiseAddrPort()
	if err != nil {
		return nil, err
	}

	sm := &MembershipImpl{
		config:                config,
		shutdownCh:            make(chan struct{}),
		bootstrapNodeProvider: bootstrapNodeProvider,
		nodeNameMap:           make(map[string]bool),
		nodes:                 make([]NodeInfo, 0),
		logger:                logger,
		refreshDone:           make(chan struct{}),
		version:               0,
	}

	mlConfig.Events = sm

	// Create memberlist
	ml, err := memberlist.Create(mlConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create memberlist: %w", err)
	}

	sm.memberlist = ml
	return sm, nil
}

func (sm *MembershipImpl) GetVersion() int64 {
	sm.RLock()
	defer sm.RUnlock()
	return sm.version
}

// Start initializes and starts joining the cluster
func (sm *MembershipImpl) Start() error {
	sm.logger.Info("Starting membership service...")

	// Join existing cluster if bootstrap nodes provided
	bootstrapNodes, err := sm.bootstrapNodeProvider.GetBootstrapNodes()
	if err != nil {
		return fmt.Errorf("failed to get bootstrap nodes: %w", err)
	}
	var successCount int
	bootstrapAttempts := sm.config.ClusterConfig.BootstrapTimeoutSeconds
	if bootstrapAttempts <= 0 {
		bootstrapAttempts = 60
	}
	if len(bootstrapNodes) > 0 {
		sm.logger.Info("Joining cluster via bootstrap nodes", tag.Value(bootstrapNodes))
		for i := 0; i < bootstrapAttempts; i++ {
			successCount, err = sm.memberlist.Join(bootstrapNodes)
			if err != nil {
				sm.logger.Warn("Warning: failed to join some bootstrap nodes", tag.Error(err))
			}
			// check if the success count is greater than half of the bootstrap nodes
			if successCount <= len(bootstrapNodes)/2 {
				sm.logger.Warn("Warning: failed to join cluster via bootstrap nodes on startup", tag.Value(successCount), tag.Value(len(bootstrapNodes)))
				// retry after 1 second
				time.Sleep(time.Second)
			} else {
				break
			}
		}
		if successCount <= len(bootstrapNodes)/2 {
			return fmt.Errorf("failed to join cluster via bootstrap nodes, success count: %d, total bootstrap nodes: %d", successCount, len(bootstrapNodes))
		}
	}

	sm.updateNodes()

	sm.logger.Info("Membership service started. Local node", tag.Value(sm.config.NodeConfig.NodeName), tag.Value(sm.config.NodeConfig.GossipBindAddrPort))

	// start a goroutine to refresh the membership information
	go sm.refreshMembership()

	return nil
}

// updateNodes updates the whole nodes list
func (sm *MembershipImpl) updateNodes() {
	sm.Lock()
	defer sm.Unlock()
	oldNodeNameMap := sm.nodeNameMap
	hasChanged := false

	newNodesList := make([]NodeInfo, 0, len(sm.memberlist.Members()))
	newNodeNameMap := make(map[string]bool)

	for _, node := range sm.memberlist.Members() {
		if newNodeNameMap[node.Name] {
			panic(fmt.Sprintf("Fatal: node name %s is duplicated", node.Name))
		}
		newNodeNameMap[node.Name] = true

		if !oldNodeNameMap[node.Name] {
			hasChanged = true
		}

		isSelf := node.Name == sm.config.NodeConfig.NodeName
		newNodesList = append(newNodesList, NodeInfo{
			IsSelf: isSelf,
			Name:   node.Name,
			Addr:   node.Addr.String(),
			Port:   int(node.Port),
		})
	}

	if len(oldNodeNameMap) != len(newNodeNameMap) {
		hasChanged = true
	}

	if hasChanged {
		sm.version++
		sm.nodes = newNodesList
		sm.nodeNameMap = newNodeNameMap
		sm.logger.Info("INFO: update nodes list from", tag.Value(oldNodeNameMap), tag.Value(newNodeNameMap))
	} else {
		sm.logger.Info("INFO: no change in nodes list", tag.Value(newNodeNameMap))
	}
}

func (sm *MembershipImpl) refreshMembership() {
	interval := sm.config.ClusterConfig.RefreshIntervalSeconds
	if interval <= 0 {
		interval = 30
	}
	interval = int(float64(interval) * 1.1) // add 10% jitter

	refreshInterval := time.Duration(interval) * time.Second
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	defer close(sm.refreshDone) // Signal completion when goroutine exits

	for {
		select {
		case <-sm.shutdownCh:
			return
		case <-ticker.C:
			var successCount int
			bootstrapNodes, err := sm.bootstrapNodeProvider.GetBootstrapNodes()
			if err != nil {
				sm.logger.Warn("Warning: failed to get bootstrap nodes on refresh", tag.Error(err))
				continue
			}
			successCount, err = sm.memberlist.Join(bootstrapNodes)
			if err != nil {
				sm.logger.Warn("Warning: failed to join some bootstrap nodes on refresh", tag.Error(err))
				continue
			}
			// check if the success count is greater than half of the bootstrap nodes
			if successCount <= len(bootstrapNodes)/2 {
				sm.logger.Warn("Warning: failed to refresh cluster via bootstrap nodes", tag.Value(successCount), tag.Value(len(bootstrapNodes)))
				continue
			}
			sm.updateNodes()
		}
	}
}

// Stop gracefully shuts down the membership service
func (sm *MembershipImpl) Stop() error {
	sm.logger.Info("Stopping membership service...")

	var errLeave, errShutdown error
	// First leave the cluster
	if sm.memberlist != nil {
		errLeave = sm.memberlist.Leave(10 * time.Second)
		if errLeave != nil {
			sm.logger.Warn("Error leaving cluster", tag.Error(errLeave))
		}
	}

	// Signal shutdown to refresh goroutine
	close(sm.shutdownCh)

	// Wait for refresh goroutine to complete
	select {
	case <-sm.refreshDone:
		// Goroutine completed
	case <-time.After(5 * time.Second):
		sm.logger.Warn("Timeout waiting for refresh goroutine to stop")
	}

	// Shutdown the memberlist to free resources and ports
	if sm.memberlist != nil {
		errShutdown = sm.memberlist.Shutdown()
		if errShutdown != nil {
			sm.logger.Warn("Error shutting down memberlist", tag.Error(errShutdown))
		}
	}

	if errLeave != nil || errShutdown != nil {
		return fmt.Errorf("error leaving cluster: %w, error shutting down memberlist: %w", errLeave, errShutdown)
	}

	sm.logger.Info("Membership service stopped")
	return nil
}

func (sm *MembershipImpl) forceShutdownForTest() error {
	return sm.memberlist.Shutdown()
}

// GetAllNodes returns all known nodes in the cluster
func (sm *MembershipImpl) GetAllNodes() ([]NodeInfo, error) {
	sm.RLock()
	defer sm.RUnlock()
	return sm.nodes, nil
}

func (sm *MembershipImpl) NotifyJoin(node *memberlist.Node) {
	sm.Lock()
	defer sm.Unlock()
	if sm.nodeNameMap[node.Name] {
		sm.logger.Warn("Node already in the cluster", tag.Value(node.Name))
		return
	}
	
	isSelf := node.Name == sm.config.NodeConfig.NodeName
	sm.nodes = append(sm.nodes, NodeInfo{
		IsSelf: isSelf,
		Name:   node.Name,
		Addr:   node.Addr.String(),
		Port:   int(node.Port),
	})
	sm.nodeNameMap[node.Name] = true
	sm.version++
	sm.logger.Info("Node joined the cluster", tag.Value(node.Name))
}

func (sm *MembershipImpl) NotifyLeave(node *memberlist.Node) {
	sm.Lock()
	defer sm.Unlock()
	nodes := sm.nodes
	newNodes := make([]NodeInfo, 0, len(nodes))
	for _, nd := range nodes {
		if nd.Name == node.Name {
			continue
		}
		newNodes = append(newNodes, nd)
	}
	sm.nodes = newNodes
	delete(sm.nodeNameMap, node.Name)
	sm.version++
	sm.logger.Info("Node left the cluster", tag.Value(node.Name))
}

func (sm *MembershipImpl) NotifyUpdate(node *memberlist.Node) {
	// NOOP
}
