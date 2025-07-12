# AI Search Engine with Microservices Architecture

A high-performance, scalable AI-powered search engine built with Go microservices, designed for Kubernetes deployment with support for both streaming and non-streaming AI summaries.

## 🏗️ Architecture Overview

### Microservices Architecture
- **Gateway Service**: HTTP API gateway with Server-Sent Events (SSE) streaming
- **Search Service**: Google Custom Search API integration with fallback mock data
- **Tokenizer Service**: CPU-intensive text tokenization (optimized for CPU resources)
- **Inference Service**: GPU-intensive AI model inference (optimized for GPU resources)
- **Safety Service**: Input validation and output sanitization
- **Redis**: Caching and session management

### Key Features
- ✅ **Safety-first**: Comprehensive input validation and output sanitization
- ✅ **Streaming Support**: Real-time AI summary generation with HTTP streaming
- ✅ **Non-blocking**: Search results appear immediately while AI summary generates
- ✅ **Microservices**: Separate CPU and GPU intensive services for optimal resource allocation
- ✅ **Kubernetes Ready**: Full K8s deployment with auto-scaling (HPA)
- ✅ **Mac M4 Pro Optimized**: Easy local development and testing
- ✅ **Production Ready**: Docker containers, health checks, monitoring

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Docker & Docker Compose
- Protocol Buffers compiler (`protoc`)
- kubectl (for Kubernetes deployment)

### Local Development Setup

1. **Clone and setup**:
```bash
git clone <repository-url>
cd ai-search-service
make dev-setup
```

2. **Start services locally**:
```bash
make run-local
```

3. **Access the application**:
Open http://localhost:8080 in your browser

### Google Search API Setup (Optional)
```bash
export GOOGLE_API_KEY="your-api-key"
export GOOGLE_CX="your-custom-search-engine-id"
```

Without these credentials, the system will use mock search data for testing.

## 🔧 Development

### Building Services
```bash
# Build all services
make build

# Build specific service
make build-service SERVICE=gateway

# Generate protocol buffers
make proto
```

### Running Individual Services
```bash
# Run gateway service
make run-service SERVICE=gateway

# Run in separate terminals:
make run-service SERVICE=search
make run-service SERVICE=tokenizer
make run-service SERVICE=inference
make run-service SERVICE=safety
```

### Testing
```bash
# Run tests
make test

# Test streaming vs non-streaming
go run scripts/test_streaming.go
```

## ☸️ Kubernetes Deployment

### Deploy to Kubernetes
```bash
# Deploy all services
make deploy-k8s

# Create Google API secret (optional)
kubectl create secret generic google-api-secret \
  --from-literal=api-key=YOUR_API_KEY \
  --from-literal=cx=YOUR_CX \
  -n ai-search

# Check status
make status

# Port forward for local access
make port-forward
```

### Scaling Configuration
- **Tokenizer**: 2-10 replicas (CPU intensive)
- **Inference**: 1-5 replicas (GPU intensive)
- **Gateway**: 2-8 replicas (Load balancing)
- **Search**: 2-6 replicas (API rate limiting)
- **Safety**: 2 replicas (Fast response)

## 🌐 API Documentation

### Search Endpoint
```bash
POST /api/v1/search
Content-Type: application/json

{
  "query": "artificial intelligence",
  "safe_search": true,
  "streaming": true,
  "num_results": 5
}
```

### Response
```json
{
  "task_id": "task_1234567890",
  "query": "artificial intelligence",
  "status": "pending",
  "streaming": true
}
```

### Status Endpoint
```bash
GET /api/v1/search/status/{task_id}
```

### Streaming Endpoint
```bash
SSE: /api/v1/search/stream/{task_id}
```

## 🔒 Safety Features

### Input Validation
- Length limits (500 chars for queries)
- SQL injection prevention
- Command injection prevention
- XSS protection
- Inappropriate content detection

### Output Sanitization
- HTML escaping
- Length limits (1000 chars for summaries)
- Dangerous pattern removal
- Unicode normalization

## 📊 Monitoring & Observability

### Comprehensive Monitoring Stack
- **Prometheus**: Metrics collection and alerting
- **Grafana**: Visualization dashboards
- **Custom Metrics**: CPU, GPU, memory, and AI-specific metrics
- **Real-time Alerts**: Performance and health monitoring

### Key Metrics Tracked
- **CPU Usage**: Per-service CPU utilization
- **Memory Usage**: System and process memory consumption
- **GPU Metrics**: GPU utilization and memory (inference service)
- **Request Metrics**: Request rates, latencies, and error rates
- **AI Metrics**: Token processing, inference latency, Ollama performance
- **Service Health**: Uptime and availability monitoring

### Monitoring Endpoints
```bash
# Prometheus metrics
curl http://localhost:8080/metrics    # Gateway metrics
curl http://localhost:8083/metrics    # Inference metrics (includes GPU)
curl http://localhost:8081/metrics    # Search metrics
curl http://localhost:8082/metrics    # Tokenizer metrics (CPU intensive)
curl http://localhost:8084/metrics    # Safety metrics

# Monitoring dashboards
http://localhost:3000                 # Grafana (admin/admin)
http://localhost:9090                 # Prometheus
http://localhost:8085                 # cAdvisor (container metrics)
```

## 🚀 Quick Start with Monitoring

### Easy Startup Script
We've created a simple startup script that makes it easy to run the application with monitoring:

```bash
# Start everything (app + monitoring) - RECOMMENDED
./start.sh

# Start application only (no monitoring)
./start.sh app-only

# Start monitoring stack only
./start.sh monitoring

# Stop all services
./start.sh stop

# Show service status
./start.sh status

# Show logs for a specific service
./start.sh logs gateway

# Clean up everything
./start.sh clean

# Show help
./start.sh help
```

### Using Makefile Commands
Alternatively, you can use the Makefile commands:

```bash
# Start everything with monitoring
make run-local-with-monitoring

# Start application only
make run-local

# Start monitoring only
make run-monitoring

# Stop services
make stop-local

# Clean up
make stop-clean
```

### Starting with Monitoring
# Start all services including monitoring stack
docker-compose up -d

# Wait for services to start
sleep 30

# Run monitoring test to generate metrics
go run scripts/monitoring_test.go

# Access dashboards
open http://localhost:3000  # Grafana
open http://localhost:9090  # Prometheus
```

### Dashboard Features
- **Real-time CPU/GPU monitoring** across all services
- **AI inference performance** tracking
- **Request flow visualization** with latency percentiles
- **Error rate monitoring** and alerting
- **Resource utilization** trends and capacity planning
- **Service health** status and uptime tracking

### Alerts Configuration
Automatic alerts for:
- High CPU usage (>80% warning, >95% critical)
- High memory usage (>85% warning, >95% critical)
- GPU overutilization (>90% warning)
- Service downtime (>1 minute)
- High error rates (>10% warning)
- Slow AI inference (>30 seconds)

### Monitoring Architecture
```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Services  │───▶│ Prometheus  │───▶│   Grafana   │
│  (Metrics)  │    │ (Collection)│    │(Dashboards) │
└─────────────┘    └─────────────┘    └─────────────┘
       │                   │                   │
       ▼                   ▼                   ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ Node Export │    │ Alert Rules │    │  Alerting   │
│(System CPU) │    │(Thresholds) │    │(Notifications)│
└─────────────┘    └─────────────┘    └─────────────┘
```

### Custom Metrics
```go
// CPU and Memory metrics
ai_search_cpu_usage_percent{service="inference", instance="inference-123"}
ai_search_memory_usage_percent{service="tokenizer", instance="tokenizer-456"}

// GPU metrics (inference service)
ai_search_gpu_usage_percent{service="inference", instance="inference-123", gpu_id="0"}
ai_search_gpu_memory_usage_bytes{service="inference", instance="inference-123", gpu_id="0"}

// Request metrics
ai_search_requests_total{service="gateway", method="search", status="success"}
ai_search_request_duration_seconds{service="gateway", method="search"}

// AI-specific metrics
ai_search_tokens_processed_total{service="inference", model="llama3.2:3b"}
ai_search_inference_latency_seconds{service="inference", model="llama3.2:3b", streaming="true"}
ai_search_ollama_requests_total{service="inference", model="llama3.2:3b", status="success"}
```

### Performance Monitoring
Monitor these key performance indicators:
- **Tokenizer CPU**: Should scale with request volume
- **Inference GPU**: High utilization during AI processing
- **Gateway Latency**: Overall request processing time
- **Ollama Response Time**: AI model inference speed
- **Error Rates**: Service reliability metrics

### Scaling Decisions
Use monitoring data to make scaling decisions:
- **High CPU on tokenizer**: Scale tokenizer replicas
- **High GPU on inference**: Scale inference replicas or add GPU resources
- **High gateway latency**: Scale gateway replicas
- **High error rates**: Investigate service health and dependencies

## 🔄 Streaming vs Non-Streaming Mode

### Non-Streaming Mode
1. Submit search query
2. Get task ID immediately
3. Poll status endpoint
4. Receive complete results when ready

### Streaming Mode
1. Submit search query with `streaming: true`
2. Connect to HTTP stream using EventSource
3. Receive search results immediately
4. Receive AI summary tokens in real-time
5. Get final complete summary

## 🛠️ Architecture Decisions

### Why Microservices?
- **Resource Optimization**: Separate CPU (tokenizer) and GPU (inference) services
- **Scalability**: Independent scaling based on load
- **Fault Isolation**: Service failures don't affect others
- **Technology Diversity**: Can use different tech stacks per service

### Why Go?
- **Performance**: Excellent for high-throughput services
- **Concurrency**: Built-in goroutines for handling multiple requests
- **Deployment**: Single binary deployment
- **Ecosystem**: Rich gRPC and HTTP libraries

### Why gRPC?
- **Performance**: Binary protocol, faster than REST
- **Type Safety**: Protocol buffers ensure consistent APIs
- **Streaming**: Built-in streaming support
- **Load Balancing**: Automatic load balancing

## 📁 Project Structure

```
ai-search-service/
├── cmd/                    # Service entry points
│   ├── gateway/           # API gateway
│   ├── search/            # Search service
│   ├── tokenizer/         # Tokenizer service
│   ├── inference/         # Inference service
│   └── safety/            # Safety service
├── internal/              # Internal packages
│   ├── config/           # Configuration management
│   ├── logger/           # Logging utilities
│   ├── gateway/          # Gateway implementation
│   └── services/         # Service implementations
├── proto/                 # Protocol buffer definitions
├── web/                   # Frontend templates and static files
├── k8s/                   # Kubernetes manifests
├── docker-compose.yml     # Local development
├── Makefile              # Build and deployment scripts
└── README.md             # This file
```

## 🚨 Troubleshooting

### Common Issues

1. **Protocol buffer compilation errors**:
```bash
make dev-setup
make proto
```

2. **Docker build failures**:
```bash
make clean
make build
```

3. **Kubernetes deployment issues**:
```bash
make undeploy-k8s
make deploy-k8s
make status
```

4. **Service connection errors**:
```bash
make logs SERVICE=gateway
kubectl get pods -n ai-search
```

### Performance Tuning

1. **Increase tokenizer replicas for high CPU load**:
```bash
kubectl scale deployment tokenizer --replicas=5 -n ai-search
```

2. **Increase inference replicas for high GPU load**:
```bash
kubectl scale deployment inference --replicas=3 -n ai-search
```

3. **Monitor resource usage**:
```bash
kubectl top pods -n ai-search
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🙏 Acknowledgments

- Built with Go and gRPC
- Designed for Kubernetes deployment
- Optimized for Mac M4 Pro development
- Inspired by modern microservices architecture patterns 