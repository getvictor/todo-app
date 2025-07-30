package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type Handlers struct {
	db              *DB
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
