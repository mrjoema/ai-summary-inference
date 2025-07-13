package llm

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	pb "ai-search-service/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// LLMRequest represents a request for LLM processing
type LLMRequest struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	MaxTokens int32     `json:"max_tokens"`
	Stream    bool      `json:"stream"`
	CreatedAt time.Time `json:"created_at"`
}

// LLMResponse represents the response from LLM processing
type LLMResponse struct {
	ID       string   `json:"id"`
	Tokens   []string `json:"tokens,omitempty"`
	Summary  string   `json:"summary,omitempty"`
	Error    string   `json:"error,omitempty"`
	Complete bool     `json:"complete"`
}

// LLMOrchestrator manages enterprise tokenization and inference services
type LLMOrchestrator struct {
	tokenizerClient pb.TokenizerServiceClient  // Enterprise tokenizer
	inferenceClient pb.InferenceServiceClient

	// Request tracking for streaming
	activeRequests map[string]*RequestProcessor
	requestsMutex  sync.RWMutex

	// Backpressure configuration
	maxConcurrentRequests int
	requestTimeout        time.Duration

	// Service integration
	service *LLMService
	
	// Shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// RequestProcessor handles individual streaming requests
type RequestProcessor struct {
	ID        string
	Ctx       context.Context
	Cancel    context.CancelFunc
	Status    string // processing, completed, failed
	Result    *LLMResponse
	Error     error
	CreatedAt time.Time
}

// NewLLMOrchestrator creates a new enterprise LLM orchestrator with tokenization
func NewLLMOrchestrator(
	tokenizerAddr string,
	inferenceAddr string,
	maxConcurrentRequests int,
	service *LLMService,
) (*LLMOrchestrator, error) {
	// Connect to enterprise tokenizer service
	tokenizerConn, err := grpc.Dial(tokenizerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to tokenizer: %w", err)
	}

	// Connect to inference service
	inferenceConn, err := grpc.Dial(inferenceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to inference: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	orchestrator := &LLMOrchestrator{
		tokenizerClient:       pb.NewTokenizerServiceClient(tokenizerConn),
		inferenceClient:       pb.NewInferenceServiceClient(inferenceConn),
		activeRequests:        make(map[string]*RequestProcessor),
		maxConcurrentRequests: maxConcurrentRequests,
		requestTimeout:        time.Minute * 5,
		service:               service,
		ctx:                   ctx,
		cancel:                cancel,
	}

	return orchestrator, nil
}

// Start initializes the orchestrator (no workers needed for direct streaming)
func (o *LLMOrchestrator) Start() {
	log.Printf("Starting LLM orchestrator with direct gRPC streaming (max concurrent: %d)", o.maxConcurrentRequests)
	// No background workers needed - processing is done on-demand via direct gRPC calls
}

// Stop gracefully shuts down the orchestrator
func (o *LLMOrchestrator) Stop() {
	log.Println("Stopping LLM orchestrator...")
	o.cancel()
	
	// Cancel all active requests
	o.requestsMutex.Lock()
	for _, processor := range o.activeRequests {
		processor.Cancel()
	}
	o.requestsMutex.Unlock()
	
	log.Println("LLM orchestrator stopped")
}

// ProcessRequest processes a NON-STREAMING request directly via gRPC
func (o *LLMOrchestrator) ProcessRequest(req *LLMRequest) (*LLMResponse, error) {
	if req.Stream {
		return nil, fmt.Errorf("use ProcessStreamingRequest for streaming requests")
	}

	// Check concurrent request limit
	o.requestsMutex.RLock()
	activeCount := len(o.activeRequests)
	o.requestsMutex.RUnlock()

	if activeCount >= o.maxConcurrentRequests {
		return nil, fmt.Errorf("too many concurrent requests (%d/%d)", activeCount, o.maxConcurrentRequests)
	}

	// Create request processor
	ctx, cancel := context.WithTimeout(o.ctx, o.requestTimeout)
	processor := &RequestProcessor{
		ID:        req.ID,
		Ctx:       ctx,
		Cancel:    cancel,
		Status:    "processing",
		CreatedAt: time.Now(),
	}

	// Track the request
	o.requestsMutex.Lock()
	o.activeRequests[req.ID] = processor
	o.requestsMutex.Unlock()

	// Process immediately
	go o.processLLMRequest(processor, req)

	log.Printf("Processing non-streaming LLM request %s (active: %d/%d)", req.ID, activeCount+1, o.maxConcurrentRequests)

	// Wait for completion
	return o.waitForCompletion(req.ID)
}

// ProcessStreamingRequest processes a STREAMING request directly
func (o *LLMOrchestrator) ProcessStreamingRequest(req *LLMRequest, streamCallback func(string, string, bool, int32)) error {
	// Check concurrent request limit
	o.requestsMutex.RLock()
	activeCount := len(o.activeRequests)
	o.requestsMutex.RUnlock()

	if activeCount >= o.maxConcurrentRequests {
		return fmt.Errorf("too many concurrent requests (%d/%d)", activeCount, o.maxConcurrentRequests)
	}

	// Create request processor
	ctx, cancel := context.WithTimeout(o.ctx, o.requestTimeout)
	processor := &RequestProcessor{
		ID:        req.ID,
		Ctx:       ctx,
		Cancel:    cancel,
		Status:    "processing",
		CreatedAt: time.Now(),
	}

	// Track the request
	o.requestsMutex.Lock()
	o.activeRequests[req.ID] = processor
	o.requestsMutex.Unlock()

	log.Printf("Processing streaming LLM request %s (active: %d/%d)", req.ID, activeCount+1, o.maxConcurrentRequests)

	// Process streaming directly
	go o.processStreamingLLMRequest(processor, req, streamCallback)

	return nil
}

// GetRequestStatus retrieves the current status of a request
func (o *LLMOrchestrator) GetRequestStatus(requestID string) (*RequestProcessor, bool) {
	o.requestsMutex.RLock()
	defer o.requestsMutex.RUnlock()
	
	processor, exists := o.activeRequests[requestID]
	return processor, exists
}

// waitForCompletion waits for a non-streaming request to complete
func (o *LLMOrchestrator) waitForCompletion(requestID string) (*LLMResponse, error) {
	for {
		select {
		case <-o.ctx.Done():
			return nil, fmt.Errorf("context cancelled")
		default:
			processor, exists := o.GetRequestStatus(requestID)
			if !exists {
				return nil, fmt.Errorf("request not found")
			}

			switch processor.Status {
			case "completed":
				// Clean up the request
				o.requestsMutex.Lock()
				delete(o.activeRequests, requestID)
				o.requestsMutex.Unlock()
				return processor.Result, nil
				
			case "failed":
				// Clean up the request
				o.requestsMutex.Lock()
				delete(o.activeRequests, requestID)
				o.requestsMutex.Unlock()
				return nil, processor.Error
				
			default:
				// Still processing, wait a bit
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

// processLLMRequest handles NON-STREAMING LLM processing via direct gRPC
func (o *LLMOrchestrator) processLLMRequest(processor *RequestProcessor, req *LLMRequest) {
	defer func() {
		// Clean up on completion - for non-streaming, we wait for result so don't delete here
		processor.Cancel()
	}()

	// CLEAN TOKEN-NATIVE FLOW: tokenize → inference → detokenize
	
	// Step 1: Call tokenizer service to tokenize input text
	tokenizeResp, err := o.performTokenization(processor.Ctx, req.Text, "facebook/bart-large-cnn", req.MaxTokens)
	if err != nil {
		log.Printf("Tokenization failed for request %s: %v", req.ID, err)
		processor.Status = "failed"
		processor.Error = fmt.Errorf("tokenization failed: %w", err)
		return
	}

	log.Printf("Step 1 complete - Tokenization: %d tokens (%.2fms, %s)", 
		tokenizeResp.TokenCount, tokenizeResp.ProcessingTimeMs, tokenizeResp.CacheStatus)

	// Step 2: Call inference service with token IDs
	inferenceResp, err := o.performInference(processor.Ctx, req, tokenizeResp.TokenIds, tokenizeResp.ModelUsed)
	if err != nil {
		log.Printf("Inference failed for request %s: %v", req.ID, err)
		processor.Status = "failed"
		processor.Error = fmt.Errorf("inference failed: %w", err)
		return
	}

	log.Printf("Step 2 complete - Inference: generated summary")

	// Step 3: Call tokenizer service to detokenize generated tokens (if any)
	finalSummary := inferenceResp.Summary
	if len(inferenceResp.GeneratedTokenIds) > 0 {
		detokenizeResp, err := o.performDetokenization(processor.Ctx, inferenceResp.GeneratedTokenIds, tokenizeResp.ModelUsed)
		if err != nil {
			log.Printf("Detokenization failed for request %s: %v, using fallback text", req.ID, err)
			// Use the summary text as fallback
		} else {
			log.Printf("Step 3 complete - Detokenization: %d chars", len(detokenizeResp.Text))
			finalSummary = detokenizeResp.Text
		}
	}

	// Complete response
	processor.Status = "completed"
	processor.Result = &LLMResponse{
		ID:       req.ID,
		Summary:  finalSummary,
		Complete: true,
	}
}

// processStreamingLLMRequest handles STREAMING LLM processing via direct gRPC
func (o *LLMOrchestrator) processStreamingLLMRequest(processor *RequestProcessor, req *LLMRequest, streamCallback func(string, string, bool, int32)) {
	defer func() {
		// Clean up on completion - for streaming, delete immediately
		o.requestsMutex.Lock()
		delete(o.activeRequests, req.ID)
		o.requestsMutex.Unlock()
		processor.Cancel()
	}()

	// CLEAN TOKEN-NATIVE STREAMING FLOW: tokenize → inference → detokenize (streaming)
	
	// Step 1: Call tokenizer service to tokenize input text
	tokenizeResp, err := o.performTokenization(processor.Ctx, req.Text, "facebook/bart-large-cnn", req.MaxTokens)
	if err != nil {
		log.Printf("Tokenization failed for streaming request %s: %v", req.ID, err)
		processor.Status = "failed"
		processor.Error = fmt.Errorf("tokenization failed: %w", err)
		streamCallback(req.ID, "", true, 0) // Send error signal
		return
	}

	log.Printf("Step 1 complete - Streaming tokenization: %d tokens (%.2fms, %s)", 
		tokenizeResp.TokenCount, tokenizeResp.ProcessingTimeMs, tokenizeResp.CacheStatus)

	// Step 2: Call inference service for streaming with token IDs
	o.performStreamingInference(processor, req, streamCallback, tokenizeResp.TokenIds, tokenizeResp.ModelUsed)
}

// performTokenization calls the tokenizer service to tokenize text
func (o *LLMOrchestrator) performTokenization(ctx context.Context, text, modelName string, maxTokens int32) (*pb.TokenizeResponse, error) {
	// Build complete prompt for summarization
	completePrompt := o.buildSummarizationPrompt(text)
	log.Printf("Complete prompt: '%s' (max tokens: %d)", completePrompt, maxTokens)
	return o.tokenizerClient.Tokenize(ctx, &pb.TokenizeRequest{
		Text:                  completePrompt,
		ModelName:            modelName,
		MaxTokens:            maxTokens,
		IncludeSpecialTokens: true,
		RequestId:            fmt.Sprintf("llm_%d", time.Now().UnixNano()),
	})
}

// performInference calls the inference service with token IDs
func (o *LLMOrchestrator) performInference(ctx context.Context, req *LLMRequest, tokenIds []int32, modelName string) (*pb.SummarizeResponse, error) {
	// Create inference request with tokens as primary input
	inferenceReq := &pb.SummarizeRequest{
		TokenIds:  tokenIds,
		ModelName: modelName,
		MaxLength: req.MaxTokens,
		Streaming: false,
		RequestId: req.ID,
	}
	
	log.Printf("Calling inference service with %d tokens", len(tokenIds))
	
	return o.inferenceClient.Summarize(ctx, inferenceReq)
}

// performDetokenization calls the tokenizer service to detokenize token IDs
func (o *LLMOrchestrator) performDetokenization(ctx context.Context, tokenIds []int32, modelName string) (*pb.DetokenizeResponse, error) {
	return o.tokenizerClient.Detokenize(ctx, &pb.DetokenizeRequest{
		TokenIds:          tokenIds,
		ModelName:         modelName,
		SkipSpecialTokens: true, // Skip special tokens for clean output
		RequestId:         fmt.Sprintf("detok_%d", time.Now().UnixNano()),
	})
}

// buildSummarizationPrompt constructs the complete prompt for tokenization
func (o *LLMOrchestrator) buildSummarizationPrompt(searchResults string) string {
	// BART-optimized prompt - minimal instructions, focus on content
	// BART works best with direct content for summarization
	return searchResults
}

// performStreamingInference handles streaming inference via direct gRPC with tokens
func (o *LLMOrchestrator) performStreamingInference(processor *RequestProcessor, req *LLMRequest, streamCallback func(string, string, bool, int32), tokenIds []int32, modelName string) {
	// Create streaming inference request with tokens as input
	inferenceReq := &pb.SummarizeRequest{
		TokenIds:  tokenIds,
		ModelName: modelName,
		MaxLength: req.MaxTokens,
		Streaming: true,
		RequestId: req.ID,
	}
	
	log.Printf("Starting streaming inference with %d tokens", len(tokenIds))

	stream, err := o.inferenceClient.SummarizeStream(processor.Ctx, inferenceReq)
	if err != nil {
		processor.Status = "failed"
		processor.Error = fmt.Errorf("streaming inference failed: %w", err)
		streamCallback(req.ID, "", true, 0) // Send error
		return
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				// Stream complete - send final callback to signal completion
				processor.Status = "completed"
				streamCallback(req.ID, "", true, 0) // Signal final completion
				return
			}
			processor.Status = "failed"
			processor.Error = fmt.Errorf("streaming error: %w", err)
			streamCallback(req.ID, "", true, 0) // Send error
			return
		}

		// TOKEN-NATIVE STREAMING: Detokenize token ID if available
		finalToken := resp.Token
		if resp.GeneratedTokenId != 0 && !resp.IsFinal {
			// Call detokenizer for this single token ID
			detokenizeResp, err := o.performDetokenization(processor.Ctx, []int32{resp.GeneratedTokenId}, modelName)
			if err != nil {
				log.Printf("Streaming detokenization failed for token %d: %v, using fallback", resp.GeneratedTokenId, err)
				// Keep using the fallback token text from inference service
			} else {
				// Use the properly detokenized text
				finalToken = detokenizeResp.Text
				log.Printf("Detokenized streaming token %d: '%s'", resp.GeneratedTokenId, finalToken)
			}
		}

		// Send token via callback (either detokenized or fallback)
		streamCallback(req.ID, finalToken, resp.IsFinal, resp.Position)

		if resp.IsFinal {
			processor.Status = "completed"
			break
		}
	}
}


// GetStats returns orchestrator statistics
func (o *LLMOrchestrator) GetStats() map[string]interface{} {
	o.requestsMutex.RLock()
	activeRequests := len(o.activeRequests)
	
	// Count by status
	processing := 0
	completed := 0
	failed := 0
	
	for _, processor := range o.activeRequests {
		switch processor.Status {
		case "processing":
			processing++
		case "completed":
			completed++
		case "failed":
			failed++
		}
	}
	o.requestsMutex.RUnlock()

	return map[string]interface{}{
		"active_requests":        activeRequests,
		"max_concurrent":         o.maxConcurrentRequests,
		"processing_requests":    processing,
		"completed_requests":     completed,
		"failed_requests":        failed,
		"utilization_percent":    float64(activeRequests) / float64(o.maxConcurrentRequests) * 100,
	}
}
