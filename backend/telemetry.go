package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.opentelemetry.io/otel/trace"
)

func InitTelemetry(ctx context.Context) (shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error

	shutdown = func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = fn(ctx)
		}
		return err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("todo-app"),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return shutdown, fmt.Errorf("failed to create resource: %w", err)
	}

	// Set up trace exporter based on environment
	var traceExporter sdktrace.SpanExporter
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	if otlpEndpoint != "" {
		fmt.Printf("Connecting to OTLP endpoint: %s\n", otlpEndpoint)
		// Use OTLP exporter for production
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		traceExporter, err = otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(otlpEndpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return shutdown, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
		}
	} else {
		// Use console exporter for development
		traceExporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return shutdown, fmt.Errorf("failed to create stdout trace exporter: %w", err)
		}
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Set up metric exporter based on environment
	var metricExporter sdkmetric.Exporter

	if otlpEndpoint != "" {
		// Use OTLP exporter for production
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		metricExporter, err = otlpmetricgrpc.New(ctx,
			otlpmetricgrpc.WithEndpoint(otlpEndpoint),
			otlpmetricgrpc.WithInsecure(),
		)
		if err != nil {
			return shutdown, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
		}
	} else {
		// Use console exporter for development
		metricExporter, err = stdoutmetric.New()
		if err != nil {
			return shutdown, fmt.Errorf("failed to create stdout metric exporter: %w", err)
		}
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

	// Set up log exporter based on environment
	var logExporter log.Exporter

	if otlpEndpoint != "" {
		// Use OTLP exporter for production
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		logExporter, err = otlploggrpc.New(ctx,
			otlploggrpc.WithEndpoint(otlpEndpoint),
			otlploggrpc.WithInsecure(),
		)
		if err != nil {
			return shutdown, fmt.Errorf("failed to create OTLP log exporter: %w", err)
		}
	} else {
		// Use console exporter for development
		logExporter, err = stdoutlog.New()
		if err != nil {
			return shutdown, fmt.Errorf("failed to create stdout log exporter: %w", err)
		}
	}

	loggerProvider := log.NewLoggerProvider(
		log.WithProcessor(log.NewBatchProcessor(logExporter)),
		log.WithResource(res),
	)
	shutdownFuncs = append(shutdownFuncs, loggerProvider.Shutdown)
	global.SetLoggerProvider(loggerProvider)

	// Set up slog with OpenTelemetry bridge
	logger := otelslog.NewLogger("todo-app")
	slog.SetDefault(logger)

	return shutdown, nil
}

// GetTracer returns the OpenTelemetry tracer for the todo-app
func GetTracer() trace.Tracer {
	return otel.Tracer("todo-app")
}

// GetMeter returns the OpenTelemetry meter for the todo-app
func GetMeter() metric.Meter {
	return otel.Meter("todo-app")
}
