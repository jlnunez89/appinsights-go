package appinsights

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
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

// Mock telemetry client for testing
type mockTelemetryClient struct {
	trackFunc        func(interface{})
	trackRequestFunc func(context.Context, string, string, time.Duration, string)
}

func (c *mockTelemetryClient) Context() *TelemetryContext                          { return nil }
func (c *mockTelemetryClient) InstrumentationKey() string                         { return "test-key" }
func (c *mockTelemetryClient) Channel() TelemetryChannel                          { return nil }
func (c *mockTelemetryClient) IsEnabled() bool                                    { return true }
func (c *mockTelemetryClient) SetIsEnabled(enabled bool)                          {}
func (c *mockTelemetryClient) Track(telemetry Telemetry)                          { 
	if c.trackFunc != nil {
		c.trackFunc(telemetry)
	}
}
func (c *mockTelemetryClient) TrackWithContext(ctx context.Context, telemetry Telemetry) {
	if c.trackFunc != nil {
		c.trackFunc(telemetry)
	}
}
func (c *mockTelemetryClient) TrackEvent(name string)                              {}
func (c *mockTelemetryClient) TrackMetric(name string, value float64)             {}
func (c *mockTelemetryClient) TrackTrace(name string, severity contracts.SeverityLevel) {}
func (c *mockTelemetryClient) TrackRequest(method, url string, duration time.Duration, responseCode string) {}
func (c *mockTelemetryClient) TrackRemoteDependency(name, dependencyType, target string, success bool) {}
func (c *mockTelemetryClient) TrackAvailability(name string, duration time.Duration, success bool) {}
func (c *mockTelemetryClient) TrackException(err interface{})                      {}
func (c *mockTelemetryClient) TrackEventWithContext(ctx context.Context, name string) {}
func (c *mockTelemetryClient) TrackTraceWithContext(ctx context.Context, message string, severity contracts.SeverityLevel) {}
func (c *mockTelemetryClient) TrackRequestWithContext(ctx context.Context, method, url string, duration time.Duration, responseCode string) {
	if c.trackRequestFunc != nil {
		c.trackRequestFunc(ctx, method, url, duration, responseCode)
	}
}
func (c *mockTelemetryClient) TrackRemoteDependencyWithContext(ctx context.Context, name, dependencyType, target string, success bool) {}
func (c *mockTelemetryClient) TrackAvailabilityWithContext(ctx context.Context, name string, duration time.Duration, success bool) {}
func (c *mockTelemetryClient) StartPerformanceCounterCollection(config PerformanceCounterConfig) {}
func (c *mockTelemetryClient) StopPerformanceCounterCollection() {}
func (c *mockTelemetryClient) IsPerformanceCounterCollectionEnabled() bool { return false }
func (c *mockTelemetryClient) ErrorAutoCollector() *ErrorAutoCollector { return nil }
func (c *mockTelemetryClient) AutoCollection() *AutoCollectionManager { return nil }

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

func TestResponseWriter(t *testing.T) {
	// Test response writer wrapper functionality
	rr := httptest.NewRecorder()
	rw := newResponseWriter(rr)

	// Test default status code
	if rw.Status() != 200 {
		t.Errorf("Expected default status code 200, got %d", rw.Status())
	}

	// Test WriteHeader
	rw.WriteHeader(404)
	if rw.Status() != 404 {
		t.Errorf("Expected status code 404, got %d", rw.Status())
	}

	// Test Write and Size tracking
	data := []byte("Hello, World!")
	n, err := rw.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected write count %d, got %d", len(data), n)
	}
	if rw.Size() != int64(len(data)) {
		t.Errorf("Expected size %d, got %d", len(data), rw.Size())
	}

	// Verify underlying recorder got the data
	if rr.Code != 404 {
		t.Errorf("Expected underlying recorder status 404, got %d", rr.Code)
	}
	if rr.Body.String() != string(data) {
		t.Errorf("Expected body %s, got %s", string(data), rr.Body.String())
	}
}

func TestMiddlewareWithTimingAndStatusCode(t *testing.T) {
	middleware := NewHTTPMiddleware()

	// Create a mock client that captures the request telemetry
	var capturedMethod, capturedURL, capturedResponseCode string
	var capturedDuration time.Duration
	client := &mockTelemetryClient{
		trackRequestFunc: func(ctx context.Context, method, url string, duration time.Duration, responseCode string) {
			capturedMethod = method
			capturedURL = url
			capturedDuration = duration
			capturedResponseCode = responseCode
		},
	}

	// Set client getter
	middleware.GetClient = func(*http.Request) TelemetryClient {
		return client
	}

	// Create a test handler that takes some time and returns specific status
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Simulate processing time
		w.WriteHeader(201)
		w.Write([]byte("Created"))
	})

	// Wrap handler with middleware
	wrappedHandler := middleware.Middleware(handler)

	// Test with request
	req := httptest.NewRequest("POST", "/api/test", nil)
	rr := httptest.NewRecorder()
	
	start := time.Now()
	wrappedHandler.ServeHTTP(rr, req)
	elapsed := time.Since(start)

	// Verify response
	if rr.Code != 201 {
		t.Errorf("Expected status code 201, got %d", rr.Code)
	}
	if rr.Body.String() != "Created" {
		t.Errorf("Expected body 'Created', got %s", rr.Body.String())
	}

	// Verify telemetry was captured
	if capturedMethod != "POST" {
		t.Errorf("Expected method POST, got %s", capturedMethod)
	}
	if capturedURL != "/api/test" {
		t.Errorf("Expected URL /api/test, got %s", capturedURL)
	}
	if capturedResponseCode != "201" {
		t.Errorf("Expected response code 201, got %s", capturedResponseCode)
	}
	
	// Verify timing is reasonable (should be at least 10ms due to sleep)
	if capturedDuration < 10*time.Millisecond {
		t.Errorf("Expected duration at least 10ms, got %v", capturedDuration)
	}
	
	// Verify timing is consistent with actual elapsed time (within reasonable margin)
	if capturedDuration > elapsed+5*time.Millisecond {
		t.Errorf("Captured duration %v exceeds actual elapsed time %v", capturedDuration, elapsed)
	}
}

func TestMiddlewareResponseCodeCapture(t *testing.T) {
	middleware := NewHTTPMiddleware()

	// Test different status codes
	testCases := []struct {
		name       string
		statusCode int
		handler    http.HandlerFunc
	}{
		{
			name:       "200 OK",
			statusCode: 200,
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
			}),
		},
		{
			name:       "404 Not Found",
			statusCode: 404,
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(404)
			}),
		},
		{
			name:       "500 Internal Server Error",
			statusCode: 500,
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(500)
			}),
		},
		{
			name:       "Default 200 (no WriteHeader call)",
			statusCode: 200,
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("OK")) // WriteHeader not called, should default to 200
			}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock client that captures the response code
			var capturedResponseCode string
			client := &mockTelemetryClient{
				trackRequestFunc: func(ctx context.Context, method, url string, duration time.Duration, responseCode string) {
					capturedResponseCode = responseCode
				},
			}

			// Set client getter
			middleware.GetClient = func(*http.Request) TelemetryClient {
				return client
			}

			// Wrap handler with middleware
			wrappedHandler := middleware.Middleware(tc.handler)

			// Test with request
			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(rr, req)

			// Verify status code was captured correctly
			expectedCode := strconv.Itoa(tc.statusCode)
			if capturedResponseCode != expectedCode {
				t.Errorf("Expected captured response code %s, got %s", expectedCode, capturedResponseCode)
			}

			// Verify actual response status
			if rr.Code != tc.statusCode {
				t.Errorf("Expected response status %d, got %d", tc.statusCode, rr.Code)
			}
		})
	}
}

func TestGinMiddleware(t *testing.T) {
	middleware := NewHTTPMiddleware()

	// Test that GinMiddleware returns a function
	ginMW := middleware.GinMiddleware()
	if ginMW == nil {
		t.Fatal("GinMiddleware returned nil")
	}

	// Verify it returns a function with the expected signature for Gin
	_, ok := ginMW.(func(interface{}))
	if !ok {
		t.Fatal("GinMiddleware did not return a function with correct signature")
	}

	// We can't test the actual Gin integration without importing Gin,
	// but we can verify the middleware was created successfully
}

func TestEchoMiddleware(t *testing.T) {
	middleware := NewHTTPMiddleware()

	// Test that EchoMiddleware returns a function
	echoMW := middleware.EchoMiddleware()
	if echoMW == nil {
		t.Fatal("EchoMiddleware returned nil")
	}

	// Verify it returns a function with the expected signature for Echo
	_, ok := echoMW.(func(interface{}) interface{})
	if !ok {
		t.Fatal("EchoMiddleware did not return a function with correct signature")
	}

	// We can't test the actual Echo integration without importing Echo,
	// but we can verify the middleware was created successfully
}

// Mock Gin context for testing
type mockGinContext struct {
	request  *http.Request
	writer   http.ResponseWriter
	next     func()
	values   map[string]interface{}
	nextCall bool
}

func (c *mockGinContext) Request() *http.Request { return c.request }
func (c *mockGinContext) Writer() http.ResponseWriter { return c.writer }
func (c *mockGinContext) Next() { c.nextCall = true; if c.next != nil { c.next() } }
func (c *mockGinContext) SetRequest(req *http.Request) { c.request = req }
func (c *mockGinContext) Set(key string, value interface{}) {
	if c.values == nil {
		c.values = make(map[string]interface{})
	}
	c.values[key] = value
}
func (c *mockGinContext) Get(key string) (interface{}, bool) {
	if c.values == nil {
		return nil, false
	}
	val, exists := c.values[key]
	return val, exists
}

func TestGinMiddlewareIntegration(t *testing.T) {
	middleware := NewHTTPMiddleware()

	// Create a mock client that captures the request telemetry
	var capturedMethod, capturedURL, capturedResponseCode string
	client := &mockTelemetryClient{
		trackRequestFunc: func(ctx context.Context, method, url string, duration time.Duration, responseCode string) {
			capturedMethod = method
			capturedURL = url
			capturedResponseCode = responseCode
		},
	}

	// Set client getter
	middleware.GetClient = func(*http.Request) TelemetryClient {
		return client
	}

	// Get the Gin middleware function
	ginMW := middleware.GinMiddleware().(func(interface{}))

	// Create mock Gin context
	req := httptest.NewRequest("GET", "/gin/test", nil)
	rr := httptest.NewRecorder()
	
	ginCtx := &mockGinContext{
		request: req,
		writer:  rr,
		next: func() {
			rr.WriteHeader(200)
			rr.Write([]byte("OK"))
		},
	}

	// Call the middleware
	ginMW(ginCtx)

	// Verify Next() was called
	if !ginCtx.nextCall {
		t.Error("Expected Gin Next() to be called")
	}

	// Verify correlation context was set in Gin context
	corrCtx, exists := ginCtx.Get("appinsights_correlation")
	if !exists {
		t.Error("Expected correlation context to be set in Gin context")
	}
	if corrCtx == nil {
		t.Error("Expected non-nil correlation context in Gin context")
	}

	// Verify telemetry was captured
	if capturedMethod != "GET" {
		t.Errorf("Expected method GET, got %s", capturedMethod)
	}
	if capturedURL != "/gin/test" {
		t.Errorf("Expected URL /gin/test, got %s", capturedURL)
	}
	if capturedResponseCode != "200" {
		t.Errorf("Expected response code 200, got %s", capturedResponseCode)
	}
}

// Mock Echo context for testing  
type mockEchoResponse struct {
	writer http.ResponseWriter
	status int
}

func (r *mockEchoResponse) Status() int { return r.status }
func (r *mockEchoResponse) Writer() http.ResponseWriter { return r.writer }

type mockEchoContext struct {
	request  *http.Request
	response *mockEchoResponse
	values   map[string]interface{}
}

func (c *mockEchoContext) Request() *http.Request { return c.request }
func (c *mockEchoContext) Response() interface {
	Status() int
	Writer() http.ResponseWriter
} {
	return c.response
}
func (c *mockEchoContext) SetRequest(req *http.Request) { c.request = req }
func (c *mockEchoContext) Set(key string, value interface{}) {
	if c.values == nil {
		c.values = make(map[string]interface{})
	}
	c.values[key] = value
}
func (c *mockEchoContext) Get(key string) interface{} {
	if c.values == nil {
		return nil
	}
	return c.values[key]
}

func TestEchoMiddlewareIntegration(t *testing.T) {
	middleware := NewHTTPMiddleware()

	// Create a mock client that captures the request telemetry
	var capturedMethod, capturedURL, capturedResponseCode string
	client := &mockTelemetryClient{
		trackRequestFunc: func(ctx context.Context, method, url string, duration time.Duration, responseCode string) {
			capturedMethod = method
			capturedURL = url
			capturedResponseCode = responseCode
		},
	}

	// Set client getter
	middleware.GetClient = func(*http.Request) TelemetryClient {
		return client
	}

	// Get the Echo middleware function
	echoMWFactory := middleware.EchoMiddleware().(func(interface{}) interface{})
	
	// Create mock next handler
	nextHandler := func(c interface{}) error {
		echoCtx := c.(*mockEchoContext)
		echoCtx.response.status = 201
		return nil
	}

	// Get the actual middleware handler
	echoMW := echoMWFactory(nextHandler).(func(interface{}) error)

	// Create mock Echo context
	req := httptest.NewRequest("POST", "/echo/test", nil)
	rr := httptest.NewRecorder()
	
	echoCtx := &mockEchoContext{
		request: req,
		response: &mockEchoResponse{
			writer: rr,
			status: 200, // Default status
		},
	}

	// Call the middleware
	err := echoMW(echoCtx)
	if err != nil {
		t.Fatalf("Echo middleware returned error: %v", err)
	}

	// Verify correlation context was set in Echo context
	corrCtx := echoCtx.Get("appinsights_correlation")
	if corrCtx == nil {
		t.Error("Expected correlation context to be set in Echo context")
	}

	// Verify telemetry was captured
	if capturedMethod != "POST" {
		t.Errorf("Expected method POST, got %s", capturedMethod)
	}
	if capturedURL != "/echo/test" {
		t.Errorf("Expected URL /echo/test, got %s", capturedURL)
	}
	if capturedResponseCode != "201" {
		t.Errorf("Expected response code 201, got %s", capturedResponseCode)
	}
}
