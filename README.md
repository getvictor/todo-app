# Todo App - OpenTelemetry Exploration

A simple todo application built to explore and demonstrate OpenTelemetry (OTel) features including distributed tracing, metrics, and logging with correlation.

## Overview

This application consists of:
- **Backend**: Go API with SQLite database
- **Frontend**: Simple HTML/JavaScript interface
- **Full OTel instrumentation**: Traces, metrics, and logs

## Features Demonstrated

### Tracing
- HTTP server instrumentation with request/response body capture
- Database query tracing with actual SQL parameters
- External API calls to httpbin.org with distributed trace propagation
- Custom spans with attributes and events
- Error tracking with stack traces
- Trace context propagation via W3C Trace Context

### Metrics
- HTTP request duration and count
- Database connection pool statistics
- Custom application metrics

### Logging
- Structured logging with slog
- Automatic trace context injection (trace_id, span_id)
- Log-to-trace correlation

## Running the Application

### Backend
```bash
cd backend
go run .
```

The backend runs on **port 8082** by default.

### Frontend
Open `frontend/index.html` in a web browser or serve it with any static file server.

## Configuration

### Environment Variables

- `OTEL_EXPORTER_OTLP_ENDPOINT`: OTLP collector endpoint (e.g., `localhost:4317`)
  - If not set, telemetry outputs to console
  - If set, exports to OTLP gRPC endpoint

### Port Configuration

The backend port is defined as a constant in `backend/main.go`:
```go
const PORT = ":8082"
```

## Special Features

### Error Simulation
Create a task with the title "errorTest" to trigger a simulated error that demonstrates:
- Error recording in spans
- Stack trace capture
- Error propagation through trace hierarchy

### External API Integration
Every task creation triggers an async call to httpbin.org that demonstrates:
- Distributed tracing across services
- HTTP client instrumentation
- Request/response body capture in traces

### SQL Query Visibility
All database queries show:
- Original SQL with placeholders (`db.statement`)
- Formatted SQL with actual values (`db.statement.formatted`)
- Query execution timing

## Telemetry Outputs

### Console Mode (Development)
When `OTEL_EXPORTER_OTLP_ENDPOINT` is not set:
- Traces output as formatted JSON
- Metrics output periodically
- Logs include trace context

### OTLP Mode (Production)
When `OTEL_EXPORTER_OTLP_ENDPOINT` is set:
- All telemetry sent to configured endpoint
- Compatible with Jaeger, Tempo, Datadog, etc.

## API Endpoints

- `GET /tasks` - List all tasks
- `POST /tasks` - Create a new task
- `POST /tasks/:id/complete` - Mark task as complete
- `DELETE /tasks/:id` - Delete a task

## Development Notes

This app intentionally includes extensive telemetry for learning purposes. In production, you might want to:
- Reduce span verbosity
- Sample traces
- Exclude sensitive data from logs/traces
- Use async exporters with batching
