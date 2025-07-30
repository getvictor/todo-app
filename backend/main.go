package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const PORT = ":8082"

func main() {
	ctx := context.Background()

	// Initialize telemetry
	shutdown, err := InitTelemetry(ctx)
	if err != nil {
		log.Fatal("Failed to initialize telemetry:", err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			slog.Error("Failed to shutdown telemetry", "error", err)
		}
	}()

	slog.Info("Starting TODO app with OpenTelemetry instrumentation")
	db, err := NewDB("./tasks.db")
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	handlers := NewHandlers(db)

	// Serve frontend files
	fs := http.FileServer(http.Dir("../frontend"))
	http.Handle("/", fs)

	// Wrap task handlers with OpenTelemetry instrumentation and body tracing
	http.Handle("/tasks", otelhttp.NewHandler(BodyTracingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET", "OPTIONS":
			handlers.GetTasks(w, r)
		case "POST":
			handlers.CreateTask(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})), "tasks"))

	http.Handle("/tasks/", otelhttp.NewHandler(BodyTracingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" || r.Method == "OPTIONS" {
			handlers.DeleteTask(w, r)
		} else if r.Method == "POST" && len(r.URL.Path) > len("/tasks/") {
			pathSuffix := r.URL.Path[len("/tasks/"):]
			if len(pathSuffix) > 0 && pathSuffix[len(pathSuffix)-9:] == "/complete" {
				handlers.CompleteTask(w, r)
			} else {
				http.Error(w, "Not found", http.StatusNotFound)
			}
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})), "tasks/*"))

	// Create server with timeouts
	srv := &http.Server{
		Addr:         PORT,
		Handler:      nil, // Use default ServeMux
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		slog.Info("Server starting", "port", PORT)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed to start", "error", err)
			log.Fatal("Server failed to start:", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server exited")
}
