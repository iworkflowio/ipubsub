package engine

import (
	"sync"
	"time"

	"github.com/google/uuid"
	genapi "github.com/iworkflowio/async-output-service/genapi/go"
)

type InMemoryMatchingEngine struct {
	streams map[string]InMemoeryStream // streamId -> stream
	sync.RWMutex
	stopped bool
}

const defaultInMemoryStreamSize = 100

func NewInMemoryMatchingEngine() MatchingEngine {
	return &InMemoryMatchingEngine{
		streams: make(map[string]InMemoeryStream),
		stopped: false,
	}
}

// Start implements MatchingEngine.
func (i *InMemoryMatchingEngine) Start() error {
	i.Lock()
	defer i.Unlock()

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

	// Stop all streams
	for _, stream := range i.streams {
		stream.Stop()
	}

	// Clear the streams map
	i.streams = make(map[string]InMemoeryStream)

	return nil
}

// Send implements MatchingEngine.
func (i *InMemoryMatchingEngine) Send(req genapi.SendRequest) (errorType ErrorType, err error) {
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
		int(req.BlockingWriteTimeoutSeconds),
	)
}

// Receive implements MatchingEngine.
func (i *InMemoryMatchingEngine) Receive(req ReceiveRequest) (resp *genapi.ReceiveResponse, errorType ErrorType, err error) {
	// Quick check if stopped (read lock not needed for boolean read)
	if i.stopped {
		return nil, ErrorTypeStreamStopped, ErrStreamStopped
	}

	streamId := req.StreamId
	var stream InMemoeryStream
	var streamExists bool

	// Get stream with read lock
	i.RLock()
	stream, streamExists = i.streams[streamId]
	stopped := i.stopped
	i.RUnlock()

	// Check stopped after releasing read lock
	if stopped {
		return nil, ErrorTypeStreamStopped, ErrStreamStopped
	}

	// If stream doesn't exist, return timeout immediately
	if !streamExists {
		return nil, ErrorTypeWaitingTimeout, nil
	}

	// Receive from the stream (no locks needed, stream handles its own concurrency)
	return stream.Receive(req.TimeoutSeconds)
}
