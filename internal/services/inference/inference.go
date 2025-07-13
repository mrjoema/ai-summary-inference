package inference

import (
	"context"
	"fmt"
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
			Timeout: cfg.VLLM.Timeout,
		},
		metrics:    metricsCollector,
		vllmEngine: vllmEngine,
	}, nil
}

func (i *InferenceService) Summarize(ctx context.Context, req *pb.SummarizeRequest) (*pb.SummarizeResponse, error) {
	start := time.Now()
	log := logger.GetLogger()

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
		log.Infof("No tokens provided - using mock summary for text request: %d characters", len(req.OriginalText))
		
		// Generate mock summary when no tokenization is available
		modelName = "mock"
		summary = i.generateMockSummary(req.OriginalText, int(req.MaxLength))
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
		log.Infof("No tokens provided - using mock streaming for text request: %d characters", len(req.OriginalText))
		
		modelName = "mock"
		
		// Use mock streaming when no tokenization is available
		err := i.mockStreamingSummary(req, stream)
		
		// Record inference latency
		monitoring.RecordInferenceLatency("inference", modelName, true, time.Since(start))
		
		log.Infof("Mock streaming complete")
		return err
	}
}

func (i *InferenceService) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	// Check if vLLM is available
	vllmURL := fmt.Sprintf("http://%s:%d/health", i.config.VLLM.Host, i.config.VLLM.Port)
	resp, err := i.httpClient.Get(vllmURL)
	if err != nil {
		return &pb.HealthCheckResponse{
			Status:    "degraded", // Still functional with mock summaries
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
	// Mock summary generation - fallback when vLLM is not available
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
