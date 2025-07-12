package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type MonitoringSearchRequest struct {
	Query      string `json:"query"`
	SafeSearch bool   `json:"safe_search"`
	Streaming  bool   `json:"streaming"`
	NumResults int    `json:"num_results"`
}

type MonitoringSearchResponse struct {
	TaskID    string `json:"task_id"`
	Query     string `json:"query"`
	Status    string `json:"status"`
	Streaming bool   `json:"streaming"`
}

const monitoringBaseURL = "http://localhost:8080"

func main() {
	fmt.Println("🔍 AI Search Engine Monitoring Test")
	fmt.Println("===================================")

	// Test queries to generate load
	queries := []string{
		"artificial intelligence",
		"machine learning algorithms",
		"kubernetes deployment",
		"docker containers",
		"golang microservices",
		"prometheus monitoring",
		"grafana dashboards",
		"ollama llama models",
		"gpu computing",
		"cpu optimization",
	}

	fmt.Println("🚀 Starting monitoring test...")
	fmt.Println("This will generate load to test CPU, GPU, and service metrics")
	fmt.Println("Check the monitoring dashboard at:")
	fmt.Println("📊 Grafana: http://localhost:3000 (admin/admin)")
	fmt.Println("📈 Prometheus: http://localhost:9090")
	fmt.Println("🔍 Metrics: http://localhost:8080/metrics")
	fmt.Println("")

	// Test different load patterns
	fmt.Println("🔄 Testing different load patterns...")

	// Pattern 1: Steady load
	fmt.Println("1. Steady load test (10 requests/second for 30 seconds)")
	runSteadyMonitoringLoad(queries, 10, 30*time.Second)

	time.Sleep(10 * time.Second)

	// Pattern 2: Burst load
	fmt.Println("2. Burst load test (50 concurrent requests)")
	runBurstMonitoringLoad(queries, 50)

	fmt.Println("")
	fmt.Println("✅ Monitoring test completed!")
	fmt.Println("Check the dashboards to see the metrics in action:")
	fmt.Println("- CPU usage spikes during tokenization")
	fmt.Println("- GPU usage during AI inference")
	fmt.Println("- Request rates and latencies")
	fmt.Println("- Error rates and service health")
}

func runSteadyMonitoringLoad(queries []string, rps int, duration time.Duration) {
	ticker := time.NewTicker(time.Second / time.Duration(rps))
	defer ticker.Stop()

	timeout := time.After(duration)
	count := 0

	for {
		select {
		case <-timeout:
			fmt.Printf("   Completed %d requests\n", count)
			return
		case <-ticker.C:
			query := queries[count%len(queries)]
			go makeMonitoringRequest(query, false)
			count++
		}
	}
}

func runBurstMonitoringLoad(queries []string, concurrent int) {
	var wg sync.WaitGroup

	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			query := queries[index%len(queries)]
			makeMonitoringRequest(query, false)
		}(i)
	}

	wg.Wait()
	fmt.Printf("   Completed %d concurrent requests\n", concurrent)
}

func makeMonitoringRequest(query string, streaming bool) {
	req := MonitoringSearchRequest{
		Query:      query,
		SafeSearch: true,
		Streaming:  streaming,
		NumResults: 3,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		log.Printf("Error marshaling request: %v", err)
		return
	}

	resp, err := http.Post(monitoringBaseURL+"/api/v1/search", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error making request: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Request failed with status: %d", resp.StatusCode)
		return
	}

	var searchResp MonitoringSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		log.Printf("Error decoding response: %v", err)
		return
	}
}
