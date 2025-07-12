package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type SearchRequest struct {
	Query      string `json:"query"`
	SafeSearch bool   `json:"safe_search"`
	Streaming  bool   `json:"streaming"`
	NumResults int    `json:"num_results"`
}

type SearchResponse struct {
	TaskID    string `json:"task_id"`
	Query     string `json:"query"`
	Status    string `json:"status"`
	Streaming bool   `json:"streaming"`
}

type StatusResponse struct {
	TaskID        string         `json:"task_id"`
	Query         string         `json:"query"`
	Status        string         `json:"status"`
	SearchResults []SearchResult `json:"search_results"`
	Summary       string         `json:"summary"`
	Error         string         `json:"error"`
	Streaming     bool           `json:"streaming"`
}

type SearchResult struct {
	Title      string `json:"title"`
	URL        string `json:"url"`
	Snippet    string `json:"snippet"`
	DisplayURL string `json:"display_url"`
}

const baseURL = "http://localhost:8080"

func main() {
	fmt.Println("🔍 AI Search Engine Performance Test")
	fmt.Println("=====================================")

	// Test queries
	queries := []string{
		"artificial intelligence",
		"machine learning algorithms",
		"kubernetes deployment",
	}

	for _, query := range queries {
		fmt.Printf("\n🔸 Testing query: %s\n", query)

		// Test non-streaming mode
		fmt.Println("\n📊 Non-Streaming Mode:")
		testNonStreaming(query)

		// Test streaming mode
		fmt.Println("\n🚀 Streaming Mode:")
		testStreaming(query)

		fmt.Println("\n" + strings.Repeat("=", 50))
	}
}

func testNonStreaming(query string) {
	start := time.Now()

	// Submit search request
	taskID, err := submitSearch(query, false)
	if err != nil {
		fmt.Printf("❌ Error submitting search: %v\n", err)
		return
	}

	fmt.Printf("✅ Search submitted (Task ID: %s) - %v\n", taskID, time.Since(start))

	// Poll for results
	pollStart := time.Now()
	var searchResultsTime, summaryTime time.Duration
	var hasSearchResults, hasSummary bool

	for {
		status, err := getStatus(taskID)
		if err != nil {
			fmt.Printf("❌ Error getting status: %v\n", err)
			return
		}

		// Check for search results
		if !hasSearchResults && len(status.SearchResults) > 0 {
			searchResultsTime = time.Since(pollStart)
			hasSearchResults = true
			fmt.Printf("🔍 Search results received - %v\n", searchResultsTime)
		}

		// Check for summary
		if !hasSummary && status.Summary != "" {
			summaryTime = time.Since(pollStart)
			hasSummary = true
			fmt.Printf("🤖 AI summary received - %v\n", summaryTime)
		}

		if status.Status == "completed" {
			fmt.Printf("✅ Search completed - Total time: %v\n", time.Since(start))
			fmt.Printf("📋 Summary: %s\n", truncateString(status.Summary, 100))
			break
		}

		if status.Status == "failed" {
			fmt.Printf("❌ Search failed: %s\n", status.Error)
			break
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func testStreaming(query string) {
	start := time.Now()

	// Submit streaming search request
	taskID, err := submitSearch(query, true)
	if err != nil {
		fmt.Printf("❌ Error submitting search: %v\n", err)
		return
	}

	fmt.Printf("✅ Streaming search submitted (Task ID: %s) - %v\n", taskID, time.Since(start))

	// Note: In a real implementation, you would connect to EventSource here
	// For this demo, we'll poll but note the streaming capability
	fmt.Println("🔄 In streaming mode, you would receive:")
	fmt.Println("   1. Immediate search results")
	fmt.Println("   2. Real-time AI summary tokens")
	fmt.Println("   3. Progressive summary building")

	// Poll for final results (simulating what HTTP streaming would provide)
	pollStart := time.Now()
	var searchResultsTime, summaryTime time.Duration
	var hasSearchResults, hasSummary bool

	for {
		status, err := getStatus(taskID)
		if err != nil {
			fmt.Printf("❌ Error getting status: %v\n", err)
			return
		}

		// Check for search results
		if !hasSearchResults && len(status.SearchResults) > 0 {
			searchResultsTime = time.Since(pollStart)
			hasSearchResults = true
			fmt.Printf("🔍 Search results available - %v\n", searchResultsTime)
		}

		// Check for summary
		if !hasSummary && status.Summary != "" {
			summaryTime = time.Since(pollStart)
			hasSummary = true
			fmt.Printf("🤖 AI summary streaming - %v\n", summaryTime)
		}

		if status.Status == "completed" {
			fmt.Printf("✅ Streaming completed - Total time: %v\n", time.Since(start))
			fmt.Printf("📋 Final summary: %s\n", truncateString(status.Summary, 100))
			break
		}

		if status.Status == "failed" {
			fmt.Printf("❌ Streaming failed: %s\n", status.Error)
			break
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func submitSearch(query string, streaming bool) (string, error) {
	req := SearchRequest{
		Query:      query,
		SafeSearch: true,
		Streaming:  streaming,
		NumResults: 5,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(baseURL+"/api/v1/search", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return "", err
	}

	return searchResp.TaskID, nil
}

func getStatus(taskID string) (*StatusResponse, error) {
	resp, err := http.Get(baseURL + "/api/v1/search/status/" + taskID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var status StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
