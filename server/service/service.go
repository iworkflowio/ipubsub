// Copyright 2025 IWorkflow.io
// SPDX-License-Identifier: BUSL-1.1

package service

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iworkflowio/async-output-service/config"
	"github.com/iworkflowio/async-output-service/membership"
)

const (
	SEND_API_PATH           = "/api/v1/streams/send"
	RECEIVE_API_PATH        = "/api/v1/streams/receive"
	SEND_AND_STORE_API_PATH = "/api/v1/streams/sendAndStore"
)

type Service struct {
	config     *config.Config
	ginEngine  *gin.Engine
	membership membership.NodeMembership
}

func NewService(config *config.Config) *Service {
	return &Service{
		config: config,
	}
}

func (s *Service) Start() {
	log.Printf("Starting service")

	ginEngine := gin.Default()
	ginEngine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	ginEngine.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"hello": "This is the async output service"})
	})
	ginEngine.POST(SEND_API_PATH, s.handleSend)
	ginEngine.POST(SEND_AND_STORE_API_PATH, s.handleSendAndStore)
	ginEngine.GET(RECEIVE_API_PATH, s.handleReceive)

	err := s.bootstrap()
	if err != nil {
		log.Fatalf("Failed to bootstrap: %v", err)
	}

	go func() {
		ginEngine.Run(s.config.NodeConfig.GetHTTPBindAddrPort())
	}()

	log.Printf("Service started")
}

func (s *Service) Stop() {
	log.Printf("Stopping service")
}

func (s *Service) handleSend(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Service) handleSendAndStore(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Service) handleReceive(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Service) bootstrap() error {
	membership, err := membership.NewStreamMembership(s.config)
	if err != nil {
		return err
	}
	s.membership = membership
	return s.membership.Start()
}
