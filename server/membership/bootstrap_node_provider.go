package membership

import (
	"fmt"

	"github.com/iworkflowio/ipubsub/config"
)

func NewBootstrapNodeProvider(config *config.Config) *BootstrapNodeProvider {
	return &BootstrapNodeProvider{
		config: config,
	}
}

type BootstrapNodeProvider struct {
	config *config.Config
}

var (
	localOverrideForBootstrapNodesForTests = []string{}
)

func (b *BootstrapNodeProvider) GetBootstrapNodes() ([]string, error) {
	if len(localOverrideForBootstrapNodesForTests) > 0 {
		return localOverrideForBootstrapNodesForTests, nil
	}

	if b.config.ClusterConfig.BootstrapType == "static" {
		return b.config.ClusterConfig.StaticBootstrapNodeAddrPorts, nil
	}

	return nil, fmt.Errorf("bootstrap type not supported: %s", b.config.ClusterConfig.BootstrapType)
}

// for testing only
func setLocalOverrideForBootstrapNodesForTests(bootstrapNodes []string) {
	localOverrideForBootstrapNodesForTests = bootstrapNodes
}