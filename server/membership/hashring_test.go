package membership

import (
	"fmt"
	"testing"

	"github.com/iworkflowio/async-output-service/service/log/loggerimpl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashring_SingleNode(t *testing.T) {
	logger, err := loggerimpl.NewDevelopment()
	require.NoError(t, err)

	hashring := NewHashring(logger, 100)

	nodes := []NodeInfo{
		{Name: "node1", Addr: "127.0.0.1", Port: 8080, IsSelf: true},
	}

	// Test single node always returns the same node
	for i := 0; i < 10; i++ {
		streamId := fmt.Sprintf("stream-%d", i)
		node, err := hashring.GetNodeForStreamId(streamId, 1, nodes)
		require.NoError(t, err)
		assert.Equal(t, "node1", node.Name)
	}
}

func TestHashring_MultipleNodes(t *testing.T) {
	logger, err := loggerimpl.NewDevelopment()
	require.NoError(t, err)

	hashring := NewHashring(logger, 100)

	nodes := []NodeInfo{
		{Name: "node1", Addr: "127.0.0.1", Port: 8080, IsSelf: true},
		{Name: "node2", Addr: "127.0.0.1", Port: 8081, IsSelf: false},
		{Name: "node3", Addr: "127.0.0.1", Port: 8082, IsSelf: false},
	}

	// Test that different streamIds can map to different nodes
	nodeDistribution := make(map[string]int)
	numStreams := 300

	for i := 0; i < numStreams; i++ {
		streamId := fmt.Sprintf("stream-%d", i)
		node, err := hashring.GetNodeForStreamId(streamId, 1, nodes)
		require.NoError(t, err)
		nodeDistribution[node.Name]++
	}

	// Verify all nodes got at least some streams (with high virtual nodes, distribution should be good)
	assert.True(t, nodeDistribution["node1"] > 0, "node1 should handle some streams")
	assert.True(t, nodeDistribution["node2"] > 0, "node2 should handle some streams")
	assert.True(t, nodeDistribution["node3"] > 0, "node3 should handle some streams")

	// Verify total adds up
	total := nodeDistribution["node1"] + nodeDistribution["node2"] + nodeDistribution["node3"]
	assert.Equal(t, numStreams, total)

	t.Logf("Distribution: node1=%d, node2=%d, node3=%d",
		nodeDistribution["node1"], nodeDistribution["node2"], nodeDistribution["node3"])
}

func TestHashring_Consistency(t *testing.T) {
	logger, err := loggerimpl.NewDevelopment()
	require.NoError(t, err)

	hashring := NewHashring(logger, 100)

	nodes := []NodeInfo{
		{Name: "node1", Addr: "127.0.0.1", Port: 8080, IsSelf: true},
		{Name: "node2", Addr: "127.0.0.1", Port: 8081, IsSelf: false},
	}

	streamIds := []string{"stream-a", "stream-b", "stream-c", "stream-d", "stream-e"}

	// Get initial mappings
	initialMappings := make(map[string]string)
	for _, streamId := range streamIds {
		node, err := hashring.GetNodeForStreamId(streamId, 1, nodes)
		require.NoError(t, err)
		initialMappings[streamId] = node.Name
	}

	// Call again with same version - should get same results
	for _, streamId := range streamIds {
		node, err := hashring.GetNodeForStreamId(streamId, 1, nodes)
		require.NoError(t, err)
		assert.Equal(t, initialMappings[streamId], node.Name,
			"Same streamId should map to same node with same membership version")
	}

	// Call with same membership version but reordered nodes - should still be consistent
	reorderedNodes := []NodeInfo{
		{Name: "node2", Addr: "127.0.0.1", Port: 8081, IsSelf: false},
		{Name: "node1", Addr: "127.0.0.1", Port: 8080, IsSelf: true},
	}

	for _, streamId := range streamIds {
		node, err := hashring.GetNodeForStreamId(streamId, 1, reorderedNodes)
		require.NoError(t, err)
		assert.Equal(t, initialMappings[streamId], node.Name,
			"Node order should not affect consistent hashing")
	}
}

func TestHashring_MembershipVersionUpdate(t *testing.T) {
	logger, err := loggerimpl.NewDevelopment()
	require.NoError(t, err)

	hashring := NewHashring(logger, 100)

	// Start with 2 nodes
	nodes := []NodeInfo{
		{Name: "node1", Addr: "127.0.0.1", Port: 8080, IsSelf: true},
		{Name: "node2", Addr: "127.0.0.1", Port: 8081, IsSelf: false},
	}

	streamId := "test-stream"

	// Get mapping with version 1
	node1, err := hashring.GetNodeForStreamId(streamId, 1, nodes)
	require.NoError(t, err)

	// Call again with same version - should use cached ring
	node2, err := hashring.GetNodeForStreamId(streamId, 1, nodes)
	require.NoError(t, err)
	assert.Equal(t, node1.Name, node2.Name)

	// Add a new node with higher version
	newNodes := []NodeInfo{
		{Name: "node1", Addr: "127.0.0.1", Port: 8080, IsSelf: true},
		{Name: "node2", Addr: "127.0.0.1", Port: 8081, IsSelf: false},
		{Name: "node3", Addr: "127.0.0.1", Port: 8082, IsSelf: false},
	}

	// This should rebuild the ring
	node3, err := hashring.GetNodeForStreamId(streamId, 2, newNodes)
	require.NoError(t, err)

	// The node might change due to the new ring structure, but it should be one of the valid nodes
	assert.Contains(t, []string{"node1", "node2", "node3"}, node3.Name)
}

func TestHashring_NoNodes(t *testing.T) {
	logger, err := loggerimpl.NewDevelopment()
	require.NoError(t, err)

	hashring := NewHashring(logger, 100)

	// Test with empty node list
	nodes := []NodeInfo{}

	_, err = hashring.GetNodeForStreamId("test-stream", 1, nodes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no nodes available")
}

func TestHashring_LoadDistribution(t *testing.T) {
	logger, err := loggerimpl.NewDevelopment()
	require.NoError(t, err)

	hashring := NewHashring(logger, 150) // Higher virtual nodes for better distribution

	nodes := []NodeInfo{
		{Name: "node1", Addr: "127.0.0.1", Port: 8080, IsSelf: true},
		{Name: "node2", Addr: "127.0.0.1", Port: 8081, IsSelf: false},
		{Name: "node3", Addr: "127.0.0.1", Port: 8082, IsSelf: false},
		{Name: "node4", Addr: "127.0.0.1", Port: 8083, IsSelf: false},
	}

	nodeDistribution := make(map[string]int)
	numStreams := 1000

	for i := 0; i < numStreams; i++ {
		streamId := fmt.Sprintf("stream-%d", i)
		node, err := hashring.GetNodeForStreamId(streamId, 1, nodes)
		require.NoError(t, err)
		nodeDistribution[node.Name]++
	}

	// With good virtual node count, each node should get roughly 25% of streams
	expectedPerNode := numStreams / len(nodes)
	tolerance := expectedPerNode / 2 // Allow 50% deviation

	for _, nodeName := range []string{"node1", "node2", "node3", "node4"} {
		count := nodeDistribution[nodeName]
		assert.True(t, count >= expectedPerNode-tolerance && count <= expectedPerNode+tolerance,
			"Node %s got %d streams, expected around %d±%d", nodeName, count, expectedPerNode, tolerance)
	}

	t.Logf("Distribution: node1=%d, node2=%d, node3=%d, node4=%d",
		nodeDistribution["node1"], nodeDistribution["node2"],
		nodeDistribution["node3"], nodeDistribution["node4"])
}

func TestHashring_VirtualNodeCount(t *testing.T) {
	logger, err := loggerimpl.NewDevelopment()
	require.NoError(t, err)

	nodes := []NodeInfo{
		{Name: "node1", Addr: "127.0.0.1", Port: 8080, IsSelf: true},
		{Name: "node2", Addr: "127.0.0.1", Port: 8081, IsSelf: false},
	}

	// Test with different virtual node counts
	for _, virtualNodes := range []int{1, 10, 50, 100} {
		t.Run(fmt.Sprintf("VirtualNodes_%d", virtualNodes), func(t *testing.T) {
			hashring := NewHashring(logger, virtualNodes)

			// Should work with any virtual node count
			node, err := hashring.GetNodeForStreamId("test-stream", 1, nodes)
			require.NoError(t, err)
			assert.Contains(t, []string{"node1", "node2"}, node.Name)
		})
	}
}

func TestHashring_ConcurrentAccess(t *testing.T) {
	logger, err := loggerimpl.NewDevelopment()
	require.NoError(t, err)

	hashring := NewHashring(logger, 100)

	nodes := []NodeInfo{
		{Name: "node1", Addr: "127.0.0.1", Port: 8080, IsSelf: true},
		{Name: "node2", Addr: "127.0.0.1", Port: 8081, IsSelf: false},
	}

	// Test concurrent access to the hashring
	numGoroutines := 10
	numRequests := 100
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineId int) {
			for j := 0; j < numRequests; j++ {
				streamId := fmt.Sprintf("stream-%d-%d", goroutineId, j)
				_, err := hashring.GetNodeForStreamId(streamId, 1, nodes)
				if err != nil {
					results <- err
					return
				}
			}
			results <- nil
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		err := <-results
		require.NoError(t, err)
	}
}
