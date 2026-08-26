// SPDX-License-Identifier: Apache-2.0

// Package tracing provides OpenTelemetry distributed tracing support
package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds tracing configuration
type Config struct {
	// Enabled determines if tracing is enabled
	Enabled bool

	// ServiceName is the name of the service
	ServiceName string

	// ServiceVersion is the version of the service
	ServiceVersion string

	// Environment is the deployment environment (dev, staging, prod)
	Environment string

	// Exporter specifies which exporter to use (jaeger, otlp, stdout)
	Exporter string

	// JaegerEndpoint is the Jaeger collector endpoint
	JaegerEndpoint string

	// OTLPEndpoint is the OTLP collector endpoint
	OTLPEndpoint string

	// SamplingRate is the trace sampling rate (0.0 to 1.0)
	SamplingRate float64

	// MaxExportBatchSize is the maximum batch size for export
	MaxExportBatchSize int

	// MaxQueueSize is the maximum queue size for spans
	MaxQueueSize int

	// ExportTimeout is the timeout for exporting spans
	ExportTimeout time.Duration
}

// DefaultConfig returns default tracing configuration
func DefaultConfig(serviceName string) *Config {
	return &Config{
		Enabled:            false,
		ServiceName:        serviceName,
		ServiceVersion:     "1.0.0",
		Environment:        "development",
		Exporter:           "stdout",
		JaegerEndpoint:     "http://localhost:14268/api/traces",
		OTLPEndpoint:       "localhost:4317",
		SamplingRate:       1.0,
		MaxExportBatchSize: 512,
		MaxQueueSize:       2048,
		ExportTimeout:      30 * time.Second,
	}
}

// Provider wraps the OpenTelemetry trace provider
type Provider struct {
	provider *sdktrace.TracerProvider
	config   *Config
}

// NewProvider creates a new tracing provider
func NewProvider(config *Config) (*Provider, error) {
	if !config.Enabled {
		// Return a no-op provider
		return &Provider{
			provider: sdktrace.NewTracerProvider(),
			config:   config,
		}, nil
	}

	// Create resource
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			semconv.DeploymentEnvironment(config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter
	exporter, err := createExporter(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create sampler
	var sampler sdktrace.Sampler
	if config.SamplingRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if config.SamplingRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(config.SamplingRate)
	}

	// Create provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(
			exporter,
			sdktrace.WithMaxExportBatchSize(config.MaxExportBatchSize),
			sdktrace.WithMaxQueueSize(config.MaxQueueSize),
			sdktrace.WithExportTimeout(config.ExportTimeout),
		),
		sdktrace.WithSampler(sampler),
	)

	// Set global provider
	otel.SetTracerProvider(provider)

	// Set global propagator
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return &Provider{
		provider: provider,
		config:   config,
	}, nil
}

// Shutdown shuts down the tracing provider
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.provider == nil {
		return nil
	}
	return p.provider.Shutdown(ctx)
}

// Tracer returns a tracer for the given name
func (p *Provider) Tracer(name string) trace.Tracer {
	if p.provider == nil {
		return otel.Tracer(name)
	}
	return p.provider.Tracer(name)
}

// createExporter creates a trace exporter based on configuration
func createExporter(config *Config) (sdktrace.SpanExporter, error) {
	switch config.Exporter {
	case "jaeger":
		return jaeger.New(
			jaeger.WithCollectorEndpoint(
				jaeger.WithEndpoint(config.JaegerEndpoint),
			),
		)

	case "otlp":
		client := otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(config.OTLPEndpoint),
			otlptracegrpc.WithInsecure(), // Use TLS in production
		)
		return otlptrace.New(context.Background(), client)

	case "stdout":
		return stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)

	default:
		return nil, fmt.Errorf("unsupported exporter: %s", config.Exporter)
	}
}

// SpanFromContext returns the span from context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// StartSpan starts a new span
func StartSpan(ctx context.Context, tracer trace.Tracer, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return tracer.Start(ctx, name, opts...)
}

// AddEvent adds an event to the current span
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetAttributes sets attributes on the current span
func SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// RecordError records an error on the current span
func RecordError(ctx context.Context, err error, opts ...trace.EventOption) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err, opts...)
}

// SetStatus sets the status of the current span
func SetStatus(ctx context.Context, code codes.Code, description string) {
	span := trace.SpanFromContext(ctx)
	span.SetStatus(code, description)
}

// Common attribute keys
var (
	AttrJobID        = attribute.Key("job.id")
	AttrJobName      = attribute.Key("job.name")
	AttrJobStatus    = attribute.Key("job.status")
	AttrVMName       = attribute.Key("vm.name")
	AttrVMPath       = attribute.Key("vm.path")
	AttrProvider     = attribute.Key("provider")
	AttrOperation    = attribute.Key("operation")
	AttrUserID       = attribute.Key("user.id")
	AttrUserName     = attribute.Key("user.name")
	AttrHTTPMethod   = attribute.Key("http.method")
	AttrHTTPPath     = attribute.Key("http.path")
	AttrHTTPStatus   = attribute.Key("http.status_code")
	AttrErrorType    = attribute.Key("error.type")
	AttrErrorMessage = attribute.Key("error.message")
)

// Helper functions for common span operations

// TraceJobExport traces a VM export job
func TraceJobExport(ctx context.Context, tracer trace.Tracer, jobID, jobName, vmName string) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, "job.export",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			AttrJobID.String(jobID),
			AttrJobName.String(jobName),
			AttrVMName.String(vmName),
		),
	)
	return ctx, span
}

// TraceProviderOperation traces a provider operation
func TraceProviderOperation(ctx context.Context, tracer trace.Tracer, provider, operation string) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, fmt.Sprintf("provider.%s.%s", provider, operation),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			AttrProvider.String(provider),
			AttrOperation.String(operation),
		),
	)
	return ctx, span
}

// TraceHTTPRequest traces an HTTP request
func TraceHTTPRequest(ctx context.Context, tracer trace.Tracer, method, path string) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, fmt.Sprintf("HTTP %s %s", method, path),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			AttrHTTPMethod.String(method),
			AttrHTTPPath.String(path),
		),
	)
	return ctx, span
}

// TraceDBOperation traces a database operation
func TraceDBOperation(ctx context.Context, tracer trace.Tracer, operation, table string) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, fmt.Sprintf("db.%s", operation),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.operation", operation),
			attribute.String("db.table", table),
		),
	)
	return ctx, span
}
