// Copyright 2025 IWorkflow.io
// SPDX-License-Identifier: BUSL-1.1

package service

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/iworkflowio/ipubsub/config"
	"github.com/iworkflowio/ipubsub/engine"
	"github.com/iworkflowio/ipubsub/genapi"
	"github.com/iworkflowio/ipubsub/membership"
	"github.com/iworkflowio/ipubsub/service/log"
	"github.com/iworkflowio/ipubsub/service/log/tag"
)

const (
	SEND_API_PATH    = "/api/v1/streams/send"
	RECEIVE_API_PATH = "/api/v1/streams/receive"
)

const DefaultReceiveTimeoutSeconds = 30

type Service struct {
	config *config.Config
	logger log.Logger

	membership membership.NodeMembership
	engine     engine.MatchingEngine
	hashring   membership.Hashring
}

func NewService(config *config.Config, logger log.Logger) *Service {
	ms, err := membership.NewNodeMembershipImpl(config, logger)
	if err != nil {
		panic(err)
	}

	engine := engine.NewInMemoryMatchingEngine()
	hashring := membership.NewHashring(logger, config.HashRingConfig.VirtualNodes)

	return &Service{
		config:     config,
		logger:     logger,
		membership: ms,
		engine:     engine,
		hashring:   hashring,
	}
}

func (s *Service) Start() {
	s.logger.Info("Starting service")

	ginEngine := gin.Default()
	ginEngine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	ginEngine.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"hello": "This is the iPubSub service"})
	})
	ginEngine.POST(SEND_API_PATH, s.handleSend)
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

func (s *Service) bootstrap() error {
	err := s.engine.Start()
	if err != nil {
		return err
	}

	return s.membership.Start()
}

func (s *Service) handleSend(c *gin.Context) {
	var req genapi.SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request schema"})
		return
	}

	nodes, err := s.membership.GetAllNodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get all nodes"})
		return
	}

	ownerNode, err := s.hashring.GetNodeForStreamId(req.StreamId, s.membership.GetVersion(), nodes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get hashring node"})
		return
	}

	// default to 0 if not set to avoid NPE
	if req.InMemoryStreamSize == nil {
		req.InMemoryStreamSize = genapi.PtrInt32(0)
	}
	if req.BlockingSendTimeoutSeconds == nil {
		req.BlockingSendTimeoutSeconds = genapi.PtrInt32(0)
	}

	if ownerNode.IsSelf {
		// owner node, handle the request locally
		errType, err := s.engine.Send(&engine.InternalSendRequest{
			StreamId:                   req.StreamId,
			Message:                    req.Message,
			MessageUuid:                req.MessageUuid,
			Timestamp:                  time.Now(),
			InMemoryStreamSize:         int(*req.InMemoryStreamSize),
			BlockingSendTimeoutSeconds: int(*req.BlockingSendTimeoutSeconds),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send"})
			return
		}
		if errType == engine.ErrorTypeWaitingTimeout {
			c.JSON(http.StatusFailedDependency, gin.H{"error": "long poll waiting timeout"})
			return
		}
		if errType == engine.ErrorTypeInvalidRequest {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request schema"})
			return
		}
		if errType != engine.ErrorTypeNone {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send for " + errType.String()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"errorType": errType})
	} else {
		// forward the request to the owner node
		apiClient := genapi.NewAPIClient(&genapi.Configuration{
			Servers: []genapi.ServerConfiguration{
				{
					URL: ownerNode.Addr,
				},
			},
		})
		request := apiClient.DefaultAPI.ApiV1StreamsSendPost(c.Request.Context())
		request.SendRequest(req)
		httpResponse, err := request.Execute()
		if err != nil || httpResponse.StatusCode != http.StatusOK {
			bodyMsg, _ := io.ReadAll(httpResponse.Body)
			c.JSON(httpResponse.StatusCode, gin.H{"error": string(bodyMsg) +
				" " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": "true"})
	}
}

func (s *Service) handleReceive(c *gin.Context) {
	// get streamId from query params
	streamId := c.Query("streamId")
	if streamId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "streamId is required"})
		return
	}

	timeoutSecondsStr := c.Query("timeoutSeconds")
	readFromDB := c.Query("readFromDB")
	if readFromDB != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "not supported yet"})
		return
	}

	var timeoutSeconds int
	if timeoutSecondsStr == "" {
		timeoutSeconds = DefaultReceiveTimeoutSeconds
	}
	timeoutSeconds, err := strconv.Atoi(timeoutSecondsStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid timeoutSeconds"})
		return
	}

	nodes, err := s.membership.GetAllNodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get all nodes"})
		return
	}

	ownerNode, err := s.hashring.GetNodeForStreamId(streamId, s.membership.GetVersion(), nodes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get hashring node"})
		return
	}

	if ownerNode.IsSelf {
		// owner node, handle the request locally
		resp, errType, err := s.engine.Receive(&engine.InternalReceiveRequest{
			StreamId:       streamId,
			TimeoutSeconds: timeoutSeconds,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to receive"})
			return
		}
		if errType == engine.ErrorTypeWaitingTimeout {
			c.JSON(http.StatusFailedDependency, gin.H{"error": "long poll waiting timeout"})
			return
		}
		if errType != engine.ErrorTypeNone {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to receive for " + errType.String()})
			return
		}

		// Convert internal response to API response
		messageUuidStr := resp.MessageUuid.String()
		apiResponse := &genapi.ReceiveResponse{
			MessageUuid: &messageUuidStr,
			Message:     resp.Message,
			Timestamp:   resp.Timestamp,
		}
		c.JSON(http.StatusOK, apiResponse)
	} else {
		// forward the request to the owner node

		apiClient := genapi.NewAPIClient(&genapi.Configuration{
			// TODO add headers to tell how many jumps we have done
			Servers: []genapi.ServerConfiguration{
				{
					URL: ownerNode.Addr,
				},
			},
		})
		request := apiClient.DefaultAPI.ApiV1StreamsReceiveGet(c.Request.Context())
		request.StreamId(streamId)
		request.TimeoutSeconds(int32(timeoutSeconds))
		response, httpResponse, err := request.Execute()
		if err != nil || httpResponse.StatusCode != http.StatusOK {
			bodyMsg, _ := io.ReadAll(httpResponse.Body)
			c.JSON(httpResponse.StatusCode, gin.H{"error": string(bodyMsg) +
				" " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, response)
	}

}
