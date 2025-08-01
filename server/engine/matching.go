package engine

import "github.com/iworkflowio/async-output-service/genapi/go"

// OutputType is the type of the output data
// a shortcut for the output data type
type OutputType = map[string]interface{}

type MatchingEngine interface {
	Start() error
	Send(genapi.SendRequest) (error)
	Receive(ReceiveRequest) (resp *genapi.ReceiveResponse, isTimeout bool, err error)
	Stop() error
}

type ReceiveRequest struct {
	StreamId string
	TimeoutSeconds int

	// Will be implemented in phase2, currently not used
	ReadFromDB bool
	DbResumeToken string
}
