#!/bin/bash

echo "Testing Output Safety Validation..."

# Test 1: Direct safety service output sanitization
echo "=== Test 1: Direct Safety Service Test ==="
curl -X POST http://localhost:8084/api/v1/sanitize-output \
  -H "Content-Type: application/json" \
  -d '{"text": "This is a test summary with some damn problematic content that should be filtered."}' | jq .

echo ""

# Test 2: Check if safety service is reachable from gateway
echo "=== Test 2: Gateway to Safety Service Connection ==="
curl -X POST http://localhost:8080/api/v1/validate \
  -H "Content-Type: application/json" \
  -d '{"text": "test validation"}' | jq .

echo ""

# Test 3: Full search with monitoring
echo "=== Test 3: Full Search with Log Monitoring ==="
echo "Starting search request..."
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{"query": "AI safety test", "safe_search": false}' &

# Wait a moment then check logs
sleep 3
echo "Checking safety service logs for output sanitization..."
docker-compose logs safety | grep -i "sanitiz" | tail -5

echo "Checking gateway logs..."
docker-compose logs gateway | grep -i "sanitiz" | tail -5