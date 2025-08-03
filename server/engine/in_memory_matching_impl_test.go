package engine

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryMatchingEngine_StartStop(t *testing.T) {
	engine := NewInMemoryMatchingEngine()

	t.Run("StartEngine", func(t *testing.T) {
		err := engine.Start()
		require.NoError(t, err)
	})

	t.Run("StartAlreadyStartedEngine", func(t *testing.T) {
		// Starting an already started engine should succeed (idempotent)
		err := engine.Start()
		assert.NoError(t, err)
	})

	t.Run("StopEngine", func(t *testing.T) {
		err := engine.Stop()
		require.NoError(t, err)
	})

	t.Run("StopAlreadyStoppedEngine", func(t *testing.T) {
		err := engine.Stop()
		assert.NoError(t, err) // Should be idempotent
	})

	t.Run("StartAfterStop", func(t *testing.T) {
		// Starting a stopped engine should return an error based on the implementation
		err := engine.Start()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already stopped")
	})
}

func TestInMemoryMatchingEngine_Send(t *testing.T) {
	engine := NewInMemoryMatchingEngine()
	err := engine.Start()
	require.NoError(t, err)
	defer engine.Stop()

	t.Run("SendToNewStream", func(t *testing.T) {
		req := InternalSendRequest{
			StreamId:                    "test-stream-1",
			MessageUuid:                  uuid.New().String(),
			InMemoryStreamSize:          10,
			BlockingSendTimeoutSeconds: 0,
			Message:                      map[string]interface{}{"message": "hello"},
		}

		errorType, err := engine.Send(&req)
		assert.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)
	})

	t.Run("SendToExistingStream", func(t *testing.T) {
		streamId := "test-stream-2"

		// First send creates the stream
		req1 := InternalSendRequest{
			StreamId:                    streamId,
			MessageUuid:                  uuid.New().String(),
			InMemoryStreamSize:          10,
			BlockingSendTimeoutSeconds: 0,
			Message:                      map[string]interface{}{"message": "first"},
		}
		errorType, err := engine.Send(&req1)
		assert.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)

		// Second send to same stream
		req2 := InternalSendRequest{
			StreamId:                    streamId,
			MessageUuid:                  uuid.New().String(),
			InMemoryStreamSize:          5, // Should be ignored for existing stream
			BlockingSendTimeoutSeconds: 0,
			Message:                      map[string]interface{}{"message": "second"},
		}
		errorType, err = engine.Send(&req2)
		assert.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)
	})

	t.Run("SendWithDefaultStreamSize", func(t *testing.T) {
		req := &InternalSendRequest{
			StreamId:                    "test-stream-default",
			MessageUuid:                  uuid.New().String(),
			InMemoryStreamSize:          0, // Should use default
			BlockingSendTimeoutSeconds: 0,
			Message:                      map[string]interface{}{"message": "default"},
		}

		errorType, err := engine.Send(req)
		assert.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)
	})

	t.Run("SendWithInvalidUUID", func(t *testing.T) {
		req := &InternalSendRequest{
			StreamId:                    "test-stream-invalid",
			MessageUuid:                  "invalid-uuid",
			InMemoryStreamSize:          10,
			BlockingSendTimeoutSeconds: 0,
			Message:                      map[string]interface{}{"message": "test"},
		}

		errorType, err := engine.Send(req)
		assert.Error(t, err)
		assert.Equal(t, ErrorTypeInvalidRequest, errorType)
	})

	t.Run("SendToStoppedEngine", func(t *testing.T) {
		stoppedEngine := NewInMemoryMatchingEngine()
		stoppedEngine.Start()
		stoppedEngine.Stop()

		req := &InternalSendRequest{
			StreamId:                    "test-stream-stopped",
			MessageUuid:                  uuid.New().String(),
			InMemoryStreamSize:          10,
			BlockingSendTimeoutSeconds: 0,
			Message:                      map[string]interface{}{"message": "test"},
		}

		errorType, err := stoppedEngine.Send(req)
		assert.Error(t, err)
		assert.Equal(t, ErrorTypeStreamStopped, errorType)
	})
}

func TestInMemoryMatchingEngine_Receive(t *testing.T) {
	engine := NewInMemoryMatchingEngine()
	err := engine.Start()
	require.NoError(t, err)
	defer engine.Stop()

	t.Run("ReceiveFromExistingStream", func(t *testing.T) {
		streamId := "test-stream-receive-1"

		// First, send data to create stream
		sendReq := &InternalSendRequest{
			StreamId:                    streamId,
			MessageUuid:                  uuid.New().String(),
			InMemoryStreamSize:          10,
			BlockingSendTimeoutSeconds: 0,
			Message:                      map[string]interface{}{"message": "test-data"},
		}
		errorType, err := engine.Send(sendReq)
		require.NoError(t, err)
		require.Equal(t, ErrorTypeNone, errorType)

		// Then receive from the stream
		receiveReq := InternalReceiveRequest{
			StreamId:       streamId,
			TimeoutSeconds: 5,
		}
		resp, errorType, err := engine.Receive(&receiveReq)
		assert.NoError(t, err)
		assert.Equal(t, ErrorTypeNone, errorType)
		assert.NotNil(t, resp)
		assert.Equal(t, map[string]interface{}{"message": "test-data"}, resp.Message)
	})

	t.Run("ReceiveTimeoutFromNonExistentStream", func(t *testing.T) {
		receiveReq := InternalReceiveRequest{
			StreamId:       "non-existent-stream",
			TimeoutSeconds: 1, // Short timeout for test speed
		}

		start := time.Now()
		resp, errorType, err := engine.Receive(&receiveReq)
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.Equal(t, ErrorTypeWaitingTimeout, errorType)
		assert.Nil(t, resp)
		assert.True(t, duration >= 1*time.Second)
		assert.True(t, duration < 2*time.Second) // Should not take much longer than timeout
	})

	t.Run("ReceiveFromStoppedEngine", func(t *testing.T) {
		stoppedEngine := NewInMemoryMatchingEngine()
		stoppedEngine.Start()
		stoppedEngine.Stop()

		receiveReq := InternalReceiveRequest{
			StreamId:       "test-stream",
			TimeoutSeconds: 5,
		}

		resp, errorType, err := stoppedEngine.Receive(&receiveReq)
		assert.Error(t, err)
		assert.Equal(t, ErrorTypeStreamStopped, errorType)
		assert.Nil(t, resp)
	})
}

func TestInMemoryMatchingEngine_WaitForStreamCreation(t *testing.T) {
	engine := NewInMemoryMatchingEngine()
	err := engine.Start()
	require.NoError(t, err)
	defer engine.Stop()

	t.Run("ReceiveWaitsForSend", func(t *testing.T) {
		streamId := "test-stream-wait"

		var receiveResp *InternalReceiveResponse
		var receiveErrorType ErrorType
		var receiveErr error

		// Start receiving in a goroutine (this should wait)
		go func() {
			receiveReq := InternalReceiveRequest{
				StreamId:       streamId,
				TimeoutSeconds: 10,
			}
			receiveResp, receiveErrorType, receiveErr = engine.Receive(&receiveReq)
		}()

		// Wait a bit to ensure receive is waiting
		time.Sleep(100 * time.Millisecond)

		// Send data (this should wake up the receiver)
		sendReq := &InternalSendRequest{
			StreamId:                    streamId,
			MessageUuid:                  uuid.New().String(),
			InMemoryStreamSize:          10,
			BlockingSendTimeoutSeconds: 0,
			Message:                      map[string]interface{}{"message": "delayed-data"},
		}
		sendErrorType, sendErr := engine.Send(sendReq)
		require.NoError(t, sendErr)
		require.Equal(t, ErrorTypeNone, sendErrorType)

		// Wait for receive to complete
		time.Sleep(100 * time.Millisecond)

		// Check receive results
		assert.NoError(t, receiveErr)
		assert.Equal(t, ErrorTypeNone, receiveErrorType)
		assert.NotNil(t, receiveResp)
		assert.Equal(t, map[string]interface{}{"message": "delayed-data"}, receiveResp.Message)
	})
}

func TestInMemoryMatchingEngine_MultipleWaiters(t *testing.T) {
	engine := NewInMemoryMatchingEngine()
	err := engine.Start()
	require.NoError(t, err)
	defer engine.Stop()

	t.Run("MultiplReceiveWaitForSingleSend", func(t *testing.T) {
		streamId := "test-stream-multiple"
		numWaiters := 3

		var wg sync.WaitGroup
		results := make([]struct {
			resp      *InternalReceiveResponse
			errorType ErrorType
			err       error
		}, numWaiters)

		// Start multiple receivers
		for i := 0; i < numWaiters; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				receiveReq := InternalReceiveRequest{
					StreamId:       streamId,
					TimeoutSeconds: 10,
				}
				results[index].resp, results[index].errorType, results[index].err = engine.Receive(&receiveReq)
			}(i)
		}

		// Wait a bit to ensure all receivers are waiting
		time.Sleep(200 * time.Millisecond)

		// Send multiple messages
		for i := 0; i < numWaiters; i++ {
			sendReq := &InternalSendRequest{
				StreamId:                    streamId,
				MessageUuid:                  uuid.New().String(),
				InMemoryStreamSize:          10,
				BlockingSendTimeoutSeconds: 0,
				Message:                      map[string]interface{}{"message": "data", "index": i},
			}
			sendErrorType, sendErr := engine.Send(sendReq)
			require.NoError(t, sendErr)
			require.Equal(t, ErrorTypeNone, sendErrorType)
		}

		// Wait for all receivers to complete
		wg.Wait()

		// Check that all receivers got data (at least one should succeed)
		successCount := 0
		for i := 0; i < numWaiters; i++ {
			if results[i].errorType == ErrorTypeNone && results[i].resp != nil {
				successCount++
			}
		}
		assert.True(t, successCount > 0, "At least one receiver should have succeeded")
	})
}

func TestInMemoryMatchingEngine_StopInterruptsWaitingReceive(t *testing.T) {
	engine := NewInMemoryMatchingEngine()
	err := engine.Start()
	require.NoError(t, err)

	t.Run("StopWakesUpWaitingReceive", func(t *testing.T) {
		var receiveResp *InternalReceiveResponse
		var receiveErrorType ErrorType
		var receiveErr error
		done := make(chan bool)

		// Start receiving in a goroutine (this should wait)
		go func() {
			receiveReq := InternalReceiveRequest{
				StreamId:       "test-stream-stop",
				TimeoutSeconds: 30, // Long timeout
			}
			receiveResp, receiveErrorType, receiveErr = engine.Receive(&receiveReq)
			done <- true
		}()

		// Wait a bit to ensure receive is waiting
		time.Sleep(100 * time.Millisecond)

		// Stop the engine (should wake up the receiver)
		stopErr := engine.Stop()
		require.NoError(t, stopErr)

		// Wait for receive to complete
		select {
		case <-done:
			// Good, receive completed
		case <-time.After(5 * time.Second):
			t.Fatal("Receive did not complete after engine stop")
		}

		// Check receive results
		assert.Error(t, receiveErr)
		assert.Equal(t, ErrorTypeStreamStopped, receiveErrorType)
		assert.Nil(t, receiveResp)
	})
}

func TestInMemoryMatchingEngine_ConcurrentSendReceive(t *testing.T) {
	engine := NewInMemoryMatchingEngine()
	err := engine.Start()
	require.NoError(t, err)
	defer engine.Stop()

	t.Run("ConcurrentOperations", func(t *testing.T) {
		numStreams := 10
		numMessagesPerStream := 5

		var wg sync.WaitGroup

		// Concurrent senders
		for streamIndex := 0; streamIndex < numStreams; streamIndex++ {
			wg.Add(1)
			go func(streamId int) {
				defer wg.Done()
				for msgIndex := 0; msgIndex < numMessagesPerStream; msgIndex++ {
					sendReq := &InternalSendRequest{
						StreamId:                    fmt.Sprintf("concurrent-stream-%d", streamId),
						MessageUuid:                  uuid.New().String(),
						InMemoryStreamSize:          10,
						BlockingSendTimeoutSeconds: 0,
						Message:                      map[string]interface{}{"stream": streamId, "message": msgIndex},
					}
					errorType, err := engine.Send(sendReq)
					assert.NoError(t, err)
					assert.Equal(t, ErrorTypeNone, errorType)
				}
			}(streamIndex)
		}

		// Concurrent receivers
		for streamIndex := 0; streamIndex < numStreams; streamIndex++ {
			wg.Add(1)
			go func(streamId int) {
				defer wg.Done()
				for msgIndex := 0; msgIndex < numMessagesPerStream; msgIndex++ {
					receiveReq := InternalReceiveRequest{
						StreamId:       fmt.Sprintf("concurrent-stream-%d", streamId),
						TimeoutSeconds: 10,
					}
					resp, errorType, err := engine.Receive(&receiveReq)
					assert.NoError(t, err)
					assert.Equal(t, ErrorTypeNone, errorType)
					assert.NotNil(t, resp)
				}
			}(streamIndex)
		}

		wg.Wait()
	})
}

func TestInMemoryMatchingEngine_RemainingTimeoutCalculation(t *testing.T) {
	engine := NewInMemoryMatchingEngine()
	err := engine.Start()
	require.NoError(t, err)
	defer engine.Stop()

	t.Run("RemainingTimeoutAfterWait", func(t *testing.T) {
		streamId := "test-timeout-calc"

		var receiveResp *InternalReceiveResponse
		var receiveErrorType ErrorType
		var receiveErr error
		done := make(chan bool)

		// Start receiving with 5 second timeout
		go func() {
			receiveReq := InternalReceiveRequest{
				StreamId:       streamId,
				TimeoutSeconds: 5,
			}
			receiveResp, receiveErrorType, receiveErr = engine.Receive(&receiveReq)
			done <- true
		}()

		// Wait 2 seconds, then send data
		time.Sleep(2 * time.Second)

		sendReq := &InternalSendRequest{
			StreamId:                    streamId,
			MessageUuid:                  uuid.New().String(),
			InMemoryStreamSize:          10,
			BlockingSendTimeoutSeconds: 0,
			Message:                      map[string]interface{}{"message": "timeout-test"},
		}
		sendErrorType, sendErr := engine.Send(sendReq)
		require.NoError(t, sendErr)
		require.Equal(t, ErrorTypeNone, sendErrorType)

		// Wait for receive to complete
		select {
		case <-done:
			// Good
		case <-time.After(10 * time.Second):
			t.Fatal("Receive did not complete")
		}

		// Should have succeeded (remaining timeout should have been >= 1 second)
		assert.NoError(t, receiveErr)
		assert.Equal(t, ErrorTypeNone, receiveErrorType)
		assert.NotNil(t, receiveResp)
	})

	t.Run("InsufficientRemainingTimeout", func(t *testing.T) {
		streamId := "test-insufficient-timeout"

		var receiveResp *InternalReceiveResponse
		var receiveErrorType ErrorType
		var receiveErr error
		done := make(chan bool)

		// Start receiving with 2 second timeout
		go func() {
			receiveReq := InternalReceiveRequest{
				StreamId:       streamId,
				TimeoutSeconds: 2,
			}
			receiveResp, receiveErrorType, receiveErr = engine.Receive(&receiveReq)
			done <- true
		}()

		// Wait 1.8 seconds, then send data (leaving < 1 second)
		time.Sleep(1800 * time.Millisecond)

		sendReq := &InternalSendRequest{
			StreamId:                    streamId,
			MessageUuid:                  uuid.New().String(),
			InMemoryStreamSize:          10,
			BlockingSendTimeoutSeconds: 0,
			Message:                      map[string]interface{}{"message": "insufficient-timeout"},
		}
		sendErrorType, sendErr := engine.Send(sendReq)
		require.NoError(t, sendErr)
		require.Equal(t, ErrorTypeNone, sendErrorType)

		// Wait for receive to complete
		select {
		case <-done:
			// Good
		case <-time.After(5 * time.Second):
			t.Fatal("Receive did not complete")
		}

		// Should timeout due to insufficient remaining time
		assert.NoError(t, receiveErr)
		assert.Equal(t, ErrorTypeWaitingTimeout, receiveErrorType)
		assert.Nil(t, receiveResp)
	})
}
