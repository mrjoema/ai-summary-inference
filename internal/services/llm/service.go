package llm

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"ai-search-service/internal/config"
	"ai-search-service/internal/logger"
	"ai-search-service/internal/monitoring"
	pb "ai-search-service/proto"
)

// LLMService implements the gRPC LLMOrchestratorService
type LLMService struct {
	pb.UnimplementedLLMOrchestratorServiceServer
	orchestrator   *LLMOrchestrator
	config         *config.Config
	activeRequests map[string]*RequestTracker
	requestsMutex  sync.RWMutex
	streamingChans map[string]chan *pb.LLMStreamResponse
	streamMutex    sync.RWMutex
}

// RequestTracker tracks the status of individual requests
type RequestTracker struct {
	RequestID     string
	Status        string // pending, processing, completed, failed
	QueuePosition int32
	CreatedAt     time.Time
	CompletedAt   *time.Time
	Error         string
	Response      *LLMResponse
}

// NewLLMService creates a new LLM service
func NewLLMService(cfg *config.Config) (*LLMService, error) {
	// Create LLM orchestrator with direct gRPC streaming
	orchestrator, err := NewLLMOrchestrator(
		cfg.GetInferenceAddress(),
		cfg.LLM.MaxWorkers, // Now used as max concurrent requests
		nil, // Will be set after service creation
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM orchestrator: %w", err)
	}

	service := &LLMService{
		orchestrator:   orchestrator,
		config:         cfg,
		activeRequests: make(map[string]*RequestTracker),
		streamingChans: make(map[string]chan *pb.LLMStreamResponse),
	}

	// Set the service reference in orchestrator
	orchestrator.service = service

	// Start the orchestrator
	orchestrator.Start()

	// Start request cleanup
	go service.cleanupOldRequests()

	return service, nil
}

// ProcessRequest handles incoming LLM processing requests
func (s *LLMService) ProcessRequest(ctx context.Context, req *pb.LLMRequest) (*pb.LLMResponse, error) {
	log := logger.GetLogger()
	start := time.Now()

	log.Infof("Processing LLM request %s", req.Id)

	// Check if request already exists
	s.requestsMutex.Lock()
	if _, exists := s.activeRequests[req.Id]; exists {
		s.requestsMutex.Unlock()
		return &pb.LLMResponse{
			Id:       req.Id,
			Error:    "Request ID already exists",
			Complete: true,
		}, nil
	}

	// Create request tracker
	tracker := &RequestTracker{
		RequestID: req.Id,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	s.activeRequests[req.Id] = tracker
	s.requestsMutex.Unlock()

	// Convert proto request to internal request
	llmReq := &LLMRequest{
		ID:        req.Id,
		Text:      req.Text,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
		CreatedAt: time.Unix(req.CreatedAt, 0),
	}

	// Process the request directly via orchestrator
	result, err := s.orchestrator.ProcessRequest(llmReq)
	if err != nil {
		s.requestsMutex.Lock()
		tracker.Status = "failed"
		tracker.Error = err.Error()
		s.requestsMutex.Unlock()

		log.Errorf("Failed to process request %s: %v", req.Id, err)
		return &pb.LLMResponse{
			Id:       req.Id,
			Error:    fmt.Sprintf("Failed to process request: %v", err),
			Complete: true,
		}, nil
	}

	// Update tracker status
	s.requestsMutex.Lock()
	tracker.Status = "processing"
	s.requestsMutex.Unlock()

	// For non-streaming requests, return the result directly
	if !req.Stream {
		monitoring.RecordRequest("llm", "process_request", "success")
		monitoring.RecordRequestDuration("llm", "process_request", time.Since(start))
		
		return &pb.LLMResponse{
			Id:       result.ID,
			Tokens:   result.Tokens,
			Summary:  result.Summary,
			Error:    result.Error,
			Complete: result.Complete,
		}, nil
	}

	// For streaming requests, return immediately with pending status
	monitoring.RecordRequest("llm", "process_request", "success")
	monitoring.RecordRequestDuration("llm", "process_request", time.Since(start))

	return &pb.LLMResponse{
		Id:       req.Id,
		Complete: false,
	}, nil
}

// GetStatus returns the status of a request
func (s *LLMService) GetStatus(ctx context.Context, req *pb.LLMStatusRequest) (*pb.LLMStatusResponse, error) {
	// First check local tracker
	s.requestsMutex.RLock()
	tracker, exists := s.activeRequests[req.RequestId]
	s.requestsMutex.RUnlock()

	if !exists {
		// Check orchestrator for active processing
		processor, orchestratorExists := s.orchestrator.GetRequestStatus(req.RequestId)
		if !orchestratorExists {
			return &pb.LLMStatusResponse{
				RequestId: req.RequestId,
				Status:    "not_found",
			}, nil
		}
		
		// Use orchestrator status
		return &pb.LLMStatusResponse{
			RequestId:         req.RequestId,
			Status:            processor.Status,
			QueuePosition:     0, // No queue in direct processing
			EstimatedWaitTime: 0,
			Error:             "",
		}, nil
	}

	// Calculate position based on active requests
	stats := s.orchestrator.GetStats()
	activeRequests, _ := stats["active_requests"].(int)
	maxConcurrent, _ := stats["max_concurrent"].(int)

	var queuePosition int32
	var estimatedWaitTime int32

	if tracker.Status == "pending" || tracker.Status == "processing" {
		queuePosition = int32(activeRequests)
		// Rough estimate: 10 seconds per active request
		if maxConcurrent > 0 {
			estimatedWaitTime = int32(activeRequests * 10)
		}
	}

	return &pb.LLMStatusResponse{
		RequestId:         req.RequestId,
		Status:            tracker.Status,
		QueuePosition:     queuePosition,
		EstimatedWaitTime: estimatedWaitTime,
		Error:             tracker.Error,
	}, nil
}

// HealthCheck returns the health status of the LLM service
func (s *LLMService) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	stats := s.orchestrator.GetStats()
	activeRequests, _ := stats["active_requests"].(int)
	maxConcurrent, _ := stats["max_concurrent"].(int)
	utilization, _ := stats["utilization_percent"].(float64)

	status := "healthy"
	if utilization > 90.0 { // 90% utilization threshold
		status = "degraded"
	}
	if activeRequests >= maxConcurrent {
		status = "overloaded"
	}

	return &pb.HealthCheckResponse{
		Status:    status,
		Service:   "llm-orchestrator",
		Timestamp: time.Now().Unix(),
	}, nil
}

// cleanupOldRequests removes completed requests older than 1 hour
func (s *LLMService) cleanupOldRequests() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.requestsMutex.Lock()
			cutoff := time.Now().Add(-1 * time.Hour)
			var toDelete []string

			for id, tracker := range s.activeRequests {
				if tracker.Status == "completed" || tracker.Status == "failed" {
					if tracker.CompletedAt != nil && tracker.CompletedAt.Before(cutoff) {
						toDelete = append(toDelete, id)
					}
				}
			}

			for _, id := range toDelete {
				delete(s.activeRequests, id)
			}

			if len(toDelete) > 0 {
				logger.GetLogger().Infof("Cleaned up %d old LLM requests", len(toDelete))
			}
			s.requestsMutex.Unlock()
		}
	}
}

// UpdateRequestStatus updates the status of a request (for non-streaming requests only)
func (s *LLMService) UpdateRequestStatus(requestID string, status string, response *LLMResponse, err error) {
	s.requestsMutex.Lock()
	defer s.requestsMutex.Unlock()

	if tracker, exists := s.activeRequests[requestID]; exists {
		tracker.Status = status
		if response != nil {
			tracker.Response = response
		}
		if err != nil {
			tracker.Error = err.Error()
		}
		if status == "completed" || status == "failed" {
			now := time.Now()
			tracker.CompletedAt = &now
		}
	}
}

// StreamRequest handles streaming LLM requests
func (s *LLMService) StreamRequest(req *pb.LLMRequest, stream pb.LLMOrchestratorService_StreamRequestServer) error {
	log := logger.GetLogger()
	log.Infof("Starting streaming request %s", req.Id)

	// Create streaming channel
	streamChan := make(chan *pb.LLMStreamResponse, 100)
	s.streamMutex.Lock()
	s.streamingChans[req.Id] = streamChan
	s.streamMutex.Unlock()

	// Cleanup on exit
	defer func() {
		s.streamMutex.Lock()
		delete(s.streamingChans, req.Id)
		s.streamMutex.Unlock()
		close(streamChan)
	}()

	// Start processing in background
	go func() {
		// Create request tracker
		s.requestsMutex.Lock()
		tracker := &RequestTracker{
			RequestID: req.Id,
			Status:    "processing",
			CreatedAt: time.Now(),
		}
		s.activeRequests[req.Id] = tracker
		s.requestsMutex.Unlock()

		// Convert proto request to internal request
		llmReq := &LLMRequest{
			ID:        req.Id,
			Text:      req.Text,
			MaxTokens: req.MaxTokens,
			Stream:    true,
			CreatedAt: time.Unix(req.CreatedAt, 0),
		}

		// Create callback function for streaming
		streamCallback := func(requestID, token string, isFinal bool, position int32) {
			streamChan <- &pb.LLMStreamResponse{
				Id:       requestID,
				Token:    token,
				IsFinal:  isFinal,
				Position: position,
			}
		}

		// Process via orchestrator streaming method (direct, no ProcessRequest)
		err := s.orchestrator.ProcessStreamingRequest(llmReq, streamCallback)
		if err != nil {
			streamChan <- &pb.LLMStreamResponse{
				Id:      req.Id,
				Token:   "",
				IsFinal: true,
				Error:   err.Error(),
			}
		}
	}()

	// Stream responses to client
	for {
		select {
		case response, ok := <-streamChan:
			if !ok {
				return nil
			}
			
			if err := stream.Send(response); err != nil {
				log.Errorf("Failed to send stream response: %v", err)
				return err
			}
			
			if response.IsFinal {
				return nil
			}
			
		case <-stream.Context().Done():
			log.Infof("Stream context cancelled for request %s", req.Id)
			return stream.Context().Err()
		}
	}
}

// SendStreamChunk is deprecated - streaming now uses direct callbacks
// This method is kept for compatibility but should not be used
func (s *LLMService) SendStreamChunk(requestID, token string, isFinal bool, position int32) {
	// This method is no longer needed - streaming uses direct callbacks
	logger.GetLogger().Warnf("SendStreamChunk is deprecated - use streaming callbacks instead")
}

// Stop gracefully shuts down the service
func (s *LLMService) Stop() {
	log.Println("Stopping LLM service...")
	s.orchestrator.Stop()
	log.Println("LLM service stopped")
}
