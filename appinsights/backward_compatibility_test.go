package appinsights

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

// TestBackwardCompatibilityWithLegacySystems tests that the SDK can
// interoperate with legacy systems that only support Request-Id headers
func TestBackwardCompatibilityWithLegacySystems(t *testing.T) {
	middleware := NewHTTPMiddleware()
	
	// Legacy system that only understands Request-Id headers
	legacySystem := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Legacy system only looks for Request-Id header
		requestID := r.Header.Get("Request-Id")
		if requestID == "" {
			t.Error("Legacy system expects Request-Id header")
			return
		}
		
		// Legacy system echoes back the Request-Id
		w.Header().Set("Request-Id", requestID)
		w.WriteHeader(http.StatusOK)
	}))
	defer legacySystem.Close()
	
	// Modern client creates W3C correlation context
	corrCtx := NewCorrelationContext()
	corrCtx.OperationName = "CallLegacySystem"
	
	// Make request to legacy system
	req, _ := http.NewRequest("GET", legacySystem.URL, nil)
	middleware.InjectHeaders(req, corrCtx)
	
	// Verify both headers are set for compatibility
	w3cHeader := req.Header.Get("traceparent")
	requestIDHeader := req.Header.Get("Request-Id")
	
	if w3cHeader == "" {
		t.Error("W3C traceparent header should be set")
	}
	if requestIDHeader == "" {
		t.Error("Request-Id header should be set for legacy compatibility")
	}
	
	// Make the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request to legacy system failed: %v", err)
	}
	defer resp.Body.Close()
	
	// Verify legacy system echoed back the Request-Id
	echoedRequestID := resp.Header.Get("Request-Id")
	if echoedRequestID != requestIDHeader {
		t.Errorf("Expected echoed Request-Id %s, got %s", requestIDHeader, echoedRequestID)
	}
}

// TestMigrationFromRequestIdToW3C tests scenarios where systems are
// migrating from Request-Id to W3C Trace Context
func TestMigrationFromRequestIdToW3C(t *testing.T) {
	middleware := NewHTTPMiddleware()
	
	// System A: Still using Request-Id only
	var systemBURL string
	systemA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract using Request-Id only
		requestID := r.Header.Get("Request-Id")
		if requestID == "" {
			t.Error("System A expects Request-Id header")
			return
		}
		
		// Parse and create correlation context
		corrCtx, err := ParseRequestID(requestID)
		if err != nil {
			t.Errorf("Failed to parse Request-Id: %v", err)
			return
		}
		
		// System A makes call to System B, adding both headers for compatibility
		systemBReq, _ := http.NewRequest("GET", systemBURL, nil)
		systemBReq.Header.Set("Request-Id", corrCtx.ToRequestID())
		systemBReq.Header.Set("traceparent", corrCtx.ToW3CTraceParent())
		
		systemBResp, err := http.DefaultClient.Do(systemBReq)
		if err != nil {
			t.Errorf("System A failed to call System B: %v", err)
			return
		}
		defer systemBResp.Body.Close()
		
		w.WriteHeader(http.StatusOK)
	}))
	defer systemA.Close()
	
	// System B: Modern system that prefers W3C but supports Request-Id
	systemB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract correlation using middleware (prefers W3C)
		corrCtx := middleware.ExtractHeaders(r)
		if corrCtx == nil {
			t.Error("System B should extract correlation context")
			return
		}
		
		// Verify W3C header was used preferentially
		w3cHeader := r.Header.Get("traceparent")
		requestIDHeader := r.Header.Get("Request-Id")
		
		if w3cHeader == "" {
			t.Error("System B should receive W3C header")
		}
		if requestIDHeader == "" {
			t.Error("System B should receive Request-Id header for compatibility")
		}
		
		w.WriteHeader(http.StatusOK)
	}))
	defer systemB.Close()
	systemBURL = systemB.URL
	
	// Initial request using only Request-Id (legacy)
	originalCorr := NewCorrelationContext()
	req, _ := http.NewRequest("GET", systemA.URL, nil)
	req.Header.Set("Request-Id", originalCorr.ToRequestID())
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
}

// TestLegacyTelemetryConstructorCompatibility ensures that existing
// telemetry constructors continue to work without breaking changes
func TestLegacyTelemetryConstructorCompatibility(t *testing.T) {
	// Test that all existing constructors still work
	
	// EventTelemetry - should work without context
	event := NewEventTelemetry("LegacyEvent")
	if event == nil {
		t.Error("NewEventTelemetry should work without context")
	}
	
	// TraceTelemetry - should work without context
	trace := NewTraceTelemetry("Legacy message", contracts.Information)
	if trace == nil {
		t.Error("NewTraceTelemetry should work without context")
	}
	
	// RequestTelemetry - should generate UUID like before
	request := NewRequestTelemetry("GET", "/legacy", time.Second, "200")
	if request == nil {
		t.Error("NewRequestTelemetry should work without context")
	}
	if request.Id == "" {
		t.Error("Legacy RequestTelemetry should still generate ID")
	}
	
	// RemoteDependencyTelemetry - should work without ID (legacy behavior)
	dependency := NewRemoteDependencyTelemetry("LegacyDep", "HTTP", "example.com", true)
	if dependency == nil {
		t.Error("NewRemoteDependencyTelemetry should work without context")
	}
	// Note: Legacy constructor should not set ID (existing behavior)
	if dependency.Id != "" {
		t.Error("Legacy RemoteDependencyTelemetry should not have ID")
	}
	
	// AvailabilityTelemetry - should work without ID (legacy behavior)
	availability := NewAvailabilityTelemetry("LegacyAvailability", time.Second, true)
	if availability == nil {
		t.Error("NewAvailabilityTelemetry should work without context")
	}
	// Note: Legacy constructor should not set ID (existing behavior)
	if availability.Id != "" {
		t.Error("Legacy AvailabilityTelemetry should not have ID")
	}
}

// TestContextAwareTelemetryConstructors tests the new context-aware constructors
func TestContextAwareTelemetryConstructors(t *testing.T) {
	// Create correlation context
	corrCtx := NewCorrelationContext()
	corrCtx.OperationName = "ContextAwareTest"
	ctx := WithCorrelationContext(context.Background(), corrCtx)
	
	// RequestTelemetry with context should use correlation span ID
	request := NewRequestTelemetryWithContext(ctx, "GET", "/test", time.Second, "200")
	if request.Id != corrCtx.SpanID {
		t.Errorf("Request telemetry should use correlation span ID %s, got %s", corrCtx.SpanID, request.Id)
	}
	
	// RemoteDependencyTelemetry with context should use correlation span ID
	dependency := NewRemoteDependencyTelemetryWithContext(ctx, "TestDep", "HTTP", "example.com", true)
	if dependency.Id != corrCtx.SpanID {
		t.Errorf("Dependency telemetry should use correlation span ID %s, got %s", corrCtx.SpanID, dependency.Id)
	}
	
	// AvailabilityTelemetry with context should use correlation span ID
	availability := NewAvailabilityTelemetryWithContext(ctx, "TestAvailability", time.Second, true)
	if availability.Id != corrCtx.SpanID {
		t.Errorf("Availability telemetry should use correlation span ID %s, got %s", corrCtx.SpanID, availability.Id)
	}
	
	// Test with nil context - should generate UUIDs
	requestNil := NewRequestTelemetryWithContext(nil, "GET", "/test", time.Second, "200")
	if requestNil.Id == "" {
		t.Error("Request telemetry with nil context should generate ID")
	}
	if requestNil.Id == corrCtx.SpanID {
		t.Error("Request telemetry with nil context should not use correlation span ID")
	}
}

// TestApplicationInsightsRequestIdFormat tests compatibility with
// Application Insights legacy Request-Id format
func TestApplicationInsightsRequestIdFormat(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
		expectOK  bool
	}{
		{
			name:      "Standard AI format",
			requestID: "|abcdef0123456789abcdef0123456789.abcdef0123456789.",
			expectOK:  true,
		},
		{
			name:      "AI format without pipes",
			requestID: "abcdef0123456789abcdef0123456789.abcdef0123456789",
			expectOK:  true,
		},
		{
			name:      "Short legacy format",
			requestID: "|12345.67890.",
			expectOK:  true, // Should create new context
		},
		{
			name:      "Hierarchical format",
			requestID: "|abcdef0123456789abcdef0123456789.abcdef0123456789.1.",
			expectOK:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corrCtx, err := ParseRequestID(tt.requestID)
			
			if !tt.expectOK && err == nil {
				t.Errorf("Expected error for %s but got none", tt.requestID)
				return
			}
			
			if tt.expectOK && err != nil {
				t.Errorf("Unexpected error for %s: %v", tt.requestID, err)
				return
			}
			
			if corrCtx != nil {
				// Should be able to round-trip
				newRequestID := corrCtx.ToRequestID()
				newCorrCtx, err := ParseRequestID(newRequestID)
				
				if err != nil {
					t.Errorf("Failed to parse generated Request-Id: %v", err)
				}
				
				if newCorrCtx.TraceID != corrCtx.TraceID {
					t.Error("Round-trip should preserve trace ID")
				}
			}
		})
	}
}

// TestRequestIdToW3CInteroperability tests interoperability between
// Request-Id and W3C Trace Context formats
func TestRequestIdToW3CInteroperability(t *testing.T) {
	// Start with Request-Id
	originalRequestID := "|abcdef0123456789abcdef0123456789.abcdef0123456789."
	
	// Parse to correlation context
	corrCtx, err := ParseRequestID(originalRequestID)
	if err != nil {
		t.Fatalf("Failed to parse Request-Id: %v", err)
	}
	
	// Convert to W3C format
	w3cHeader := corrCtx.ToW3CTraceParent()
	
	// Parse W3C format back
	w3cCorrCtx, err := ParseW3CTraceParent(w3cHeader)
	if err != nil {
		t.Fatalf("Failed to parse W3C header: %v", err)
	}
	
	// Should maintain same trace and span IDs
	if w3cCorrCtx.TraceID != corrCtx.TraceID {
		t.Error("W3C conversion should preserve trace ID")
	}
	if w3cCorrCtx.SpanID != corrCtx.SpanID {
		t.Error("W3C conversion should preserve span ID")
	}
	
	// Convert back to Request-Id
	newRequestID := w3cCorrCtx.ToRequestID()
	
	// Should be compatible
	newCorrCtx, err := ParseRequestID(newRequestID)
	if err != nil {
		t.Fatalf("Failed to parse converted Request-Id: %v", err)
	}
	
	if newCorrCtx.TraceID != corrCtx.TraceID {
		t.Error("Full round-trip should preserve trace ID")
	}
	if newCorrCtx.SpanID != corrCtx.SpanID {
		t.Error("Full round-trip should preserve span ID")
	}
}

// TestLegacyClientMethodCompatibility ensures that all existing client
// methods continue to work without breaking changes
func TestLegacyClientMethodCompatibility(t *testing.T) {
	client := NewTelemetryClient("test-key")
	
	// Test all legacy tracking methods (should not panic)
	client.Track(NewEventTelemetry("LegacyEvent"))
	client.TrackEvent("LegacyEvent")
	client.TrackTrace("Legacy trace", contracts.Information)
	client.TrackRequest("GET", "/legacy", time.Second, "200")
	client.TrackRemoteDependency("LegacyDep", "HTTP", "example.com", true)
	client.TrackAvailability("LegacyAvailability", time.Second, true)
	client.TrackMetric("legacy.metric", 1.0)
	client.TrackException(NewTraceTelemetry("exception", contracts.Error))
	
	// Client should remain functional
	if !client.IsEnabled() {
		t.Error("Client should remain enabled after legacy method calls")
	}
}

// TestEnvelopeBackwardCompatibility tests that telemetry envelopes
// maintain backward compatibility with existing formats
func TestEnvelopeBackwardCompatibility(t *testing.T) {
	telCtx := NewTelemetryContext("test-key")
	
	// Test envelope without correlation context (legacy scenario)
	event := NewEventTelemetry("LegacyEvent")
	envelope := telCtx.envelopWithContext(context.Background(), event)
	
	// Should have generated operation ID (UUID format)
	operationID := envelope.Tags[contracts.OperationId]
	if operationID == "" {
		t.Error("Envelope should have operation ID even without correlation context")
	}
	
	// Should be UUID format (36 characters with dashes)
	if len(operationID) != 36 || !strings.Contains(operationID, "-") {
		t.Errorf("Operation ID should be UUID format, got: %s", operationID)
	}
	
	// Should not have parent ID
	if _, exists := envelope.Tags[contracts.OperationParentId]; exists {
		t.Error("Envelope should not have parent ID without correlation context")
	}
	
	// Test envelope with correlation context
	corrCtx := NewCorrelationContext()
	corrCtx.OperationName = "TestOperation"
	ctx := WithCorrelationContext(context.Background(), corrCtx)
	
	eventWithCorr := NewEventTelemetry("CorrelatedEvent")
	envelopeWithCorr := telCtx.envelopWithContext(ctx, eventWithCorr)
	
	// Should use correlation operation ID
	if envelopeWithCorr.Tags[contracts.OperationId] != corrCtx.GetOperationID() {
		t.Error("Envelope should use correlation operation ID")
	}
	
	// Should have operation name
	if envelopeWithCorr.Tags[contracts.OperationName] != corrCtx.OperationName {
		t.Error("Envelope should have operation name from correlation context")
	}
}

// TestDataFormatCompatibility tests that telemetry data formats
// remain compatible with Application Insights backend
func TestDataFormatCompatibility(t *testing.T) {
	// Test that telemetry data structures maintain expected formats
	
	// RequestData should have proper ID format
	request := NewRequestTelemetry("GET", "/test", time.Second, "200")
	requestData := request.TelemetryData().(*contracts.RequestData)
	
	if requestData.Id == "" {
		t.Error("RequestData should have ID")
	}
	
	// RemoteDependencyData format
	dependency := NewRemoteDependencyTelemetry("TestDep", "HTTP", "example.com", true)
	dependencyData := dependency.TelemetryData().(*contracts.RemoteDependencyData)
	
	if dependencyData.Name != "TestDep" {
		t.Error("DependencyData should maintain name")
	}
	if dependencyData.Type != "HTTP" {
		t.Error("DependencyData should maintain type")
	}
	
	// AvailabilityData format
	availability := NewAvailabilityTelemetry("TestAvailability", time.Second, true)
	availabilityData := availability.TelemetryData().(*contracts.AvailabilityData)
	
	if availabilityData.Name != "TestAvailability" {
		t.Error("AvailabilityData should maintain name")
	}
	if !availabilityData.Success {
		t.Error("AvailabilityData should maintain success flag")
	}
}

// TestHttpMiddlewareBackwardCompatibility tests that HTTP middleware
// doesn't break existing HTTP handling patterns
func TestHttpMiddlewareBackwardCompatibility(t *testing.T) {
	middleware := NewHTTPMiddleware()
	
	// Test with existing HTTP handler pattern
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	// Wrap with middleware
	wrappedHandler := middleware.Middleware(handler)
	
	// Make request without any correlation headers
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	
	wrappedHandler.ServeHTTP(rr, req)
	
	// Handler should have been called
	if !handlerCalled {
		t.Error("Original handler should have been called")
	}
	
	// Response should be as expected
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	
	if rr.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got %s", rr.Body.String())
	}
	
	// Middleware should add correlation headers to response
	if rr.Header().Get("Request-Id") == "" {
		t.Error("Middleware should add Request-Id header to response")
	}
}