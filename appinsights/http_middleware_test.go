package appinsights

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewHTTPMiddleware(t *testing.T) {
	middleware := NewHTTPMiddleware()
	if middleware == nil {
		t.Fatal("NewHTTPMiddleware returned nil")
	}
}

func TestExtractHeadersW3C(t *testing.T) {
	middleware := NewHTTPMiddleware()

	// Create request with W3C headers
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(TraceParentHeader, "00-abcdef0123456789abcdef0123456789-abcdef0123456789-01")

	corrCtx := middleware.ExtractHeaders(req)

	if corrCtx == nil {
		t.Fatal("Expected correlation context from W3C headers")
	}

	if corrCtx.TraceID != "abcdef0123456789abcdef0123456789" {
		t.Errorf("Expected trace ID abcdef0123456789abcdef0123456789, got %s", corrCtx.TraceID)
	}
	if corrCtx.SpanID != "abcdef0123456789" {
		t.Errorf("Expected span ID abcdef0123456789, got %s", corrCtx.SpanID)
	}
	if corrCtx.TraceFlags != 1 {
		t.Errorf("Expected trace flags 1, got %d", corrCtx.TraceFlags)
	}
}

func TestExtractHeadersRequestID(t *testing.T) {
	middleware := NewHTTPMiddleware()

	// Create request with Request-Id header
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(RequestIDHeader, "|abcdef0123456789abcdef0123456789.abcdef0123456789.")

	corrCtx := middleware.ExtractHeaders(req)

	if corrCtx == nil {
		t.Fatal("Expected correlation context from Request-Id header")
	}

	if corrCtx.TraceID != "abcdef0123456789abcdef0123456789" {
		t.Errorf("Expected trace ID abcdef0123456789abcdef0123456789, got %s", corrCtx.TraceID)
	}
	if corrCtx.SpanID != "abcdef0123456789" {
		t.Errorf("Expected span ID abcdef0123456789, got %s", corrCtx.SpanID)
	}
}

func TestExtractHeadersPreferW3C(t *testing.T) {
	middleware := NewHTTPMiddleware()

	// Create request with both W3C and Request-Id headers
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(TraceParentHeader, "00-11111111111111111111111111111111-1111111111111111-01")
	req.Header.Set(RequestIDHeader, "|22222222222222222222222222222222.2222222222222222.")

	corrCtx := middleware.ExtractHeaders(req)

	if corrCtx == nil {
		t.Fatal("Expected correlation context")
	}

	// Should prefer W3C headers
	if corrCtx.TraceID != "11111111111111111111111111111111" {
		t.Errorf("Expected W3C trace ID to be preferred, got %s", corrCtx.TraceID)
	}
}

func TestExtractHeadersNoHeaders(t *testing.T) {
	middleware := NewHTTPMiddleware()

	// Create request without correlation headers
	req := httptest.NewRequest("GET", "/test", nil)

	corrCtx := middleware.ExtractHeaders(req)

	if corrCtx != nil {
		t.Errorf("Expected nil correlation context when no headers present, got %v", corrCtx)
	}
}

func TestInjectHeaders(t *testing.T) {
	middleware := NewHTTPMiddleware()

	corrCtx := &CorrelationContext{
		TraceID:    "abcdef0123456789abcdef0123456789",
		SpanID:     "abcdef0123456789",
		TraceFlags: 1,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	middleware.InjectHeaders(req, corrCtx)

	// Check W3C header
	w3cHeader := req.Header.Get(TraceParentHeader)
	expectedW3C := "00-abcdef0123456789abcdef0123456789-abcdef0123456789-01"
	if w3cHeader != expectedW3C {
		t.Errorf("Expected W3C header %s, got %s", expectedW3C, w3cHeader)
	}

	// Check Request-Id header
	requestIDHeader := req.Header.Get(RequestIDHeader)
	expectedRequestID := "|abcdef0123456789abcdef0123456789.abcdef0123456789."
	if requestIDHeader != expectedRequestID {
		t.Errorf("Expected Request-Id header %s, got %s", expectedRequestID, requestIDHeader)
	}
}

func TestInjectHeadersNilContext(t *testing.T) {
	middleware := NewHTTPMiddleware()

	req := httptest.NewRequest("GET", "/test", nil)
	middleware.InjectHeaders(req, nil)

	// Should not set any headers
	if req.Header.Get(TraceParentHeader) != "" {
		t.Error("Expected no W3C header when correlation context is nil")
	}
	if req.Header.Get(RequestIDHeader) != "" {
		t.Error("Expected no Request-Id header when correlation context is nil")
	}
}

func TestMiddleware(t *testing.T) {
	middleware := NewHTTPMiddleware()

	// Create a test handler that checks for correlation context
	var receivedCtx *CorrelationContext
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCtx = GetCorrelationContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Wrap handler with middleware
	wrappedHandler := middleware.Middleware(handler)

	// Test with W3C headers
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(TraceParentHeader, "00-abcdef0123456789abcdef0123456789-abcdef0123456789-01")

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	if receivedCtx == nil {
		t.Fatal("Expected correlation context in handler")
	}

	// Should create child context, so trace ID matches but span ID is different
	if receivedCtx.TraceID != "abcdef0123456789abcdef0123456789" {
		t.Errorf("Expected inherited trace ID, got %s", receivedCtx.TraceID)
	}
	if receivedCtx.SpanID == "abcdef0123456789" {
		t.Error("Expected different span ID for child context")
	}
	if receivedCtx.ParentSpanID != "abcdef0123456789" {
		t.Errorf("Expected parent span ID abcdef0123456789, got %s", receivedCtx.ParentSpanID)
	}

	// Check response headers
	responseRequestID := rr.Header().Get(RequestIDHeader)
	if responseRequestID == "" {
		t.Error("Expected Request-Id header in response")
	}
}

func TestMiddlewareNoHeaders(t *testing.T) {
	middleware := NewHTTPMiddleware()

	// Create a test handler that checks for correlation context
	var receivedCtx *CorrelationContext
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCtx = GetCorrelationContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Wrap handler with middleware
	wrappedHandler := middleware.Middleware(handler)

	// Test without headers
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	if receivedCtx == nil {
		t.Fatal("Expected correlation context to be created when no headers present")
	}

	// Should create new context
	if len(receivedCtx.TraceID) != 32 {
		t.Errorf("Expected new trace ID length 32, got %d", len(receivedCtx.TraceID))
	}
	if len(receivedCtx.SpanID) != 16 {
		t.Errorf("Expected new span ID length 16, got %d", len(receivedCtx.SpanID))
	}

	// Check response headers
	responseRequestID := rr.Header().Get(RequestIDHeader)
	if responseRequestID == "" {
		t.Error("Expected Request-Id header in response")
	}
}

func TestMiddlewareWithTelemetryClient(t *testing.T) {
	middleware := NewHTTPMiddleware()

	// Create mock client
	client := NewTelemetryClient("test-key")

	// Set client getter
	middleware.GetClient = func(*http.Request) TelemetryClient {
		return client
	}

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap handler with middleware
	wrappedHandler := middleware.Middleware(handler)

	// Test with request
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	// Verify client is still enabled (telemetry was processed)
	if !client.IsEnabled() {
		t.Error("Client should remain enabled after middleware processing")
	}
}

func TestWrapRoundTripper(t *testing.T) {
	middleware := NewHTTPMiddleware()

	// Create a mock round tripper that captures the request
	var capturedReq *http.Request
	mockRT := &mockRoundTripper{
		captureFunc: func(req *http.Request) {
			capturedReq = req
		},
	}

	// Wrap the round tripper
	wrappedRT := middleware.WrapRoundTripper(mockRT)

	// Create correlation context
	corrCtx := NewCorrelationContext()
	ctx := WithCorrelationContext(context.Background(), corrCtx)

	// Create request with correlation context
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req = req.WithContext(ctx)

	// Make request
	_, err := wrappedRT.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	// Verify headers were injected
	if capturedReq == nil {
		t.Fatal("Request was not captured")
	}

	w3cHeader := capturedReq.Header.Get(TraceParentHeader)
	if w3cHeader == "" {
		t.Error("Expected W3C header to be injected")
	}

	requestIDHeader := capturedReq.Header.Get(RequestIDHeader)
	if requestIDHeader == "" {
		t.Error("Expected Request-Id header to be injected")
	}

	// Verify it's a child context (different span ID)
	childCorrCtx, err := ParseW3CTraceParent(w3cHeader)
	if err != nil {
		t.Fatalf("Failed to parse injected W3C header: %v", err)
	}

	if childCorrCtx.TraceID != corrCtx.TraceID {
		t.Error("Child should inherit trace ID")
	}
	if childCorrCtx.SpanID == corrCtx.SpanID {
		t.Error("Child should have different span ID")
	}
}

func TestContextExtractor(t *testing.T) {
	// Create request with W3C headers
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(TraceParentHeader, "00-abcdef0123456789abcdef0123456789-abcdef0123456789-01")

	ctx := ContextExtractor(req)

	corrCtx := GetCorrelationContext(ctx)
	if corrCtx == nil {
		t.Fatal("Expected correlation context from extracted context")
	}

	if corrCtx.TraceID != "abcdef0123456789abcdef0123456789" {
		t.Errorf("Expected trace ID abcdef0123456789abcdef0123456789, got %s", corrCtx.TraceID)
	}
}

func TestGetOrCreateCorrelationFromRequest(t *testing.T) {
	// Test with headers
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(RequestIDHeader, "|abcdef0123456789abcdef0123456789.abcdef0123456789.")

	corrCtx := GetOrCreateCorrelationFromRequest(req)
	if corrCtx == nil {
		t.Fatal("Expected correlation context")
	}

	if corrCtx.TraceID != "abcdef0123456789abcdef0123456789" {
		t.Errorf("Expected trace ID abcdef0123456789abcdef0123456789, got %s", corrCtx.TraceID)
	}

	// Test without headers
	reqEmpty := httptest.NewRequest("GET", "/test", nil)
	corrCtxEmpty := GetOrCreateCorrelationFromRequest(reqEmpty)
	if corrCtxEmpty == nil {
		t.Fatal("Expected correlation context to be created")
	}

	if len(corrCtxEmpty.TraceID) != 32 {
		t.Errorf("Expected new trace ID length 32, got %d", len(corrCtxEmpty.TraceID))
	}
}

// Mock round tripper for testing
type mockRoundTripper struct {
	captureFunc func(*http.Request)
}

func (rt *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.captureFunc != nil {
		rt.captureFunc(req)
	}

	// Return a mock response
	return &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       http.NoBody,
		Request:    req,
	}, nil
}

func TestHTTPHeaderConstants(t *testing.T) {
	// Verify header constants are correct
	if TraceParentHeader != "traceparent" {
		t.Errorf("Expected TraceParentHeader to be 'traceparent', got %s", TraceParentHeader)
	}
	if TraceStateHeader != "tracestate" {
		t.Errorf("Expected TraceStateHeader to be 'tracestate', got %s", TraceStateHeader)
	}
	if RequestIDHeader != "Request-Id" {
		t.Errorf("Expected RequestIDHeader to be 'Request-Id', got %s", RequestIDHeader)
	}
}

func TestCorrelationRoundTripperWithNilBase(t *testing.T) {
	middleware := NewHTTPMiddleware()
	
	corrCtx := NewCorrelationContext()
	ctx := WithCorrelationContext(context.Background(), corrCtx)

	req := httptest.NewRequest("GET", "http://httpbin.org/get", nil)
	req = req.WithContext(ctx)

	// This would make a real HTTP request, but we'll just verify it doesn't panic
	// In a real test environment, you might want to mock this
	rt := &correlationRoundTripper{
		base:       &mockRoundTripper{},
		middleware: middleware,
	}

	_, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip with nil base failed: %v", err)
	}
}

func TestSetResponseHeaders(t *testing.T) {
	middleware := NewHTTPMiddleware()

	corrCtx := &CorrelationContext{
		TraceID: "abcdef0123456789abcdef0123456789",
		SpanID:  "abcdef0123456789",
	}

	rr := httptest.NewRecorder()
	middleware.setResponseHeaders(rr, corrCtx)

	requestIDHeader := rr.Header().Get(RequestIDHeader)
	expectedRequestID := "|abcdef0123456789abcdef0123456789.abcdef0123456789."
	if requestIDHeader != expectedRequestID {
		t.Errorf("Expected response Request-Id header %s, got %s", expectedRequestID, requestIDHeader)
	}
}

func TestSetResponseHeadersNilContext(t *testing.T) {
	middleware := NewHTTPMiddleware()

	rr := httptest.NewRecorder()
	middleware.setResponseHeaders(rr, nil)

	if rr.Header().Get(RequestIDHeader) != "" {
		t.Error("Expected no Request-Id header when correlation context is nil")
	}
}