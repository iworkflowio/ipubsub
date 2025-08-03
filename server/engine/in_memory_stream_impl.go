package engine

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// StreamEntry represents a single message entry in the stream
type StreamEntry struct {
	MessageUUID uuid.UUID
	Message     MessageType
	Timestamp   time.Time
}

type InMemoryStreamImpl struct {
	messages chan StreamEntry
	// indicates if the stream is stopped
	stopped bool
	// channel capacity for reference
	capacity int
	// channel to signal stop
	stopCh chan struct{}
	// protect the channel and state
	sync.RWMutex
}

var ErrStreamStopped = errors.New("stream is stopped")

var circularBufferMaxIterations = 100

func SetCircularBufferMaxIterations(maxIterations int) {
	circularBufferMaxIterations = maxIterations
}

func NewInMemoryStreamImpl(size int) InMemoeryStream {
	return &InMemoryStreamImpl{
		messages: make(chan StreamEntry, size),
		capacity: size,
		stopped:  false,
		stopCh:   make(chan struct{}),
	}
}

// Send implements InMemoeryStream.
func (i *InMemoryStreamImpl) Send(message MessageType, messageUuid uuid.UUID, timestamp time.Time, blockingSendTimeoutSeconds int) (errorType ErrorType, err error) {
	// Check if stopped first
	if i.stopped {
		return ErrorTypeStreamStopped, ErrStreamStopped
	}

	entry := StreamEntry{
		MessageUUID: messageUuid,
		Message:     message,
		Timestamp:   timestamp,
	}

	// If blockingSendTimeoutSeconds is 0 or not specified, use circular buffer mode
	if blockingSendTimeoutSeconds <= 0 {
		return i.sendCircularBufferWithChannel(entry, i.messages)
	}

	// Use blocking queue mode with timeout
	return i.sendBlockingQueueWithChannel(entry, blockingSendTimeoutSeconds, i.messages)
}

// sendCircularBufferWithChannel implements circular buffer behavior - overwrites oldest data when full
func (i *InMemoryStreamImpl) sendCircularBufferWithChannel(entry StreamEntry, messagesChan chan StreamEntry) (errorType ErrorType, err error) {
	// Not allowed for zero capacity circular buffer
	if i.capacity == 0 {
		return ErrorTypeInvalidRequest, errors.New("zero capacity circular buffer is not allowed")
	}

	select {
	case messagesChan <- entry:
		// Successfully wrote to channel
		return ErrorTypeNone, nil
	case <-i.stopCh:
		return ErrorTypeStreamStopped, ErrStreamStopped
	default:
		// Channel is full, remove oldest entry and add new one
		// Use write lock to protect the two operations below
		i.Lock()
		defer i.Unlock()

		// Check if stopped while waiting for lock
		if i.stopped {
			return ErrorTypeStreamStopped, ErrStreamStopped
		}

		iterations := 0
		for {
			iterations++
			if iterations > circularBufferMaxIterations {
				return ErrorTypeCircularBufferIterationLimit, fmt.Errorf("failed to write to circular buffer, buffer is still full after removing oldest entry for %d iterations", iterations)
			}
			// However, this is best effort only because other operations are not using locks.
			<-messagesChan // Remove oldest
			select {
			case messagesChan <- entry:
				// Successfully wrote to channel
				return ErrorTypeNone, nil
			case <-i.stopCh:
				return ErrorTypeStreamStopped, ErrStreamStopped
			default:
				// Channel is still full, do it again
				continue
			}
		}
	}
}

// sendBlockingQueueWithChannel implements blocking queue behavior - waits for space and returns error on timeout
func (i *InMemoryStreamImpl) sendBlockingQueueWithChannel(entry StreamEntry, timeoutSeconds int, messagesChan chan StreamEntry) (errorType ErrorType, err error) {
	select {
	case messagesChan <- entry:
		// Successfully wrote to channel
		return ErrorTypeNone, nil
	case <-i.stopCh:
		return ErrorTypeStreamStopped, ErrStreamStopped
	case <-time.After(time.Duration(timeoutSeconds) * time.Second):
		// NOTE: As of Go 1.23, the garbage collector can recover unreferenced unstopped timers. There is no reason to prefer NewTimer when After will do.
		return ErrorTypeWaitingTimeout, errors.New("timeout waiting for stream space (424)")
	}
}

// Receive implements InMemoeryStream.
func (i *InMemoryStreamImpl) Receive(timeoutSeconds int) (message *InternalReceiveResponse, errorType ErrorType, err error) {
	// Quick check if stopped (without lock since it's just a read)
	if i.stopped {
		return nil, ErrorTypeStreamStopped, ErrStreamStopped
	}

	select {
	case entry := <-i.messages:
		// Successfully received an entry
		return &InternalReceiveResponse{
			MessageUuid: entry.MessageUUID,
			Message:     entry.Message,
			Timestamp:   entry.Timestamp,
		}, ErrorTypeNone, nil
	case <-i.stopCh:
		return nil, ErrorTypeStreamStopped, ErrStreamStopped
	case <-time.After(time.Duration(timeoutSeconds) * time.Second):
		// NOTE: As of Go 1.23, the garbage collector can recover unreferenced unstopped timers. There is no reason to prefer NewTimer when After will do.
		return nil, ErrorTypeWaitingTimeout, nil
	}
}

// Stop implements InMemoeryStream.
func (i *InMemoryStreamImpl) Stop() error {
	i.Lock()
	defer i.Unlock()

	if i.stopped {
		return nil
	}

	i.stopped = true
	close(i.stopCh)
	// TODO move the received messages to the new node that owned the streamId
	close(i.messages)
	return nil
}
