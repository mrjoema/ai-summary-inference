package tokenizer

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"ai-search-service/internal/config"
	"ai-search-service/internal/logger"
	pb "ai-search-service/proto"
)

type TokenizerService struct {
	pb.UnimplementedTokenizerServiceServer
	config *config.Config
}

func NewTokenizerService(cfg *config.Config) (*TokenizerService, error) {
	return &TokenizerService{
		config: cfg,
	}, nil
}

func (t *TokenizerService) Tokenize(ctx context.Context, req *pb.TokenizeRequest) (*pb.TokenizeResponse, error) {
	log := logger.GetLogger()

	log.Infof("Tokenizing text with length: %d", len(req.Text))

	// Basic text preprocessing
	text := t.preprocessText(req.Text)

	// Simple word-based tokenization (in production, use proper tokenizer like tiktoken)
	tokens := t.simpleTokenize(text)

	// Truncate if necessary
	maxLength := int(req.MaxLength)
	if maxLength > 0 && len(tokens) > maxLength {
		tokens = tokens[:maxLength]
		// Reconstruct truncated text
		text = t.detokenize(tokens)
	}

	log.Infof("Tokenization complete. Token count: %d", len(tokens))

	return &pb.TokenizeResponse{
		Tokens:        tokens,
		TokenCount:    int32(len(tokens)),
		TruncatedText: text,
		Success:       true,
	}, nil
}

func (t *TokenizerService) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{
		Status:    "healthy",
		Service:   "tokenizer",
		Timestamp: time.Now().Unix(),
	}, nil
}

func (t *TokenizerService) preprocessText(text string) string {
	// Remove extra whitespace
	text = strings.TrimSpace(text)

	// Normalize whitespace
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	// Remove control characters
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, text)

	return text
}

func (t *TokenizerService) simpleTokenize(text string) []int32 {
	// Simple word-based tokenization
	// In production, use proper tokenizer like tiktoken or sentencepiece

	words := strings.Fields(text)
	tokens := make([]int32, 0, len(words))

	for _, word := range words {
		// Convert word to simple hash-based token ID
		tokenID := t.wordToTokenID(word)
		tokens = append(tokens, tokenID)
	}

	return tokens
}

func (t *TokenizerService) wordToTokenID(word string) int32 {
	// Simple hash function to convert word to token ID
	// In production, use proper vocabulary mapping

	hash := int32(0)
	for _, char := range word {
		hash = hash*31 + int32(char)
	}

	// Ensure positive token ID
	if hash < 0 {
		hash = -hash
	}

	// Keep token IDs in reasonable range
	return hash % 50000
}

func (t *TokenizerService) detokenize(tokens []int32) string {
	// Simple detokenization - in production, use proper vocabulary
	words := make([]string, len(tokens))
	for i, token := range tokens {
		words[i] = fmt.Sprintf("token_%d", token)
	}
	return strings.Join(words, " ")
}
