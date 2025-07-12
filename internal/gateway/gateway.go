package gateway

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
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
	redis           *redis.Client
	searchClient    pb.SearchServiceClient
	safetyClient    pb.SafetyServiceClient
	tokenizerClient pb.TokenizerServiceClient
	inferenceClient pb.InferenceServiceClient
	tasks           map[string]*SearchTask
	tasksMutex      sync.RWMutex
	metrics         *monitoring.MetricsCollector
}

type SearchTask struct {
	ID            string         `json:"id"`
	Query         string         `json:"query"`
	Status        string         `json:"status"` // pending, searching, summarizing, completed, failed
	SearchResults []SearchResult `json:"search_results"`
	Summary       string         `json:"summary"`
	Error         string         `json:"error,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	Streaming     bool           `json:"streaming"`
	StreamTokens  []string       `json:"stream_tokens,omitempty"`
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
	TaskID        string         `json:"task_id"`
	Query         string         `json:"query"`
	Status        string         `json:"status"`
	SearchResults []SearchResult `json:"search_results,omitempty"`
	Summary       string         `json:"summary,omitempty"`
	Error         string         `json:"error,omitempty"`
	Streaming     bool           `json:"streaming"`
}

func NewGateway(cfg *config.Config) (*Gateway, error) {
	// Initialize metrics collector
	metricsCollector, err := monitoring.NewMetricsCollector("gateway")
	if err != nil {
		logger.GetLogger().Warnf("Failed to initialize metrics collector: %v", err)
	}

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.GetLogger().Warnf("Redis connection failed: %v", err)
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

	tokenizerConn, err := grpc.Dial(
		fmt.Sprintf("%s:%d", cfg.Services.Tokenizer.Host, cfg.Services.Tokenizer.Port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to tokenizer service: %w", err)
	}

	inferenceConn, err := grpc.Dial(
		fmt.Sprintf("%s:%d", cfg.Services.Inference.Host, cfg.Services.Inference.Port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to inference service: %w", err)
	}

	return &Gateway{
		config:          cfg,
		redis:           redisClient,
		searchClient:    pb.NewSearchServiceClient(searchConn),
		safetyClient:    pb.NewSafetyServiceClient(safetyConn),
		tokenizerClient: pb.NewTokenizerServiceClient(tokenizerConn),
		inferenceClient: pb.NewInferenceServiceClient(inferenceConn),
		tasks:           make(map[string]*SearchTask),
		metrics:         metricsCollector,
	}, nil
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
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		monitoring.RecordRequest("gateway", "search", "error")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate task ID
	taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())

	// Create search task
	task := &SearchTask{
		ID:        taskID,
		Query:     req.Query,
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Streaming: req.Streaming,
	}

	// Store task
	g.tasksMutex.Lock()
	g.tasks[taskID] = task
	g.tasksMutex.Unlock()

	// Start search process asynchronously
	go g.processSearch(task, req)

	// Record metrics
	monitoring.RecordRequest("gateway", "search", "success")
	monitoring.RecordRequestDuration("gateway", "search", time.Since(start))

	// Return immediate response
	c.JSON(http.StatusOK, SearchResponse{
		TaskID:    taskID,
		Query:     req.Query,
		Status:    "pending",
		Streaming: req.Streaming,
	})
}

func (g *Gateway) SearchStatus(c *gin.Context) {
	taskID := c.Param("taskId")

	g.tasksMutex.RLock()
	task, exists := g.tasks[taskID]
	g.tasksMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	response := SearchResponse{
		TaskID:        task.ID,
		Query:         task.Query,
		Status:        task.Status,
		SearchResults: task.SearchResults,
		Summary:       task.Summary,
		Error:         task.Error,
		Streaming:     task.Streaming,
	}

	c.JSON(http.StatusOK, response)
}

func (g *Gateway) SearchStream(c *gin.Context) {
	taskID := c.Param("taskId")

	g.tasksMutex.RLock()
	task, exists := g.tasks[taskID]
	g.tasksMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	if !task.Streaming {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task is not in streaming mode"})
		return
	}

	// Set up Server-Sent Events headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")

	// Stream updates
	g.streamTaskUpdates(c, task)
}

func (g *Gateway) processSearch(task *SearchTask, req SearchRequest) {
	ctx := context.Background()
	log := logger.GetLogger()

	// Update task status
	g.updateTaskStatus(task, "validating")

	// Validate input
	safetyResp, err := g.safetyClient.ValidateInput(ctx, &pb.ValidateInputRequest{
		Text:     req.Query,
		ClientIp: "",
	})
	if err != nil {
		g.updateTaskError(task, fmt.Sprintf("Safety validation failed: %v", err))
		return
	}

	if !safetyResp.IsSafe {
		g.updateTaskError(task, "Query contains unsafe content")
		return
	}

	// Update task status
	g.updateTaskStatus(task, "searching")

	// Perform search
	searchResp, err := g.searchClient.Search(ctx, &pb.SearchRequest{
		Query:      safetyResp.SanitizedText,
		SafeSearch: req.SafeSearch,
		NumResults: int32(req.NumResults),
	})
	if err != nil {
		g.updateTaskError(task, fmt.Sprintf("Search failed: %v", err))
		return
	}

	if !searchResp.Success {
		g.updateTaskError(task, searchResp.Error)
		return
	}

	// Convert search results
	searchResults := make([]SearchResult, len(searchResp.Results))
	for i, result := range searchResp.Results {
		searchResults[i] = SearchResult{
			Title:      result.Title,
			URL:        result.Url,
			Snippet:    result.Snippet,
			DisplayURL: result.DisplayUrl,
		}
	}

	// Update task with search results
	g.tasksMutex.Lock()
	task.SearchResults = searchResults
	task.Status = "summarizing"
	task.UpdatedAt = time.Now()
	g.tasksMutex.Unlock()

	// Prepare text for summarization
	var textToSummarize string
	for _, result := range searchResults {
		textToSummarize += result.Title + " " + result.Snippet + " "
	}

	// Tokenize text
	tokenResp, err := g.tokenizerClient.Tokenize(ctx, &pb.TokenizeRequest{
		Text:      textToSummarize,
		MaxLength: 512,
	})
	if err != nil {
		log.Errorf("Tokenization failed: %v", err)
		g.updateTaskError(task, fmt.Sprintf("Tokenization failed: %v", err))
		return
	}

	if !tokenResp.Success {
		g.updateTaskError(task, tokenResp.Error)
		return
	}

	// Generate summary
	if req.Streaming {
		g.generateStreamingSummary(task, tokenResp.Tokens, textToSummarize)
	} else {
		g.generateSummary(task, tokenResp.Tokens, textToSummarize)
	}
}

func (g *Gateway) generateSummary(task *SearchTask, tokens []int32, originalText string) {
	ctx := context.Background()

	summaryResp, err := g.inferenceClient.Summarize(ctx, &pb.SummarizeRequest{
		Tokens:       tokens,
		OriginalText: originalText,
		Streaming:    false,
		MaxLength:    150,
	})

	if err != nil {
		g.updateTaskError(task, fmt.Sprintf("Summarization failed: %v", err))
		return
	}

	if !summaryResp.Success {
		g.updateTaskError(task, summaryResp.Error)
		return
	}

	// Sanitize output
	sanitizeResp, err := g.safetyClient.SanitizeOutput(ctx, &pb.SanitizeOutputRequest{
		Text: summaryResp.Summary,
	})
	if err != nil {
		logger.GetLogger().Errorf("Output sanitization failed: %v", err)
		g.updateTaskError(task, "Output sanitization failed")
		return
	}

	// Update task with summary
	g.tasksMutex.Lock()
	task.Summary = sanitizeResp.SanitizedText
	task.Status = "completed"
	task.UpdatedAt = time.Now()
	g.tasksMutex.Unlock()
}

func (g *Gateway) generateStreamingSummary(task *SearchTask, tokens []int32, originalText string) {
	ctx := context.Background()

	stream, err := g.inferenceClient.SummarizeStream(ctx, &pb.SummarizeRequest{
		Tokens:       tokens,
		OriginalText: originalText,
		Streaming:    true,
		MaxLength:    150,
	})

	if err != nil {
		g.updateTaskError(task, fmt.Sprintf("Streaming summarization failed: %v", err))
		return
	}

	var summary string
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			g.updateTaskError(task, fmt.Sprintf("Stream error: %v", err))
			return
		}

		if resp.Error != "" {
			g.updateTaskError(task, resp.Error)
			return
		}

		summary += resp.Token

		// Update task with streaming token
		g.tasksMutex.Lock()
		task.StreamTokens = append(task.StreamTokens, resp.Token)
		task.UpdatedAt = time.Now()
		g.tasksMutex.Unlock()

		if resp.IsFinal {
			break
		}
	}

	// Sanitize final output
	sanitizeResp, err := g.safetyClient.SanitizeOutput(ctx, &pb.SanitizeOutputRequest{
		Text: summary,
	})
	if err != nil {
		logger.GetLogger().Errorf("Output sanitization failed: %v", err)
		g.updateTaskError(task, "Output sanitization failed")
		return
	}

	// Update task with final summary
	g.tasksMutex.Lock()
	task.Summary = sanitizeResp.SanitizedText
	task.Status = "completed"
	task.UpdatedAt = time.Now()
	g.tasksMutex.Unlock()
}

func (g *Gateway) updateTaskStatus(task *SearchTask, status string) {
	g.tasksMutex.Lock()
	task.Status = status
	task.UpdatedAt = time.Now()
	g.tasksMutex.Unlock()
}

func (g *Gateway) updateTaskError(task *SearchTask, errorMsg string) {
	g.tasksMutex.Lock()
	task.Status = "failed"
	task.Error = errorMsg
	task.UpdatedAt = time.Now()
	g.tasksMutex.Unlock()
}

func (g *Gateway) streamTaskUpdates(c *gin.Context, task *SearchTask) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	lastTokenCount := 0

	for {
		select {
		case <-ticker.C:
			g.tasksMutex.RLock()

			// Send status update
			update := map[string]interface{}{
				"task_id":        task.ID,
				"status":         task.Status,
				"search_results": task.SearchResults,
				"summary":        task.Summary,
				"error":          task.Error,
				"updated_at":     task.UpdatedAt,
			}

			// Send new tokens if streaming
			if task.Streaming && len(task.StreamTokens) > lastTokenCount {
				newTokens := task.StreamTokens[lastTokenCount:]
				update["new_tokens"] = newTokens
				lastTokenCount = len(task.StreamTokens)
			}

			g.tasksMutex.RUnlock()

			// Send SSE event
			c.SSEvent("update", update)
			c.Writer.Flush()

			// Close connection if task is completed or failed
			if task.Status == "completed" || task.Status == "failed" {
				return
			}
		}
	}
}
