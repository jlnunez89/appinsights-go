package appinsights

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStartSpan(t *testing.T) {
	client := NewTelemetryClient("test-key")
	ctx := context.Background()
	operationName := "test-operation"

	// Test with no parent context
	newCtx, span := StartSpan(ctx, operationName, client)

	if span == nil {
		t.Fatal("StartSpan should return a span context")
	}

	if span.Context == nil {
		t.Fatal("Span context should not be nil")
	}

	if span.Context.OperationName != operationName {
		t.Errorf("Expected operation name %s, got %s", operationName, span.Context.OperationName)
	}

	if span.Client != client {
		t.Error("Span should reference the provided client")
	}

	// Verify correlation context is attached to the new context
	corrCtx := GetCorrelationContext(newCtx)
	if corrCtx == nil {
		t.Fatal("New context should have correlation context attached")
	}

	if corrCtx != span.Context {
		t.Error("Context correlation should match span correlation")
	}
}

func TestStartSpanWithParent(t *testing.T) {
	client := NewTelemetryClient("test-key")
	
	// Create parent context
	parentCorr := NewCorrelationContext()
	parentCorr.OperationName = "parent-operation"
	parentCtx := WithCorrelationContext(context.Background(), parentCorr)

	// Start child span
	childCtx, childSpan := StartSpan(parentCtx, "child-operation", client)

	if childSpan.Context.TraceID != parentCorr.TraceID {
		t.Error("Child span should inherit parent trace ID")
	}

	if childSpan.Context.ParentSpanID != parentCorr.SpanID {
		t.Error("Child span parent ID should match parent span ID")
	}

	if childSpan.Context.OperationName != "child-operation" {
		t.Error("Child span should have its own operation name")
	}

	// Verify context chain
	childCorr := GetCorrelationContext(childCtx)
	if childCorr == nil || childCorr != childSpan.Context {
		t.Error("Child context should have child correlation attached")
	}
}

func TestFinishSpan(t *testing.T) {
	client := NewTelemetryClient("test-key")
	ctx := context.Background()

	// Start a span
	spanCtx, span := StartSpan(ctx, "test-operation", client)

	// Simulate some work
	time.Sleep(1 * time.Millisecond)

	// Finish the span
	span.FinishSpan(spanCtx, true, map[string]string{"custom": "property"})

	// Note: In a real test, you'd verify that telemetry was sent
	// For now, we just verify the method doesn't panic
}

func TestFinishSpanWithNilSpan(t *testing.T) {
	ctx := context.Background()
	
	// Should not panic
	var span *SpanContext
	span.FinishSpan(ctx, true, nil)
}

func TestWithSpan(t *testing.T) {
	client := NewTelemetryClient("test-key")
	ctx := context.Background()
	operationName := "test-with-span"

	executed := false
	err := WithSpan(ctx, operationName, client, func(spanCtx context.Context) error {
		executed = true
		
		// Verify correlation context is available in the function
		corrCtx := GetCorrelationContext(spanCtx)
		if corrCtx == nil {
			t.Error("Span context should have correlation context")
		}
		
		if corrCtx.OperationName != operationName {
			t.Errorf("Expected operation name %s, got %s", operationName, corrCtx.OperationName)
		}
		
		return nil
	})

	if !executed {
		t.Error("Function should have been executed")
	}

	if err != nil {
		t.Errorf("WithSpan should not return error for successful function: %v", err)
	}
}

func TestWithSpanError(t *testing.T) {
	client := NewTelemetryClient("test-key")
	ctx := context.Background()
	testError := errors.New("test error")

	err := WithSpan(ctx, "test-operation", client, func(context.Context) error {
		return testError
	})

	if err != testError {
		t.Errorf("WithSpan should return the function's error: %v", err)
	}
}

func TestWithSpanPanic(t *testing.T) {
	client := NewTelemetryClient("test-key")
	ctx := context.Background()

	defer func() {
		if r := recover(); r == nil {
			t.Error("WithSpan should re-throw panic")
		}
	}()

	WithSpan(ctx, "test-operation", client, func(context.Context) error {
		panic("test panic")
	})
}

func TestStartOperation(t *testing.T) {
	client := NewTelemetryClient("test-key")
	ctx := context.Background()
	operationName := "test-operation"

	newCtx, opCtx := StartOperation(ctx, operationName, client)

	if opCtx == nil {
		t.Fatal("StartOperation should return an operation context")
	}

	if opCtx.OperationName != operationName {
		t.Errorf("Expected operation name %s, got %s", operationName, opCtx.OperationName)
	}

	if opCtx.Client != client {
		t.Error("Operation should reference the provided client")
	}

	// Verify correlation context is attached
	corrCtx := GetCorrelationContext(newCtx)
	if corrCtx == nil {
		t.Fatal("New context should have correlation context attached")
	}

	if corrCtx.OperationName != operationName {
		t.Error("Correlation context should have the operation name")
	}
}

func TestFinishOperation(t *testing.T) {
	client := NewTelemetryClient("test-key")
	ctx := context.Background()

	// Start an operation
	opCtx, op := StartOperation(ctx, "test-operation", client)

	// Simulate some work
	time.Sleep(1 * time.Millisecond)

	// Finish the operation
	op.FinishOperation(opCtx, "200", true, "/test", map[string]string{"custom": "property"})

	// Note: In a real test, you'd verify that request telemetry was sent
}

func TestHTTPRequestCorrelationHelper(t *testing.T) {
	client := NewTelemetryClient("test-key")
	helper := NewHTTPRequestCorrelationHelper(client)

	if helper == nil {
		t.Fatal("NewHTTPRequestCorrelationHelper should return a helper")
	}

	if helper.Client != client {
		t.Error("Helper should reference the provided client")
	}
}

func TestStartHTTPOperation(t *testing.T) {
	client := NewTelemetryClient("test-key")
	helper := NewHTTPRequestCorrelationHelper(client)

	req := httptest.NewRequest("GET", "/test", nil)
	operationName := "test-http-operation"

	newCtx, httpOpCtx := helper.StartHTTPOperation(req, operationName)

	if httpOpCtx == nil {
		t.Fatal("StartHTTPOperation should return an HTTP operation context")
	}

	if httpOpCtx.OperationName != operationName {
		t.Errorf("Expected operation name %s, got %s", operationName, httpOpCtx.OperationName)
	}

	if httpOpCtx.Request != req {
		t.Error("HTTP operation should reference the provided request")
	}

	// Verify correlation context is attached
	corrCtx := GetCorrelationContext(newCtx)
	if corrCtx == nil {
		t.Fatal("Context should have correlation context attached")
	}
}

func TestStartHTTPOperationWithExistingCorrelation(t *testing.T) {
	client := NewTelemetryClient("test-key")
	helper := NewHTTPRequestCorrelationHelper(client)

	// Create request with correlation headers
	req := httptest.NewRequest("GET", "/test", nil)
	parentCorr := NewCorrelationContext()
	parentCorr.OperationName = "parent-operation"
	req.Header.Set("traceparent", parentCorr.ToW3CTraceParent())

	newCtx, _ := helper.StartHTTPOperation(req, "child-operation")

	// Should create child correlation context
	corrCtx := GetCorrelationContext(newCtx)
	if corrCtx == nil {
		t.Fatal("Context should have correlation context attached")
	}

	if corrCtx.TraceID != parentCorr.TraceID {
		t.Error("Should inherit trace ID from parent")
	}

	if corrCtx.ParentSpanID != parentCorr.SpanID {
		t.Error("Should set parent span ID")
	}

	if corrCtx.OperationName != "child-operation" {
		t.Error("Should have child operation name")
	}
}

func TestFinishHTTPOperation(t *testing.T) {
	client := NewTelemetryClient("test-key")
	helper := NewHTTPRequestCorrelationHelper(client)

	req := httptest.NewRequest("GET", "/test", nil)
	ctx, httpOpCtx := helper.StartHTTPOperation(req, "test-operation")

	// Simulate some work
	time.Sleep(1 * time.Millisecond)

	// Finish the operation
	httpOpCtx.FinishHTTPOperation(ctx, "200", true)

	// Note: In a real test, you'd verify that request telemetry was sent
}

func TestInjectHeadersForOutgoingRequest(t *testing.T) {
	client := NewTelemetryClient("test-key")
	helper := NewHTTPRequestCorrelationHelper(client)

	req := httptest.NewRequest("GET", "/test", nil)
	ctx, httpOpCtx := helper.StartHTTPOperation(req, "test-operation")

	// Create outgoing request
	outgoingReq := httptest.NewRequest("GET", "http://example.com/api", nil)

	// Inject headers
	httpOpCtx.InjectHeadersForOutgoingRequest(outgoingReq)

	// Verify headers were injected
	traceParent := outgoingReq.Header.Get("traceparent")
	if traceParent == "" {
		t.Error("Should inject traceparent header")
	}

	requestID := outgoingReq.Header.Get("Request-Id")
	if requestID == "" {
		t.Error("Should inject Request-Id header")
	}

	// Extract and verify correlation context
	middleware := NewHTTPMiddleware()
	extractedCorr := middleware.ExtractHeaders(outgoingReq)
	if extractedCorr == nil {
		t.Fatal("Should be able to extract correlation from injected headers")
	}

	// Should be child of the operation context
	operationCorr := GetCorrelationContext(ctx)
	if extractedCorr.TraceID != operationCorr.TraceID {
		t.Error("Extracted correlation should inherit trace ID")
	}
}

func TestCorrelationContextBuilder(t *testing.T) {
	builder := NewCorrelationContextBuilder()

	corrCtx := builder.
		WithOperationName("test-operation").
		WithSampled(true).
		Build()

	if corrCtx.OperationName != "test-operation" {
		t.Errorf("Expected operation name 'test-operation', got %s", corrCtx.OperationName)
	}

	// Check sampled flag (bit 0 of trace flags)
	if corrCtx.TraceFlags&0x01 == 0 {
		t.Error("Sampled flag should be set")
	}
}

func TestCorrelationContextBuilderWithParent(t *testing.T) {
	parent := NewCorrelationContext()
	parent.OperationName = "parent-operation"

	builder := NewChildCorrelationContextBuilder(parent)
	corrCtx := builder.WithOperationName("child-operation").Build()

	if corrCtx.TraceID != parent.TraceID {
		t.Error("Child should inherit parent trace ID")
	}

	if corrCtx.ParentSpanID != parent.SpanID {
		t.Error("Child should have parent span ID set")
	}

	if corrCtx.OperationName != "child-operation" {
		t.Error("Child should have its own operation name")
	}
}

func TestCorrelationContextBuilderWithContext(t *testing.T) {
	builder := NewCorrelationContextBuilder()
	testCtx := builder.WithOperationName("test-operation").BuildWithContext(context.Background())

	corrCtx := GetCorrelationContext(testCtx)
	if corrCtx == nil {
		t.Fatal("Context should have correlation context attached")
	}

	if corrCtx.OperationName != "test-operation" {
		t.Error("Correlation context should have the operation name")
	}
}

func TestWithNewRootSpan(t *testing.T) {
	ctx := WithNewRootSpan(context.Background(), "root-operation")

	corrCtx := GetCorrelationContext(ctx)
	if corrCtx == nil {
		t.Fatal("Context should have correlation context attached")
	}

	if corrCtx.OperationName != "root-operation" {
		t.Error("Should have the specified operation name")
	}

	if corrCtx.ParentSpanID != "" {
		t.Error("Root span should not have parent span ID")
	}
}

func TestWithChildSpan(t *testing.T) {
	// Create parent context
	parentCorr := NewCorrelationContext()
	parentCorr.OperationName = "parent-operation"
	parentCtx := WithCorrelationContext(context.Background(), parentCorr)

	// Create child span
	childCtx := WithChildSpan(parentCtx, "child-operation")

	childCorr := GetCorrelationContext(childCtx)
	if childCorr == nil {
		t.Fatal("Child context should have correlation context attached")
	}

	if childCorr.TraceID != parentCorr.TraceID {
		t.Error("Child should inherit parent trace ID")
	}

	if childCorr.ParentSpanID != parentCorr.SpanID {
		t.Error("Child should have parent span ID set")
	}

	if childCorr.OperationName != "child-operation" {
		t.Error("Child should have its own operation name")
	}
}

func TestWithChildSpanNilParent(t *testing.T) {
	// Create child span without parent
	childCtx := WithChildSpan(context.Background(), "operation")

	childCorr := GetCorrelationContext(childCtx)
	if childCorr == nil {
		t.Fatal("Child context should have correlation context attached")
	}

	if childCorr.OperationName != "operation" {
		t.Error("Should have the specified operation name")
	}

	// Should create new root context since no parent exists
	if childCorr.ParentSpanID != "" {
		t.Error("Should create root context when no parent exists")
	}
}

func TestGetOrCreateSpan(t *testing.T) {
	// Test with existing correlation
	existingCorr := NewCorrelationContext()
	existingCorr.OperationName = "existing-operation"
	existingCtx := WithCorrelationContext(context.Background(), existingCorr)

	_, corrCtx := GetOrCreateSpan(existingCtx, "new-operation")
	if corrCtx != existingCorr {
		t.Error("Should return existing correlation context")
	}

	// Test without existing correlation
	ctx2, corrCtx2 := GetOrCreateSpan(context.Background(), "new-operation")
	if corrCtx2 == nil {
		t.Fatal("Should create new correlation context")
	}

	if corrCtx2.OperationName != "new-operation" {
		t.Error("Should have the specified operation name")
	}

	// Verify context has the correlation attached
	retrievedCorr := GetCorrelationContext(ctx2)
	if retrievedCorr != corrCtx2 {
		t.Error("Context should have the correlation attached")
	}
}

func TestCopyCorrelationToRequest(t *testing.T) {
	corrCtx := NewCorrelationContext()
	corrCtx.OperationName = "test-operation"
	ctx := WithCorrelationContext(context.Background(), corrCtx)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	CopyCorrelationToRequest(ctx, req)

	// Verify headers were set
	traceParent := req.Header.Get("traceparent")
	if traceParent == "" {
		t.Error("Should set traceparent header")
	}

	requestID := req.Header.Get("Request-Id")
	if requestID == "" {
		t.Error("Should set Request-Id header")
	}

	// Verify headers are correct
	expectedTraceParent := corrCtx.ToW3CTraceParent()
	if traceParent != expectedTraceParent {
		t.Errorf("Expected traceparent %s, got %s", expectedTraceParent, traceParent)
	}
}

func TestCopyCorrelationToRequestNilContext(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com", nil)
	
	// Should not panic with no correlation context
	CopyCorrelationToRequest(context.Background(), req)

	// Headers should not be set
	if req.Header.Get("traceparent") != "" {
		t.Error("Should not set headers when no correlation context")
	}
}

func TestTrackDependencyWithSpan(t *testing.T) {
	client := NewTelemetryClient("test-key")
	ctx := context.Background()

	executed := false
	err := TrackDependencyWithSpan(ctx, client, "test-dependency", "HTTP", "example.com", true, func(spanCtx context.Context) error {
		executed = true
		
		// Verify correlation context is available
		corrCtx := GetCorrelationContext(spanCtx)
		if corrCtx == nil {
			t.Error("Span context should have correlation context")
		}
		
		return nil
	})

	if !executed {
		t.Error("Function should have been executed")
	}

	if err != nil {
		t.Errorf("Should not return error for successful function: %v", err)
	}
}

func TestTrackDependencyWithSpanError(t *testing.T) {
	client := NewTelemetryClient("test-key")
	ctx := context.Background()
	testError := errors.New("dependency error")

	err := TrackDependencyWithSpan(ctx, client, "test-dependency", "HTTP", "example.com", true, func(context.Context) error {
		return testError
	})

	if err != testError {
		t.Errorf("Should return the function's error: %v", err)
	}
}

func TestTrackHTTPDependency(t *testing.T) {
	client := NewTelemetryClient("test-key")
	ctx := context.Background()

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify correlation headers were injected
		if r.Header.Get("traceparent") == "" {
			t.Error("Request should have traceparent header")
		}
		
		if r.Header.Get("Request-Id") == "" {
			t.Error("Request should have Request-Id header")
		}
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	httpClient := server.Client()

	resp, err := TrackHTTPDependency(ctx, client, req, httpClient, "test-target")

	if err != nil {
		t.Errorf("HTTP request should succeed: %v", err)
	}

	if resp == nil {
		t.Fatal("Response should not be nil")
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestTrackHTTPDependencyError(t *testing.T) {
	client := NewTelemetryClient("test-key")
	ctx := context.Background()

	// Create request to non-existent server
	req, err := http.NewRequest("GET", "http://localhost:0", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	httpClient := &http.Client{Timeout: 1 * time.Millisecond}

	resp, err := TrackHTTPDependency(ctx, client, req, httpClient, "test-target")

	// Should handle errors gracefully
	if err == nil {
		t.Error("Expected error for invalid request")
	}

	if resp != nil {
		t.Error("Response should be nil on error")
	}

	// The function should still complete and track the failed dependency
}