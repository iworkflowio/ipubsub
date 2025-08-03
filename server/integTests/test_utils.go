package integTests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/iworkflowio/ipubsub/config"
	"github.com/iworkflowio/ipubsub/genapi"
	"github.com/iworkflowio/ipubsub/service"
	"github.com/iworkflowio/ipubsub/service/log/loggerimpl"
	"github.com/iworkflowio/ipubsub/service/log/tag"
)

// TestNode represents a running iPubSub service instance
type TestNode struct {
	Name       string
	HTTPAddr   string
	GossipAddr string
	Config     *config.Config
	Service    *service.Service
	Client     *http.Client
	stopCh     chan struct{}
	stopped    bool
	mu         sync.Mutex
}

// TestCluster manages multiple TestNode instances
type TestCluster struct {
	Nodes map[string]*TestNode
	mu    sync.RWMutex
	t     *testing.T
}

// NewTestCluster creates a new test cluster
func NewTestCluster(t *testing.T) *TestCluster {
	return &TestCluster{
		Nodes: make(map[string]*TestNode),
		t:     t,
	}
}

// CreateTestConfig creates a test configuration for a node
func CreateTestConfig(nodeName string, httpPort, gossipPort int, bootstrapPorts []int) *config.Config {
	// Convert bootstrap ports to addresses
	var bootstrapAddrs []string
	for _, port := range bootstrapPorts {
		bootstrapAddrs = append(bootstrapAddrs, fmt.Sprintf("127.0.0.1:%d", port))
	}

	return &config.Config{
		Log: config.Logger{
			Stdout:   true,
			Level:    "info",
			LevelKey: "level",
		},
		NodeConfig: config.NodeConfig{
			NodeName:                nodeName,
			GossipBindAddrPort:      fmt.Sprintf("127.0.0.1:%d", gossipPort),
			GossipAdvertiseAddrPort: fmt.Sprintf("127.0.0.1:%d", gossipPort),
			HttpBindAddrPort:        fmt.Sprintf("127.0.0.1:%d", httpPort),
			HttpAdvertiseAddrPort:   fmt.Sprintf("127.0.0.1:%d", httpPort),
		},
		ClusterConfig: config.ClusterConfig{
			ClusterName:                  "test-cluster",
			BootstrapType:                "static",
			StaticBootstrapNodeAddrPorts: bootstrapAddrs,
			BootstrapTimeoutSeconds:      10,
			RefreshIntervalSeconds:       5,
		},
		MatchConfig: config.MatchConfig{
			ReceiveDefaultTimeout: "10s",
			SendDefaultTimeout:    "10s",
		},
		HashRingConfig: config.HashRingConfig{
			VirtualNodes: 50,
		},
	}
}

// StartNode starts a single iPubSub node with the given config
func (tc *TestCluster) StartNode(nodeName string, httpPort, gossipPort int, bootstrapPorts []int) (*TestNode, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Create configuration
	cfg := CreateTestConfig(nodeName, httpPort, gossipPort, bootstrapPorts)

	// Create zap logger
	zapLogger, err := zap.NewDevelopment()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	logger := loggerimpl.NewLogger(zapLogger).WithTags(tag.NodeName(cfg.NodeConfig.NodeName))

	// Create and start service
	svc := service.NewService(cfg, logger)

	node := &TestNode{
		Name:       nodeName,
		HTTPAddr:   fmt.Sprintf("127.0.0.1:%d", httpPort),
		GossipAddr: fmt.Sprintf("127.0.0.1:%d", gossipPort),
		Config:     cfg,
		Service:    svc,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		stopCh: make(chan struct{}),
	}

	// Start service in goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				tc.t.Logf("Service panic in node %s: %v", nodeName, r)
			}
		}()
		svc.Start()
	}()

	tc.Nodes[nodeName] = node

	// Wait for the service to be ready
	if err := tc.waitForNodeReady(node, 30*time.Second); err != nil {
		tc.StopNode(nodeName)
		return nil, fmt.Errorf("node failed to become ready: %w", err)
	}

	tc.t.Logf("Started node %s on HTTP %s, Gossip %s", nodeName, node.HTTPAddr, node.GossipAddr)
	return node, nil
}

// StartSingleNode starts a single node cluster
func (tc *TestCluster) StartSingleNode(nodeName string, httpPort, gossipPort int) (*TestNode, error) {
	return tc.StartNode(nodeName, httpPort, gossipPort, []int{gossipPort})
}

// StopNode stops a specific node
func (tc *TestCluster) StopNode(nodeName string) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	node, exists := tc.Nodes[nodeName]
	if !exists {
		return fmt.Errorf("node %s not found", nodeName)
	}

	node.mu.Lock()
	if !node.stopped {
		node.stopped = true
		close(node.stopCh)
		if node.Service != nil {
			node.Service.Stop()
		}
	}
	node.mu.Unlock()

	delete(tc.Nodes, nodeName)
	tc.t.Logf("Stopped node %s", nodeName)
	return nil
}

// StopAll stops all nodes in the cluster
func (tc *TestCluster) StopAll() {
	tc.mu.Lock()
	nodeNames := make([]string, 0, len(tc.Nodes))
	for name := range tc.Nodes {
		nodeNames = append(nodeNames, name)
	}
	tc.mu.Unlock()

	for _, name := range nodeNames {
		tc.StopNode(name)
	}
}

// waitForNodeReady waits for a node to be ready to accept HTTP requests
func (tc *TestCluster) waitForNodeReady(node *TestNode, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	healthURL := fmt.Sprintf("http://%s/health", node.HTTPAddr)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for node %s to be ready", node.Name)
		case <-ticker.C:
			resp, err := node.Client.Get(healthURL)
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return nil
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
	}
}

// SendMessage sends a message to a specific node
func (tc *TestCluster) SendMessage(nodeName string, req *genapi.SendRequest) error {
	tc.mu.RLock()
	node, exists := tc.Nodes[nodeName]
	tc.mu.RUnlock()

	if !exists {
		return fmt.Errorf("node %s not found", nodeName)
	}

	sendURL := fmt.Sprintf("http://%s/api/v1/streams/send", node.HTTPAddr)

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := node.Client.Post(sendURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ReceiveMessage receives a message from a specific node
func (tc *TestCluster) ReceiveMessage(nodeName, streamId string, timeoutSeconds int) (*genapi.ReceiveResponse, error) {
	tc.mu.RLock()
	node, exists := tc.Nodes[nodeName]
	tc.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("node %s not found", nodeName)
	}

	receiveURL := fmt.Sprintf("http://%s/api/v1/streams/receive?streamId=%s&timeoutSeconds=%d",
		node.HTTPAddr, streamId, timeoutSeconds)

	resp, err := node.Client.Get(receiveURL)
	if err != nil {
		return nil, fmt.Errorf("failed to receive request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("receive failed with status %d: %s", resp.StatusCode, string(body))
	}

	var receiveResp genapi.ReceiveResponse
	if err := json.Unmarshal(body, &receiveResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &receiveResp, nil
}

// Helper functions for creating test requests

// CreateSendRequest creates a SendRequest with the given parameters
func CreateSendRequest(streamId string, message interface{}, options ...func(*genapi.SendRequest)) *genapi.SendRequest {
	messageUuid := uuid.New().String()
	req := &genapi.SendRequest{
		StreamId:    streamId,
		MessageUuid: messageUuid,
		Message:     message,
	}

	// Apply optional configuration
	for _, opt := range options {
		opt(req)
	}

	return req
}

// WithWriteToDB sets the writeToDB option
func WithWriteToDB(writeToDB bool) func(*genapi.SendRequest) {
	return func(req *genapi.SendRequest) {
		req.WriteToDB = &writeToDB
	}
}

// WithInMemoryStreamSize sets the inMemoryStreamSize option
func WithInMemoryStreamSize(size int32) func(*genapi.SendRequest) {
	return func(req *genapi.SendRequest) {
		req.InMemoryStreamSize = &size
	}
}

// WithBlockingSendTimeout sets the blockingSendTimeoutSeconds option
func WithBlockingSendTimeout(timeout int32) func(*genapi.SendRequest) {
	return func(req *genapi.SendRequest) {
		req.BlockingSendTimeoutSeconds = &timeout
	}
}

// WithDBTTL sets the dbTTLSeconds option
func WithDBTTL(ttl int32) func(*genapi.SendRequest) {
	return func(req *genapi.SendRequest) {
		req.DbTTLSeconds = &ttl
	}
}

// AssertMessageEquals asserts that two messages are equal
func AssertMessageEquals(t *testing.T, expected, actual interface{}) {
	// Convert both to JSON for comparison to handle different types
	expectedJSON, err := json.Marshal(expected)
	require.NoError(t, err)

	actualJSON, err := json.Marshal(actual)
	require.NoError(t, err)

	assert.JSONEq(t, string(expectedJSON), string(actualJSON))
}

// WaitForClusterStable waits for all nodes in the cluster to see each other
func (tc *TestCluster) WaitForClusterStable(expectedNodeCount int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for cluster to be stable with %d nodes", expectedNodeCount)
		case <-ticker.C:
			stable := true
			tc.mu.RLock()
			for _, node := range tc.Nodes {
				// Check health endpoint which should reflect cluster state
				healthURL := fmt.Sprintf("http://%s/health", node.HTTPAddr)
				resp, err := node.Client.Get(healthURL)
				if err != nil || resp.StatusCode != http.StatusOK {
					stable = false
					if resp != nil {
						resp.Body.Close()
					}
					break
				}
				resp.Body.Close()
			}
			tc.mu.RUnlock()

			if stable && len(tc.Nodes) == expectedNodeCount {
				return nil
			}
		}
	}
}

// GetAvailablePort finds an available port for testing
func GetAvailablePort() (int, error) {
	// Simple approach: use a range of test ports
	// In a real scenario, you might want to check if ports are actually free
	basePort := 18000
	for i := 0; i < 100; i++ {
		port := basePort + i
		// Here you could add actual port availability check if needed
		return port, nil
	}
	return 0, fmt.Errorf("no available ports found")
}
