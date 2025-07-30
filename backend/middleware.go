package main

import (
	"bytes"
	"io"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// responseWriter wraps http.ResponseWriter to capture response body
type responseWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// BodyTracingMiddleware captures request and response bodies and adds them as events to the span
func BodyTracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		span := trace.SpanFromContext(r.Context())

		// Capture request body
		if r.Body != nil && r.Method != "GET" && r.Method != "DELETE" {
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil {
				// Add request body as an event
				span.AddEvent("http.request.body",
					trace.WithAttributes(
						attribute.String("body", string(bodyBytes)),
						attribute.Int("size", len(bodyBytes)),
					),
				)
				// Restore the body for the handler
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}

		// Wrap response writer to capture response body
		rw := &responseWriter{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
			statusCode:     http.StatusOK,
		}

		// Call the next handler
		next.ServeHTTP(rw, r)

		// Add response body as an event
		if rw.body.Len() > 0 {
			span.AddEvent("http.response.body",
				trace.WithAttributes(
					attribute.String("body", rw.body.String()),
					attribute.Int("size", rw.body.Len()),
					attribute.Int("status_code", rw.statusCode),
				),
			)
		}
	})
}
