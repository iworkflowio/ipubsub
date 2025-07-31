package config

type (
	Config struct {
		NodeConfig NodeConfig `yaml:"node"`
		ClusterConfig ClusterConfig `yaml:"cluster"`
		MatchConfig MatchConfig `yaml:"match"`
		HashRingConfig HashRingConfig `yaml:"hash_ring"`
	}
)

type NodeConfig struct {
	// NodeName is the name of the node.
	NodeName string `yaml:"node_name"`
	// NodeNameFromEnv is the name of the node from environment variable.
	// If NodeNameFromEnv is not set, NodeName will be used.
	NodeNameFromEnv string `yaml:"node_name_from_env"`
	// GossipBindAddrPort is the bind address and port for gossip.
	GossipBindAddrPort string `yaml:"gossip_bind_addr_port"`
	// GossipBindAddrPortFromEnv is the bind address and port for gossip from environment variable.
	// If GossipBindAddrPortFromEnv is not set, GossipBindAddrPort will be used.
	GossipBindAddrPortFromEnv string `yaml:"gossip_bind_addr_port_from_env"`
	// GossipAdvertiseAddrPort is the advertise address and port for gossip.
	// Advertise address is the address that will be used to advertise the node to the cluster.
	// So it should be the address of the node that is accessible from other nodes.
	GossipAdvertiseAddrPort string `yaml:"gossip_advertise_addr_port"`
	// GossipAdvertiseAddrPortFromEnv is the advertise address and port for gossip from environment variable.
	// If GossipAdvertiseAddrPortFromEnv is not set, GossipAdvertiseAddrPort will be used.
	GossipAdvertiseAddrPortFromEnv string `yaml:"gossip_advertise_addr_port_from_env"`
	// HttpBindAddrPort is the bind address and port for http.
	HttpBindAddrPort string `yaml:"http_bind_addr_port"`
	// HttpBindAddrPortFromEnv is the bind address and port for http from environment variable.
	// If HttpBindAddrPortFromEnv is not set, HttpBindAddrPort will be used.
	HttpBindAddrPortFromEnv string `yaml:"http_bind_addr_port_from_env"`
	// HttpAdvertiseAddrPort is the advertise address and port for http.
	// Advertise address is the address that will be used to advertise the node to the cluster.
	// So it should be the address of the node that is accessible from other nodes.
	HttpAdvertiseAddrPort string `yaml:"http_advertise_addr_port"`
	// HttpAdvertiseAddrPortFromEnv is the advertise address and port for http from environment variable.
	// If HttpAdvertiseAddrPortFromEnv is not set, HttpAdvertiseAddrPort will be used.
	HttpAdvertiseAddrPortFromEnv string `yaml:"http_advertise_addr_port_from_env"`
}

type ClusterConfig struct {
	// ClusterName is the name of the cluster.
	ClusterName string `yaml:"cluster_name"`
	// BootstrapType is the type of the bootstrap.
	// It can be "static" 
	// Or "aws_alb" for getting the nodes behind a aws application load balancer
	// Or "k8s_service_pods" for getting the pods behind a kubernetes service
	BootstrapType string `yaml:"bootstrap_type"`
	// StarticBootstrapNodeAddrPorts is the addresses and ports of the bootstrap nodes for static bootstrap.
	// It is used to bootstrap the cluster.
	StaticBootstrapNodeAddrPorts []string `yaml:"bootstrap_node_addr_ports"`
	// DynamicBootstrapParam is the parameter for dynamic bootstrap.
	// For aws_alb, it is the ARN of the aws application load balancer.
	// For k8s_service_pods, it is the name of the service.
	DynamicBootstrapParam string `yaml:"dynamic_bootstrap_param"`
}

type MatchConfig struct {
	// ReceiveDefaultTimeout is the default timeout for receiving a message.
	// It is used to set the timeout for receiving a message.
	// Default is 30 seconds.
	ReceiveDefaultTimeout string `yaml:"receive_default_timeout"`
	// SendDefaultTimeout is the default timeout for sending a message.
	// It is used to set the timeout for sending a message.
	// Default is 30 seconds.
	SendDefaultTimeout string `yaml:"send_default_timeout"`	
}

type HashRingConfig struct {
	// VirtualNodes is the number of virtual nodes for each physical node.
	// Default is 100.
	VirtualNodes int `yaml:"virtual_nodes"`
}

