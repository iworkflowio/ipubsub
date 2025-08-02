package engine

import (
	"time"

	"github.com/google/uuid"
)

// OutputType is the type of the output data
// a shortcut for the output data type
type OutputType = map[string]interface{}

type ErrorType int

const (
	ErrorTypeNone ErrorType = iota
	ErrorTypeUnknown
	ErrorTypeInvalidRequest
	ErrorTypeCircularBufferIterationLimit
	ErrorTypeWaitingTimeout
	ErrorTypeStreamStopped
	ErrorTypeStreamFull
	ErrorTypeStreamEmpty
)

type MatchingEngine interface {
	Start() error
	Send(*InternalSendRequest) (errorType ErrorType, err error)
	Receive(*InternalReceiveRequest) (resp *InternalReceiveResponse, errorType ErrorType, err error)
	Stop() error
}

type InMemoeryStream interface {
	Send(output OutputType, outputUuid uuid.UUID, timestamp time.Time, blockingWriteTimeoutSeconds int) (errorType ErrorType, err error)
	Receive(timeoutSeconds int) (output *InternalReceiveResponse, errorType ErrorType, err error)
	Stop() error
}

type InternalReceiveRequest struct {
	StreamId       string
	TimeoutSeconds int

	// Will be implemented in phase2, currently not used
	ReadFromDB    bool
	DbResumeToken string
}

type InternalReceiveResponse struct {
	// Unique identifier for the output
	OutputUuid uuid.UUID
	// The received output data as JSON object
	Output OutputType
	// When the output was generated
	Timestamp time.Time
}

type InternalSendRequest struct {
	OutputUuid                  string
	StreamId                    string
	Output                      OutputType
	Timestamp                   time.Time
	InMemoryStreamSize          int
	BlockingWriteTimeoutSeconds int
}
