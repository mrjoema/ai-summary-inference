# AI Search Engine with Microservices Architecture

A high-performance, scalable AI-powered search engine built with Go microservices, designed for production deployment with comprehensive fault tolerance, backpressure handling, and modern streaming architecture.

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
- ✅ **Streaming Support**: Real-time AI summary generation with Server-Sent Events (SSE)
- ✅ **Non-blocking**: Search results appear immediately while AI summary generates
- ✅ **Fault Tolerant**: Circuit breakers, graceful degradation, and retry mechanisms
- ✅ **Backpressure Handling**: Intelligent load balancing and queue management
- ✅ **Microservices**: Separate CPU and GPU intensive services for optimal resource allocation
- ✅ **Kubernetes Ready**: Full K8s deployment with auto-scaling (HPA)
- ✅ **Local Development**: Simple one-command setup for development and testing
- ✅ **Production Ready**: Docker containers, health checks, comprehensive monitoring

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

## 🛠️ Architecture Decisions & Scaling Strategy

### Why Microservices?
- **Resource Optimization**: Separate CPU (tokenizer) and GPU (inference) services
- **Scalability**: Independent scaling based on load
- **Fault Isolation**: Service failures don't affect others
- **Technology Diversity**: Can use different tech stacks per service

### Communication Patterns
- **Internal Services**: gRPC for high-performance service-to-service communication
- **Client Streaming**: Server-Sent Events (SSE) for real-time updates to web clients
- **Backpressure**: Built-in gRPC streaming backpressure for flow control
- **No Message Queues**: Simplified architecture with direct gRPC streaming between services

### Fault Tolerance & Resilience
- **Circuit Breakers**: Prevent cascading failures across services
- **Graceful Degradation**: System continues operating with reduced functionality during overload
- **Health Checks**: Comprehensive service health monitoring
- **Timeout Management**: Request-level timeouts to prevent hanging operations
- **Retry Mechanisms**: Intelligent retry with exponential backoff

### Scaling Architecture

#### Horizontal Scaling Guidelines
```bash
# CPU-intensive services (scale based on CPU usage)
kubectl scale deployment tokenizer --replicas=5 -n ai-search

# GPU-intensive services (scale based on GPU availability)
kubectl scale deployment inference --replicas=3 -n ai-search

# Load balancing services (scale based on request volume)
kubectl scale deployment gateway --replicas=8 -n ai-search
```

#### Auto-scaling Configuration
- **HPA (Horizontal Pod Autoscaler)**: Automatically scales based on CPU/memory/custom metrics
- **CPU Targets**: 70% CPU utilization trigger for scaling
- **Memory Targets**: 80% memory utilization trigger for scaling
- **Custom Metrics**: Request latency and queue depth for intelligent scaling

### Why Go?
- **Performance**: Excellent for high-throughput services
- **Concurrency**: Built-in goroutines for handling multiple requests
- **Deployment**: Single binary deployment
- **Ecosystem**: Rich gRPC and HTTP libraries

### Why gRPC for Internal Communication?
- **Performance**: Binary protocol, faster than REST (2-10x throughput improvement)
- **Type Safety**: Protocol buffers ensure consistent APIs
- **Streaming**: Built-in bidirectional streaming with backpressure
- **Load Balancing**: Automatic load balancing and service discovery
- **Low Latency**: Sub-millisecond internal service communication

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

### Performance Tuning & Production Optimization

#### 1. Scaling Based on Metrics
```bash
# Monitor current resource usage
kubectl top pods -n ai-search

# Scale CPU-intensive services
kubectl scale deployment tokenizer --replicas=5 -n ai-search

# Scale GPU-intensive services (based on GPU availability)
kubectl scale deployment inference --replicas=3 -n ai-search

# Scale based on request volume
kubectl scale deployment gateway --replicas=8 -n ai-search
```

#### 2. Load Testing & Capacity Planning
```bash
# Run load tests to determine optimal scaling
make test-backpressure

# Test with different concurrency levels
./scripts/load-test.sh --concurrent-users=100 --ramp-duration=60s
./scripts/load-test.sh --concurrent-users=500 --ramp-duration=120s
```

#### 3. Circuit Breaker Configuration
Monitor and tune circuit breaker thresholds based on your traffic patterns:
- **Failure Threshold**: 50% error rate triggers circuit open
- **Reset Timeout**: 30 seconds before attempting to close circuit
- **Success Threshold**: 3 consecutive successes to close circuit

#### 4. gRPC Streaming Optimization
- **Connection Pooling**: Reuse gRPC connections for better performance
- **Streaming Buffer Size**: Tune based on token generation speed
- **Backpressure Thresholds**: Adjust based on downstream service capacity

#### 5. Resource Requests & Limits
```yaml
# Recommended resource settings for production
resources:
  requests:
    cpu: "500m"      # Tokenizer: CPU-bound
    memory: "512Mi"
  limits:
    cpu: "2000m"     # Inference: GPU + CPU
    memory: "2Gi"
```

## 🏭 Production Deployment Best Practices

### Environment Configuration
```bash
# Production environment variables
export ENVIRONMENT=production
export LOG_LEVEL=warn
export REDIS_CLUSTER_MODE=true
export ENABLE_METRICS=true
export ENABLE_TRACING=true
```

### High Availability Setup
```yaml
# Multi-zone deployment for fault tolerance
apiVersion: apps/v1
kind: Deployment
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  template:
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: app
                  operator: In
                  values: ["ai-search-gateway"]
              topologyKey: topology.kubernetes.io/zone
```

### Security Hardening
- **Network Policies**: Restrict inter-service communication
- **Pod Security Standards**: Enforce restricted security contexts
- **RBAC**: Minimal required permissions for service accounts
- **Secrets Management**: Use Kubernetes secrets or external secret managers
- **Image Scanning**: Regular vulnerability scanning of container images

### Monitoring & Alerting in Production
```yaml
# Critical alerts for production
alerts:
  - name: ServiceDown
    condition: up == 0
    for: 1m
    severity: critical
  
  - name: HighErrorRate
    condition: rate(http_requests_total{status=~"5.."}[5m]) > 0.1
    for: 2m
    severity: warning
  
  - name: HighLatency
    condition: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 1
    for: 5m
    severity: warning
```

### Backup & Disaster Recovery
- **Redis Backup**: Automated Redis persistence and backup
- **Configuration Backup**: Version-controlled configuration management
- **Multi-Region**: Deploy across multiple regions for disaster recovery
- **RTO/RPO Targets**: Recovery Time Objective < 15 minutes, Recovery Point Objective < 5 minutes

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🔄 Modern Streaming Architecture

This system implements industry-standard streaming patterns optimized for AI inference workloads:

### Communication Flow
```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Client    │───▶│   Gateway   │───▶│   Search    │
│ (SSE Stream)│    │(HTTP + gRPC)│    │  Service    │
└─────────────┘    └─────────────┘    └─────────────┘
       ▲                   │                   │
       │                   ▼                   ▼
       │            ┌─────────────┐    ┌─────────────┐
       │            │  Tokenizer  │    │    Redis    │
       │            │   Service   │    │   Cache     │
       │            └─────────────┘    └─────────────┘
       │                   │
       │                   ▼
       │            ┌─────────────┐
       └────────────│  Inference  │
         (Streaming)│   Service   │
                    └─────────────┘
```

### Why This Architecture?
- **SSE for Clients**: Universal browser support, simple debugging
- **gRPC Internal**: High-performance service-to-service communication
- **No Message Queues**: Simplified operations, lower latency
- **Direct Streaming**: Real-time token streaming from inference to client

### Industry Alignment
- **OpenAI Pattern**: Similar to ChatGPT's streaming architecture
- **Cloud Native**: Follows CNCF best practices for microservices
- **Production Proven**: Based on patterns used by major AI companies

## 🙏 Acknowledgments

- Built with Go and gRPC for high-performance streaming
- Designed for Kubernetes deployment with cloud-native patterns
- Optimized for both local development and production scaling
- Inspired by modern AI inference architectures (OpenAI, Anthropic)
- Follows industry best practices for fault tolerance and observability 