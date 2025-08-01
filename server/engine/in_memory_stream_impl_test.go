package engine

import (
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
		// Fill the buffer to capacity (3)
		outputs := []OutputType{
			{"message": "msg1", "step": 1},
			{"message": "msg2", "step": 2},
			{"message": "msg3", "step": 3},
		}

		// Send 3 messages to fill buffer
		for i := 0; i < 3; i++ {
			errorType, err := stream.Send(outputs[i], uuid.New(), time.Now(), 0)
			require.NoError(t, err)
			assert.Equal(t, ErrorTypeNone, errorType)
		}

		// Send 4th message, should overwrite the first one
		output4 := OutputType{"message": "msg4", "step": 4}
		errorType, err := stream.Send(output4, uuid.New(), time.Now(), 0)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		// First receive should get msg2 (msg1 was overwritten)
		resp, errorType, err := stream.Receive(1)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)
		assert.Equal(t, outputs[1], resp.Output) // Should be msg2

		// Second receive should get msg3
		resp, errorType, err = stream.Receive(1)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)
		assert.Equal(t, outputs[2], resp.Output) // Should be msg3

		// Third receive should get msg4
		resp, errorType, err = stream.Receive(1)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)
		assert.Equal(t, output4, resp.Output) // Should be msg4

		// Fourth receive should timeout (buffer empty)
		resp, errorType, err = stream.Receive(1)
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
	// Test blocking queue mode (when blockingWriteTimeoutSeconds > 0)
	stream := NewInMemoryStreamImpl(2) // Buffer size 2
	defer stream.Stop()

	t.Run("BlockingQueueBasicOperation", func(t *testing.T) {
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
		errorType, err := testStream.Send(OutputType{"message": "initial"}, uuid.New(), time.Now(), 5)
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

		// Receive to make space
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
		// Start a consumer in a goroutine
		receiveResult := make(chan struct {
			resp      *genapi.ReceiveResponse
			errorType ErrorType
			err       error
		}, 1)

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

		// Now send should succeed immediately
		output := OutputType{"message": "sync success"}
		uuid1 := uuid.New()
		errorType, err := stream.Send(output, uuid1, time.Now(), 2)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

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

		// Fill the buffer
		errorType, err := testStream.Send(OutputType{"message": "fill"}, uuid.New(), time.Now(), 0)
		require.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		// Start a blocking send
		sendResult := make(chan struct {
			errorType ErrorType
			err       error
		}, 1)
		go func() {
			errorType, err := testStream.Send(OutputType{"message": "blocking"}, uuid.New(), time.Now(), 10)
			sendResult <- struct {
				errorType ErrorType
				err       error
			}{errorType, err}
		}()

		// Start a blocking receive
		receiveResult := make(chan struct {
			resp      *genapi.ReceiveResponse
			errorType ErrorType
			err       error
		}, 1)
		// First consume the fill message
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
		time.Sleep(100 * time.Millisecond)

		// Stop should unblock both operations
		err = testStream.Stop()
		require.NoError(t, err)

		// Check send result
		select {
		case result := <-sendResult:
			assert.Error(t, result.err)
			assert.Equal(t, ErrorTypeStreamStopped, result.errorType)
		case <-time.After(1 * time.Second):
			t.Fatal("Send should have been unblocked by stop")
		}

		// Check receive result
		select {
		case result := <-receiveResult:
			assert.Error(t, result.err)
			assert.Equal(t, ErrorTypeStreamStopped, result.errorType)
		case <-time.After(1 * time.Second):
			t.Fatal("Receive should have been unblocked by stop")
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
}
