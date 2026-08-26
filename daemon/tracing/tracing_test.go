// SPDX-License-Identifier: Apache-2.0

package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig("test-service")

	if config.ServiceName != "test-service" {
		t.Errorf("expected service name 'test-service', got %s", config.ServiceName)
	}

	if config.Enabled {
		t.Error("expected tracing to be disabled by default")
	}

	if config.Exporter != "stdout" {
		t.Errorf("expected stdout exporter, got %s", config.Exporter)
	}

	if config.SamplingRate != 1.0 {
		t.Errorf("expected sampling rate 1.0, got %f", config.SamplingRate)
	}
}

func TestNewProvider_Disabled(t *testing.T) {
	config := &Config{
		Enabled:     false,
		ServiceName: "test",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	if provider == nil {
		t.Fatal("expected provider to be created")
	}

	err = provider.Shutdown(context.Background())
	if err != nil {
		t.Errorf("shutdown failed: %v", err)
	}
}

func TestNewProvider_Stdout(t *testing.T) {
	config := &Config{
		Enabled:        true,
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		Exporter:       "stdout",
		SamplingRate:   1.0,
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	if provider == nil {
		t.Fatal("expected provider to be created")
	}

	defer func() { _ = provider.Shutdown(context.Background()) }()

	// Create a span to verify provider works
	tracer := provider.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	span.End()

	if ctx == nil {
		t.Error("expected context to be returned")
	}
}

func TestNewProvider_InvalidExporter(t *testing.T) {
	config := &Config{
		Enabled:     true,
		ServiceName: "test",
		Exporter:    "invalid",
	}

	_, err := NewProvider(config)
	if err == nil {
		t.Error("expected error for invalid exporter")
	}
}

func TestNewProvider_Sampling(t *testing.T) {
	tests := []struct {
		name         string
		samplingRate float64
	}{
		{"always sample", 1.0},
		{"never sample", 0.0},
		{"partial sample", 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Enabled:      true,
				ServiceName:  "test",
				Exporter:     "stdout",
				SamplingRate: tt.samplingRate,
			}

			provider, err := NewProvider(config)
			if err != nil {
				t.Fatalf("failed to create provider: %v", err)
			}
			defer func() { _ = provider.Shutdown(context.Background()) }()

			if provider == nil {
				t.Error("expected provider to be created")
			}
		})
	}
}

func TestStartSpan(t *testing.T) {
	config := &Config{
		Enabled:     true,
		ServiceName: "test",
		Exporter:    "stdout",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer func() { _ = provider.Shutdown(context.Background()) }()

	tracer := provider.Tracer("test")

	// Test StartSpan helper function
	ctx, span := StartSpan(context.Background(), tracer, "test-span-helper")
	defer span.End()

	if ctx == nil {
		t.Error("expected context to be returned")
	}

	if span == nil {
		t.Error("expected span to be created")
	}

	// Verify span is in context
	retrievedSpan := trace.SpanFromContext(ctx)
	if !retrievedSpan.SpanContext().IsValid() {
		t.Error("expected valid span in context")
	}

	// Test with additional span options
	ctx2, span2 := StartSpan(context.Background(), tracer, "test-span-with-attrs",
		trace.WithAttributes(attribute.String("key", "value")),
	)
	defer span2.End()

	if ctx2 == nil {
		t.Error("expected context to be returned with options")
	}
}

func TestSpanHelpers(t *testing.T) {
	config := &Config{
		Enabled:     true,
		ServiceName: "test",
		Exporter:    "stdout",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer func() { _ = provider.Shutdown(context.Background()) }()

	tracer := provider.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	// Test AddEvent
	AddEvent(ctx, "test-event",
		attribute.String("key", "value"),
	)

	// Test SetAttributes
	SetAttributes(ctx,
		attribute.String("attr1", "value1"),
		attribute.Int("attr2", 42),
	)

	// Test RecordError
	testErr := &testError{msg: "test error"}
	RecordError(ctx, testErr)

	// Test SetStatus
	SetStatus(ctx, codes.Error, "test error")

	// Test SpanFromContext
	retrievedSpan := SpanFromContext(ctx)
	if retrievedSpan == nil {
		t.Error("expected span from context")
	}
}

func TestTraceJobExport(t *testing.T) {
	config := &Config{
		Enabled:     true,
		ServiceName: "test",
		Exporter:    "stdout",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer func() { _ = provider.Shutdown(context.Background()) }()

	tracer := provider.Tracer("test")
	ctx, span := TraceJobExport(context.Background(), tracer, "job123", "export-vm", "test-vm")
	defer span.End()

	if ctx == nil {
		t.Error("expected context to be returned")
	}

	if span == nil {
		t.Error("expected span to be created")
	}
}

func TestTraceProviderOperation(t *testing.T) {
	config := &Config{
		Enabled:     true,
		ServiceName: "test",
		Exporter:    "stdout",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer func() { _ = provider.Shutdown(context.Background()) }()

	tracer := provider.Tracer("test")
	ctx, span := TraceProviderOperation(context.Background(), tracer, "aws", "export-instance")
	defer span.End()

	if ctx == nil {
		t.Error("expected context to be returned")
	}
}

func TestTraceHTTPRequest(t *testing.T) {
	config := &Config{
		Enabled:     true,
		ServiceName: "test",
		Exporter:    "stdout",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer func() { _ = provider.Shutdown(context.Background()) }()

	tracer := provider.Tracer("test")
	ctx, span := TraceHTTPRequest(context.Background(), tracer, "GET", "/api/jobs")
	defer span.End()

	if ctx == nil {
		t.Error("expected context to be returned")
	}
}

func TestTraceDBOperation(t *testing.T) {
	config := &Config{
		Enabled:     true,
		ServiceName: "test",
		Exporter:    "stdout",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer func() { _ = provider.Shutdown(context.Background()) }()

	tracer := provider.Tracer("test")
	ctx, span := TraceDBOperation(context.Background(), tracer, "SELECT", "jobs")
	defer span.End()

	if ctx == nil {
		t.Error("expected context to be returned")
	}
}

func TestHTTPMiddleware(t *testing.T) {
	config := &Config{
		Enabled:     true,
		ServiceName: "test",
		Exporter:    "stdout",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer func() { _ = provider.Shutdown(context.Background()) }()

	tracer := provider.Tracer("test")
	middleware := NewHTTPMiddleware(tracer)

	// Create test handler
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify span is in context
		span := trace.SpanFromContext(r.Context())
		if !span.SpanContext().IsValid() {
			t.Error("expected valid span in context")
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	// Execute request
	handler.ServeHTTP(rr, req)

	// Verify response
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHTTPMiddleware_ErrorStatus(t *testing.T) {
	config := &Config{
		Enabled:     true,
		ServiceName: "test",
		Exporter:    "stdout",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer func() { _ = provider.Shutdown(context.Background()) }()

	tracer := provider.Tracer("test")
	middleware := NewHTTPMiddleware(tracer)

	// Create handler that returns error
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Error"))
	}))

	req := httptest.NewRequest("POST", "/error", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestHTTPMiddleware_TraceContextPropagation(t *testing.T) {
	config := &Config{
		Enabled:     true,
		ServiceName: "test",
		Exporter:    "stdout",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer func() { _ = provider.Shutdown(context.Background()) }()

	tracer := provider.Tracer("test")

	// Create initial span
	ctx, span := tracer.Start(context.Background(), "parent-span")
	defer span.End()

	// Create request with trace context
	req := httptest.NewRequest("GET", "/test", nil)
	InjectTraceContext(ctx, req)

	// Verify trace context was injected
	if req.Header.Get("traceparent") == "" {
		t.Error("expected traceparent header to be set")
	}

	// Extract context
	extractedCtx := ExtractTraceContext(context.Background(), req)
	extractedSpan := trace.SpanFromContext(extractedCtx)

	// Verify span context was extracted
	if !extractedSpan.SpanContext().IsValid() {
		t.Error("expected valid span context after extraction")
	}
}

func TestStatusRecorder(t *testing.T) {
	rr := httptest.NewRecorder()
	recorder := &statusRecorder{
		ResponseWriter: rr,
		statusCode:     http.StatusOK,
	}

	// Test WriteHeader
	recorder.WriteHeader(http.StatusCreated)
	if recorder.statusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", recorder.statusCode)
	}

	// Test Write
	data := []byte("test data")
	n, err := recorder.Write(data)
	if err != nil {
		t.Errorf("write failed: %v", err)
	}

	if n != len(data) {
		t.Errorf("expected to write %d bytes, wrote %d", len(data), n)
	}

	if recorder.written != int64(len(data)) {
		t.Errorf("expected written %d, got %d", len(data), recorder.written)
	}
}

func TestAttributeKeys(t *testing.T) {
	// Verify attribute keys are defined
	attrs := []attribute.Key{
		AttrJobID,
		AttrJobName,
		AttrJobStatus,
		AttrVMName,
		AttrVMPath,
		AttrProvider,
		AttrOperation,
		AttrUserID,
		AttrUserName,
		AttrHTTPMethod,
		AttrHTTPPath,
		AttrHTTPStatus,
		AttrErrorType,
		AttrErrorMessage,
	}

	if len(attrs) != 14 {
		t.Errorf("expected 14 attribute keys, got %d", len(attrs))
	}

	// Test attribute usage
	attr := AttrJobID.String("job123")
	if attr.Key != AttrJobID {
		t.Error("attribute key mismatch")
	}
}

func TestProviderShutdown(t *testing.T) {
	config := &Config{
		Enabled:     true,
		ServiceName: "test",
		Exporter:    "stdout",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Create some spans
	tracer := provider.Tracer("test")
	_, span1 := tracer.Start(context.Background(), "span1")
	span1.End()

	_, span2 := tracer.Start(context.Background(), "span2")
	span2.End()

	// Shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = provider.Shutdown(ctx)
	if err != nil {
		t.Errorf("shutdown failed: %v", err)
	}

	// Verify shutdown can be called multiple times
	err = provider.Shutdown(context.Background())
	if err != nil {
		t.Errorf("second shutdown failed: %v", err)
	}
}

// Helper types for testing

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
