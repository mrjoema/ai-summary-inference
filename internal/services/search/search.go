package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ai-search-service/internal/config"
	"ai-search-service/internal/logger"
	pb "ai-search-service/proto"
)

type SearchService struct {
	pb.UnimplementedSearchServiceServer
	config     *config.Config
	httpClient *http.Client
}

type GoogleSearchResponse struct {
	Items []GoogleSearchItem `json:"items"`
	Error *GoogleError       `json:"error,omitempty"`
}

type GoogleSearchItem struct {
	Title        string `json:"title"`
	Link         string `json:"link"`
	Snippet      string `json:"snippet"`
	DisplayLink  string `json:"displayLink"`
	FormattedUrl string `json:"formattedUrl"`
}

type GoogleError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewSearchService(cfg *config.Config) (*SearchService, error) {
	return &SearchService{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (s *SearchService) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	log := logger.GetLogger()

	log.Infof("Performing search for query: %s", req.Query)

	// Check if Google API credentials are configured
	if s.config.Google.APIKey == "" || s.config.Google.CX == "" {
		log.Warn("Google API credentials not configured, using mock data")
		return s.getMockSearchResults(req), nil
	}

	// Perform actual Google search
	results, err := s.performGoogleSearch(ctx, req)
	if err != nil {
		log.Errorf("Google search failed: %v", err)
		return &pb.SearchResponse{
			Success: false,
			Error:   fmt.Sprintf("Search failed: %v", err),
		}, nil
	}

	return results, nil
}

func (s *SearchService) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{
		Status:    "healthy",
		Service:   "search",
		Timestamp: time.Now().Unix(),
	}, nil
}

func (s *SearchService) performGoogleSearch(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	// Build Google Custom Search API URL
	baseURL := "https://www.googleapis.com/customsearch/v1"
	params := url.Values{}
	params.Add("key", s.config.Google.APIKey)
	params.Add("cx", s.config.Google.CX)
	params.Add("q", req.Query)
	params.Add("num", fmt.Sprintf("%d", req.NumResults))

	if req.SafeSearch {
		params.Add("safe", "active")
	}

	searchURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Perform request
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var googleResp GoogleSearchResponse
	if err := json.Unmarshal(body, &googleResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API errors
	if googleResp.Error != nil {
		return nil, fmt.Errorf("Google API error: %s", googleResp.Error.Message)
	}

	// Convert to protobuf format
	results := make([]*pb.SearchResult, len(googleResp.Items))
	for i, item := range googleResp.Items {
		results[i] = &pb.SearchResult{
			Title:      s.sanitizeText(item.Title),
			Url:        item.Link,
			Snippet:    s.sanitizeText(item.Snippet),
			DisplayUrl: item.DisplayLink,
		}
	}

	return &pb.SearchResponse{
		Results: results,
		Query:   req.Query,
		Success: true,
	}, nil
}

func (s *SearchService) getMockSearchResults(req *pb.SearchRequest) *pb.SearchResponse {
	// Generate mock results for testing
	mockResults := []*pb.SearchResult{
		{
			Title:      fmt.Sprintf("Mock Result 1 for '%s'", req.Query),
			Url:        "https://example.com/1",
			Snippet:    fmt.Sprintf("This is a mock search result for the query '%s'. It contains relevant information about the topic.", req.Query),
			DisplayUrl: "example.com",
		},
		{
			Title:      fmt.Sprintf("Mock Result 2 for '%s'", req.Query),
			Url:        "https://example.com/2",
			Snippet:    fmt.Sprintf("Another mock search result for '%s' with additional context and information.", req.Query),
			DisplayUrl: "example.com",
		},
		{
			Title:      fmt.Sprintf("Mock Result 3 for '%s'", req.Query),
			Url:        "https://example.com/3",
			Snippet:    fmt.Sprintf("Third mock result providing more details about '%s' and related topics.", req.Query),
			DisplayUrl: "example.com",
		},
	}

	// Limit results based on request
	numResults := int(req.NumResults)
	if numResults == 0 {
		numResults = 3
	}
	if numResults > len(mockResults) {
		numResults = len(mockResults)
	}

	return &pb.SearchResponse{
		Results: mockResults[:numResults],
		Query:   req.Query,
		Success: true,
	}
}

func (s *SearchService) sanitizeText(text string) string {
	// Basic text sanitization
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	// Remove multiple spaces
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}

	return text
}
