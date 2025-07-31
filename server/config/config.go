package config

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"

	"gopkg.in/yaml.v2"
)

type (
	Config struct {
		NodeConfig NodeConfig `yaml:"node"`
		ClusterConfig ClusterConfig `yaml:"cluster"`
		MatchConfig MatchConfig `yaml:"match"`
		HashRingConfig HashRingConfig `yaml:"hash_ring"`
		// TODO: add Gin engine config to customize the HTTP server using gin
	}
)

type NodeConfig struct {
	// NodeName is the name of the node.
	NodeName string `yaml:"node_name"`
	// NodeNameFromEnv is the name of the node from environment variable.
	// If NodeNameFromEnv is not set, NodeName will be used.
	// NodeName is used to identify the node in the cluster that cannot be duplicated.
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
	StaticBootstrapNodeAddrPorts []string `yaml:"static_bootstrap_node_addr_ports"`
	// DynamicBootstrapParam is the parameter for dynamic bootstrap.
	// For aws_alb, it is the ARN of the aws application load balancer.
	// For k8s_service_pods, it is the name of the service.
	// TODO add config for other bootstrap types

	// BootstrapTimeout is the timeout for the bootstrap process.
	// To avoid network partitioning at startup, we require majority of the nodes to be connected(including self)
	// within the timeout.
	// Default is 60s
	BootstrapTimeoutSeconds int `yaml:"bootstrap_timeout_seconds"`
	// RefreshInterval is the interval for refreshing the membership information.
	// Default is 30s + 10% jitter
	RefreshIntervalSeconds int `yaml:"refresh_interval_seconds"`
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

func LoadConfig(configPath string) (*Config, error) {
	log.Printf("Loading configFile=%v\n", configPath)

	config := &Config{}

	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	d := yaml.NewDecoder(file)

	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

func (nc *NodeConfig) GetHTTPBindAddrPort() string {
	if nc.HttpBindAddrPortFromEnv != "" {
		return os.Getenv(nc.HttpAdvertiseAddrPortFromEnv)
	}
	return nc.HttpBindAddrPort
}

func (nc *NodeConfig) GetGossipBindAddrPort() (string, int, error) {
	adrr, port, err := net.SplitHostPort(nc.GossipBindAddrPort)
	if err != nil {
		return "", 0, fmt.Errorf("failed to split host port: %s, %w", nc.GossipBindAddrPort, err)
	}
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, fmt.Errorf("failed to convert port to int: %w", err)
	}
	return adrr, portInt, nil
}

func (nc *NodeConfig) GetGossipAdvertiseAddrPort() (string, int, error) {
	adrr, port, err := net.SplitHostPort(nc.GossipAdvertiseAddrPort)
	if err != nil {
		return "", 0, fmt.Errorf("failed to split host port: %s, %w", nc.GossipAdvertiseAddrPort, err)
	}
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, fmt.Errorf("failed to convert port to int: %w", err)
	}
	return adrr, portInt, nil
}