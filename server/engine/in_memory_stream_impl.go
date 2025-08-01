package engine

import (
	"sync"
	"time"

	"github.com/google/uuid"
	genapi "github.com/iworkflowio/async-output-service/genapi/go"
)

type InMemoryStreamImpl struct {
	outputs chan OutputType
	// protect the channel
	sync.RWMutex
}

func NewInMemoryStreamImpl(size int) InMemoeryStream {
	return &InMemoryStreamImpl{
		outputs: make(chan OutputType, size),
	}
}

// Receive implements Stream.
func (i *InMemoryStreamImpl) Receive(timeoutSeconds int) (output *genapi.ReceiveResponse, isTimeout bool, err error) {
	panic("unimplemented")
}

// Send implements Stream.
func (i *InMemoryStreamImpl) Send(output OutputType, outputUuid uuid.UUID, timestamp time.Time, blockingWriteTimeoutSeconds int) error {
	panic("unimplemented")
}

// Stop implements Stream.
func (i *InMemoryStreamImpl) Stop() error {
	panic("unimplemented")
}
