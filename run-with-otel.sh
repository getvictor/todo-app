#!/bin/bash

echo "Starting TODO app with Open Telemetry..."
echo ""

# Change to backend directory
cd backend

# Export environment variables
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
export OTEL_SERVICE_NAME=todo-app
export OTEL_SERVICE_VERSION=1.0.0

echo "Environment variables set:"
echo "  OTEL_EXPORTER_OTLP_ENDPOINT=$OTEL_EXPORTER_OTLP_ENDPOINT"
echo "  OTEL_SERVICE_NAME=$OTEL_SERVICE_NAME"
echo "  OTEL_SERVICE_VERSION=$OTEL_SERVICE_VERSION"
echo ""

# Run the app
go run .