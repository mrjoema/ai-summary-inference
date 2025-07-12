#!/bin/sh

# Entrypoint script for microservices
# The SERVICE_NAME environment variable should be set by docker-compose

if [ -z "$SERVICE_NAME" ]; then
    echo "ERROR: SERVICE_NAME environment variable is not set"
    exit 1
fi

echo "Starting $SERVICE_NAME service..."
exec ./$SERVICE_NAME 