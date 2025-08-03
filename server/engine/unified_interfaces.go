package engine

import (
	"time"

	"github.com/google/uuid"
)

// MessageType is the type of the message data
// a shortcut for the message data type
type MessageType = interface{}

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

func (e ErrorType) String() string {
	switch e {
	case ErrorTypeNone:
		return "None"
	case ErrorTypeUnknown:
		return "Unknown"
	case ErrorTypeInvalidRequest:
		return "InvalidRequest"
	case ErrorTypeCircularBufferIterationLimit:
		return "CircularBufferIterationLimit"
	case ErrorTypeWaitingTimeout:
		return "WaitingTimeout"
	case ErrorTypeStreamStopped:
		return "StreamStopped"
	case ErrorTypeStreamFull:
		return "StreamFull"
	case ErrorTypeStreamEmpty:
		return "StreamEmpty"
	default:
		return "UnknownErrorType"
	}
}

type MatchingEngine interface {
	Start() error
	Send(*InternalSendRequest) (errorType ErrorType, err error)
	Receive(*InternalReceiveRequest) (resp *InternalReceiveResponse, errorType ErrorType, err error)
	Stop() error
}

type InMemoeryStream interface {
	Send(message MessageType, messageUuid uuid.UUID, timestamp time.Time, blockingSendTimeoutSeconds int) (errorType ErrorType, err error)
	Receive(timeoutSeconds int) (message *InternalReceiveResponse, errorType ErrorType, err error)
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
	// Unique identifier for the message
	MessageUuid uuid.UUID
	// The received message data as JSON object
	Message MessageType
	// When the message was generated
	Timestamp time.Time
}

type InternalSendRequest struct {
	MessageUuid                string
	StreamId                   string
	Message                    MessageType
	Timestamp                  time.Time
	InMemoryStreamSize         int
	BlockingSendTimeoutSeconds int
}
