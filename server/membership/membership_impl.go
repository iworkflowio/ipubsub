package membership

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/iworkflowio/async-output-service/config"
)

// StreamMembershipImpl implements StreamMembership using HashiCorp's memberlist
type StreamMembershipImpl struct {
	config                *config.Config
	memberlist            *memberlist.Memberlist
	nodes                 []NodeInfo
	nodeNameMap           map[string]bool
	shutdownCh            chan struct{}
	bootstrapNodeProvider *BootstrapNodeProvider
	// protect the nodes list and node name map
	sync.RWMutex
}

// NewStreamMembership creates a new StreamMembership implementation
func NewStreamMembership(config *config.Config) (NodeMembership, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	bootstrapNodeProvider := NewBootstrapNodeProvider(config)

	// Create memberlist configuration
	mlConfig := memberlist.DefaultWANConfig()
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

	sm := &StreamMembershipImpl{
		config:                config,
		shutdownCh:            make(chan struct{}),
		bootstrapNodeProvider: bootstrapNodeProvider,
		nodeNameMap:           make(map[string]bool),
		nodes:                 make([]NodeInfo, 0),
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

// Start initializes and starts joining the cluster
func (sm *StreamMembershipImpl) Start() error {
	log.Println("Starting stream membership service...")

	// Join existing cluster if bootstrap nodes provided
	bootstrapNodes, err := sm.bootstrapNodeProvider.GetBootstrapNodes()
	if err != nil {
		return fmt.Errorf("failed to get bootstrap nodes: %w", err)
	}
	var successCount int
	bootstrapAttempts := sm.config.ClusterConfig.BootstrapTimeoutSeconds
	if len(bootstrapNodes) > 0 {
		log.Printf("Joining cluster via bootstrap nodes: %v", bootstrapNodes)
		for i := 0; i < bootstrapAttempts; i++ {
			successCount, err = sm.memberlist.Join(bootstrapNodes)
			if err != nil {
				log.Printf("Warning: failed to join some bootstrap nodes: %v \n", err)
			}
			// check if the success count is greater than half of the bootstrap nodes
			if successCount <= len(bootstrapNodes)/2 {
				log.Printf("Warning: failed to join cluster via bootstrap nodes, success count: %d, total bootstrap nodes: %d \n", successCount, len(bootstrapNodes))
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

	log.Printf("Stream membership service started. Local node: %s (%s)",
		sm.config.NodeConfig.NodeName, sm.config.NodeConfig.GossipBindAddrPort)

	// start a goroutine to refresh the membership information
	go sm.refreshMembership()

	return nil
}

// updateNodes updates the nodes list and returns true if the nodes list is changed
func (sm *StreamMembershipImpl) updateNodes() {
	sm.Lock()
	defer sm.Unlock()
	oldNodeNameMap := sm.nodeNameMap
	hasChanged := false

	newNodesList := make([]NodeInfo, 0, len(sm.memberlist.Members()))
	addr, port, err := sm.config.NodeConfig.GetGossipAdvertiseAddrPort()
	if err != nil {
		panic(fmt.Sprintf("Fatal: failed to get advertise addr port: %v from config", err))
	}

	selfNode := NodeInfo{
		IsSelf: true,
		Name:   sm.config.NodeConfig.NodeName,
		Addr:   addr,
		Port:   int(port),
	}

	newNodesList = append(newNodesList, selfNode)
	newNodeNameMap := make(map[string]bool)
	newNodeNameMap[selfNode.Name] = true

	for _, node := range sm.memberlist.Members() {
		if node.Name == sm.config.NodeConfig.NodeName {
			// skip self node
			continue
		}
		
		if newNodeNameMap[node.Name] {
			panic(fmt.Sprintf("Fatal: node name %s is duplicated", node.Name))
		}
		newNodeNameMap[node.Name] = true


		if !oldNodeNameMap[node.Name] {
			hasChanged = true
		}

		newNodesList = append(newNodesList, NodeInfo{
			IsSelf: false,
			Name:   node.Name,
			Addr:   node.Addr.String(),
			Port:   int(node.Port),
		})
	}

	if len(oldNodeNameMap) != len(newNodeNameMap) {
		hasChanged = true
	}
	if hasChanged {
		log.Printf("INFO: update nodes list from %v to %v, \n", oldNodeNameMap, newNodeNameMap)
	}

	sm.nodes = newNodesList
	sm.nodeNameMap = newNodeNameMap

}

func (sm *StreamMembershipImpl) refreshMembership() {
	interval := sm.config.ClusterConfig.RefreshIntervalSeconds
	if interval <= 0 {
		interval = 30
	}
	refreshInterval := time.Duration(interval) * time.Second
	for {
		select {
		case <-sm.shutdownCh:
			return
		default:
			time.Sleep(refreshInterval)
			var successCount int
			bootstrapNodes, err := sm.bootstrapNodeProvider.GetBootstrapNodes()
			if err != nil {
				log.Printf("Warning: failed to get bootstrap nodes on refresh: %v \n", err)
				continue
			}
			successCount, err = sm.memberlist.Join(bootstrapNodes)
			if err != nil {
				log.Printf("Warning: failed to join some bootstrap nodes on refresh: %v \n", err)
				continue
			}
			// check if the success count is greater than half of the bootstrap nodes
			if successCount <= len(bootstrapNodes)/2 {
				log.Printf("Warning: failed to refresh cluster via bootstrap nodes, success count: %d, total bootstrap nodes: %d \n", successCount, len(bootstrapNodes))
				continue
			}
			sm.updateNodes()
		}
	}
}

// Stop gracefully shuts down the membership service
func (sm *StreamMembershipImpl) Stop() error {
	log.Println("Stopping stream membership service...")
	sm.memberlist.Leave(1 * time.Second)
	close(sm.shutdownCh)
	log.Println("Stream membership service stopped")
	return nil
}

// GetAllNodes returns all known nodes in the cluster
func (sm *StreamMembershipImpl) GetAllNodes() ([]NodeInfo, error) {
	sm.RLock()
	defer sm.RUnlock()
	return sm.nodes, nil
}

func (sm *StreamMembershipImpl) NotifyJoin(node *memberlist.Node) {
	sm.Lock()
	defer sm.Unlock()
	sm.nodes = append(sm.nodes, NodeInfo{
		IsSelf: false,
		Name:   node.Name,
		Addr:   node.Addr.String(),
		Port:   int(node.Port),
	})
	log.Printf("Node %s joined the cluster \n", node.Name)
}

func (sm *StreamMembershipImpl) NotifyLeave(node *memberlist.Node) {
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
	log.Printf("Node %s left the cluster \n", node.Name)
}

func (sm *StreamMembershipImpl) NotifyUpdate(node *memberlist.Node) {
	// NOOP
}
