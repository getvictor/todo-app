# TODO App Architecture

## Overview
A simple TODO application designed to demonstrate OpenTelemetry features with observability built-in from the ground up.

## Technology Stack
- **Frontend**: Vanilla JavaScript (HTML/CSS/JS)
- **Backend**: Go with standard library HTTP server
- **Database**: SQLite
- **Observability**: OpenTelemetry (backend only)

## System Architecture

```
┌─────────────────┐
│   Frontend      │
│ (Vanilla JS)    │
│                 │
│ - HTML/CSS/JS   │
│ - Fetch API     │
└────────┬────────┘
         │ HTTP
         │
┌────────▼────────┐
│   Backend API   │
│      (Go)       │
│                 │
│ - net/http      │
│ - OpenTelemetry │
└────────┬────────┘
         │
┌────────▼────────┐
│    Database     │
│    (SQLite)     │
│                 │
│ - tasks table   │
└─────────────────┘
```

## API Endpoints

### GET /tasks
- **Description**: Retrieve all tasks
- **Response**: JSON array of task objects
- **Example Response**:
```json
[
  {
    "id": 1,
    "title": "Complete architecture document",
    "completed": false,
    "created_at": "2025-01-29T10:00:00Z"
  }
]
```

### POST /tasks
- **Description**: Create a new task
- **Request Body**:
```json
{
  "title": "New task title",
  "completed": false
}
```
- **Response**: Created task object with generated ID

### DELETE /tasks/:id
- **Description**: Delete a task by ID
- **Parameters**: `id` - Task ID (integer)
- **Response**: 204 No Content on success

### POST /tasks/:id/complete
- **Description**: Mark a task as complete
- **Parameters**: `id` - Task ID (integer)
- **Response**: Updated task object
- **Example Response**:
```json
{
  "id": 1,
  "title": "Complete architecture document",
  "completed": true,
  "created_at": "2025-01-29T10:00:00Z"
}
```

## Database Schema

### tasks table
```sql
CREATE TABLE tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    completed BOOLEAN DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## OpenTelemetry Integration

### Instrumentation Points
1. **HTTP Requests**: Auto-instrumentation for Go HTTP handlers
2. **Database Operations**: Manual spans for SQLite queries
3. **Business Logic**: Custom spans for task operations

### Telemetry Data
- **Traces**: Request flow from frontend to database
- **Metrics**: Request count, latency, error rates
- **Logs**: Structured logging with trace correlation

### Exporters
- Console exporter for development
- OTLP exporter for production (configurable endpoint)

## Project Structure
```
todo-app/
├── frontend/
│   ├── index.html
│   ├── style.css
│   └── app.js
├── backend/
│   ├── main.go
│   ├── handlers.go
│   ├── db.go
│   ├── models.go
│   └── telemetry.go
├── go.mod
├── go.sum
├── .gitignore
└── README.md
```

## Development Workflow
1. Initialize SQLite database with schema
2. Set up OpenTelemetry SDK and instrumentation
3. Implement API endpoints with proper error handling
4. Create frontend UI with task management features
5. Add telemetry visualization dashboard (optional)

## Key Features to Demonstrate
- Request tracing through HTTP handlers to database
- Error tracking and alerting
- Performance monitoring
- Resource utilization metrics
- Custom business metrics (tasks created/completed per hour)
- Span attributes for debugging (task IDs, operation types)