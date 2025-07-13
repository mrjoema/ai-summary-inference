# Variables
DOCKER_REGISTRY ?= ai-search
VERSION ?= latest
SERVICES = gateway search inference llm safety

.PHONY: all build push deploy clean test proto

# Default target
all: proto build

# Generate protocol buffer files
proto:
	@echo "Generating protocol buffer files..."
	@export PATH=$$PATH:$$(go env GOPATH)/bin && \
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/search.proto

# Build all services
build:
	@echo "Building services..."
	go build -o gateway ./cmd/gateway
	go build -o search ./cmd/search
	go build -o inference ./cmd/inference
	go build -o llm ./cmd/llm
	go build -o safety ./cmd/safety
	@echo "Build complete"

# Push all images to registry
push: build
	@echo "Pushing all images..."
	@for service in $(SERVICES); do \
		echo "Pushing $$service..."; \
		docker push $(DOCKER_REGISTRY)/$$service:$(VERSION); \
	done

# Run locally with Docker Compose (app only)
run-local:
	@echo "Starting application services locally..."
	docker-compose up --build gateway search inference llm safety redis ollama

# Run locally with Docker Compose (app + monitoring)
run-local-with-monitoring:
	@echo "Starting application and monitoring services locally..."
	docker-compose up --build

# Run monitoring stack only
run-monitoring:
	@echo "Starting monitoring stack only..."
	docker-compose up --build prometheus grafana node-exporter cadvisor

# Stop local services
stop-local:
	@echo "Stopping local services..."
	docker-compose down

# Stop and clean up everything
stop-clean:
	@echo "Stopping and cleaning up all services..."
	docker-compose down -v
	docker system prune -f

# Deploy to Kubernetes
deploy-k8s:
	@echo "Deploying to Kubernetes..."
	kubectl apply -f k8s/namespace.yaml
	kubectl apply -f k8s/configmap.yaml
	kubectl apply -f k8s/redis.yaml
	kubectl apply -f k8s/microservices.yaml
	kubectl apply -f k8s/hpa.yaml

# Remove from Kubernetes
undeploy-k8s:
	@echo "Removing from Kubernetes..."
	kubectl delete -f k8s/hpa.yaml --ignore-not-found=true
	kubectl delete -f k8s/microservices.yaml --ignore-not-found=true
	kubectl delete -f k8s/redis.yaml --ignore-not-found=true
	kubectl delete -f k8s/configmap.yaml --ignore-not-found=true
	kubectl delete -f k8s/namespace.yaml --ignore-not-found=true

# Create Google API secret (run manually with your credentials)
create-secret:
	@echo "Creating Google API secret..."
	@echo "Please run: kubectl create secret generic google-api-secret --from-literal=api-key=YOUR_API_KEY --from-literal=cx=YOUR_CX -n ai-search"

# Test the application
test:
	@echo "Running tests..."
	go test -v ./...

# Build and test individual service
build-service:
	@if [ -z "$(SERVICE)" ]; then echo "Usage: make build-service SERVICE=<service-name>"; exit 1; fi
	@echo "Building $(SERVICE)..."
	@if [ "$(SERVICE)" = "gateway" ]; then \
		docker build -f Dockerfile.gateway -t $(DOCKER_REGISTRY)/$(SERVICE):$(VERSION) .; \
	else \
		docker build -f Dockerfile.microservice --build-arg SERVICE_NAME=$(SERVICE) -t $(DOCKER_REGISTRY)/$(SERVICE):$(VERSION) .; \
	fi

# Run single service locally
run-service:
	@if [ -z "$(SERVICE)" ]; then echo "Usage: make run-service SERVICE=<service-name>"; exit 1; fi
	@echo "Running $(SERVICE) locally..."
	@if [ "$(SERVICE)" = "gateway" ]; then \
		go run ./cmd/gateway; \
	else \
		go run ./cmd/$(SERVICE); \
	fi

# Clean up
clean:
	@echo "Cleaning up..."
	docker system prune -f
	docker image prune -f

# Show service logs in Kubernetes
logs:
	@if [ -z "$(SERVICE)" ]; then echo "Usage: make logs SERVICE=<service-name>"; exit 1; fi
	kubectl logs -f deployment/$(SERVICE) -n ai-search

# Show service status in Kubernetes
status:
	@echo "Service status:"
	kubectl get pods -n ai-search
	kubectl get services -n ai-search
	kubectl get hpa -n ai-search

# Port forward for local access
port-forward:
	@echo "Port forwarding gateway service..."
	kubectl port-forward service/gateway-service 8080:80 -n ai-search

# Development setup
dev-setup:
	@echo "Setting up development environment..."
	go mod download
	@echo "Installing protoc plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Development setup complete!"

# Help
help:
	@echo "Available targets:"
	@echo "  all                    - Generate proto files and build all services"
	@echo "  proto                  - Generate protocol buffer files"
	@echo "  build                  - Build all Docker images"
	@echo "  push                   - Push all images to registry"
	@echo "  run-local              - Run application services locally (no monitoring)"
	@echo "  run-local-with-monitoring - Run app + monitoring services locally"
	@echo "  run-monitoring         - Run monitoring stack only"
	@echo "  stop-local             - Stop local services"
	@echo "  stop-clean             - Stop and clean up everything"
	@echo "  deploy-k8s             - Deploy to Kubernetes"
	@echo "  undeploy-k8s           - Remove from Kubernetes"
	@echo "  create-secret          - Show command to create Google API secret"
	@echo "  test                   - Run tests"
	@echo "  build-service          - Build single service (SERVICE=name)"
	@echo "  run-service            - Run single service locally (SERVICE=name)"
	@echo "  clean                  - Clean up Docker images"
	@echo "  logs                   - Show service logs (SERVICE=name)"
	@echo "  status                 - Show Kubernetes status"
	@echo "  port-forward           - Port forward gateway service"
	@echo "  dev-setup              - Set up development environment"
	@echo "  help                   - Show this help"
	@echo ""
	@echo "Monitoring URLs:"
	@echo "  Grafana Dashboard:     http://localhost:3000 (admin/admin)"
	@echo "  Prometheus:            http://localhost:9090"
	@echo "  Application Gateway:   http://localhost:8080" 