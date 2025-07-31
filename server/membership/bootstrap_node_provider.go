package membership

import (
	"fmt"

	"github.com/iworkflowio/async-output-service/config"
)

func NewBootstrapNodeProvider(config *config.Config) *BootstrapNodeProvider {
	return &BootstrapNodeProvider{
		config: config,
	}
}

type BootstrapNodeProvider struct {
	config *config.Config
}

func (b *BootstrapNodeProvider) GetBootstrapNodes() ([]string, error) {
	if b.config.ClusterConfig.BootstrapType == "static" {
		return b.config.ClusterConfig.StaticBootstrapNodeAddrPorts, nil
	}

	return nil, fmt.Errorf("bootstrap type not supported: %s", b.config.ClusterConfig.BootstrapType)
}
