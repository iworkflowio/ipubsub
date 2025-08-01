package engine

import (
	"github.com/google/uuid"
	genapi "github.com/iworkflowio/async-output-service/genapi/go"
	"sync"
	"time"
)

type InMemoryStream struct {
	outputs chan OutputType
	// protect the channel
	sync.RWMutex
}

func NewInMemoryStream(size int) Stream {
	return &InMemoryStream{
		outputs: make(chan OutputType, size),
	}
}

// Receive implements Stream.
func (i *InMemoryStream) Receive(timeoutSeconds int) (output *genapi.ReceiveResponse, isTimeout bool, err error) {
	panic("unimplemented")
}

// Send implements Stream.
func (i *InMemoryStream) Send(output OutputType, outputUuid uuid.UUID, timestamp time.Time) error {
	panic("unimplemented")
}

// Stop implements Stream.
func (i *InMemoryStream) Stop() error {
	panic("unimplemented")
}

