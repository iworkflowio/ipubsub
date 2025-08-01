package engine

import (
	"sync"

	genapi "github.com/iworkflowio/async-output-service/genapi/go"
)

type InMemoryMatchingEngine struct {
	streams map[string]InMemoeryStream // streamId -> stream
	sync.RWMutex
}

func NewInMemoryMatchingEngine() MatchingEngine {
	return &InMemoryMatchingEngine{
		streams: make(map[string]InMemoeryStream),
	}
}

// Receive implements MatchingEngine.
func (i *InMemoryMatchingEngine) Receive(ReceiveRequest) (resp *genapi.ReceiveResponse, errorType ErrorType, err error) {
	panic("unimplemented")
}

// Send implements MatchingEngine.
func (i *InMemoryMatchingEngine) Send(genapi.SendRequest) (errorType ErrorType, err error) {
	panic("unimplemented")
}

// Start implements MatchingEngine.
func (i *InMemoryMatchingEngine) Start() error {
	panic("unimplemented")
}

// Stop implements MatchingEngine.
func (i *InMemoryMatchingEngine) Stop() error {
	panic("unimplemented")
}
