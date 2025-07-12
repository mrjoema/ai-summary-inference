package monitoring

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

// Prometheus metrics
var (
	// CPU metrics
	CPUUsagePercent = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ai_search_cpu_usage_percent",
			Help: "CPU usage percentage by service",
		},
		[]string{"service", "instance"},
	)

	// Memory metrics
	MemoryUsageBytes = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ai_search_memory_usage_bytes",
			Help: "Memory usage in bytes by service",
		},
		[]string{"service", "instance"},
	)

	MemoryUsagePercent = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ai_search_memory_usage_percent",
			Help: "Memory usage percentage by service",
		},
		[]string{"service", "instance"},
	)

	// GPU metrics (for inference service)
	GPUUsagePercent = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ai_search_gpu_usage_percent",
			Help: "GPU usage percentage by service",
		},
		[]string{"service", "instance", "gpu_id"},
	)

	GPUMemoryUsageBytes = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ai_search_gpu_memory_usage_bytes",
			Help: "GPU memory usage in bytes by service",
		},
		[]string{"service", "instance", "gpu_id"},
	)

	// Service-specific metrics
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_search_requests_total",
			Help: "Total number of requests by service",
		},
		[]string{"service", "method", "status"},
	)

	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ai_search_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method"},
	)

	// AI-specific metrics
	TokensProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_search_tokens_processed_total",
			Help: "Total number of tokens processed",
		},
		[]string{"service", "model"},
	)

	InferenceLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ai_search_inference_latency_seconds",
			Help:    "AI inference latency in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
		},
		[]string{"service", "model", "streaming"},
	)

	// Ollama-specific metrics
	OllamaRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_search_ollama_requests_total",
			Help: "Total number of requests to Ollama",
		},
		[]string{"service", "model", "status"},
	)

	OllamaResponseTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ai_search_ollama_response_time_seconds",
			Help:    "Ollama response time in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
		},
		[]string{"service", "model"},
	)
)

// MetricsCollector handles system metrics collection
type MetricsCollector struct {
	serviceName string
	instanceID  string
	process     *process.Process
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(serviceName string) (*MetricsCollector, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Get current process
	pid := os.Getpid()
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get process info: %w", err)
	}

	instanceID := fmt.Sprintf("%s-%d", serviceName, pid)

	collector := &MetricsCollector{
		serviceName: serviceName,
		instanceID:  instanceID,
		process:     proc,
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start collecting metrics
	go collector.collectMetrics()

	return collector, nil
}

// Stop stops the metrics collector
func (mc *MetricsCollector) Stop() {
	mc.cancel()
}

// collectMetrics runs the metrics collection loop
func (mc *MetricsCollector) collectMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-mc.ctx.Done():
			return
		case <-ticker.C:
			mc.collectSystemMetrics()
			if mc.serviceName == "inference" {
				mc.collectGPUMetrics()
			}
		}
	}
}

// collectSystemMetrics collects CPU and memory metrics
func (mc *MetricsCollector) collectSystemMetrics() {
	// CPU usage
	if cpuPercent, err := cpu.Percent(time.Second, false); err == nil && len(cpuPercent) > 0 {
		CPUUsagePercent.WithLabelValues(mc.serviceName, mc.instanceID).Set(cpuPercent[0])
	}

	// Process-specific CPU usage
	if cpuPercent, err := mc.process.CPUPercent(); err == nil {
		CPUUsagePercent.WithLabelValues(mc.serviceName+"_process", mc.instanceID).Set(cpuPercent)
	}

	// Memory usage
	if memInfo, err := mem.VirtualMemory(); err == nil {
		MemoryUsageBytes.WithLabelValues(mc.serviceName+"_system", mc.instanceID).Set(float64(memInfo.Used))
		MemoryUsagePercent.WithLabelValues(mc.serviceName+"_system", mc.instanceID).Set(memInfo.UsedPercent)
	}

	// Process-specific memory usage
	if memInfo, err := mc.process.MemoryInfo(); err == nil {
		MemoryUsageBytes.WithLabelValues(mc.serviceName+"_process", mc.instanceID).Set(float64(memInfo.RSS))
	}
}

// collectGPUMetrics collects GPU metrics (for inference service)
func (mc *MetricsCollector) collectGPUMetrics() {
	// This would require nvidia-ml-go or similar library
	// For now, we'll simulate GPU metrics or use nvidia-smi command
	gpuUsage, gpuMemory := mc.getGPUMetrics()

	if gpuUsage >= 0 {
		GPUUsagePercent.WithLabelValues(mc.serviceName, mc.instanceID, "0").Set(gpuUsage)
	}

	if gpuMemory >= 0 {
		GPUMemoryUsageBytes.WithLabelValues(mc.serviceName, mc.instanceID, "0").Set(gpuMemory)
	}
}

// getGPUMetrics gets GPU metrics (placeholder implementation)
func (mc *MetricsCollector) getGPUMetrics() (float64, float64) {
	// TODO: Implement actual GPU metrics collection
	// This could use nvidia-ml-go or parse nvidia-smi output
	// For now, return -1 to indicate no GPU metrics available
	return -1, -1
}

// RecordRequest records a request metric
func RecordRequest(service, method, status string) {
	RequestsTotal.WithLabelValues(service, method, status).Inc()
}

// RecordRequestDuration records request duration
func RecordRequestDuration(service, method string, duration time.Duration) {
	RequestDuration.WithLabelValues(service, method).Observe(duration.Seconds())
}

// RecordTokensProcessed records tokens processed
func RecordTokensProcessed(service, model string, count int) {
	TokensProcessed.WithLabelValues(service, model).Add(float64(count))
}

// RecordInferenceLatency records inference latency
func RecordInferenceLatency(service, model string, streaming bool, duration time.Duration) {
	streamingStr := "false"
	if streaming {
		streamingStr = "true"
	}
	InferenceLatency.WithLabelValues(service, model, streamingStr).Observe(duration.Seconds())
}

// RecordOllamaRequest records Ollama request
func RecordOllamaRequest(service, model, status string) {
	OllamaRequestsTotal.WithLabelValues(service, model, status).Inc()
}

// RecordOllamaResponseTime records Ollama response time
func RecordOllamaResponseTime(service, model string, duration time.Duration) {
	OllamaResponseTime.WithLabelValues(service, model).Observe(duration.Seconds())
}
