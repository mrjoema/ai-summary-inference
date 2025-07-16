#!/bin/bash

# AI Search Engine with Monitoring - Startup Script
# This script provides an easy way to start the application and monitoring

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if Docker is running
check_docker() {
    if ! docker info > /dev/null 2>&1; then
        print_error "Docker is not running. Please start Docker and try again."
        exit 1
    fi
    print_success "Docker is running"
}

# Function to check if ports are available
check_ports() {
    local ports=("8080" "3000" "9090" "9100" "8085")
    local occupied_ports=()
    
    for port in "${ports[@]}"; do
        if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
            occupied_ports+=($port)
        fi
    done
    
    if [ ${#occupied_ports[@]} -ne 0 ]; then
        print_warning "The following ports are already in use: ${occupied_ports[*]}"
        print_warning "This might cause conflicts. Consider stopping other services using these ports."
        read -p "Continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
}

# Function to show usage
show_usage() {
    echo "AI Search Engine with Monitoring - Startup Script"
    echo ""
    echo "Usage: $0 [OPTION]"
    echo ""
    echo "Options:"
    echo "  app-only        Start application services only (no monitoring)"
    echo "  monitoring      Start monitoring stack only"
    echo "  full            Start everything (app + monitoring) [default]"
    echo "  stop            Stop all services"
    echo "  clean           Stop and clean up everything"
    echo "  status          Show service status"
    echo "  logs            Show logs for a specific service"
    echo "  help            Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0              # Start everything"
    echo "  $0 app-only     # Start app without monitoring"
    echo "  $0 stop         # Stop all services"
    echo "  $0 logs gateway # Show gateway logs"
    echo ""
    echo "Access URLs:"
    echo "  Application:    http://localhost:8080"
    echo "  Grafana:        http://localhost:3000 (admin/admin)"
    echo "  Prometheus:     http://localhost:9090"
    echo "  cAdvisor:       http://localhost:8085"
}

# Function to start services
start_services() {
    local mode=$1
    
    print_status "Checking prerequisites..."
    check_docker
    check_ports
    
    case $mode in
        "app-only")
            print_status "Starting application services only..."
            docker-compose up --build -d gateway search tokenizer inference safety llm
            print_success "Application services started!"
            print_status "Access your application at: http://localhost:8080"
            ;;
        "monitoring")
            print_status "Starting monitoring stack only..."
            docker-compose up --build -d prometheus grafana node-exporter cadvisor
            print_success "Monitoring stack started!"
            print_status "Access Grafana at: http://localhost:3000 (admin/admin)"
            print_status "Access Prometheus at: http://localhost:9090"
            ;;
        "full")
            print_status "Starting all services (application + monitoring)..."
            docker-compose up --build -d
            print_success "All services started!"
            print_status "Access URLs:"
            print_status "  Application:    http://localhost:8080"
            print_status "  Grafana:        http://localhost:3000 (admin/admin)"
            print_status "  Prometheus:     http://localhost:9090"
            ;;
    esac
    
    print_status "Waiting for services to be ready..."
    sleep 10
    
    print_status "Service status:"
    docker-compose ps
}

# Function to stop services
stop_services() {
    print_status "Stopping all services..."
    docker-compose down
    print_success "All services stopped!"
}

# Function to clean up
clean_up() {
    print_status "Stopping and cleaning up everything..."
    docker-compose down -v
    docker system prune -f
    print_success "Cleanup completed!"
}

# Function to show status
show_status() {
    print_status "Service status:"
    docker-compose ps
    echo ""
    print_status "Resource usage:"
    docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}"
}

# Function to show logs
show_logs() {
    local service=$1
    if [ -z "$service" ]; then
        print_error "Please specify a service name"
        echo "Available services: gateway, search, tokenizer, inference, safety, llm, prometheus, grafana"
        exit 1
    fi
    
    print_status "Showing logs for $service..."
    docker-compose logs -f $service
}

# Main script logic
case "${1:-full}" in
    "app-only"|"monitoring"|"full")
        start_services $1
        ;;
    "stop")
        stop_services
        ;;
    "clean")
        clean_up
        ;;
    "status")
        show_status
        ;;
    "logs")
        show_logs $2
        ;;
    "help"|"-h"|"--help")
        show_usage
        ;;
    *)
        print_error "Unknown option: $1"
        echo ""
        show_usage
        exit 1
        ;;
esac 