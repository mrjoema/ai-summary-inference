#!/bin/bash

echo "🚀 Testing Single API Refactored System"
echo "======================================="

# Wait for services to be ready
wait_for_service() {
    echo "⏳ Waiting for services to be ready..."
    for i in {1..30}; do
        if curl -s http://localhost:8080/health > /dev/null 2>&1; then
            echo "✅ Services are ready!"
            return 0
        fi
        echo "   Attempt $i/30: Waiting..."
        sleep 2
    done
    echo "❌ Services failed to start within timeout"
    return 1
}

# Test JSON API (Non-streaming)
test_json_api() {
    echo ""
    echo "📝 Testing JSON API (Non-streaming)"
    echo "POST /api/v1/search"
    echo "-----------------------------------"
    
    curl -X POST http://localhost:8080/api/v1/search \
        -H "Content-Type: application/json" \
        -d '{
            "query": "artificial intelligence",
            "safe_search": true,
            "num_results": 3
        }' \
        -w "\nHTTP Status: %{http_code}\n" \
        2>/dev/null
}

# Test Streaming API (SSE)
test_streaming_api() {
    echo ""
    echo "🔄 Testing Streaming API (Server-Sent Events)"
    echo "GET /api/v1/search?query=AI&streaming=true"
    echo "--------------------------------------------"
    
    timeout 15 curl -N -H "Accept: text/event-stream" \
        "http://localhost:8080/api/v1/search?query=machine%20learning&streaming=true&safe_search=true&num_results=3" \
        2>/dev/null | head -20
}

# Test health endpoints
test_health() {
    echo ""
    echo "🏥 Testing Health Endpoints"
    echo "---------------------------"
    
    echo "Gateway Health:"
    curl -s http://localhost:8080/health | jq . 2>/dev/null || echo "Health check response"
    
    echo ""
    echo "Metrics Available:"
    curl -s http://localhost:8080/metrics | head -5
}

# Main execution
main() {
    echo "Starting services with: make run-local"
    echo ""
    
    # Start services in background
    make run-local > /dev/null 2>&1 &
    COMPOSE_PID=$!
    
    # Wait for services
    if wait_for_service; then
        test_health
        test_json_api
        test_streaming_api
        
        echo ""
        echo "🎉 Single API Testing Complete!"
        echo ""
        echo "Key Improvements:"
        echo "✅ Single endpoint for both streaming and non-streaming"
        echo "✅ No task management overhead"
        echo "✅ Immediate response streaming"
        echo "✅ Industry-standard SSE pattern"
        echo ""
        echo "Access Points:"
        echo "🌐 Web UI: http://localhost:8080"
        echo "📊 Metrics: http://localhost:8080/metrics"
        echo "🧪 Test Page: http://localhost:8080/test-single-api.html"
    else
        echo "❌ Failed to start services"
    fi
    
    # Cleanup
    echo ""
    echo "🧹 Cleaning up..."
    kill $COMPOSE_PID 2>/dev/null
    make stop-local > /dev/null 2>&1
}

main "$@"