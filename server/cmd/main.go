// Copyright 2025 IWorkflow.io
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"flag"
	rawLog "log"

	"os"
	"os/signal"
	"syscall"

	"github.com/iworkflowio/ipubsub/config"
	"github.com/iworkflowio/ipubsub/service"
	"github.com/iworkflowio/ipubsub/service/log/loggerimpl"
	"github.com/iworkflowio/ipubsub/service/log/tag"
)

func main() {
	configPath := flag.String("config", "", "path to config file, e.g. --path config/single-node.yaml, --path config/multi-node-1.yaml, --path config/multi-node-2.yaml, --path config/multi-node-3.yaml")
	flag.Parse()

	if *configPath == "" {
		rawLog.Fatalf("config path is required, e.g. --path config/single-node.yaml, --path config/multi-node-1.yaml, --path config/multi-node-2.yaml, --path config/multi-node-3.yaml")
	}

	config, err := config.LoadConfig(*configPath)
	if err != nil {
		rawLog.Fatalf("Failed to load config: %v", err)
	}

	zapLogger, err := config.Log.NewZapLogger()
	if err != nil {
		rawLog.Fatalf("Unable to create a new zap logger %v", err)
	}
	logger := loggerimpl.NewLogger(zapLogger).WithTags(tag.NodeName(config.NodeConfig.NodeName))

	svc := service.NewService(config, logger)

	svc.Start()

	// Listen for shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for system signals to shutdown
	sig := <-sigCh
	logger.Info("Received signal %v, shutting down...", tag.Value(sig))

	svc.Stop()

	logger.Info("Server stopped")

}
