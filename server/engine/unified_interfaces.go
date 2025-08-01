package engine

import (
	"time"

	"github.com/google/uuid"
	genapi "github.com/iworkflowio/async-output-service/genapi/go"
)

// OutputType is the type of the output data
// a shortcut for the output data type
type OutputType = map[string]interface{}

type ErrorType int

const (
	ErrorTypeNone ErrorType = iota
	ErrorTypeUnknown 
	ErrorTypeWaitingTimeout
	ErrorTypeStreamStopped
	ErrorTypeStreamFull
	ErrorTypeStreamEmpty
)

type MatchingEngine interface {
	Start() error
	Send(genapi.SendRequest) (errorType ErrorType, err error)
	Receive(ReceiveRequest) (resp *genapi.ReceiveResponse, errorType ErrorType, err error)
	Stop() error
}

type InMemoeryStream interface {
	Send(output OutputType, outputUuid uuid.UUID, timestamp time.Time, blockingWriteTimeoutSeconds int) (errorType ErrorType, err error)
	Receive(timeoutSeconds int) (output *genapi.ReceiveResponse, errorType ErrorType, err error)
	Stop() error
}

type ReceiveRequest struct {
	StreamId       string
	TimeoutSeconds int

	// Will be implemented in phase2, currently not used
	ReadFromDB    bool
	DbResumeToken string
}
