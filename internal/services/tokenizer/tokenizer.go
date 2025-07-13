package tokenizer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ai-search-service/internal/config"
	"ai-search-service/internal/logger"
	pb "ai-search-service/proto"

	"github.com/go-redis/redis/v8"
)

// TokenizerService implements the TokenizerServiceServer interface
type TokenizerService struct {
	pb.UnimplementedTokenizerServiceServer
	cfg         *config.Config
	redisClient *redis.Client

	// In-memory vocabulary for demonstration
	vocabulary   map[string]int32
	reverseVocab map[int32]string
}

// NewTokenizerService creates a new tokenizer service
func NewTokenizerService(cfg *config.Config) (*TokenizerService, error) {
	logger.GetLogger().Info("Initializing Tokenizer Service")

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.GetRedisAddress(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.GetLogger().Warn("Redis connection failed, continuing without cache", "error", err)
	}

	// Create basic vocabulary for demonstration
	vocabulary := createBasicVocabulary()
	reverseVocab := make(map[int32]string)
	for token, id := range vocabulary {
		reverseVocab[id] = token
	}

	return &TokenizerService{
		cfg:          cfg,
		redisClient:  redisClient,
		vocabulary:   vocabulary,
		reverseVocab: reverseVocab,
	}, nil
}

// Tokenize processes a single text input into tokens
func (s *TokenizerService) Tokenize(ctx context.Context, req *pb.TokenizeRequest) (*pb.TokenizeResponse, error) {
	startTime := time.Now()

	logger.GetLogger().Info("Tokenizing text", "request_id", req.RequestId, "text_length", len(req.Text))

	// Check cache first
	cacheKey := fmt.Sprintf("tokenize:%s:%s", req.ModelName, req.Text)
	cacheStatus := "miss"

	if s.redisClient != nil {
		_, err := s.redisClient.Get(ctx, cacheKey).Result()
		if err == nil {
			cacheStatus = "hit"
			logger.GetLogger().Info("Cache hit for tokenization", "request_id", req.RequestId)

			// Parse cached result (simplified for demo)
			return &pb.TokenizeResponse{
				TokenIds:         []int32{1, 2, 3}, // Would parse from cache
				TokenStrings:     []string{"cached", "response"},
				TokenCount:       2,
				ModelUsed:        req.ModelName,
				ProcessingTimeMs: float32(time.Since(startTime).Milliseconds()),
				CacheStatus:      cacheStatus,
				Success:          true,
			}, nil
		}
	}

	// Tokenize the text
	tokens, tokenIds, truncated := s.tokenizeText(req.Text, req.MaxTokens, req.IncludeSpecialTokens)

	response := &pb.TokenizeResponse{
		TokenIds:         tokenIds,
		TokenStrings:     tokens,
		TokenCount:       int32(len(tokens)),
		TruncatedText:    req.Text,
		WasTruncated:     truncated,
		ModelUsed:        req.ModelName,
		ProcessingTimeMs: float32(time.Since(startTime).Milliseconds()),
		CacheStatus:      cacheStatus,
		Success:          true,
	}

	// Cache the result
	if s.redisClient != nil {
		s.redisClient.Set(ctx, cacheKey, "cached_result", 10*time.Minute)
	}

	logger.GetLogger().Info("Tokenization completed",
		"request_id", req.RequestId,
		"token_count", response.TokenCount,
		"processing_time_ms", response.ProcessingTimeMs,
		"cache_status", cacheStatus)

	return response, nil
}

// BatchTokenize processes multiple texts in batch
func (s *TokenizerService) BatchTokenize(ctx context.Context, req *pb.BatchTokenizeRequest) (*pb.BatchTokenizeResponse, error) {
	startTime := time.Now()

	logger.GetLogger().Info("Batch tokenizing", "batch_size", len(req.Requests))

	responses := make([]*pb.TokenizeResponse, len(req.Requests))
	cacheHits := int32(0)
	cacheMisses := int32(0)

	// Process each request
	for i, tokenReq := range req.Requests {
		resp, err := s.Tokenize(ctx, tokenReq)
		if err != nil {
			return nil, err
		}

		responses[i] = resp
		if resp.CacheStatus == "hit" {
			cacheHits++
		} else {
			cacheMisses++
		}
	}

	response := &pb.BatchTokenizeResponse{
		Responses:             responses,
		TotalProcessingTimeMs: float32(time.Since(startTime).Milliseconds()),
		CacheHits:             cacheHits,
		CacheMisses:           cacheMisses,
	}

	logger.GetLogger().Info("Batch tokenization completed",
		"batch_size", len(req.Requests),
		"total_processing_time_ms", response.TotalProcessingTimeMs,
		"cache_hits", cacheHits,
		"cache_misses", cacheMisses)

	return response, nil
}

// GetVocabularyInfo returns information about the vocabulary
func (s *TokenizerService) GetVocabularyInfo(ctx context.Context, req *pb.VocabularyInfoRequest) (*pb.VocabularyInfoResponse, error) {
	logger.GetLogger().Info("Getting vocabulary info", "model_name", req.ModelName)

	specialTokens := []string{"<PAD>", "<UNK>", "<START>", "<END>"}

	return &pb.VocabularyInfoResponse{
		VocabSize:     int32(len(s.vocabulary)),
		SpecialTokens: specialTokens,
		EncodingName:  "basic_tokenizer",
		ModelName:     req.ModelName,
	}, nil
}

// HealthCheck returns the health status of the service
func (s *TokenizerService) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	// Check Redis connection
	redisStatus := "unknown"
	if s.redisClient != nil {
		if err := s.redisClient.Ping(ctx).Err(); err == nil {
			redisStatus = "healthy"
		} else {
			redisStatus = "unhealthy"
		}
	}

	logger.GetLogger().Info("Health check", "redis_status", redisStatus)

	return &pb.HealthCheckResponse{
		Status:    fmt.Sprintf("healthy (redis: %s)", redisStatus),
		Service:   "tokenizer",
		Timestamp: time.Now().Unix(),
	}, nil
}

// tokenizeText performs basic text tokenization
func (s *TokenizerService) tokenizeText(text string, maxTokens int32, includeSpecial bool) ([]string, []int32, bool) {
	// Simple whitespace tokenization for demonstration
	words := strings.Fields(strings.ToLower(text))

	tokens := make([]string, 0, len(words))
	tokenIds := make([]int32, 0, len(words))

	// Add special start token if requested
	if includeSpecial {
		tokens = append(tokens, "<START>")
		tokenIds = append(tokenIds, s.vocabulary["<START>"])
	}

	// Tokenize words
	for _, word := range words {
		if maxTokens > 0 && int32(len(tokens)) >= maxTokens {
			break
		}

		// Get token ID from vocabulary
		tokenId, exists := s.vocabulary[word]
		if !exists {
			tokenId = s.vocabulary["<UNK>"]
			word = "<UNK>"
		}

		tokens = append(tokens, word)
		tokenIds = append(tokenIds, tokenId)
	}

	// Add special end token if requested
	if includeSpecial {
		tokens = append(tokens, "<END>")
		tokenIds = append(tokenIds, s.vocabulary["<END>"])
	}

	truncated := maxTokens > 0 && len(words) > int(maxTokens)

	return tokens, tokenIds, truncated
}

// createBasicVocabulary creates a basic vocabulary for demonstration
func createBasicVocabulary() map[string]int32 {
	vocab := map[string]int32{
		"<PAD>":   0,
		"<UNK>":   1,
		"<START>": 2,
		"<END>":   3,
	}

	// Add common words
	commonWords := []string{
		"the", "a", "an", "and", "or", "but", "in", "on", "at", "to", "for", "of", "with", "by",
		"from", "up", "about", "into", "through", "during", "before", "after", "above", "below",
		"between", "among", "is", "am", "are", "was", "were", "be", "been", "being", "have",
		"has", "had", "do", "does", "did", "will", "would", "could", "should", "may", "might",
		"must", "can", "this", "that", "these", "those", "i", "you", "he", "she", "it", "we",
		"they", "me", "him", "her", "us", "them", "my", "your", "his", "her", "its", "our",
		"their", "what", "which", "who", "when", "where", "why", "how", "all", "any", "both",
		"each", "few", "more", "most", "other", "some", "such", "no", "nor", "not", "only",
		"own", "same", "so", "than", "too", "very", "search", "query", "result", "summary",
		"text", "document", "article", "web", "page", "site", "content", "information", "data",
		"analysis", "research", "study", "report", "news", "technology", "science", "business",
		"health", "education", "government", "politics", "sports", "entertainment", "culture",
		"social", "economic", "environmental", "global", "international", "national", "local",
		"public", "private", "personal", "professional", "academic", "industrial", "commercial",
		"financial", "legal", "medical", "technical", "scientific", "educational", "historical",
		"cultural", "social", "political", "economic", "environmental", "technological",
	}

	for i, word := range commonWords {
		vocab[word] = int32(i + 4) // Start after special tokens
	}

	return vocab
}
