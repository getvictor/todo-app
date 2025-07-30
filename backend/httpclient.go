package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// HTTPClient wraps http.Client with OpenTelemetry instrumentation
type HTTPClient struct {
	client *http.Client
}

// NewHTTPClient creates a new instrumented HTTP client
func NewHTTPClient() *HTTPClient {
	// Create transport with OTel instrumentation
	transport := otelhttp.NewTransport(http.DefaultTransport)

	return &HTTPClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
	}
}

// DoWithBodyCapture performs an HTTP request and captures request/response bodies as span events
func (c *HTTPClient) DoWithBodyCapture(ctx context.Context, req *http.Request) (*http.Response, error) {
	span := trace.SpanFromContext(ctx)

	// Capture request body if present
	var requestBody []byte
	if req.Body != nil {
		var err error
		requestBody, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		// Restore the body
		req.Body = io.NopCloser(bytes.NewReader(requestBody))

		// Add request body as event
		span.AddEvent("http.request.body",
			trace.WithAttributes(
				attribute.String("body", string(requestBody)),
				attribute.Int("size", len(requestBody)),
			),
		)
	}

	// Add request details
	span.SetAttributes(
		attribute.String("http.method", req.Method),
		attribute.String("http.url", req.URL.String()),
		attribute.String("http.host", req.Host),
	)

	// Perform the request
	resp, err := c.client.Do(req)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	// Capture response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Add response body as event
	span.AddEvent("http.response.body",
		trace.WithAttributes(
			attribute.String("body", string(responseBody)),
			attribute.Int("size", len(responseBody)),
			attribute.Int("status_code", resp.StatusCode),
		),
	)

	// Restore response body for caller
	resp.Body = io.NopCloser(bytes.NewReader(responseBody))

	// Add response attributes
	span.SetAttributes(
		attribute.Int("http.status_code", resp.StatusCode),
		attribute.String("http.status_text", http.StatusText(resp.StatusCode)),
	)

	return resp, nil
}
