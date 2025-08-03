package integTests

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSingleNodeBasicSendReceive(t *testing.T) {
	cluster := NewTestCluster(t)
	defer cluster.StopAll()

	// Start single node
	_, err := cluster.StartSingleNode("test-node-1", 18001, 17001)
	require.NoError(t, err)

	// Test basic send/receive flow
	streamId := "test-stream-basic"
	message := map[string]interface{}{
		"type":      "greeting",
		"content":   "Hello, iPubSub!",
		"timestamp": time.Now().Unix(),
	}

	// Send message
	sendReq := CreateSendRequest(streamId, message)
	err = cluster.SendMessage("test-node-1", sendReq)
	require.NoError(t, err)

	// Receive message
	resp, err := cluster.ReceiveMessage("test-node-1", streamId, 5)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, sendReq.MessageUuid, *resp.MessageUuid)
	AssertMessageEquals(t, message, resp.Message)
}

func TestSingleNodeCircularBufferMode(t *testing.T) {
	cluster := NewTestCluster(t)
	defer cluster.StopAll()

	_, err := cluster.StartSingleNode("test-node-1", 18002, 17002)
	require.NoError(t, err)

	streamId := "test-stream-circular"
	bufferSize := int32(3)

	// Send messages to fill the buffer
	var sentMessages []interface{}
	var sentUUIDs []string

	for i := 0; i < 5; i++ { // Send more than buffer size
		message := map[string]interface{}{
			"id":      i,
			"content": fmt.Sprintf("Message %d", i),
		}

		sendReq := CreateSendRequest(streamId, message,
			WithInMemoryStreamSize(bufferSize),
			WithWriteToDB(false))

		sentMessages = append(sentMessages, message)
		sentUUIDs = append(sentUUIDs, sendReq.MessageUuid)

		err = cluster.SendMessage("test-node-1", sendReq)
		require.NoError(t, err)

		// Small delay to ensure ordering
		time.Sleep(10 * time.Millisecond)
	}

	// Receive messages - should only get the last 3 due to circular buffer
	var receivedMessages []interface{}
	var receivedUUIDs []string

	for i := 0; i < 3; i++ {
		resp, err := cluster.ReceiveMessage("test-node-1", streamId, 2)
		require.NoError(t, err)

		receivedMessages = append(receivedMessages, resp.Message)
		receivedUUIDs = append(receivedUUIDs, *resp.MessageUuid)
	}

	// Should have received messages 2, 3, 4 (the last 3)
	for i := 0; i < 3; i++ {
		expectedIndex := i + 2 // Messages at index 2, 3, 4
		AssertMessageEquals(t, sentMessages[expectedIndex], receivedMessages[i])
		assert.Equal(t, sentUUIDs[expectedIndex], receivedUUIDs[i])
	}

	// Next receive should timeout as buffer is empty
	resp, err := cluster.ReceiveMessage("test-node-1", streamId, 1)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestSingleNodeBlockingQueueMode(t *testing.T) {
	cluster := NewTestCluster(t)
	defer cluster.StopAll()

	_, err := cluster.StartSingleNode("test-node-1", 18003, 17003)
	require.NoError(t, err)

	streamId := "test-stream-blocking"
	bufferSize := int32(2)
	blockingTimeout := int32(3)

	// Fill the buffer
	for i := 0; i < 2; i++ {
		message := map[string]interface{}{"id": i}
		sendReq := CreateSendRequest(streamId, message,
			WithInMemoryStreamSize(bufferSize),
			WithBlockingSendTimeout(blockingTimeout),
			WithWriteToDB(false))

		err = cluster.SendMessage("test-node-1", sendReq)
		require.NoError(t, err)
	}

	// Try to send another message - should timeout due to full buffer
	message := map[string]interface{}{"id": 999}
	sendReq := CreateSendRequest(streamId, message,
		WithInMemoryStreamSize(bufferSize),
		WithBlockingSendTimeout(1), // Short timeout
		WithWriteToDB(false))

	err = cluster.SendMessage("test-node-1", sendReq)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "424") // Failed Dependency
}

func TestSingleNodeSyncMatchQueue(t *testing.T) {
	cluster := NewTestCluster(t)
	defer cluster.StopAll()

	_, err := cluster.StartSingleNode("test-node-1", 18004, 17004)
	require.NoError(t, err)

	streamId := "test-stream-sync"

	// Start a receiver in a goroutine
	var wg sync.WaitGroup
	var receivedMessage interface{}
	var receiveErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(100 * time.Millisecond) // Small delay to ensure sender waits
		resp, err := cluster.ReceiveMessage("test-node-1", streamId, 5)
		receiveErr = err
		if resp != nil {
			receivedMessage = resp.Message
		}
	}()

	// Send message with zero buffer size (sync match queue)
	message := map[string]interface{}{
		"type": "sync-message",
		"data": "immediate delivery required",
	}

	sendReq := CreateSendRequest(streamId, message,
		WithInMemoryStreamSize(0), // Zero buffer = sync match queue
		WithBlockingSendTimeout(3),
		WithWriteToDB(false))

	start := time.Now()
	err = cluster.SendMessage("test-node-1", sendReq)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.True(t, duration > 80*time.Millisecond) // Should have waited for receiver

	wg.Wait()
	require.NoError(t, receiveErr)
	AssertMessageEquals(t, message, receivedMessage)
}

func TestMultiNodeCluster(t *testing.T) {
	cluster := NewTestCluster(t)
	defer cluster.StopAll()

	// Start three nodes - node 1 is the bootstrap node
	_, err := cluster.StartNode("test-node-1", 18005, 17005, []int{17005})
	require.NoError(t, err)

	time.Sleep(1 * time.Second) // Give first node time to start

	_, err = cluster.StartNode("test-node-2", 18006, 17006, []int{17005})
	require.NoError(t, err)

	_, err = cluster.StartNode("test-node-3", 18007, 17007, []int{17005})
	require.NoError(t, err)

	// Wait for cluster to stabilize
	err = cluster.WaitForClusterStable(3, 10*time.Second)
	require.NoError(t, err)

	// Test message routing - send to one node, receive from another
	streamId := "test-stream-multinode"
	message := map[string]interface{}{
		"route":   "cross-node",
		"payload": "distributed message",
	}

	// Send message to node 1
	sendReq := CreateSendRequest(streamId, message)
	err = cluster.SendMessage("test-node-1", sendReq)
	require.NoError(t, err)

	// Try to receive from all nodes to see which one gets it
	// Due to consistent hashing, the message should route to a specific node
	var successfulReceives int
	var lastSuccessfulMessage interface{}

	for _, nodeName := range []string{"test-node-1", "test-node-2", "test-node-3"} {
		resp, err := cluster.ReceiveMessage(nodeName, streamId, 2)
		if err == nil && resp != nil {
			successfulReceives++
			lastSuccessfulMessage = resp.Message
		}
	}

	// Should receive from exactly one node (the one responsible for this stream)
	assert.Equal(t, 1, successfulReceives)
	AssertMessageEquals(t, message, lastSuccessfulMessage)
}

func TestConcurrentSendReceive(t *testing.T) {
	cluster := NewTestCluster(t)
	defer cluster.StopAll()

	_, err := cluster.StartSingleNode("test-node-1", 18008, 17008)
	require.NoError(t, err)

	streamId := "test-stream-concurrent"
	numMessages := 20
	bufferSize := int32(10)

	// Start receivers
	var wg sync.WaitGroup
	receivedMessages := make([]interface{}, 0, numMessages)
	var receiveMutex sync.Mutex

	// Start multiple receivers
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(receiverID int) {
			defer wg.Done()
			for j := 0; j < numMessages/3+2; j++ { // Receive more than expected to handle distribution
				resp, err := cluster.ReceiveMessage("test-node-1", streamId, 5)
				if err == nil && resp != nil {
					receiveMutex.Lock()
					receivedMessages = append(receivedMessages, resp.Message)
					receiveMutex.Unlock()
				} else {
					break // No more messages
				}
			}
		}(i)
	}

	// Send messages concurrently
	var sendWg sync.WaitGroup
	for i := 0; i < numMessages; i++ {
		sendWg.Add(1)
		go func(msgID int) {
			defer sendWg.Done()
			message := map[string]interface{}{
				"id":        msgID,
				"sender":    "concurrent-test",
				"timestamp": time.Now().UnixNano(),
			}

			sendReq := CreateSendRequest(streamId, message,
				WithInMemoryStreamSize(bufferSize),
				WithWriteToDB(false))

			err := cluster.SendMessage("test-node-1", sendReq)
			assert.NoError(t, err)
		}(i)
	}

	sendWg.Wait() // Wait for all sends to complete

	// Give receivers time to process
	time.Sleep(2 * time.Second)

	wg.Wait() // Wait for all receives to complete

	// Verify we received the expected number of messages
	receiveMutex.Lock()
	assert.Equal(t, numMessages, len(receivedMessages))
	receiveMutex.Unlock()
}

func TestMessageWithDifferentTypes(t *testing.T) {
	cluster := NewTestCluster(t)
	defer cluster.StopAll()

	_, err := cluster.StartSingleNode("test-node-1", 18009, 17009)
	require.NoError(t, err)

	streamId := "test-stream-types"

	// Test different message types
	testCases := []struct {
		name    string
		message interface{}
	}{
		{
			name:    "string_message",
			message: "Hello World",
		},
		{
			name:    "number_message",
			message: 42,
		},
		{
			name:    "boolean_message",
			message: true,
		},
		{
			name:    "array_message",
			message: []interface{}{"item1", "item2", 123, true},
		},
		{
			name: "complex_object",
			message: map[string]interface{}{
				"user": map[string]interface{}{
					"id":   123,
					"name": "John Doe",
					"tags": []string{"admin", "user"},
				},
				"metadata": map[string]interface{}{
					"timestamp": time.Now().Unix(),
					"version":   "1.0",
				},
			},
		},
		{
			name:    "null_message",
			message: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Send message
			sendReq := CreateSendRequest(streamId+"-"+tc.name, tc.message)
			err := cluster.SendMessage("test-node-1", sendReq)
			require.NoError(t, err)

			// Receive message
			resp, err := cluster.ReceiveMessage("test-node-1", streamId+"-"+tc.name, 5)
			require.NoError(t, err)
			require.NotNil(t, resp)

			AssertMessageEquals(t, tc.message, resp.Message)
		})
	}
}

func TestHealthEndpoint(t *testing.T) {
	cluster := NewTestCluster(t)
	defer cluster.StopAll()

	node, err := cluster.StartSingleNode("test-node-1", 18010, 17010)
	require.NoError(t, err)

	// Test health endpoint
	healthURL := fmt.Sprintf("http://%s/health", node.HTTPAddr)
	resp, err := node.Client.Get(healthURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
}

func TestRootEndpoint(t *testing.T) {
	cluster := NewTestCluster(t)
	defer cluster.StopAll()

	node, err := cluster.StartSingleNode("test-node-1", 18011, 17011)
	require.NoError(t, err)

	// Test root endpoint
	rootURL := fmt.Sprintf("http://%s/", node.HTTPAddr)
	resp, err := node.Client.Get(rootURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	// Response should contain iPubSub service identification
	body := make([]byte, 1024)
	n, _ := resp.Body.Read(body)
	responseText := string(body[:n])
	assert.Contains(t, responseText, "iPubSub")
}

func TestErrorScenarios(t *testing.T) {
	cluster := NewTestCluster(t)
	defer cluster.StopAll()

	node, err := cluster.StartSingleNode("test-node-1", 18012, 17012)
	require.NoError(t, err)

	t.Run("receive_from_nonexistent_stream", func(t *testing.T) {
		// Try to receive from a stream that doesn't exist
		resp, err := cluster.ReceiveMessage("test-node-1", "nonexistent-stream", 1)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("send_with_invalid_json", func(t *testing.T) {
		// This test would require direct HTTP calls with malformed JSON
		// The test utility handles JSON marshaling, so we'll test via direct HTTP call

		url := fmt.Sprintf("http://%s/api/v1/streams/send", node.HTTPAddr)
		resp, err := node.Client.Post(url, "application/json", nil) // No body
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 400, resp.StatusCode) // Bad Request
	})
}

// Benchmark test for performance validation
func BenchmarkSendReceive(b *testing.B) {
	cluster := NewTestCluster(&testing.T{})
	defer cluster.StopAll()

	_, err := cluster.StartSingleNode("test-node-1", 18013, 17013)
	if err != nil {
		b.Fatalf("Failed to start node: %v", err)
	}

	streamId := "benchmark-stream"
	message := map[string]interface{}{
		"benchmark": true,
		"data":      "test message for benchmarking",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Send message
			sendReq := CreateSendRequest(streamId, message)
			err := cluster.SendMessage("test-node-1", sendReq)
			if err != nil {
				b.Errorf("Send failed: %v", err)
				continue
			}

			// Receive message
			_, err = cluster.ReceiveMessage("test-node-1", streamId, 5)
			if err != nil {
				b.Errorf("Receive failed: %v", err)
			}
		}
	})
}
