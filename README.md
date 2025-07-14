# AI-Powered Search Engine with Real-Time Summarization

A production-ready AI search engine built with Go microservices architecture, featuring real-time BART model inference for intelligent search result summarization with both streaming and non-streaming modes.

## 🎬 Demo

![AI Inference Demo](ai-inference-demo.gif)

## 🚀 Key Features

- ✅ **Real AI Summarization**: Facebook BART model with HuggingFace Transformers
- ✅ **Token-Native Processing**: Industry-standard tokenization → inference → detokenization pipeline
- ✅ **Dual Response Modes**: Streaming (real-time tokens) and non-streaming (complete summaries)
- ✅ **Server-Sent Events**: Real-time search results followed by AI summaries
- ✅ **Production Architecture**: Go orchestrator with Python ML services
- ✅ **Safety-First**: Input validation and output sanitization
- ✅ **Monitoring Stack**: Prometheus, Grafana, and comprehensive health checks
- ✅ **Apple Silicon Optimized**: CPU-optimized PyTorch deployment for Mac development

## 🏗️ Architecture Overview

### Current Microservices Stack
```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Gateway   │───▶│ LLM Orch.   │───▶│  Tokenizer  │
│  (Go:8080)  │    │ (Go:8086)   │    │(Python:8090)│
└─────────────┘    └─────────────┘    └─────────────┘
       │                   │                   │
       ▼                   ▼                   ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Search    │    │  Inference  │    │    Redis    │
│ (Go:8081)   │    │(Python:8083)│    │  (Cache)    │
└─────────────┘    └─────────────┘    └─────────────┘
       │                   │                   │
       ▼                   ▼                   ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Safety    │    │    BART     │    │  Prometheus │
│ (Go:8084)   │    │    Model    │    │ (Monitoring)│
└─────────────┘    └─────────────┘    └─────────────┘
```

### Services Description
- **Gateway Service** (Go, Port 8080): HTTP API with SSE streaming support
- **LLM Orchestrator** (Go, Port 8086): Coordinates token-native AI workflow
- **Search Service** (Go, Port 8081): Google Custom Search API integration
- **Tokenizer Service** (Python, Port 8090): BART tokenization and detokenization
- **Inference Service** (Python, Port 8083): BART model inference with PyTorch
- **Safety Service** (Go, Port 8084): Input validation and output sanitization

## 🎯 User Experience Flow

### Non-Streaming Mode (Recommended)
1. **Submit search query** → Immediate response with search results
2. **AI summary appears first** → Prominently displayed with gradient styling
3. **Source results below** → Clean, secondary display for reference

### Streaming Mode
1. **Submit search query** → Real-time search results appear
2. **AI tokens stream live** → Watch summary generate word-by-word
3. **Complete summary** → Final sanitized output

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Python 3.9+ with pip
- Docker & Docker Compose
- 8GB+ RAM (for BART model)

### 1. Clone and Setup
```bash
git clone <repository-url>
cd ai-summary-inference
```

### 2. Start All Services
```bash
# Build Go services
make build

# Start everything with Docker Compose
docker-compose up -d

# Wait for services to initialize (BART model loading takes ~30s)
sleep 30
```

### 3. Access the Application
- **Web Interface**: http://localhost:8080
- **API Endpoint**: http://localhost:8080/api/v1/search
- **Monitoring**: http://localhost:3000 (Grafana: admin/admin)

### Optional: Google Search API
```bash
# Set environment variables for real search (optional)
export GOOGLE_API_KEY="your-api-key"
export GOOGLE_CX="your-custom-search-engine-id"

# Restart gateway to pick up credentials
docker-compose restart gateway
```

Without Google API credentials, the system uses mock search data.

## 🌐 API Documentation

### Non-Streaming Search (SSE)
```bash
POST /api/v1/search
Content-Type: application/json
Accept: text/event-stream

{
  "query": "machine learning algorithms",
  "safe_search": true,
  "num_results": 5
}
```

**Response**: Server-Sent Events stream
```
event:status
data:{"type":"started","query":"machine learning algorithms"}

event:search_results
data:{"type":"search_results","results":[...]}

event:summary
data:{"type":"summary_complete","text":"AI summary here..."}

event:complete
data:{"type":"complete"}
```

### Non-Streaming Search (JSON)
```bash
POST /api/v1/search
Content-Type: application/json

{
  "query": "artificial intelligence",
  "safe_search": true,
  "num_results": 3
}
```

**Response**: Complete JSON with search results and AI summary
```json
{
  "query": "artificial intelligence",
  "status": "completed",
  "search_results": [...],
  "summary": "AI-generated summary text..."
}
```

### Streaming Search (Real-time Tokens)
```bash
GET /api/v1/search?query=python&streaming=true&safe_search=true&num_results=5
Accept: text/event-stream
```

**Response**: Real-time token streaming
```
event:search_results
data:{"type":"search_results","results":[...]}

event:token
data:{"type":"token","token":"Python","position":0}

event:token  
data:{"type":"token","token":" is","position":1}

event:complete
data:{"type":"complete"}
```

## 🔧 Development

### Building Services
```bash
# Build all Go services
make build

# Generate protocol buffers (if changed)
make proto

# Run tests
make test
```

### Running Individual Services
```bash
# Start core services
docker-compose up -d redis prometheus grafana

# Run Go services locally
./gateway &
./llm &
./search &
./safety &

# Python services need Docker for dependencies
docker-compose up -d python-tokenizer inference
```

### Monitoring and Debugging
```bash
# Check service status
docker-compose ps

# View logs
docker-compose logs gateway
docker-compose logs inference

# Health checks
curl http://localhost:8080/health
curl http://localhost:8086/health
```

## 🔍 AI Processing Pipeline

### Token-Native Flow
1. **Text Input**: "What is machine learning?"
2. **Tokenization**: Text → Token IDs `[2061, 16, 3563, 2069, 116]`
3. **Inference**: BART model processes token IDs → Generated token IDs
4. **Detokenization**: Token IDs → Human-readable text summary
5. **Safety Check**: Output sanitization and validation
6. **Client Display**: Final summary with source results

### Model Details
- **Model**: `facebook/bart-large-cnn` (406M parameters)
- **Framework**: HuggingFace Transformers + PyTorch
- **Device**: CPU-optimized for Apple Silicon and x86
- **Generation**: Beam search with 4 beams, 20-150 tokens
- **Optimization**: Stable library versions to prevent device placement issues

### Performance Characteristics
- **Cold Start**: ~30 seconds (model loading)
- **Inference Time**: 2-8 seconds per summary (CPU)
- **Concurrent Requests**: 8 max per inference service
- **Memory Usage**: ~2GB per inference service
- **Token Processing**: ~50 tokens/second (streaming)

## 🛡️ Safety & Validation

### Input Validation
- **Length Limits**: 500 characters for search queries
- **Content Filtering**: Inappropriate content detection
- **Injection Prevention**: SQL/Command injection protection
- **Rate Limiting**: Concurrent request management (8 per service)

### Output Sanitization
- **Content Filtering**: Dangerous pattern removal
- **Length Limits**: Summary truncation if needed
- **HTML Escaping**: XSS prevention
- **Final Validation**: Safety service approval required

## 📊 Monitoring & Observability

### Monitoring Stack
- **Prometheus**: Metrics collection (http://localhost:9090)
- **Grafana**: Visualization dashboards (http://localhost:3000)
- **cAdvisor**: Container resource monitoring (http://localhost:8087)
- **Health Endpoints**: All services expose /health endpoints

### Key Metrics Tracked
```
# Request metrics
ai_search_requests_total{service="gateway",status="success"}
ai_search_request_duration_seconds{service="gateway",method="search"}

# AI-specific metrics  
ai_search_llm_requests_total{service="orchestrator",model="bart"}
ai_search_tokenization_duration_seconds{service="tokenizer"}
ai_search_inference_duration_seconds{service="inference"}

# System metrics
ai_search_cpu_usage_percent{service="inference"}
ai_search_memory_usage_bytes{service="inference"}
```

### Alerts Configuration
- **Service Health**: Any service down > 1 minute
- **High Latency**: 95th percentile > 10 seconds
- **Error Rate**: >5% error rate sustained
- **Resource Usage**: CPU >80% or Memory >85%

## 🚨 Troubleshooting

### Common Issues

**1. BART Model Loading Errors**
```bash
# Check inference service logs
docker-compose logs inference

# Common fix: Restart inference service
docker-compose restart inference
```

**2. Device Placement Issues**
```bash
# Verify stable library versions in requirements.txt
transformers==4.35.2
torch==2.1.2

# Restart Python services
docker-compose restart python-tokenizer inference
```

**3. gRPC Connection Errors**
```bash
# Check service connectivity
docker-compose exec gateway ./gateway --help
docker-compose exec llm ./llm --help

# Restart orchestrator
docker-compose restart llm
```

**4. Frontend Issues**
```bash
# Check JavaScript console for errors
# Verify SSE connections in Network tab
# Test API directly:
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{"query":"test","safe_search":true,"num_results":3}'
```

### Performance Optimization

**For Better Performance:**
```bash
# Increase inference service replicas
docker-compose up -d --scale inference=2

# Monitor resource usage
docker stats

# Tune concurrent request limits in code:
# - orchestrator.go: maxConcurrentRequests
# - inference/main.py: max_concurrent_requests
```

## 📁 Project Structure

```
ai-summary-inference/
├── cmd/                          # Service entry points
│   ├── gateway/main.go          # HTTP API gateway
│   ├── llm/main.go              # LLM orchestration service
│   ├── search/main.go           # Search service
│   ├── safety/main.go           # Safety validation service
│   ├── tokenizer-python/main.py # BART tokenization service
│   └── inference-python/main.py # BART inference service
├── internal/                     # Internal Go packages
│   ├── config/                  # Configuration management
│   ├── gateway/                 # Gateway implementation
│   ├── logger/                  # Logging utilities
│   ├── monitoring/              # Metrics collection
│   └── services/                # Service implementations
├── proto/                        # Protocol buffer definitions
│   ├── search.proto             # Service contracts
│   ├── search.pb.go             # Generated Go code
│   └── search_pb2.py            # Generated Python code
├── web/                          # Frontend resources
│   ├── templates/index.html     # Main web interface
│   └── static/                  # CSS, JS, images
├── monitoring/                   # Monitoring configuration
│   ├── prometheus.yml           # Prometheus config
│   └── grafana/                 # Grafana dashboards
├── docker-compose.yml            # Local development setup
├── Makefile                      # Build automation
├── config.yaml                   # Service configuration
└── README.md                     # This file
```

## 🔄 Deployment Options

### Local Development (Current)
```bash
# Full stack with monitoring
docker-compose up -d

# Application only
docker-compose up -d gateway llm search safety python-tokenizer inference redis
```

### Production Considerations
- **Container Orchestration**: Kubernetes deployment ready
- **Load Balancing**: Multiple gateway replicas
- **Resource Allocation**: Separate CPU/GPU node pools
- **Monitoring**: External Prometheus/Grafana cluster
- **Secrets Management**: External secret stores
- **Caching**: Redis cluster for high availability

### Scaling Strategy
- **Gateway**: Scale horizontally based on request volume
- **LLM Orchestrator**: Scale based on coordination overhead  
- **Tokenizer**: Scale based on CPU usage (CPU-bound)
- **Inference**: Scale based on model capacity (memory-bound)
- **Search/Safety**: Scale based on API rate limits

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests for new functionality
5. Ensure all services pass health checks
6. Submit a pull request

### Development Guidelines
- Follow Go best practices and gofmt
- Add comprehensive error handling
- Include unit tests for new functions
- Update documentation for API changes
- Test with both streaming and non-streaming modes

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🎯 Production Readiness

This system demonstrates:
- **Microservices Architecture**: Proper service separation and communication
- **AI/ML Integration**: Real transformer model deployment
- **Streaming Architecture**: Modern real-time response patterns
- **Observability**: Comprehensive monitoring and logging
- **Safety Engineering**: Input validation and output sanitization
- **Performance Optimization**: Efficient resource utilization
- **Fault Tolerance**: Graceful error handling and recovery

Perfect for showcasing modern AI infrastructure engineering capabilities.