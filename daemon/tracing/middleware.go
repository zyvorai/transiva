// SPDX-License-Identifier: Apache-2.0

package tracing

import (
	"context"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware creates HTTP middleware for tracing
type HTTPMiddleware struct {
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

// NewHTTPMiddleware creates a new HTTP tracing middleware
func NewHTTPMiddleware(tracer trace.Tracer) *HTTPMiddleware {
	return &HTTPMiddleware{
		tracer:     tracer,
		propagator: propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
	}
}

// Handler wraps an HTTP handler with tracing
func (m *HTTPMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract trace context from headers
		ctx := m.propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		// Start span
		spanName := r.Method + " " + r.URL.Path
		ctx, span := m.tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.String("http.path", r.URL.Path),
				attribute.String("http.scheme", r.URL.Scheme),
				attribute.String("http.host", r.Host),
				attribute.String("http.user_agent", r.UserAgent()),
				attribute.String("http.remote_addr", r.RemoteAddr),
			),
		)
		defer span.End()

		// Wrap response writer to capture status code
		wrappedWriter := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Record start time
		start := time.Now()

		// Call next handler
		next.ServeHTTP(wrappedWriter, r.WithContext(ctx))

		// Record duration
		duration := time.Since(start)

		// Set span attributes based on response
		span.SetAttributes(
			attribute.Int("http.status_code", wrappedWriter.statusCode),
			attribute.Int64("http.response_size", wrappedWriter.written),
			attribute.Float64("http.duration_ms", float64(duration.Milliseconds())),
		)

		// Set span status based on HTTP status code
		if wrappedWriter.statusCode >= 400 {
			span.SetStatus(codes.Error, http.StatusText(wrappedWriter.statusCode))
		} else {
			span.SetStatus(codes.Ok, "")
		}
	})
}

// statusRecorder wraps http.ResponseWriter to record status code
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

// WriteHeader records the status code
func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// Write records bytes written
func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.written += int64(n)
	return n, err
}

// InjectTraceContext injects trace context into HTTP headers
func InjectTraceContext(ctx context.Context, req *http.Request) {
	propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))
}

// ExtractTraceContext extracts trace context from HTTP headers
func ExtractTraceContext(ctx context.Context, req *http.Request) context.Context {
	propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	return propagator.Extract(ctx, propagation.HeaderCarrier(req.Header))
}
