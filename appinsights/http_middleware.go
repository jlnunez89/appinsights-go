package appinsights

import (
	"context"
	"net/http"
)

// HTTP header constants
const (
	// W3C Trace Context headers
	TraceParentHeader = "traceparent"
	TraceStateHeader  = "tracestate"

	// Request-Id header for backward compatibility
	RequestIDHeader = "Request-Id"

	// Application Insights specific headers
	RequestContextHeader          = "Request-Context"
	RequestContextCorrelationKey  = "appId"
	RequestContextTargetKey       = "appId"
)

// HTTPMiddleware provides HTTP middleware for automatic header injection and extraction
type HTTPMiddleware struct {
	// Optional callback to get the telemetry client for requests
	GetClient func(*http.Request) TelemetryClient
}

// NewHTTPMiddleware creates a new HTTP middleware instance
func NewHTTPMiddleware() *HTTPMiddleware {
	return &HTTPMiddleware{}
}

// ExtractHeaders extracts correlation context from HTTP request headers
// Supports both W3C Trace Context and Request-Id headers
func (m *HTTPMiddleware) ExtractHeaders(r *http.Request) *CorrelationContext {
	// Try W3C Trace Context first (preferred)
	if traceParent := r.Header.Get(TraceParentHeader); traceParent != "" {
		if corrCtx, err := ParseW3CTraceParent(traceParent); err == nil {
			// TODO: Handle tracestate header if needed in the future
			return corrCtx
		}
	}

	// Fall back to Request-Id header for backward compatibility
	if requestID := r.Header.Get(RequestIDHeader); requestID != "" {
		if corrCtx, err := ParseRequestID(requestID); err == nil {
			return corrCtx
		}
	}

	// No correlation headers found, return nil
	return nil
}

// InjectHeaders injects correlation headers into an HTTP request
// Adds both W3C Trace Context and Request-Id headers for compatibility
func (m *HTTPMiddleware) InjectHeaders(r *http.Request, corrCtx *CorrelationContext) {
	if corrCtx == nil {
		return
	}

	// Set W3C Trace Context header (primary)
	r.Header.Set(TraceParentHeader, corrCtx.ToW3CTraceParent())

	// Set Request-Id header for backward compatibility
	r.Header.Set(RequestIDHeader, corrCtx.ToRequestID())

	// TODO: Handle tracestate header if needed in the future
}

// Middleware returns an HTTP middleware function that automatically handles correlation
// This middleware extracts correlation context from incoming requests and makes it available
// in the request context. It also enables telemetry tracking if a client getter is provided.
func (m *HTTPMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract correlation context from headers
		corrCtx := m.ExtractHeaders(r)

		// If no correlation context found, create a new one for this request
		if corrCtx == nil {
			corrCtx = NewCorrelationContext()
		} else {
			// Create a child context for this request to maintain trace hierarchy
			corrCtx = NewChildCorrelationContext(corrCtx)
		}

		// Add correlation context to request context
		ctx := WithCorrelationContext(r.Context(), corrCtx)
		r = r.WithContext(ctx)

		// Optional: Track the request if client getter is provided
		if m.GetClient != nil {
			if client := m.GetClient(r); client != nil {
				// Track the incoming request
				// Note: This is a simplified example. In practice, you might want to
				// track this after the request completes to get accurate timing and status
				client.TrackRequestWithContext(ctx, r.Method, r.URL.String(), 0, "")
			}
		}

		// Set correlation headers in response for client visibility
		m.setResponseHeaders(w, corrCtx)

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

// setResponseHeaders sets correlation headers in the HTTP response
func (m *HTTPMiddleware) setResponseHeaders(w http.ResponseWriter, corrCtx *CorrelationContext) {
	if corrCtx == nil {
		return
	}

	// Set Request-Id header in response for client correlation
	w.Header().Set(RequestIDHeader, corrCtx.ToRequestID())
}

// WrapRoundTripper wraps an http.RoundTripper to automatically inject correlation headers
// into outgoing HTTP requests. This enables distributed tracing across service boundaries.
func (m *HTTPMiddleware) WrapRoundTripper(rt http.RoundTripper) http.RoundTripper {
	return &correlationRoundTripper{
		base:       rt,
		middleware: m,
	}
}

// correlationRoundTripper implements http.RoundTripper with automatic header injection
type correlationRoundTripper struct {
	base       http.RoundTripper
	middleware *HTTPMiddleware
}

// RoundTrip implements http.RoundTripper interface
func (rt *correlationRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Extract correlation context from request context
	if corrCtx := GetCorrelationContext(req.Context()); corrCtx != nil {
		// Create a child context for the outgoing request
		childCtx := NewChildCorrelationContext(corrCtx)

		// Inject headers into the outgoing request
		rt.middleware.InjectHeaders(req, childCtx)
	}

	// Use base round tripper to make the actual request
	base := rt.base
	if base == nil {
		base = http.DefaultTransport
	}

	return base.RoundTrip(req)
}

// ContextExtractor is a helper function to extract correlation context from HTTP requests
// This can be used in HTTP handlers to get correlation context for telemetry tracking
func ContextExtractor(r *http.Request) context.Context {
	middleware := NewHTTPMiddleware()
	corrCtx := middleware.ExtractHeaders(r)

	if corrCtx != nil {
		return WithCorrelationContext(r.Context(), corrCtx)
	}

	return r.Context()
}

// GetOrCreateCorrelationFromRequest extracts or creates correlation context from an HTTP request
// This is a convenience function for HTTP handlers that need correlation context
func GetOrCreateCorrelationFromRequest(r *http.Request) *CorrelationContext {
	middleware := NewHTTPMiddleware()
	corrCtx := middleware.ExtractHeaders(r)

	if corrCtx == nil {
		corrCtx = NewCorrelationContext()
	}

	return corrCtx
}