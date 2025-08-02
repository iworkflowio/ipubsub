package engine

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

type InMemoryMatchingEngine struct {
	streams map[string]InMemoeryStream // streamId -> stream
	// Map of channels for stream creation notifications
	// Key: streamId, Value: channel that gets closed when stream is created
	streamCreationNotifiers map[string]chan struct{}
	sync.RWMutex
	stopped bool
	stopCh  chan struct{} // Channel to signal stop to waiting operations
}

const defaultInMemoryStreamSize = 100

func NewInMemoryMatchingEngine() MatchingEngine {
	return &InMemoryMatchingEngine{
		streams:                 make(map[string]InMemoeryStream),
		streamCreationNotifiers: make(map[string]chan struct{}),
		stopped:                 false,
		stopCh:                  make(chan struct{}),
	}
}

// Start implements MatchingEngine.
func (i *InMemoryMatchingEngine) Start() error {
	i.Lock()
	defer i.Unlock()

	if i.stopped {
		return errors.New("matching engine is already stopped")
	}

	i.stopped = false
	return nil
}

// Stop implements MatchingEngine.
func (i *InMemoryMatchingEngine) Stop() error {
	i.Lock()
	defer i.Unlock()

	if i.stopped {
		return nil
	}

	i.stopped = true

	// Signal all waiting operations to stop
	close(i.stopCh)

	// Close all stream creation notifier channels
	for _, notifierCh := range i.streamCreationNotifiers {
		safeClose(notifierCh)
	}

	// Stop all streams
	for _, stream := range i.streams {
		stream.Stop()
	}

	return nil
}

// Send implements MatchingEngine.
func (i *InMemoryMatchingEngine) Send(req *InternalSendRequest) (errorType ErrorType, err error) {
	// Quick check if stopped (read lock not needed for boolean read)
	if i.stopped {
		return ErrorTypeStreamStopped, ErrStreamStopped
	}

	streamId := req.StreamId
	var stream InMemoeryStream
	var streamExists bool

	// First, try to get existing stream with read lock
	i.RLock()
	stream, streamExists = i.streams[streamId]
	stopped := i.stopped
	i.RUnlock()

	// Check stopped after releasing read lock
	if stopped {
		return ErrorTypeStreamStopped, ErrStreamStopped
	}

	// If stream doesn't exist, create it with write lock
	if !streamExists {
		i.Lock()
		// Double-check pattern: another goroutine might have created it
		if stream, streamExists = i.streams[streamId]; !streamExists {
			// Check stopped again while holding write lock
			if i.stopped {
				i.Unlock()
				return ErrorTypeStreamStopped, ErrStreamStopped
			}

			// Create new stream with specified size or default
			streamSize := int(req.InMemoryStreamSize)
			if streamSize == 0 {
				streamSize = defaultInMemoryStreamSize
			}

			stream = NewInMemoryStreamImpl(streamSize)
			i.streams[streamId] = stream

			// Notify any waiting Receive() calls by closing the notifier channel
			if notifierCh, exists := i.streamCreationNotifiers[streamId]; exists {
				safeClose(notifierCh)
				// delete because we will no longer need this notifier since the stream is created
				delete(i.streamCreationNotifiers, streamId)
			}
		}
		i.Unlock()
	}

	// Now send to the stream (no locks needed, stream handles its own concurrency)
	outputUuid, err := uuid.Parse(req.OutputUuid)
	if err != nil {
		return ErrorTypeInvalidRequest, err
	}

	return stream.Send(
		req.Output,
		outputUuid,
		time.Now(),
		req.BlockingWriteTimeoutSeconds,
	)
}

// Receive implements MatchingEngine.
func (i *InMemoryMatchingEngine) Receive(req *InternalReceiveRequest) (resp *InternalReceiveResponse, errorType ErrorType, err error) {
	// Quick check if stopped (read lock not needed for boolean read)
	if i.stopped {
		return nil, ErrorTypeStreamStopped, ErrStreamStopped
	}

	streamId := req.StreamId
	timeoutSeconds := req.TimeoutSeconds
	timeout := time.Duration(timeoutSeconds) * time.Second

	// First, try to get existing stream
	i.RLock()
	stream, streamExists := i.streams[streamId]
	stopped := i.stopped
	stopCh := i.stopCh // Capture the stop channel
	i.RUnlock()

	// Check stopped after releasing read lock
	if stopped {
		return nil, ErrorTypeStreamStopped, ErrStreamStopped
	}

	if streamExists {
		// Stream already exists, receive directly
		return stream.Receive(timeoutSeconds)
	}

	// Stream doesn't exist, create a notifier channel and wait for stream creation
	var notifierCh chan struct{}

	i.Lock()
	// Double-check: stream might have been created while acquiring write lock
	if stream, streamExists = i.streams[streamId]; streamExists {
		i.Unlock()
		// Stream was created, receive directly
		return stream.Receive(timeoutSeconds)
	}

	// Check stopped again while holding write lock
	if i.stopped {
		i.Unlock()
		return nil, ErrorTypeStreamStopped, ErrStreamStopped
	}

	// Stream still doesn't exist, check if there's already a notifier for this streamId
	if existingNotifier, exists := i.streamCreationNotifiers[streamId]; exists {
		// Use existing notifier
		notifierCh = existingNotifier
	} else {
		// Create new notifier channel
		notifierCh = make(chan struct{})
		i.streamCreationNotifiers[streamId] = notifierCh
	}

	i.Unlock()

	// Wait for stream creation, timeout, or stop signal
	startTime := time.Now()
	select {
	case <-notifierCh:
		// Stream was created! Try to get it and receive
		i.RLock()
		stream, streamExists = i.streams[streamId]
		stopped = i.stopped
		i.RUnlock()

		if stopped {
			return nil, ErrorTypeStreamStopped, ErrStreamStopped
		}

		if streamExists {
			// Calculate remaining timeout for the actual receive
			remainingTime := timeout - time.Since(startTime)
			remainingSeconds := int(remainingTime.Seconds())

			// Ensure we have at least 1 second for the actual receive operation
			if remainingSeconds < 1 {
				return nil, ErrorTypeWaitingTimeout, nil
			}

			return stream.Receive(remainingSeconds)
		}

		// This shouldn't happen, but handle gracefully
		return nil, ErrorTypeUnknown, nil

	case <-time.After(timeout):
		return nil, ErrorTypeWaitingTimeout, nil

	case <-stopCh:
		// Engine stopped
		return nil, ErrorTypeStreamStopped, ErrStreamStopped
	}
}

func safeClose(ch chan struct{}) {
	select {
	case <-ch:
		// Already closed, do nothing
	default:
		close(ch)
	}
}
