// Copyright 2025 IWorkflow.io
// SPDX-License-Identifier: BUSL-1.1

package service

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iworkflowio/async-output-service/config"
	"github.com/iworkflowio/async-output-service/membership"
	"github.com/iworkflowio/async-output-service/service/log"
	"github.com/iworkflowio/async-output-service/service/log/tag"
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
	logger     log.Logger
}

func NewService(config *config.Config, logger log.Logger) *Service {
	return &Service{
		config: config,
		logger: logger,
	}
}

func (s *Service) Start() {
	s.logger.Info("Starting service")

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
		s.logger.Error("Failed to bootstrap", tag.Error(err))
	}

	go func() {
		ginEngine.Run(s.config.NodeConfig.GetHTTPBindAddrPort())
	}()

	s.logger.Info("Service started")
}

func (s *Service) Stop() {
	s.logger.Info("Stopping service")
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
	membership, err := membership.NewNodeMembershipImpl(s.config, s.logger)
	if err != nil {
		return err
	}
	s.membership = membership
	return s.membership.Start()
}
