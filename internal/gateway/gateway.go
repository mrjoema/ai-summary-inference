package gateway

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"ai-search-service/internal/config"
	"ai-search-service/internal/logger"
	"ai-search-service/internal/monitoring"
	pb "ai-search-service/proto"
)

type Gateway struct {
	config          *config.Config
	searchClient    pb.SearchServiceClient
	safetyClient    pb.SafetyServiceClient
	inferenceClient pb.InferenceServiceClient
	llmClient       pb.LLMOrchestratorServiceClient
	metrics         *monitoring.MetricsCollector
}


type SearchResult struct {
	Title      string `json:"title"`
	URL        string `json:"url"`
	Snippet    string `json:"snippet"`
	DisplayURL string `json:"display_url"`
}

type SearchRequest struct {
	Query      string `json:"query" binding:"required"`
	SafeSearch bool   `json:"safe_search"`
	Streaming  bool   `json:"streaming"`
	NumResults int    `json:"num_results"`
}

type SearchResponse struct {
	Query         string         `json:"query"`
	Status        string         `json:"status"`
	SearchResults []SearchResult `json:"search_results,omitempty"`
	Summary       string         `json:"summary,omitempty"`
	Error         string         `json:"error,omitempty"`
}

func NewGateway(cfg *config.Config) (*Gateway, error) {
	// Initialize metrics collector
	metricsCollector, err := monitoring.NewMetricsCollector("gateway")
	if err != nil {
		logger.GetLogger().Warnf("Failed to initialize metrics collector: %v", err)
	}

	// Connect to LLM orchestrator service
	llmConn, err := grpc.Dial(
		cfg.GetLLMAddress(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LLM orchestrator service: %w", err)
	}

	// Initialize gRPC clients
	searchConn, err := grpc.Dial(
		fmt.Sprintf("%s:%d", cfg.Services.Search.Host, cfg.Services.Search.Port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to search service: %w", err)
	}

	safetyConn, err := grpc.Dial(
		fmt.Sprintf("%s:%d", cfg.Services.Safety.Host, cfg.Services.Safety.Port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to safety service: %w", err)
	}

	inferenceConn, err := grpc.Dial(
		fmt.Sprintf("%s:%d", cfg.Services.Inference.Host, cfg.Services.Inference.Port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to inference service: %w", err)
	}

	// Initialize gateway
	g := &Gateway{
		config:          cfg,
		searchClient:    pb.NewSearchServiceClient(searchConn),
		safetyClient:    pb.NewSafetyServiceClient(safetyConn),
		inferenceClient: pb.NewInferenceServiceClient(inferenceConn),
		llmClient:       pb.NewLLMOrchestratorServiceClient(llmConn),
		metrics:         metricsCollector,
	}

	return g, nil
}

func (g *Gateway) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "gateway",
		"timestamp": time.Now().Unix(),
	})
}

func (g *Gateway) Index(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "AI Search Engine",
	})
}

func (g *Gateway) ValidateInput(c *gin.Context) {
	var req struct {
		Text string `json:"text" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), g.config.Services.Safety.Timeout)
	defer cancel()

	resp, err := g.safetyClient.ValidateInput(ctx, &pb.ValidateInputRequest{
		Text:     req.Text,
		ClientIp: c.ClientIP(),
	})

	if err != nil {
		logger.GetLogger().Errorf("Safety validation failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Validation failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"is_safe":        resp.IsSafe,
		"sanitized_text": resp.SanitizedText,
		"warnings":       resp.Warnings,
	})
}

func (g *Gateway) Metrics(c *gin.Context) {
	promhttp.Handler().ServeHTTP(c.Writer, c.Request)
}

func (g *Gateway) Search(c *gin.Context) {
	start := time.Now()
	log := logger.GetLogger()
	
	// Debug: Log request details
	log.Infof("🔍 Search request - Method: %s, Accept: %s, ContentType: %s", 
		c.Request.Method, c.GetHeader("Accept"), c.GetHeader("Content-Type"))
	
	// Determine mode based on request method and parameters
	if c.Request.Method == "GET" {
		// GET requests with query params are streaming mode
		log.Infof("Routing to streaming mode (GET)")
		g.searchWithStreaming(c, start)
	} else if c.Request.Method == "POST" {
		// POST requests are non-streaming mode (but may use SSE)
		log.Infof("Routing to non-streaming mode (POST)")
		g.searchWithoutStreaming(c, start)
	} else {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "Method not allowed"})
	}
}

// searchWithStreaming handles streaming requests with immediate SSE response
func (g *Gateway) searchWithStreaming(c *gin.Context, start time.Time) {
	// Set SSE headers immediately
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")
	
	// Get query parameters
	query := c.Query("query")
	safeSearchStr := c.Query("safe_search")
	numResultsStr := c.Query("num_results")
	
	if query == "" {
		c.SSEvent("error", gin.H{"message": "Query parameter required"})
		return
	}
	
	// Parse parameters
	safeSearch := safeSearchStr == "true"
	numResults := 5
	if numResultsStr != "" {
		if parsed, err := strconv.Atoi(numResultsStr); err == nil {
			numResults = parsed
		}
	}
	
	// Check system capacity
	if !g.checkSystemCapacity() {
		monitoring.RecordRequest("gateway", "search", "rejected")
		c.SSEvent("error", gin.H{
			"message": "System overloaded, please try again later",
			"retry_after": 30,
		})
		return
	}
	
	// Record metrics
	monitoring.RecordRequest("gateway", "search", "success")
	monitoring.RecordRequestDuration("gateway", "search", time.Since(start))
	
	// Start processing and stream results immediately
	g.processAndStreamSearch(c, query, safeSearch, numResults)
}

// searchWithoutStreaming handles non-streaming requests with SSE (search results first, then complete summary)
func (g *Gateway) searchWithoutStreaming(c *gin.Context, start time.Time) {
	log := logger.GetLogger()
	log.Infof("📝 Non-streaming function called - parsing JSON body")
	
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorf("Failed to parse JSON body: %v", err)
		monitoring.RecordRequest("gateway", "search", "error")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	log.Infof("✅ Parsed JSON - Query: %s, SafeSearch: %t, NumResults: %d", req.Query, req.SafeSearch, req.NumResults)
	
	// Check if client wants SSE (Accept header includes text/event-stream)
	acceptHeader := c.GetHeader("Accept")
	wantsSSE := strings.Contains(acceptHeader, "text/event-stream")
	
	// Check system capacity
	if !g.checkSystemCapacity() {
		monitoring.RecordRequest("gateway", "search", "rejected")
		if wantsSSE {
			// Set SSE headers for error response
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.SSEvent("error", gin.H{
				"message": "System overloaded, please try again later",
				"retry_after": 30,
			})
		} else {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "System overloaded, please try again later",
				"retry_after": 30,
			})
		}
		return
	}
	
	if wantsSSE {
		// Set SSE headers for non-streaming mode (like streaming, but complete summary)
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no") // Disable nginx buffering
		
		// Process search with SSE events (search results first, then complete AI summary)
		numResults := req.NumResults
		if numResults == 0 {
			numResults = 5
		}
		
		g.processNonStreamingSSE(c, req.Query, req.SafeSearch, numResults)
	} else {
		// Process as regular JSON response (non-SSE mode)
		numResults := req.NumResults
		if numResults == 0 {
			numResults = 5
		}
		
		// Process the search synchronously and return JSON
		g.processNonStreamingJSON(c, req.Query, req.SafeSearch, numResults)
	}
	
	// Record metrics
	monitoring.RecordRequest("gateway", "search", "success")
	monitoring.RecordRequestDuration("gateway", "search", time.Since(start))
}

// processAndStreamSearch handles streaming search with immediate response
func (g *Gateway) processAndStreamSearch(c *gin.Context, query string, safeSearch bool, numResults int) {
	ctx := context.Background()
	log := logger.GetLogger()
	
	// 1. Send initial status
	c.SSEvent("status", gin.H{
		"type": "started",
		"query": query,
		"timestamp": time.Now().Unix(),
	})
	c.Writer.Flush()
	
	// 2. Validate input
	c.SSEvent("status", gin.H{"type": "validating"})
	c.Writer.Flush()
	
	safetyResp, err := g.safetyClient.ValidateInput(ctx, &pb.ValidateInputRequest{
		Text:       query,
		ClientIp:   c.ClientIP(),
		SafeSearch: safeSearch,
	})
	if err != nil {
		log.Errorf("Safety validation failed: %v", err)
		c.SSEvent("error", gin.H{"message": "Safety validation failed"})
		return
	}
	
	if !safetyResp.IsSafe {
		c.SSEvent("error", gin.H{"message": "Query contains unsafe content"})
		return
	}
	
	// 3. Perform search
	c.SSEvent("status", gin.H{"type": "searching"})
	c.Writer.Flush()
	
	searchResp, err := g.searchClient.Search(ctx, &pb.SearchRequest{
		Query:      safetyResp.SanitizedText,
		SafeSearch: safeSearch,
		NumResults: int32(numResults),
	})
	if err != nil {
		log.Errorf("Search failed: %v", err)
		c.SSEvent("error", gin.H{"message": "Search failed"})
		return
	}
	
	if !searchResp.Success {
		c.SSEvent("error", gin.H{"message": searchResp.Error})
		return
	}
	
	// 4. Stream search results immediately
	searchResults := make([]SearchResult, len(searchResp.Results))
	for i, result := range searchResp.Results {
		searchResults[i] = SearchResult{
			Title:      result.Title,
			URL:        result.Url,
			Snippet:    result.Snippet,
			DisplayURL: result.DisplayUrl,
		}
	}
	
	c.SSEvent("search_results", gin.H{
		"type": "search_results",
		"results": searchResults,
	})
	c.Writer.Flush()
	
	// 5. Start AI summarization
	c.SSEvent("status", gin.H{"type": "summarizing"})
	c.Writer.Flush()
	
	// Prepare text for summarization
	var textToSummarize string
	for _, result := range searchResults {
		textToSummarize += result.Title + " " + result.Snippet + " "
	}
	
	// Submit LLM request to orchestrator service
	llmReq := &pb.LLMRequest{
		Id:        fmt.Sprintf("stream_%d", time.Now().UnixNano()),
		Text:      textToSummarize,
		MaxTokens: 150,
		Stream:    true,
		CreatedAt: time.Now().Unix(),
	}
	
	// Process the request using streaming method
	ctx, cancel := context.WithTimeout(context.Background(), g.config.Services.LLM.Timeout)
	defer cancel()
	
	stream, err := g.llmClient.StreamRequest(ctx, llmReq)
	if err != nil {
		log.Errorf("Failed to start LLM stream: %v", err)
		c.SSEvent("error", gin.H{"message": "Failed to start AI summarization"})
		return
	}

	// Collect tokens for safety validation
	var completeSummary strings.Builder
	
	// Stream tokens as they arrive
	for {
		response, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				// Stream completed - validate and send final summary
				finalSummary := completeSummary.String()
				if finalSummary != "" {
					safetyCtx, safetyCancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer safetyCancel()
					
					sanitizeResp, err := g.safetyClient.SanitizeOutput(safetyCtx, &pb.SanitizeOutputRequest{
						Text: finalSummary,
					})
					if err != nil {
						log.Errorf("Streaming output sanitization failed: %v", err)
						c.SSEvent("error", gin.H{"message": "Summary sanitization failed"})
						return
					}
					
					if len(sanitizeResp.Warnings) > 0 {
						log.Warnf("Streaming AI output sanitized with warnings: %v", sanitizeResp.Warnings)
					}
					
					// Send sanitized summary if different from original
					if sanitizeResp.SanitizedText != finalSummary {
						log.Warnf("AI output was modified by safety filter")
						c.SSEvent("summary_sanitized", gin.H{
							"type": "summary_sanitized", 
							"original_length": len(finalSummary),
							"sanitized_length": len(sanitizeResp.SanitizedText),
							"warnings": sanitizeResp.Warnings,
						})
					}
				}
				
				c.SSEvent("complete", gin.H{"type": "complete"})
				return
			}
			log.Errorf("Stream error: %v", err)
			c.SSEvent("error", gin.H{"message": "Streaming error"})
			return
		}

		// Handle error in response
		if response.Error != "" {
			c.SSEvent("error", gin.H{"message": response.Error})
			return
		}

		// Send token if available and collect for safety validation
		if response.Token != "" {
			// Collect token for final safety check
			completeSummary.WriteString(response.Token)
			
			// Send token to user for real-time display
			c.SSEvent("token", gin.H{
				"type": "token",
				"token": response.Token,
				"position": response.Position,
			})
			c.Writer.Flush()
		}

		// Check if final
		if response.IsFinal {
			// Validate complete summary before finalizing
			finalSummary := completeSummary.String()
			if finalSummary != "" {
				safetyCtx, safetyCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer safetyCancel()
				
				sanitizeResp, err := g.safetyClient.SanitizeOutput(safetyCtx, &pb.SanitizeOutputRequest{
					Text: finalSummary,
				})
				if err != nil {
					log.Errorf("Streaming output sanitization failed: %v", err)
					c.SSEvent("error", gin.H{"message": "Summary sanitization failed"})
					return
				}
				
				if len(sanitizeResp.Warnings) > 0 {
					log.Warnf("Streaming AI output sanitized with warnings: %v", sanitizeResp.Warnings)
				}
				
				// Check if content was modified by safety filter
				if sanitizeResp.SanitizedText != finalSummary {
					log.Warnf("AI output was modified by safety filter - notifying user")
					c.SSEvent("summary_sanitized", gin.H{
						"type": "summary_sanitized", 
						"message": "Summary was filtered for safety",
						"warnings": sanitizeResp.Warnings,
					})
				}
			}
			
			c.SSEvent("summary", gin.H{"type": "summary"})
			c.SSEvent("complete", gin.H{"type": "complete"})
			return
		}
	}
}


// processNonStreamingSSE handles non-streaming search with SSE (search results first, then complete AI summary)
func (g *Gateway) processNonStreamingSSE(c *gin.Context, query string, safeSearch bool, numResults int) {
	ctx := context.Background()
	log := logger.GetLogger()
	
	// 1. Send initial status
	c.SSEvent("status", gin.H{
		"type": "started",
		"query": query,
		"timestamp": time.Now().Unix(),
	})
	c.Writer.Flush()
	
	// 2. Validate input
	c.SSEvent("status", gin.H{"type": "validating"})
	c.Writer.Flush()
	
	safetyResp, err := g.safetyClient.ValidateInput(ctx, &pb.ValidateInputRequest{
		Text:       query,
		ClientIp:   c.ClientIP(),
		SafeSearch: safeSearch,
	})
	if err != nil {
		log.Errorf("Safety validation failed: %v", err)
		c.SSEvent("error", gin.H{"message": "Safety validation failed"})
		return
	}
	
	if !safetyResp.IsSafe {
		c.SSEvent("error", gin.H{"message": "Query contains unsafe content"})
		return
	}
	
	// 3. Perform search
	c.SSEvent("status", gin.H{"type": "searching"})
	c.Writer.Flush()
	
	searchResp, err := g.searchClient.Search(ctx, &pb.SearchRequest{
		Query:      safetyResp.SanitizedText,
		SafeSearch: safeSearch,
		NumResults: int32(numResults),
	})
	if err != nil {
		log.Errorf("Search failed: %v", err)
		c.SSEvent("error", gin.H{"message": "Search failed"})
		return
	}
	
	if !searchResp.Success {
		c.SSEvent("error", gin.H{"message": searchResp.Error})
		return
	}
	
	// 4. IMMEDIATELY stream search results (like streaming mode)
	searchResults := make([]SearchResult, len(searchResp.Results))
	for i, result := range searchResp.Results {
		searchResults[i] = SearchResult{
			Title:      result.Title,
			URL:        result.Url,
			Snippet:    result.Snippet,
			DisplayURL: result.DisplayUrl,
		}
	}
	
	c.SSEvent("search_results", gin.H{
		"type": "search_results",
		"results": searchResults,
	})
	c.Writer.Flush()
	
	log.Infof("🔍 Non-streaming SSE: Search results sent, now generating complete AI summary...")
	
	// 5. Start AI summarization
	c.SSEvent("status", gin.H{"type": "summarizing"})
	c.Writer.Flush()
	
	// Prepare text for summarization
	var textToSummarize string
	for _, result := range searchResults {
		textToSummarize += result.Title + " " + result.Snippet + " "
	}
	
	// Submit NON-STREAMING LLM request (complete summary, not token-by-token)
	llmReq := &pb.LLMRequest{
		Id:        fmt.Sprintf("nonstream_sse_%d", time.Now().UnixNano()),
		Text:      textToSummarize,
		MaxTokens: 150,
		Stream:    false, // Key difference: complete summary at once
		CreatedAt: time.Now().Unix(),
	}
	
	// Get complete AI summary
	response, err := g.llmClient.ProcessRequest(ctx, llmReq)
	if err != nil {
		log.Errorf("Failed to process LLM request: %v", err)
		c.SSEvent("error", gin.H{"message": "AI summarization failed"})
		return
	}
	
	var summary string
	if response.Error != "" {
		log.Infof("LLM response has error: %s", response.Error)
		summary = "Summary unavailable"
	} else {
		rawSummary := response.Summary
		if rawSummary == "" {
			// Reconstruct from tokens
			for _, token := range response.Tokens {
				rawSummary += token
			}
		}
		
		// CRITICAL: Sanitize AI output before returning to user
		safetyCtx, safetyCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer safetyCancel()
		
		sanitizeResp, err := g.safetyClient.SanitizeOutput(safetyCtx, &pb.SanitizeOutputRequest{
			Text: rawSummary,
		})
		
		if err != nil {
			log.Errorf("Failed to sanitize AI output: %v", err)
			summary = "Summary sanitization failed"
		} else {
			summary = sanitizeResp.SanitizedText
		}
	}
	
	// 6. Send complete AI summary at once (not token-by-token like streaming)
	c.SSEvent("summary", gin.H{
		"type": "summary_complete", // Different type to distinguish from streaming
		"text": summary,
	})
	c.Writer.Flush()
	
	log.Infof("✅ Non-streaming SSE completed - sent search results first, then complete AI summary")
	
	// 7. Send completion signal
	c.SSEvent("complete", gin.H{"type": "complete"})
	c.Writer.Flush()
}

// processNonStreamingJSON handles non-streaming search with JSON response
func (g *Gateway) processNonStreamingJSON(c *gin.Context, query string, safeSearch bool, numResults int) {
	ctx := context.Background()
	log := logger.GetLogger()
	
	// 1. Validate input
	safetyResp, err := g.safetyClient.ValidateInput(ctx, &pb.ValidateInputRequest{
		Text:       query,
		ClientIp:   c.ClientIP(),
		SafeSearch: safeSearch,
	})
	if err != nil {
		log.Errorf("Safety validation failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Safety validation failed"})
		return
	}
	
	if !safetyResp.IsSafe {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query contains unsafe content"})
		return
	}
	
	// 2. Perform search
	searchResp, err := g.searchClient.Search(ctx, &pb.SearchRequest{
		Query:      safetyResp.SanitizedText,
		SafeSearch: safeSearch,
		NumResults: int32(numResults),
	})
	if err != nil {
		log.Errorf("Search failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed"})
		return
	}
	
	if !searchResp.Success {
		c.JSON(http.StatusInternalServerError, gin.H{"error": searchResp.Error})
		return
	}
	
	// 3. Convert search results
	searchResults := make([]SearchResult, len(searchResp.Results))
	for i, result := range searchResp.Results {
		searchResults[i] = SearchResult{
			Title:      result.Title,
			URL:        result.Url,
			Snippet:    result.Snippet,
			DisplayURL: result.DisplayUrl,
		}
	}
	
	// 4. Generate AI summary
	var textToSummarize string
	for _, result := range searchResults {
		textToSummarize += result.Title + " " + result.Snippet + " "
	}
	
	// Submit NON-STREAMING LLM request
	llmReq := &pb.LLMRequest{
		Id:        fmt.Sprintf("json_%d", time.Now().UnixNano()),
		Text:      textToSummarize,
		MaxTokens: 150,
		Stream:    false,
		CreatedAt: time.Now().Unix(),
	}
	
	// Get complete AI summary
	response, err := g.llmClient.ProcessRequest(ctx, llmReq)
	if err != nil {
		log.Errorf("Failed to process LLM request: %v", err)
		c.JSON(http.StatusOK, SearchResponse{
			Query:         query,
			Status:        "completed",
			SearchResults: searchResults,
			Summary:       "AI summarization failed",
		})
		return
	}
	
	var summary string
	if response.Error != "" {
		log.Infof("LLM response has error: %s", response.Error)
		summary = "Summary unavailable"
	} else {
		rawSummary := response.Summary
		if rawSummary == "" {
			// Reconstruct from tokens
			for _, token := range response.Tokens {
				rawSummary += token
			}
		}
		
		// Sanitize AI output
		sanitizeResp, err := g.safetyClient.SanitizeOutput(ctx, &pb.SanitizeOutputRequest{
			Text: rawSummary,
		})
		
		if err != nil {
			log.Errorf("Failed to sanitize AI output: %v", err)
			summary = "Summary sanitization failed"
		} else {
			summary = sanitizeResp.SanitizedText
		}
	}
	
	// 5. Return complete response
	c.JSON(http.StatusOK, SearchResponse{
		Query:         query,
		Status:        "completed",
		SearchResults: searchResults,
		Summary:       summary,
	})
}

// checkSystemCapacity checks if the system can handle more requests
func (g *Gateway) checkSystemCapacity() bool {
	// Simple capacity check - can be enhanced with metrics
	// For now, we'll rely on the LLM service's internal backpressure
	// The service will return appropriate errors when overloaded
	// TODO: Add health check to LLM service for more sophisticated backpressure
	return true
}
