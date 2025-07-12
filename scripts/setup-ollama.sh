#!/bin/bash

# Setup script for Ollama with Llama 3.2 3B model
# This script will pull the model in the Docker container

echo "Setting up Ollama with Llama 3.2 3B model..."

# Wait for Ollama service to be ready
echo "Waiting for Ollama service to be ready..."
until curl -s http://localhost:11434/api/tags > /dev/null 2>&1; do
  echo "Waiting for Ollama..."
  sleep 2
done

echo "Ollama is ready! Pulling Llama 3.2 3B model..."

# Pull the model
docker exec ai-summary-inference-ollama-1 ollama pull llama3.2:3b

echo "Model setup complete!"
echo "You can now use the AI summary service with real model inference." 