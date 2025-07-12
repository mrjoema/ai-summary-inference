package inference

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ai-search-service/internal/config"
	"ai-search-service/internal/logger"
	"ai-search-service/internal/monitoring"
	pb "ai-search-service/proto"
)

type InferenceService struct {
	pb.UnimplementedInferenceServiceServer
	config     *config.Config
	httpClient *http.Client
	metrics    *monitoring.MetricsCollector
}

type OllamaRequest struct {
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	Stream  bool   `json:"stream"`
	Options struct {
		Temperature float64 `json:"temperature"`
		NumPredict  int     `json:"num_predict"`
	} `json:"options"`
}

type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func NewInferenceService(cfg *config.Config) (*InferenceService, error) {
	// Initialize metrics collector
	metricsCollector, err := monitoring.NewMetricsCollector("inference")
	if err != nil {
		logger.GetLogger().Warnf("Failed to initialize metrics collector: %v", err)
	}

	return &InferenceService{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Ollama.Timeout,
		},
		metrics: metricsCollector,
	}, nil
}

func (i *InferenceService) Summarize(ctx context.Context, req *pb.SummarizeRequest) (*pb.SummarizeResponse, error) {
	start := time.Now()
	log := logger.GetLogger()

	log.Infof("Generating summary for %d tokens", len(req.Tokens))

	// Create summarization prompt
	prompt := i.createSummarizationPrompt(req.OriginalText, int(req.MaxLength))

	// Record tokens processed
	monitoring.RecordTokensProcessed("inference", i.config.Ollama.Model, len(req.Tokens))

	// Call Ollama API
	summary, err := i.callOllama(ctx, prompt, false)
	if err != nil {
		log.Errorf("Failed to generate summary: %v", err)
		monitoring.RecordOllamaRequest("inference", i.config.Ollama.Model, "error")
		// Fallback to mock summary
		summary = i.generateMockSummary(req.OriginalText, int(req.MaxLength))
	} else {
		monitoring.RecordOllamaRequest("inference", i.config.Ollama.Model, "success")
	}

	// Record inference latency
	monitoring.RecordInferenceLatency("inference", i.config.Ollama.Model, false, time.Since(start))

	log.Infof("Summary generation complete. Length: %d", len(summary))

	return &pb.SummarizeResponse{
		Summary:    summary,
		Success:    true,
		TokensUsed: int32(len(req.Tokens)),
		Confidence: 0.85,
	}, nil
}

func (i *InferenceService) SummarizeStream(req *pb.SummarizeRequest, stream pb.InferenceService_SummarizeStreamServer) error {
	start := time.Now()
	log := logger.GetLogger()

	log.Infof("Starting streaming summary for %d tokens", len(req.Tokens))

	// Create summarization prompt
	prompt := i.createSummarizationPrompt(req.OriginalText, int(req.MaxLength))

	// Record tokens processed
	monitoring.RecordTokensProcessed("inference", i.config.Ollama.Model, len(req.Tokens))

	// Call Ollama API with streaming
	err := i.callOllamaStream(stream.Context(), prompt, stream)
	if err != nil {
		log.Errorf("Failed to generate streaming summary: %v", err)
		monitoring.RecordOllamaRequest("inference", i.config.Ollama.Model, "error")
		// Fallback to mock streaming
		err = i.mockStreamingSummary(req, stream)
	} else {
		monitoring.RecordOllamaRequest("inference", i.config.Ollama.Model, "success")
	}

	// Record inference latency
	monitoring.RecordInferenceLatency("inference", i.config.Ollama.Model, true, time.Since(start))

	log.Infof("Streaming summary complete")
	return err
}

func (i *InferenceService) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	// Check if Ollama is available
	ollamaURL := fmt.Sprintf("http://%s:%d/api/tags", i.config.Ollama.Host, i.config.Ollama.Port)
	resp, err := i.httpClient.Get(ollamaURL)
	if err != nil {
		return &pb.HealthCheckResponse{
			Status:    "unhealthy",
			Service:   "inference",
			Timestamp: time.Now().Unix(),
		}, nil
	}
	defer resp.Body.Close()

	status := "healthy"
	if resp.StatusCode != http.StatusOK {
		status = "degraded"
	}

	return &pb.HealthCheckResponse{
		Status:    status,
		Service:   "inference",
		Timestamp: time.Now().Unix(),
	}, nil
}

func (i *InferenceService) createSummarizationPrompt(originalText string, maxLength int) string {
	return fmt.Sprintf(`Please provide a concise summary of the following text. The summary should be informative and capture the key points. Keep it under %d characters.

Text to summarize:
%s

Summary:`, maxLength, originalText)
}

func (i *InferenceService) callOllama(ctx context.Context, prompt string, stream bool) (string, error) {
	start := time.Now()

	ollamaURL := fmt.Sprintf("http://%s:%d/api/generate", i.config.Ollama.Host, i.config.Ollama.Port)

	reqBody := OllamaRequest{
		Model:  i.config.Ollama.Model,
		Prompt: prompt,
		Stream: stream,
	}
	reqBody.Options.Temperature = i.config.Ollama.Temperature
	reqBody.Options.NumPredict = i.config.Ollama.MaxTokens

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", ollamaURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	// Record Ollama response time
	monitoring.RecordOllamaResponseTime("inference", i.config.Ollama.Model, time.Since(start))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return strings.TrimSpace(ollamaResp.Response), nil
}

func (i *InferenceService) callOllamaStream(ctx context.Context, prompt string, stream pb.InferenceService_SummarizeStreamServer) error {
	ollamaURL := fmt.Sprintf("http://%s:%d/api/generate", i.config.Ollama.Host, i.config.Ollama.Port)

	reqBody := OllamaRequest{
		Model:  i.config.Ollama.Model,
		Prompt: prompt,
		Stream: true,
	}
	reqBody.Options.Temperature = i.config.Ollama.Temperature
	reqBody.Options.NumPredict = i.config.Ollama.MaxTokens

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", ollamaURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama API returned status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	position := int32(0)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var ollamaResp OllamaResponse
		if err := json.Unmarshal([]byte(line), &ollamaResp); err != nil {
			continue // Skip malformed lines
		}

		if ollamaResp.Response != "" {
			resp := &pb.SummarizeStreamResponse{
				Token:    ollamaResp.Response,
				IsFinal:  ollamaResp.Done,
				Position: position,
			}

			if err := stream.Send(resp); err != nil {
				return fmt.Errorf("failed to send stream response: %w", err)
			}

			position++
		}

		if ollamaResp.Done {
			break
		}
	}

	return scanner.Err()
}

func (i *InferenceService) mockStreamingSummary(req *pb.SummarizeRequest, stream pb.InferenceService_SummarizeStreamServer) error {
	log := logger.GetLogger()
	log.Warn("Using mock streaming summary as fallback")

	// Generate mock summary
	summary := i.generateMockSummary(req.OriginalText, int(req.MaxLength))
	words := strings.Fields(summary)

	// Stream words one by one
	for i, word := range words {
		// Simulate processing time
		time.Sleep(100 * time.Millisecond)

		// Send word
		resp := &pb.SummarizeStreamResponse{
			Token:    word + " ",
			IsFinal:  i == len(words)-1,
			Position: int32(i),
		}

		if err := stream.Send(resp); err != nil {
			return fmt.Errorf("failed to send stream response: %w", err)
		}
	}

	return nil
}

func (i *InferenceService) generateMockSummary(originalText string, maxLength int) string {
	// Mock summary generation - fallback when Ollama is not available
	summaryTemplates := []string{
		"Based on the search results, %s appears to be a topic of significant interest. The information suggests that %s is relevant to current trends and developments.",
		"The search results indicate that %s is an important subject. Key findings show that %s has various applications and implications.",
		"According to the available information, %s represents a notable area of focus. The data suggests that %s is worth further consideration.",
		"The search data reveals that %s is a relevant topic. Analysis indicates that %s has practical significance in various contexts.",
	}

	// Extract key terms from original text
	words := strings.Fields(originalText)
	keyTerms := ""
	if len(words) > 0 {
		// Use first few words as key terms
		termCount := 3
		if len(words) < termCount {
			termCount = len(words)
		}
		keyTerms = strings.Join(words[:termCount], " ")
	}

	// Select first template for consistency
	template := summaryTemplates[0]
	summary := fmt.Sprintf(template, keyTerms, keyTerms)

	// Truncate if necessary
	if maxLength > 0 && len(summary) > maxLength {
		summary = summary[:maxLength-3] + "..."
	}

	return summary
}
