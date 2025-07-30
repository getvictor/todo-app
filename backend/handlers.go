package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type Handlers struct {
	db              *DB
	httpClient      *HTTPClient
	requestCounter  metric.Int64Counter
	requestDuration metric.Float64Histogram
}

func NewHandlers(db *DB) *Handlers {
	meter := GetMeter()

	requestCounter, _ := meter.Int64Counter("todo_app.requests",
		metric.WithDescription("Number of requests"),
		metric.WithUnit("1"))

	requestDuration, _ := meter.Float64Histogram("todo_app.request_duration",
		metric.WithDescription("Request duration in milliseconds"),
		metric.WithUnit("ms"))

	return &Handlers{
		db:              db,
		httpClient:      NewHTTPClient(),
		requestCounter:  requestCounter,
		requestDuration: requestDuration,
	}
}

func (h *Handlers) enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func (h *Handlers) GetTasks(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	h.enableCORS(w)

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Record metrics
	h.requestCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("method", "GET"),
			attribute.String("endpoint", "/tasks"),
		))

	span.SetAttributes(attribute.String("operation", "get_all_tasks"))
	slog.InfoContext(ctx, "Getting all tasks")

	tasks, err := h.db.GetAllTasks(ctx)
	if err != nil {
		span.RecordError(err)
		slog.ErrorContext(ctx, "Error getting tasks", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		h.recordRequestMetrics(ctx, start, "GET", "/tasks", http.StatusInternalServerError)
		return
	}

	if tasks == nil {
		tasks = []Task{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)

	slog.InfoContext(ctx, "Successfully retrieved tasks", "count", len(tasks))
	h.recordRequestMetrics(ctx, start, "GET", "/tasks", http.StatusOK)
}

func (h *Handlers) CreateTask(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	h.enableCORS(w)

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Title string `json:"title"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("operation", "create_task"),
		attribute.String("task.title", req.Title),
	)
	slog.InfoContext(ctx, "Creating new task", "title", req.Title)

	task, err := h.db.CreateTask(ctx, req.Title)
	if err != nil {
		span.RecordError(err)
		slog.ErrorContext(ctx, "Error creating task", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		h.recordRequestMetrics(ctx, start, "POST", "/tasks", http.StatusInternalServerError)
		return
	}

	// Make external API call to httpbin.org
	h.notifyExternalAPI(ctx, task)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)

	slog.InfoContext(ctx, "Task created successfully", "id", task.ID, "title", task.Title)
	h.recordRequestMetrics(ctx, start, "POST", "/tasks", http.StatusCreated)
}

func (h *Handlers) DeleteTask(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	h.enableCORS(w)

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/tasks/")
	id, err := strconv.Atoi(path)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("operation", "delete_task"),
		attribute.Int("task.id", id),
	)
	slog.InfoContext(ctx, "Deleting task", "id", id)

	err = h.db.DeleteTask(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			slog.WarnContext(ctx, "Task not found for deletion", "id", id)
			http.Error(w, "Task not found", http.StatusNotFound)
			h.recordRequestMetrics(ctx, start, "DELETE", "/tasks/:id", http.StatusNotFound)
		} else {
			span.RecordError(err)
			slog.ErrorContext(ctx, "Error deleting task", "error", err, "id", id)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			h.recordRequestMetrics(ctx, start, "DELETE", "/tasks/:id", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
	slog.InfoContext(ctx, "Task deleted successfully", "id", id)
	h.recordRequestMetrics(ctx, start, "DELETE", "/tasks/:id", http.StatusNoContent)
}

func (h *Handlers) CompleteTask(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	h.enableCORS(w)

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/tasks/")
	path = strings.TrimSuffix(path, "/complete")
	id, err := strconv.Atoi(path)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("operation", "complete_task"),
		attribute.Int("task.id", id),
	)
	slog.InfoContext(ctx, "Completing task", "id", id)

	task, err := h.db.CompleteTask(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			slog.WarnContext(ctx, "Task not found for completion", "id", id)
			http.Error(w, "Task not found", http.StatusNotFound)
			h.recordRequestMetrics(ctx, start, "POST", "/tasks/:id/complete", http.StatusNotFound)
		} else {
			span.RecordError(err)
			slog.ErrorContext(ctx, "Error completing task", "error", err, "id", id)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			h.recordRequestMetrics(ctx, start, "POST", "/tasks/:id/complete", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
	slog.InfoContext(ctx, "Task completed successfully", "id", task.ID, "title", task.Title)
	h.recordRequestMetrics(ctx, start, "POST", "/tasks/:id/complete", http.StatusOK)
}

func (h *Handlers) recordRequestMetrics(ctx context.Context, start time.Time, method, endpoint string, statusCode int) {
	duration := time.Since(start).Milliseconds()

	attrs := []attribute.KeyValue{
		attribute.String("method", method),
		attribute.String("endpoint", endpoint),
		attribute.Int("status_code", statusCode),
	}

	h.requestCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	h.requestDuration.Record(ctx, float64(duration), metric.WithAttributes(attrs...))
}

// notifyExternalAPI makes an external API call to httpbin.org after task creation
func (h *Handlers) notifyExternalAPI(ctx context.Context, task *Task) {
	// Create a new span for the external API call
	ctx, span := GetTracer().Start(ctx, "external.api.notification",
		trace.WithAttributes(
			attribute.String("api.service", "httpbin.org"),
			attribute.Int("task.id", task.ID),
			attribute.String("task.title", task.Title),
		))
	defer span.End()

	// Prepare the request with properly encoded parameters
	params := url.Values{}
	params.Add("task_id", fmt.Sprintf("%d", task.ID))
	params.Add("task_title", task.Title)
	apiURL := fmt.Sprintf("https://httpbin.org/get?%s", params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		span.RecordError(err)
		slog.ErrorContext(ctx, "Failed to create external API request", "error", err)
		return
	}

	// Add custom headers
	req.Header.Set("X-Task-ID", fmt.Sprintf("%d", task.ID))
	req.Header.Set("X-Task-Title", task.Title)
	req.Header.Set("User-Agent", "todo-app/1.0")

	// Make the request with body capture
	resp, err := h.httpClient.DoWithBodyCapture(ctx, req)
	if err != nil {
		span.RecordError(err)
		slog.ErrorContext(ctx, "External API call failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		slog.InfoContext(ctx, "Successfully notified external API",
			"task_id", task.ID,
			"status_code", resp.StatusCode)
	} else {
		slog.WarnContext(ctx, "External API returned non-success status",
			"task_id", task.ID,
			"status_code", resp.StatusCode)
	}
}
