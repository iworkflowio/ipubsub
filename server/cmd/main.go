// Copyright 2025 IWorkflow.io
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"github.com/iworkflowio/async-output-service/config"
	"github.com/iworkflowio/async-output-service/service"
)

func main() {
	configPath := flag.String("config", "", "path to config file, e.g. --path config/single-node.yaml, --path config/multi-node-1.yaml, --path config/multi-node-2.yaml, --path config/multi-node-3.yaml")
	flag.Parse()

	if *configPath == "" {
		log.Fatalf("config path is required, e.g. --path config/single-node.yaml, --path config/multi-node-1.yaml, --path config/multi-node-2.yaml, --path config/multi-node-3.yaml")
	}

	config, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	svc := service.NewService(config)

	svc.Start()

	// Listen for shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for system signals to shutdown
	sig := <-sigCh
	log.Printf("Received signal %v, shutting down...", sig)

	svc.Stop()

	log.Printf("Server stopped")

}
