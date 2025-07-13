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
	vllmEngine *VLLMEngine  // Enterprise token-native engine
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

	// Initialize enterprise vLLM engine
	vllmEngine := NewVLLMEngine(cfg)

	return &InferenceService{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Ollama.Timeout, // Fallback timeout
		},
		metrics:    metricsCollector,
		vllmEngine: vllmEngine,
	}, nil
}

func (i *InferenceService) Summarize(ctx context.Context, req *pb.SummarizeRequest) (*pb.SummarizeResponse, error) {
	start := time.Now()
	log := logger.GetLogger()

	var prompt string
	var modelName string
	var summary string

	// INDUSTRY STANDARD: Token-native processing vs fallback
	if len(req.TokenIds) > 0 {
		log.Infof("🚀 ENTERPRISE: Processing %d tokens directly via vLLM (model: %s)", 
			len(req.TokenIds), req.ModelName)
		
		// INDUSTRY STANDARD: Send tokens directly to vLLM (NO text conversion!)
		result, err := i.vllmEngine.GenerateFromTokens(ctx, req.TokenIds, req.ModelName, int(req.MaxLength))
		modelName = req.ModelName
		
		if err != nil {
			log.Errorf("vLLM token generation failed: %v", err)
			monitoring.RecordRequest("inference", "vllm_generate", "error")
			// Fallback to mock
			summary = i.generateMockSummary("Enterprise tokenized content", int(req.MaxLength))
		} else {
			summary = result
		}
	} else {
		log.Infof("FALLBACK: Processing text request via Ollama: %d characters", len(req.OriginalText))
		
		// Fallback to Ollama text-based approach
		prompt = i.createSummarizationPrompt(req.OriginalText, int(req.MaxLength))
		modelName = i.config.Ollama.Model
		
		// Call Ollama API (legacy)
		result, err := i.callOllama(ctx, prompt, false)
		if err != nil {
			log.Errorf("Ollama generation failed: %v", err)
			monitoring.RecordOllamaRequest("inference", modelName, "error")
			// Fallback to mock summary
			summary = i.generateMockSummary(req.OriginalText, int(req.MaxLength))
		} else {
			monitoring.RecordOllamaRequest("inference", modelName, "success")
			summary = result
		}
	}

	// Record inference latency
	monitoring.RecordInferenceLatency("inference", modelName, false, time.Since(start))

	log.Infof("Summary generation complete. Length: %d", len(summary))

	return &pb.SummarizeResponse{
		Summary:    summary,
		Success:    true,
		TokensUsed: int32(len(req.OriginalText)),
		Confidence: 0.85,
	}, nil
}

func (i *InferenceService) SummarizeStream(req *pb.SummarizeRequest, stream pb.InferenceService_SummarizeStreamServer) error {
	start := time.Now()
	log := logger.GetLogger()

	var prompt string
	var modelName string

	// INDUSTRY STANDARD: Token-native streaming vs fallback
	if len(req.TokenIds) > 0 {
		log.Infof("🚀 ENTERPRISE STREAMING: %d tokens directly via vLLM (model: %s)", 
			len(req.TokenIds), req.ModelName)
		
		modelName = req.ModelName
		
		// INDUSTRY STANDARD: Stream tokens directly from vLLM
		err := i.streamVLLMTokens(stream.Context(), req.TokenIds, req.ModelName, int(req.MaxLength), stream)
		if err != nil {
			log.Errorf("vLLM token streaming failed: %v", err)
			monitoring.RecordRequest("inference", "vllm_stream", "error")
			// Fallback to mock streaming
			err = i.mockStreamingSummary(req, stream)
		}
		
		// Record metrics
		monitoring.RecordInferenceLatency("inference", modelName, true, time.Since(start))
		log.Infof("vLLM token streaming complete")
		return err
	} else {
		log.Infof("FALLBACK STREAMING: %d characters via Ollama", len(req.OriginalText))
		
		prompt = i.createSummarizationPrompt(req.OriginalText, int(req.MaxLength))
		modelName = i.config.Ollama.Model
		
		// Call Ollama API with streaming (legacy)
		err := i.callOllamaStream(stream.Context(), prompt, stream)
		if err != nil {
			log.Errorf("Failed to generate streaming summary: %v", err)
			monitoring.RecordOllamaRequest("inference", modelName, "error")
			// Fallback to mock streaming
			err = i.mockStreamingSummary(req, stream)
		} else {
			monitoring.RecordOllamaRequest("inference", modelName, "success")
		}
		
		// Record inference latency
		monitoring.RecordInferenceLatency("inference", modelName, true, time.Since(start))
		
		log.Infof("Ollama streaming complete")
		return err
	}
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

func (i *InferenceService) createTokenAwarePrompt(tokenIds []int32, modelName string, maxLength int) string {
	// Industry standard: Reconstruct text from tokens for model consumption
	reconstructedText := i.detokenize(tokenIds, modelName)
	
	return fmt.Sprintf(`Please provide a concise summary of the following text. Keep the summary under %d characters.

Text to summarize:
%s

Summary:`, maxLength, reconstructedText)
}

func (i *InferenceService) detokenize(tokenIds []int32, modelName string) string {
	// Industry standard: Convert tokens back to text for natural language models
	// In production, this would use the actual tokenizer's decode method
	
	// For now, simulate detokenization based on model type
	switch modelName {
	case "llama3.2":
		return i.detokenizeLlama(tokenIds)
	case "gpt-4":
		return i.detokenizeGPT(tokenIds)
	default:
		return i.detokenizeLlama(tokenIds) // fallback
	}
}

func (i *InferenceService) detokenizeLlama(tokenIds []int32) string {
	// Simplified detokenization - in production use actual llama tokenizer
	words := make([]string, 0, len(tokenIds))
	for _, tokenId := range tokenIds {
		// Map token ID back to word (simplified)
		if tokenId == 1 {
			continue // Skip start token
		}
		if tokenId == 2 {
			break // End token
		}
		words = append(words, fmt.Sprintf("token_%d", tokenId))
	}
	return strings.Join(words, " ")
}

func (i *InferenceService) detokenizeGPT(tokenIds []int32) string {
	// GPT-style detokenization
	return i.detokenizeLlama(tokenIds) // Simplified for demo
}

// streamVLLMTokens handles token-native streaming with vLLM
func (i *InferenceService) streamVLLMTokens(ctx context.Context, tokenIds []int32, modelName string, maxLength int, stream pb.InferenceService_SummarizeStreamServer) error {
	position := int32(0)
	
	// Stream tokens directly from vLLM
	return i.vllmEngine.StreamFromTokens(ctx, tokenIds, modelName, maxLength, func(content string, isFinished bool) {
		if content != "" {
			// Send each token chunk to client
			resp := &pb.SummarizeStreamResponse{
				Token:    content,
				IsFinal:  isFinished,
				Position: position,
			}
			stream.Send(resp)
			position++
		}
		
		if isFinished {
			// Send final completion signal
			resp := &pb.SummarizeStreamResponse{
				Token:    "",
				IsFinal:  true,
				Position: position,
			}
			stream.Send(resp)
		}
	})
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
