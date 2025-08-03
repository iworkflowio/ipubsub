package membership

import (
	"testing"

	"github.com/iworkflowio/ipubsub/service/log/loggerimpl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashringIntegrationWithMembership(t *testing.T) {
	logger, err := loggerimpl.NewDevelopment()
	require.NoError(t, err)

	// Create a hashring
	hashring := NewHashring(logger, 100)

	// Create two membership nodes
	nodes := []NodeInfo{
		{Name: "node1", Addr: "127.0.0.1", Port: 8080, IsSelf: true},
		{Name: "node2", Addr: "127.0.0.1", Port: 8081, IsSelf: false},
	}

	// Test consistent routing
	streamIds := []string{
		"user-123-session",
		"order-456-processing",
		"payment-789-validation",
		"notification-999-delivery",
		"analytics-111-batch",
	}

	// Get initial routing
	routingMap := make(map[string]string)
	for _, streamId := range streamIds {
		node, err := hashring.GetNodeForStreamId(streamId, 1, nodes)
		require.NoError(t, err)
		routingMap[streamId] = node.Name
		t.Logf("Stream %s -> %s", streamId, node.Name)
	}

	// Verify routing is consistent across multiple calls
	for i := 0; i < 3; i++ {
		for _, streamId := range streamIds {
			node, err := hashring.GetNodeForStreamId(streamId, 1, nodes)
			require.NoError(t, err)
			assert.Equal(t, routingMap[streamId], node.Name,
				"Stream %s should consistently route to %s", streamId, routingMap[streamId])
		}
	}

	// Test what happens when a node is added (membership change)
	expandedNodes := []NodeInfo{
		{Name: "node1", Addr: "127.0.0.1", Port: 8080, IsSelf: true},
		{Name: "node2", Addr: "127.0.0.1", Port: 8081, IsSelf: false},
		{Name: "node3", Addr: "127.0.0.1", Port: 8082, IsSelf: false},
	}

	// Some streams might move to the new node
	newRoutingMap := make(map[string]string)
	movedCount := 0
	for _, streamId := range streamIds {
		node, err := hashring.GetNodeForStreamId(streamId, 2, expandedNodes)
		require.NoError(t, err)
		newRoutingMap[streamId] = node.Name

		if routingMap[streamId] != newRoutingMap[streamId] {
			movedCount++
			t.Logf("Stream %s moved from %s to %s", streamId, routingMap[streamId], newRoutingMap[streamId])
		}
	}

	// Verify that the new node gets some streams (with good hash distribution)
	node3Streams := 0
	for _, nodeName := range newRoutingMap {
		if nodeName == "node3" {
			node3Streams++
		}
	}

	assert.True(t, node3Streams > 0, "New node should receive some streams")
	t.Logf("Node3 received %d out of %d streams", node3Streams, len(streamIds))
	t.Logf("Moved streams: %d out of %d", movedCount, len(streamIds))

	// Test node removal
	reducedNodes := []NodeInfo{
		{Name: "node1", Addr: "127.0.0.1", Port: 8080, IsSelf: true},
		{Name: "node3", Addr: "127.0.0.1", Port: 8082, IsSelf: false},
	}

	// Streams from removed node2 should be redistributed
	finalRoutingMap := make(map[string]string)
	for _, streamId := range streamIds {
		node, err := hashring.GetNodeForStreamId(streamId, 3, reducedNodes)
		require.NoError(t, err)
		finalRoutingMap[streamId] = node.Name

		// Should only route to remaining nodes
		assert.Contains(t, []string{"node1", "node3"}, node.Name)
	}

	t.Logf("Final routing after node2 removal:")
	for _, streamId := range streamIds {
		t.Logf("Stream %s -> %s", streamId, finalRoutingMap[streamId])
	}
}

func TestHashringWithMembershipVersionTracking(t *testing.T) {
	logger, err := loggerimpl.NewDevelopment()
	require.NoError(t, err)

	hashring := NewHashring(logger, 50)

	// Simulate membership changes over time
	versions := []struct {
		version int64
		nodes   []NodeInfo
	}{
		{
			version: 1,
			nodes: []NodeInfo{
				{Name: "node1", Addr: "127.0.0.1", Port: 8080, IsSelf: true},
			},
		},
		{
			version: 2,
			nodes: []NodeInfo{
				{Name: "node1", Addr: "127.0.0.1", Port: 8080, IsSelf: true},
				{Name: "node2", Addr: "127.0.0.1", Port: 8081, IsSelf: false},
			},
		},
		{
			version: 3,
			nodes: []NodeInfo{
				{Name: "node1", Addr: "127.0.0.1", Port: 8080, IsSelf: true},
				{Name: "node2", Addr: "127.0.0.1", Port: 8081, IsSelf: false},
				{Name: "node3", Addr: "127.0.0.1", Port: 8082, IsSelf: false},
			},
		},
	}

	streamId := "test-stream-consistency"

	for i, version := range versions {
		node, err := hashring.GetNodeForStreamId(streamId, version.version, version.nodes)
		require.NoError(t, err)

		t.Logf("Version %d: Stream %s -> %s", version.version, streamId, node.Name)

		if i == 0 {
			assert.Equal(t, "node1", node.Name, "Single node should handle all streams")
		} else {
			// Node might change as cluster topology changes
			assert.Contains(t, getNodeNames(version.nodes), node.Name,
				"Should route to a valid node in the cluster")
		}
	}

	// Test that older version doesn't change the ring
	sameNode, err := hashring.GetNodeForStreamId(streamId, 2, versions[1].nodes)
	require.NoError(t, err)

	// Should use cached ring from version 3, not rebuild for version 2
	node3, err := hashring.GetNodeForStreamId(streamId, 3, versions[2].nodes)
	require.NoError(t, err)
	assert.Equal(t, node3.Name, sameNode.Name,
		"Lower version should not trigger ring rebuild")
}

func getNodeNames(nodes []NodeInfo) []string {
	names := make([]string, len(nodes))
	for i, node := range nodes {
		names[i] = node.Name
	}
	return names
}
