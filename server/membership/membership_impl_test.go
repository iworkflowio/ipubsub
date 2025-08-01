package membership

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/iworkflowio/async-output-service/config"
	"github.com/iworkflowio/async-output-service/service/log"
	"github.com/iworkflowio/async-output-service/service/log/loggerimpl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTimeout = 30 * time.Second
)

func createTestLogger() log.Logger {
	return loggerimpl.NewNopLogger()
}

func TestSingleNodeMembership(t *testing.T) {
	// Clean up any previous test state
	setLocalOverrideForBootstrapNodesForTests([]string{})
	defer setLocalOverrideForBootstrapNodesForTests([]string{})

	cfg := createTestConfig("node1", 7946, 8080)
	logger := loggerimpl.NewNopLogger()

	membership, err := NewNodeMembershipImpl(cfg, logger)
	require.NoError(t, err)
	require.NotNil(t, membership)

	err = membership.Start()
	require.NoError(t, err)

	// Wait a bit for startup
	time.Sleep(100 * time.Millisecond)

	// Check that we have only the local node
	nodes, err := membership.GetAllNodes()
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.True(t, nodes[0].IsSelf)
	assert.Equal(t, "node1", nodes[0].Name)

	err = membership.Stop()
	require.NoError(t, err)
}

func TestTwoNodeCluster(t *testing.T) {
	// Clean up any previous test state
	setLocalOverrideForBootstrapNodesForTests([]string{})
	defer setLocalOverrideForBootstrapNodesForTests([]string{})

	// Create first node
	cfg1 := createTestConfig("node1", 7940, 8070)
	logger1 := loggerimpl.NewNopLogger()

	membership1, err := NewNodeMembershipImpl(cfg1, logger1)
	require.NoError(t, err)

	err = membership1.Start()
	require.NoError(t, err)
	defer func() {
		err := membership1.Stop()
		require.NoError(t, err)
	}()

	// Wait for first node to fully start
	time.Sleep(200 * time.Millisecond)

	// Create second node that joins the first
	cfg2 := createTestConfig("node2", 7941, 8071)
	cfg2.ClusterConfig.StaticBootstrapNodeAddrPorts = []string{"127.0.0.1:7940"}
	logger2 := createTestLogger()

	membership2, err := NewNodeMembershipImpl(cfg2, logger2)
	require.NoError(t, err)

	err = membership2.Start()
	require.NoError(t, err)
	defer func() {
		err := membership2.Stop()
		require.NoError(t, err)
	}()

	// Wait for cluster formation
	waitForClusterSize(t, membership1, 2, testTimeout)
	waitForClusterSize(t, membership2, 2, testTimeout)

	// Verify both nodes see each other
	nodes1, err := membership1.GetAllNodes()
	require.NoError(t, err)
	assert.Len(t, nodes1, 2)

	nodes2, err := membership2.GetAllNodes()
	require.NoError(t, err)
	assert.Len(t, nodes2, 2)

	// Verify each node sees itself correctly
	assert.True(t, hasNodeWithName(nodes1, "node1", true))
	assert.True(t, hasNodeWithName(nodes1, "node2", false))
	assert.True(t, hasNodeWithName(nodes2, "node1", false))
	assert.True(t, hasNodeWithName(nodes2, "node2", true))
}

func TestThreeNodeCluster(t *testing.T) {
	// Clean up any previous test state
	setLocalOverrideForBootstrapNodesForTests([]string{})
	defer setLocalOverrideForBootstrapNodesForTests([]string{})

	var memberships []NodeMembership
	defer func() {
		for _, membership := range memberships {
			if membership != nil {
				err := membership.Stop()
				assert.NoError(t, err)
			}
		}
	}()

	// Start first node
	cfg1 := createTestConfig("node1", 7950, 8090)
	logger1 := createTestLogger()
	membership1, err := NewNodeMembershipImpl(cfg1, logger1)
	require.NoError(t, err)
	err = membership1.Start()
	require.NoError(t, err)
	memberships = append(memberships, membership1)

	time.Sleep(200 * time.Millisecond)

	// Start second node
	cfg2 := createTestConfig("node2", 7951, 8091)
	cfg2.ClusterConfig.StaticBootstrapNodeAddrPorts = []string{"127.0.0.1:7950"}
	logger2 := createTestLogger()
	membership2, err := NewNodeMembershipImpl(cfg2, logger2)
	require.NoError(t, err)
	err = membership2.Start()
	require.NoError(t, err)
	memberships = append(memberships, membership2)

	// Wait for two nodes to form cluster
	waitForClusterSize(t, membership1, 2, testTimeout)
	waitForClusterSize(t, membership2, 2, testTimeout)

	// Start third node
	cfg3 := createTestConfig("node3", 7952, 8092)
	cfg3.ClusterConfig.StaticBootstrapNodeAddrPorts = []string{"127.0.0.1:7950", "127.0.0.1:7951"}
	logger3 := createTestLogger()
	membership3, err := NewNodeMembershipImpl(cfg3, logger3)
	require.NoError(t, err)
	err = membership3.Start()
	require.NoError(t, err)
	memberships = append(memberships, membership3)

	// Wait for all nodes to see the full cluster
	waitForClusterSize(t, membership1, 3, testTimeout)
	waitForClusterSize(t, membership2, 3, testTimeout)
	waitForClusterSize(t, membership3, 3, testTimeout)

	// Verify all nodes see the complete cluster
	for i, membership := range memberships {
		nodes, err := membership.GetAllNodes()
		require.NoError(t, err)
		assert.Len(t, nodes, 3, "Node %d should see 3 nodes", i+1)

		// Verify it sees all three nodes
		assert.True(t, hasNodeWithName(nodes, "node1", i == 0))
		assert.True(t, hasNodeWithName(nodes, "node2", i == 1))
		assert.True(t, hasNodeWithName(nodes, "node3", i == 2))
	}
}

func TestNodeLeaveCluster(t *testing.T) {
	// Clean up any previous test state
	setLocalOverrideForBootstrapNodesForTests([]string{})
	defer setLocalOverrideForBootstrapNodesForTests([]string{})

	// Start first node
	cfg1 := createTestConfig("node1", 7960, 8100)
	logger1 := createTestLogger()
	membership1, err := NewNodeMembershipImpl(cfg1, logger1)
	require.NoError(t, err)
	err = membership1.Start()
	require.NoError(t, err)
	defer func() {
		err := membership1.Stop()
		require.NoError(t, err)
	}()

	time.Sleep(200 * time.Millisecond)

	// Start second node
	cfg2 := createTestConfig("node2", 7961, 8101)
	cfg2.ClusterConfig.StaticBootstrapNodeAddrPorts = []string{"127.0.0.1:7960"}
	logger2 := createTestLogger()
	membership2, err := NewNodeMembershipImpl(cfg2, logger2)
	require.NoError(t, err)
	err = membership2.Start()
	require.NoError(t, err)

	// Wait for cluster formation
	waitForClusterSize(t, membership1, 2, testTimeout)
	waitForClusterSize(t, membership2, 2, testTimeout)

	// Stop second node
	err = membership2.Stop()
	require.NoError(t, err)

	// Wait for first node to detect the leave
	waitForClusterSize(t, membership1, 1, testTimeout)

	// Verify first node only sees itself
	nodes, err := membership1.GetAllNodes()
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.True(t, hasNodeWithName(nodes, "node1", true))
}

func TestConcurrentNodeStartup(t *testing.T) {
	// Clean up any previous test state
	setLocalOverrideForBootstrapNodesForTests([]string{})
	defer setLocalOverrideForBootstrapNodesForTests([]string{})

	const numNodes = 5
	var memberships []NodeMembership
	var wg sync.WaitGroup

	defer func() {
		for _, membership := range memberships {
			if membership != nil {
				err := membership.Stop()
				assert.NoError(t, err)
			}
		}
	}()

	// Start first node as seed
	cfg1 := createTestConfig("seed", 7970, 8110)
	logger1 := createTestLogger()
	membership1, err := NewNodeMembershipImpl(cfg1, logger1)
	require.NoError(t, err)
	err = membership1.Start()
	require.NoError(t, err)
	memberships = append(memberships, membership1)

	time.Sleep(200 * time.Millisecond)

	// Start remaining nodes concurrently
	membershipsChan := make(chan NodeMembership, numNodes-1)
	errorsChan := make(chan error, numNodes-1)

	for i := 1; i < numNodes; i++ {
		wg.Add(1)
		go func(nodeIndex int) {
			defer wg.Done()

			cfg := createTestConfig(fmt.Sprintf("node%d", nodeIndex), 7970+nodeIndex, 8110+nodeIndex)
			cfg.ClusterConfig.StaticBootstrapNodeAddrPorts = []string{"127.0.0.1:7970"}
			logger := createTestLogger()

			membership, err := NewNodeMembershipImpl(cfg, logger)
			if err != nil {
				errorsChan <- err
				return
			}

			err = membership.Start()
			if err != nil {
				errorsChan <- err
				return
			}

			membershipsChan <- membership
		}(i)
	}

	wg.Wait()
	close(membershipsChan)
	close(errorsChan)

	// Check for errors
	for err := range errorsChan {
		if err != nil {
			t.Fatalf("Error during concurrent startup: %v", err)
		}
	}

	// Collect all memberships
	for membership := range membershipsChan {
		memberships = append(memberships, membership)
	}

	require.Len(t, memberships, numNodes)

	// Wait for all nodes to see the full cluster
	for i, membership := range memberships {
		waitForClusterSize(t, membership, numNodes, testTimeout)

		nodes, err := membership.GetAllNodes()
		require.NoError(t, err)
		assert.Len(t, nodes, numNodes, "Node %d should see %d nodes", i, numNodes)
	}
}

func TestRefreshMembershipWithBootstrapOverride(t *testing.T) {
	// This test uses setLocalOverrideForBootstrapNodesForTests to simulate
	// the refreshMembership functionality

	// Clean up any previous test state
	setLocalOverrideForBootstrapNodesForTests([]string{})
	defer setLocalOverrideForBootstrapNodesForTests([]string{})

	// Start seed node
	cfg1 := createTestConfig("seed", 7980, 8120)
	cfg1.ClusterConfig.RefreshIntervalSeconds = 1 // Fast refresh for testing
	logger1 := createTestLogger()
	membership1, err := NewNodeMembershipImpl(cfg1, logger1)
	require.NoError(t, err)
	err = membership1.Start()
	require.NoError(t, err)
	defer func() {
		err := membership1.Stop()
		require.NoError(t, err)
	}()

	time.Sleep(200 * time.Millisecond)

	// Start second node
	cfg2 := createTestConfig("node2", 7981, 8121)
	cfg2.ClusterConfig.RefreshIntervalSeconds = 1
	logger2 := createTestLogger()
	membership2, err := NewNodeMembershipImpl(cfg2, logger2)
	require.NoError(t, err)

	// Set bootstrap override to point to seed node
	setLocalOverrideForBootstrapNodesForTests([]string{"127.0.0.1:7980"})

	err = membership2.Start()
	require.NoError(t, err)
	defer func() {
		err := membership2.Stop()
		require.NoError(t, err)
	}()

	// Wait for cluster formation
	waitForClusterSize(t, membership1, 2, testTimeout)
	waitForClusterSize(t, membership2, 2, testTimeout)

	// Start third node
	cfg3 := createTestConfig("node3", 7982, 8122)
	cfg3.ClusterConfig.RefreshIntervalSeconds = 1
	logger3 := createTestLogger()
	membership3, err := NewNodeMembershipImpl(cfg3, logger3)
	require.NoError(t, err)

	// Update bootstrap override to include more nodes
	setLocalOverrideForBootstrapNodesForTests([]string{"127.0.0.1:7980", "127.0.0.1:7981"})

	err = membership3.Start()
	require.NoError(t, err)
	defer func() {
		err := membership3.Stop()
		require.NoError(t, err)
	}()

	// Wait for all nodes to see the full cluster
	waitForClusterSize(t, membership1, 3, testTimeout)
	waitForClusterSize(t, membership2, 3, testTimeout)
	waitForClusterSize(t, membership3, 3, testTimeout)

	// Verify the bootstrap override is working by checking that node3 joined
	nodes1, err := membership1.GetAllNodes()
	require.NoError(t, err)
	assert.True(t, hasNodeWithName(nodes1, "node3", false))
}

func TestMembershipStartError(t *testing.T) {
	// Test with invalid bind address
	cfg := createTestConfig("invalid", 99999, 8080) // Invalid port
	logger := createTestLogger()

	membership, err := NewNodeMembershipImpl(cfg, logger)

	// The creation might succeed but start should fail with invalid port
	if err == nil {
		err = membership.Start()
		assert.Error(t, err)
		if membership != nil {
			membership.Stop()
		}
	} else {
		// Or creation itself might fail with invalid config
		assert.Error(t, err)
	}
}

func TestNilConfig(t *testing.T) {
	logger := createTestLogger()
	membership, err := NewNodeMembershipImpl(nil, logger)
	assert.Error(t, err)
	assert.Nil(t, membership)
	assert.Contains(t, err.Error(), "config cannot be nil")
}

func TestNodeSuddenFailureDetection(t *testing.T) {
	// This test simulates a sudden node failure (crash/network partition) using forceShutdownForTest
	// and verifies that remaining nodes detect the failure after the memberlist failure detection timeout.
	// This is different from TestNodeLeaveCluster which tests graceful departure.

	// Clean up any previous test state
	setLocalOverrideForBootstrapNodesForTests([]string{})
	defer setLocalOverrideForBootstrapNodesForTests([]string{})

	// Start first node
	cfg1 := createTestConfig("node1", 7900, 8050)
	logger1 := createTestLogger()
	membership1, err := NewNodeMembershipImpl(cfg1, logger1)
	require.NoError(t, err)
	err = membership1.Start()
	require.NoError(t, err)
	defer func() {
		err := membership1.Stop()
		assert.NoError(t, err)
	}()

	time.Sleep(200 * time.Millisecond)

	// Start second node that joins the first
	cfg2 := createTestConfig("node2", 7901, 8051)
	cfg2.ClusterConfig.StaticBootstrapNodeAddrPorts = []string{"127.0.0.1:7900"}
	logger2 := createTestLogger()
	membership2, err := NewNodeMembershipImpl(cfg2, logger2)
	require.NoError(t, err)
	err = membership2.Start()
	require.NoError(t, err)

	// Wait for cluster formation
	waitForClusterSize(t, membership1, 2, testTimeout)
	waitForClusterSize(t, membership2, 2, testTimeout)

	// Verify both nodes see each other initially
	nodes1, err := membership1.GetAllNodes()
	require.NoError(t, err)
	assert.Len(t, nodes1, 2)
	assert.True(t, hasNodeWithName(nodes1, "node1", true))
	assert.True(t, hasNodeWithName(nodes1, "node2", false))

	nodes2, err := membership2.GetAllNodes()
	require.NoError(t, err)
	assert.Len(t, nodes2, 2)
	assert.True(t, hasNodeWithName(nodes2, "node1", false))
	assert.True(t, hasNodeWithName(nodes2, "node2", true))

	// Simulate sudden failure of node2 by force shutting down its memberlist
	// This simulates a crash or network partition where the node doesn't gracefully leave
	impl2, ok := membership2.(*MembershipImpl)
	require.True(t, ok, "membership2 should be of type *MembershipImpl")

	err = impl2.forceShutdownForTest()
	require.NoError(t, err)

	// Wait for node1 to detect the failure
	// Memberlist typically detects failures within a few seconds (default probe interval + timeout)
	// We'll wait up to 15 seconds for failure detection
	failureDetectionTimeout := 15 * time.Second

	t.Logf("Waiting up to %v for node1 to detect node2 failure...", failureDetectionTimeout)

	deadline := time.Now().Add(failureDetectionTimeout)
	detected := false

	for time.Now().Before(deadline) {
		nodes, err := membership1.GetAllNodes()
		require.NoError(t, err)

		// Check if node1 only sees itself (failure detected)
		if len(nodes) == 1 && hasNodeWithName(nodes, "node1", true) {
			detected = true
			t.Logf("Failure detected after %v", time.Since(deadline.Add(-failureDetectionTimeout)))
			break
		}

		// Check every 500ms
		time.Sleep(500 * time.Millisecond)
	}

	// Verify that node1 eventually detected the failure
	assert.True(t, detected, "node1 should have detected node2 failure within %v", failureDetectionTimeout)

	// Final verification - node1 should only see itself
	finalNodes, err := membership1.GetAllNodes()
	require.NoError(t, err)
	assert.Len(t, finalNodes, 1, "node1 should only see itself after detecting node2 failure")
	assert.True(t, hasNodeWithName(finalNodes, "node1", true))
	assert.False(t, hasNodeWithName(finalNodes, "node2", false), "node1 should no longer see node2")
}

// Helper functions

func createTestConfig(nodeName string, gossipPort, httpPort int) *config.Config {
	cfg := &config.Config{
		NodeConfig: config.NodeConfig{
			NodeName:                nodeName,
			GossipBindAddrPort:      fmt.Sprintf("127.0.0.1:%d", gossipPort),
			GossipAdvertiseAddrPort: fmt.Sprintf("127.0.0.1:%d", gossipPort),
			HttpBindAddrPort:        fmt.Sprintf("127.0.0.1:%d", httpPort),
		},
		ClusterConfig: config.ClusterConfig{
			BootstrapType:                "static",
			StaticBootstrapNodeAddrPorts: []string{},
			BootstrapTimeoutSeconds:      10,
			RefreshIntervalSeconds:       30,
		},
	}
	return cfg
}

func waitForClusterSize(t *testing.T, membership NodeMembership, expectedSize int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		nodes, err := membership.GetAllNodes()
		require.NoError(t, err)

		if len(nodes) == expectedSize {
			return
		}

		time.Sleep(100 * time.Millisecond)
	}

	// Final check with detailed error
	nodes, err := membership.GetAllNodes()
	require.NoError(t, err)
	assert.Len(t, nodes, expectedSize, "Timed out waiting for cluster size %d, got %d", expectedSize, len(nodes))
}

func hasNodeWithName(nodes []NodeInfo, name string, shouldBeSelf bool) bool {
	for _, node := range nodes {
		if node.Name == name && node.IsSelf == shouldBeSelf {
			return true
		}
	}
	return false
}
