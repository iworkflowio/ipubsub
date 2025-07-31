package membership

import "github.com/iworkflowio/async-output-service/config"

func NewStreamMembershipByHashicorpMemberlist(config *config.Config) StreamMembership {
	return &streamMembershipImpl{
		config: config,
	}
}

type streamMembershipImpl struct {
	config *config.Config
}

func (s *streamMembershipImpl) Start() error {
	return nil
}

func (s *streamMembershipImpl) Stop() error {
	return nil
}

func (s *streamMembershipImpl) GetAllNodes() ([]NodeInfo, error) {
	return nil, nil
}