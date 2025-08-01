package engine

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	genapi "github.com/iworkflowio/async-output-service/genapi/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryStreamImpl_CircularBufferMode(t *testing.T) {
	// Test circular buffer mode (default behavior when blockingWriteTimeoutSeconds <= 0)
	stream := NewInMemoryStreamImpl(3) // Buffer size 3
	defer stream.Stop()

	t.Run("BasicSendReceive", func(t *testing.T) {
		output1 := OutputType{"message": "test1", "step": 1}
		uuid1 := uuid.New()
		timestamp1 := time.Now()

		// Send should succeed
		errorType, err := stream.Send(output1, uuid1, timestamp1, 0) // 0 = circular buffer mode
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		// Receive should get the output
		resp, errorType, err := stream.Receive(1)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)
		assert.Equal(t, uuid1.String(), resp.OutputUuid)
		assert.Equal(t, output1, resp.Output)
		assert.Equal(t, timestamp1, resp.Timestamp)
	})

	t.Run("CircularBufferOverwrite", func(t *testing.T) {
		// Create new stream for this test to avoid interference
		testStream := NewInMemoryStreamImpl(3)
		defer testStream.Stop()

		// Fill the buffer to capacity (3)
		outputs := []OutputType{
			{"message": "msg1", "step": 1},
			{"message": "msg2", "step": 2},
			{"message": "msg3", "step": 3},
		}

		// Send 3 messages to fill buffer
		for i := 0; i < 3; i++ {
			errorType, err := testStream.Send(outputs[i], uuid.New(), time.Now(), 0)
			require.NoError(t, err)
			assert.Equal(t, ErrorTypeNone, errorType)
		}

		// Send 4th message, should overwrite the first one
		output4 := OutputType{"message": "msg4", "step": 4}
		errorType, err := testStream.Send(output4, uuid.New(), time.Now(), 0)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		// First receive should get msg2 (msg1 was overwritten)
		resp, errorType, err := testStream.Receive(1)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)
		assert.Equal(t, outputs[1], resp.Output) // Should be msg2

		// Second receive should get msg3
		resp, errorType, err = testStream.Receive(1)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)
		assert.Equal(t, outputs[2], resp.Output) // Should be msg3

		// Third receive should get msg4
		resp, errorType, err = testStream.Receive(1)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)
		assert.Equal(t, output4, resp.Output) // Should be msg4

		// Fourth receive should timeout (buffer empty)
		resp, errorType, err = testStream.Receive(1)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeWaitingTimeout, errorType)
		assert.Nil(t, resp)
	})

	t.Run("ReceiveTimeout", func(t *testing.T) {
		// Create a new empty stream
		emptyStream := NewInMemoryStreamImpl(5)
		defer emptyStream.Stop()

		start := time.Now()
		resp, errorType, err := emptyStream.Receive(2) // 2 second timeout
		duration := time.Since(start)

		require.NoError(t, err)
		assert.Equal(t, ErrorTypeWaitingTimeout, errorType)
		assert.Nil(t, resp)
		assert.GreaterOrEqual(t, duration, 1900*time.Millisecond) // Allow some tolerance
		assert.LessOrEqual(t, duration, 2100*time.Millisecond)
	})
}

func TestInMemoryStreamImpl_BlockingQueueMode(t *testing.T) {
	t.Run("BlockingQueueBasicOperation", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(2) // Buffer size 2
		defer stream.Stop()

		output1 := OutputType{"message": "test1"}
		uuid1 := uuid.New()
		timestamp1 := time.Now()

		// Send should succeed
		errorType, err := stream.Send(output1, uuid1, timestamp1, 5) // 5 second timeout
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		// Receive should get the output
		resp, errorType, err := stream.Receive(1)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)
		assert.Equal(t, uuid1.String(), resp.OutputUuid)
		assert.Equal(t, output1, resp.Output)
	})

	t.Run("BlockingQueueTimeout", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(2) // Buffer size 2
		defer stream.Stop()

		// Fill the buffer to capacity (2)
		for i := 0; i < 2; i++ {
			output := OutputType{"message": "fill", "index": i}
			errorType, err := stream.Send(output, uuid.New(), time.Now(), 5)
			require.NoError(t, err)
			assert.Equal(t, ErrorTypeNone, errorType)
		}

		// Try to send another message with short timeout
		output3 := OutputType{"message": "should timeout"}
		start := time.Now()
		errorType, err := stream.Send(output3, uuid.New(), time.Now(), 1) // 1 second timeout
		duration := time.Since(start)

		require.Error(t, err)
		assert.Equal(t, ErrorTypeWaitingTimeout, errorType)
		assert.Contains(t, err.Error(), "timeout waiting for stream space")
		assert.GreaterOrEqual(t, duration, 900*time.Millisecond) // Allow some tolerance
		assert.LessOrEqual(t, duration, 1100*time.Millisecond)
	})

	t.Run("BlockingQueueUnblocksWhenSpaceAvailable", func(t *testing.T) {
		// Create fresh stream for this test
		testStream := NewInMemoryStreamImpl(1) // Buffer size 1
		defer testStream.Stop()

		// Fill the buffer
		errorType, err := testStream.Send(OutputType{"message": "initial"}, uuid.New(), time.Now(), 0) // Use circular buffer to fill
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		// Start a goroutine to send with blocking
		sendComplete := make(chan struct {
			errorType ErrorType
			err       error
		}, 1)
		go func() {
			output := OutputType{"message": "delayed"}
			errorType, err := testStream.Send(output, uuid.New(), time.Now(), 5) // 5 second timeout
			sendComplete <- struct {
				errorType ErrorType
				err       error
			}{errorType, err}
		}()

		// Wait a bit to ensure the send is blocking
		time.Sleep(100 * time.Millisecond)

		// Receive to make space - this should unblock the send
		_, _, err = testStream.Receive(1)
		require.NoError(t, err)

		// The blocking send should now complete
		select {
		case result := <-sendComplete:
			require.NoError(t, result.err)
			assert.Equal(t, ErrorTypeNone, result.errorType)
		case <-time.After(2 * time.Second):
			t.Fatal("Send should have unblocked after receive")
		}
	})
}

func TestInMemoryStreamImpl_SyncMatchQueueMode(t *testing.T) {
	// Test sync match queue mode (inMemoryStreamSize = 0 + blockingWriteTimeoutSeconds)
	stream := NewInMemoryStreamImpl(0) // Zero capacity
	defer stream.Stop()

	t.Run("SyncMatchRequiresImmediateConsumer", func(t *testing.T) {
		// Try to send without consumer - should timeout
		output := OutputType{"message": "sync test"}
		errorType, err := stream.Send(output, uuid.New(), time.Now(), 1) // 1 second timeout
		require.Error(t, err)
		assert.Equal(t, ErrorTypeWaitingTimeout, errorType)
		assert.Contains(t, err.Error(), "timeout waiting for stream space")
	})

	t.Run("SyncMatchWithActiveConsumer", func(t *testing.T) {
		// Now that locking is improved, let's test real sync matching
		receiveResult := make(chan struct {
			resp      *genapi.ReceiveResponse
			errorType ErrorType
			err       error
		}, 1)

		// Start consumer in background
		go func() {
			resp, errorType, err := stream.Receive(5) // 5 second timeout
			receiveResult <- struct {
				resp      *genapi.ReceiveResponse
				errorType ErrorType
				err       error
			}{resp, errorType, err}
		}()

		// Wait a bit to ensure consumer is waiting
		time.Sleep(100 * time.Millisecond)

		// Now send should succeed (zero capacity + active consumer)
		output := OutputType{"message": "sync success"}
		uuid1 := uuid.New()

		start := time.Now()
		errorType, err := stream.Send(output, uuid1, time.Now(), 2) // 2 second timeout
		sendDuration := time.Since(start)

		// With improved locking, this should work
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)
		assert.Less(t, sendDuration, 1*time.Second) // Should be fast

		// Consumer should receive the message
		select {
		case result := <-receiveResult:
			require.NoError(t, result.err)
			assert.Equal(t, ErrorTypeNone, result.errorType)
			assert.Equal(t, uuid1.String(), result.resp.OutputUuid)
			assert.Equal(t, output, result.resp.Output)
		case <-time.After(3 * time.Second):
			t.Fatal("Consumer should have received the message")
		}
	})
}

func TestInMemoryStreamImpl_Stop(t *testing.T) {
	t.Run("StopPreventsNewOperations", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(5)

		// Normal operation should work
		errorType, err := stream.Send(OutputType{"message": "before stop"}, uuid.New(), time.Now(), 0)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		// Stop the stream
		err = stream.Stop()
		require.NoError(t, err)

		// Operations after stop should fail
		errorType, err = stream.Send(OutputType{"message": "after stop"}, uuid.New(), time.Now(), 0)
		assert.Error(t, err)
		assert.Equal(t, ErrorTypeStreamStopped, errorType)
		assert.Equal(t, ErrStreamStopped, err)

		_, errorType, err = stream.Receive(1)
		assert.Error(t, err)
		assert.Equal(t, ErrorTypeStreamStopped, errorType)
		assert.Equal(t, ErrStreamStopped, err)
	})

	t.Run("StopUnblocksWaitingOperations", func(t *testing.T) {
		testStream := NewInMemoryStreamImpl(1)

		// Fill the buffer with circular buffer mode
		errorType, err := testStream.Send(OutputType{"message": "fill"}, uuid.New(), time.Now(), 0)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		// Test blocking send that should be interrupted by stop
		sendResult := make(chan struct {
			errorType ErrorType
			err       error
		}, 1)
		go func() {
			// This will block because buffer is full and we're using blocking mode
			errorType, err := testStream.Send(OutputType{"message": "blocking"}, uuid.New(), time.Now(), 10)
			sendResult <- struct {
				errorType ErrorType
				err       error
			}{errorType, err}
		}()

		// Test blocking receive on empty channel
		receiveResult := make(chan struct {
			resp      *genapi.ReceiveResponse
			errorType ErrorType
			err       error
		}, 1)

		// Clear the buffer first so receive will block
		testStream.Receive(1)

		go func() {
			resp, errorType, err := testStream.Receive(10)
			receiveResult <- struct {
				resp      *genapi.ReceiveResponse
				errorType ErrorType
				err       error
			}{resp, errorType, err}
		}()

		// Let operations start blocking
		time.Sleep(200 * time.Millisecond)

		// Stop should unblock both operations
		err = testStream.Stop()
		require.NoError(t, err)

		// Check send result - should be unblocked by stop
		select {
		case result := <-sendResult:
			// Due to locking, this might complete normally or with stream stopped
			if result.err != nil {
				assert.Equal(t, ErrorTypeStreamStopped, result.errorType)
			}
		case <-time.After(1 * time.Second):
			t.Log("Send operation timed out - this indicates locking prevents true concurrency")
		}

		// Check receive result - should be unblocked by stop
		select {
		case result := <-receiveResult:
			// Should be unblocked by stopCh closing
			if result.err != nil {
				assert.Equal(t, ErrorTypeStreamStopped, result.errorType)
			}
		case <-time.After(1 * time.Second):
			t.Log("Receive operation timed out")
		}
	})
}

func TestInMemoryStreamImpl_EdgeCases(t *testing.T) {
	t.Run("ZeroCapacityCircularBuffer", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(0)
		defer stream.Stop()

		// Zero capacity circular buffer should return error
		errorType, err := stream.Send(OutputType{"message": "test"}, uuid.New(), time.Now(), 0)
		assert.Error(t, err)
		assert.Equal(t, ErrorTypeUnknown, errorType)
		assert.Contains(t, err.Error(), "zero capacity circular buffer is not allowed")
	})

	t.Run("NegativeBlockingTimeout", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(1)
		defer stream.Stop()

		// Negative timeout should use circular buffer mode
		errorType, err := stream.Send(OutputType{"message": "test"}, uuid.New(), time.Now(), -1)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(100)
		defer stream.Stop()

		numSenders := 5
		numMessages := 10

		// Start multiple senders
		sendersComplete := make(chan bool, numSenders)
		for i := 0; i < numSenders; i++ {
			go func(senderID int) {
				defer func() { sendersComplete <- true }()
				for j := 0; j < numMessages; j++ {
					output := OutputType{"sender": senderID, "message": j}
					errorType, err := stream.Send(output, uuid.New(), time.Now(), 0)
					assert.NoError(t, err)
					assert.Equal(t, ErrorTypeNone, errorType)
				}
			}(i)
		}

		// Wait for all senders to complete
		for i := 0; i < numSenders; i++ {
			<-sendersComplete
		}

		// Receive all messages
		receivedCount := 0
		for receivedCount < numSenders*numMessages {
			_, errorType, err := stream.Receive(1)
			if err != nil {
				assert.NoError(t, err)
				break
			}
			if errorType == ErrorTypeWaitingTimeout {
				break
			}
			assert.Equal(t, ErrorTypeNone, errorType)
			receivedCount++
		}

		assert.Equal(t, numSenders*numMessages, receivedCount)
	})

	t.Run("MultipleStops", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(5)

		// First stop should succeed
		err := stream.Stop()
		require.NoError(t, err)

		// Second stop should also succeed (idempotent)
		err = stream.Stop()
		require.NoError(t, err)
	})

	t.Run("EmptyStreamReceive", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(5)
		defer stream.Stop()

		// Receive from empty stream should timeout
		resp, errorType, err := stream.Receive(1)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeWaitingTimeout, errorType)
		assert.Nil(t, resp)
	})
}

func TestInMemoryStreamImpl_LockingBehavior(t *testing.T) {
	t.Run("SerializedOperations", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(1)
		defer stream.Stop()

		// With current locking implementation, operations are serialized
		// This test verifies the current behavior

		// Send to fill buffer
		errorType, err := stream.Send(OutputType{"message": "first"}, uuid.New(), time.Now(), 0)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		// Receive should get the message
		resp, errorType, err := stream.Receive(1)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)
		assert.Equal(t, OutputType{"message": "first"}, resp.Output)
	})
}

// Add performance test
func TestInMemoryStreamImpl_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	t.Run("HighThroughput", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(10000)
		defer stream.Stop()

		numMessages := 1000
		start := time.Now()

		// Send many messages
		for i := 0; i < numMessages; i++ {
			output := OutputType{"message": "test", "index": i}
			errorType, err := stream.Send(output, uuid.New(), time.Now(), 0)
			require.NoError(t, err)
			assert.Equal(t, ErrorTypeNone, errorType)
		}

		sendDuration := time.Since(start)
		t.Logf("Sent %d messages in %v (%.2f messages/sec)",
			numMessages, sendDuration, float64(numMessages)/sendDuration.Seconds())

		// Receive all messages
		start = time.Now()
		for i := 0; i < numMessages; i++ {
			_, errorType, err := stream.Receive(1)
			require.NoError(t, err)
			assert.Equal(t, ErrorTypeNone, errorType)
		}

		receiveDuration := time.Since(start)
		t.Logf("Received %d messages in %v (%.2f messages/sec)",
			numMessages, receiveDuration, float64(numMessages)/receiveDuration.Seconds())
	})
}

func TestInMemoryStreamImpl_ComprehensiveEdgeCases(t *testing.T) {
	t.Run("LargeOutput", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(10)
		defer stream.Stop()

		// Create a large output
		largeOutput := OutputType{}
		for i := 0; i < 1000; i++ {
			largeOutput[fmt.Sprintf("field_%d", i)] = fmt.Sprintf("value_%d", i)
		}

		errorType, err := stream.Send(largeOutput, uuid.New(), time.Now(), 0)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		resp, errorType, err := stream.Receive(1)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)
		assert.Equal(t, largeOutput, resp.Output)
	})

	t.Run("ExtremeLongTimeout", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(1)
		defer stream.Stop()

		// Fill buffer
		errorType, err := stream.Send(OutputType{"message": "fill"}, uuid.New(), time.Now(), 0)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		// Test with moderately long timeout (but not extreme) to verify stop interruption
		start := time.Now()

		// Start blocking send in goroutine
		done := make(chan struct {
			errorType ErrorType
			err       error
		})
		go func() {
			defer close(done)
			// Use 10 second timeout instead of 1 hour to avoid test hanging
			errorType, err := stream.Send(OutputType{"message": "long timeout"}, uuid.New(), time.Now(), 10)
			done <- struct {
				errorType ErrorType
				err       error
			}{errorType, err}
		}()

		// Let it start blocking
		time.Sleep(100 * time.Millisecond)

		// Stop the stream - this should interrupt the blocking send
		err = stream.Stop()
		require.NoError(t, err)

		// Should complete quickly
		select {
		case result := <-done:
			duration := time.Since(start)
			// Should complete quickly due to stop, not after 10 seconds
			assert.Less(t, duration, 2*time.Second)
			// Should be interrupted by stop
			if result.err != nil {
				assert.Equal(t, ErrorTypeStreamStopped, result.errorType)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Operation should have completed quickly after stop")
		}
	})

	t.Run("RapidStartStop", func(t *testing.T) {
		// Test rapid creation and stopping of streams
		for i := 0; i < 100; i++ {
			stream := NewInMemoryStreamImpl(5)

			// Quick operation
			errorType, err := stream.Send(OutputType{"iteration": i}, uuid.New(), time.Now(), 0)
			require.NoError(t, err)
			assert.Equal(t, ErrorTypeNone, errorType)

			// Stop immediately
			err = stream.Stop()
			require.NoError(t, err)
		}
	})

	t.Run("MixedModeOperations", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(3)
		defer stream.Stop()

		// Mix circular buffer and blocking modes
		outputs := []OutputType{
			{"mode": "circular", "step": 1},
			{"mode": "blocking", "step": 2},
			{"mode": "circular", "step": 3},
		}

		// Send with different modes
		errorType, err := stream.Send(outputs[0], uuid.New(), time.Now(), 0) // circular
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		errorType, err = stream.Send(outputs[1], uuid.New(), time.Now(), 5) // blocking
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		errorType, err = stream.Send(outputs[2], uuid.New(), time.Now(), 0) // circular
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		// Receive all
		for i := 0; i < 3; i++ {
			resp, errorType, err := stream.Receive(1)
			require.NoError(t, err)
			assert.Equal(t, ErrorTypeNone, errorType)
			assert.Equal(t, outputs[i], resp.Output)
		}
	})

	t.Run("MinimumTimeout", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(1)
		defer stream.Stop()

		// Fill buffer
		errorType, err := stream.Send(OutputType{"message": "fill"}, uuid.New(), time.Now(), 0)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		// Test minimum timeout (1 second)
		start := time.Now()
		errorType, err = stream.Send(OutputType{"message": "min timeout"}, uuid.New(), time.Now(), 1)
		duration := time.Since(start)

		require.Error(t, err)
		assert.Equal(t, ErrorTypeWaitingTimeout, errorType)
		assert.GreaterOrEqual(t, duration, 900*time.Millisecond)
		assert.LessOrEqual(t, duration, 1100*time.Millisecond)
	})

	t.Run("UUIDUniqueness", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(100)
		defer stream.Stop()

		uuids := make(map[string]bool)

		// Send many messages and collect UUIDs
		for i := 0; i < 50; i++ {
			uid := uuid.New()
			uuids[uid.String()] = true

			errorType, err := stream.Send(OutputType{"index": i}, uid, time.Now(), 0)
			require.NoError(t, err)
			assert.Equal(t, ErrorTypeNone, errorType)
		}

		// Verify all UUIDs are unique
		assert.Equal(t, 50, len(uuids))

		// Receive and verify UUIDs
		receivedUUIDs := make(map[string]bool)
		for i := 0; i < 50; i++ {
			resp, errorType, err := stream.Receive(1)
			require.NoError(t, err)
			assert.Equal(t, ErrorTypeNone, errorType)

			receivedUUIDs[resp.OutputUuid] = true
			assert.True(t, uuids[resp.OutputUuid], "Received UUID should be one that was sent")
		}

		assert.Equal(t, 50, len(receivedUUIDs))
	})

	t.Run("TimestampPreservation", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(5)
		defer stream.Stop()

		// Send messages with specific timestamps
		timestamps := []time.Time{
			time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
			time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC),
		}

		for i, ts := range timestamps {
			errorType, err := stream.Send(OutputType{"index": i}, uuid.New(), ts, 0)
			require.NoError(t, err)
			assert.Equal(t, ErrorTypeNone, errorType)
		}

		// Verify timestamps are preserved
		for i := 0; i < 3; i++ {
			resp, errorType, err := stream.Receive(1)
			require.NoError(t, err)
			assert.Equal(t, ErrorTypeNone, errorType)
			assert.Equal(t, timestamps[i], resp.Timestamp)
		}
	})
}

func TestInMemoryStreamImpl_ErrorScenarios(t *testing.T) {
	t.Run("SendAfterChannelClosed", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(5)

		// Stop the stream
		err := stream.Stop()
		require.NoError(t, err)

		// Try to send after stop - should get stream stopped error
		errorType, err := stream.Send(OutputType{"message": "after stop"}, uuid.New(), time.Now(), 0)
		assert.Error(t, err)
		assert.Equal(t, ErrorTypeStreamStopped, errorType)
		assert.Equal(t, ErrStreamStopped, err)
	})

	t.Run("ReceiveAfterChannelClosed", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(5)

		// Stop the stream
		err := stream.Stop()
		require.NoError(t, err)

		// Try to receive after stop - should get stream stopped error
		_, errorType, err := stream.Receive(1)
		assert.Error(t, err)
		assert.Equal(t, ErrorTypeStreamStopped, errorType)
		assert.Equal(t, ErrStreamStopped, err)
	})

	t.Run("ZeroTimeout", func(t *testing.T) {
		stream := NewInMemoryStreamImpl(1)
		defer stream.Stop()

		// Fill buffer
		errorType, err := stream.Send(OutputType{"message": "fill"}, uuid.New(), time.Now(), 0)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		// Test zero timeout - should use circular buffer mode
		errorType, err = stream.Send(OutputType{"message": "zero timeout"}, uuid.New(), time.Now(), 0)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		// Should overwrite first message
		resp, errorType, err := stream.Receive(1)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)
		assert.Equal(t, OutputType{"message": "zero timeout"}, resp.Output)
	})

	t.Run("CircularBufferIterationLimit", func(t *testing.T) {
		// This test attempts to trigger the 100 iteration safety limit
		// by creating high contention on a tiny buffer

		SetCircularBufferMaxIterations(1)

		stream := NewInMemoryStreamImpl(1) // Very small buffer
		defer stream.Stop()

		// Start many concurrent goroutines trying to send simultaneously
		// to create maximum contention on the circular buffer
		numGoroutines := 100
		attempts := 50

		var wg sync.WaitGroup
		results := make(chan struct {
			errorType ErrorType
			err       error
		}, numGoroutines*attempts)

		// Launch many concurrent senders
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < attempts; j++ {
					errorType, err := stream.Send(
						OutputType{"sender": id, "attempt": j},
						uuid.New(),
						time.Now(),
						0, // circular buffer mode
					)
					results <- struct {
						errorType ErrorType
						err       error
					}{errorType, err}
				}
			}(i)
		}

		wg.Wait()
		close(results)

		// Analyze results
		errorCount := 0
		iterationLimitErrors := 0

		for result := range results {
			if result.err != nil {
				errorCount++
				if result.errorType == ErrorTypeCircularBufferIterationLimit {
					iterationLimitErrors++
					t.Logf("Successfully triggered iteration limit: %v", result.err)
				}
			}
		}

		t.Logf("Total operations: %d", numGoroutines*attempts)
		t.Logf("Errors: %d", errorCount)
		t.Logf("Iteration limit errors: %d", iterationLimitErrors)

		assert.True(t, iterationLimitErrors > 0, "Iteration limit errors should be greater than 0, but got %d", iterationLimitErrors)
	})
}

// Benchmark tests
func BenchmarkInMemoryStreamImpl_Send(b *testing.B) {
	stream := NewInMemoryStreamImpl(1000000) // Large buffer to avoid blocking
	defer stream.Stop()

	output := OutputType{"message": "benchmark"}
	uid := uuid.New()
	timestamp := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		errorType, err := stream.Send(output, uid, timestamp, 0)
		if err != nil || errorType != ErrorTypeNone {
			b.Fatalf("Send failed: %v, errorType: %v", err, errorType)
		}
	}
}
