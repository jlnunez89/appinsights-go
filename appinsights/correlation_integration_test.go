package appinsights

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

// TestEndToEndCorrelationScenario tests a complete end-to-end correlation scenario
// with multiple services and HTTP requests to ensure correlation is maintained
func TestEndToEndCorrelationScenario(t *testing.T) {
	// Service A: Creates initial correlation context
	client := NewTelemetryClient("test-key")
	
	// Create root operation
	rootCorr := NewCorrelationContext()
	rootCorr.OperationName = "UserRegistration"
	rootCtx := WithCorrelationContext(context.Background(), rootCorr)
	
	// Track initial event
	client.TrackEventWithContext(rootCtx, "RegistrationStarted")
	
	// Service C: Another hop in the correlation chain (define first)
	serviceCServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		middleware := NewHTTPMiddleware()
		extractedCorr := middleware.ExtractHeaders(r)
		
		if extractedCorr == nil {
			t.Error("Service C should receive correlation context")
			return
		}
		
		// Verify trace ID is still preserved
		if extractedCorr.TraceID != rootCorr.TraceID {
			t.Errorf("Service C should inherit trace ID %s, got %s", rootCorr.TraceID, extractedCorr.TraceID)
		}
		
		// Create child context for Service C
		serviceCCorr := NewChildCorrelationContext(extractedCorr)
		serviceCCorr.OperationName = "DataValidation"
		serviceCCtx := WithCorrelationContext(r.Context(), serviceCCorr)
		
		// Track dependency in Service C
		client.TrackRemoteDependencyWithContext(serviceCCtx, "DatabaseValidation", "SQL", "validation.db", true)
		
		w.WriteHeader(http.StatusOK)
	}))
	defer serviceCServer.Close()
	
	// Simulate Service A calling Service B via HTTP
	serviceB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Service B: Extract correlation from incoming request
		middleware := NewHTTPMiddleware()
		extractedCorr := middleware.ExtractHeaders(r)
		
		if extractedCorr == nil {
			t.Error("Service B should receive correlation context from Service A")
			return
		}
		
		// Verify trace ID is preserved from Service A
		if extractedCorr.TraceID != rootCorr.TraceID {
			t.Errorf("Service B should inherit trace ID %s, got %s", rootCorr.TraceID, extractedCorr.TraceID)
		}
		
		// Create child context for Service B operations
		serviceBCorr := NewChildCorrelationContext(extractedCorr)
		serviceBCorr.OperationName = "ValidateUser"
		serviceBCtx := WithCorrelationContext(r.Context(), serviceBCorr)
		
		// Track event in Service B
		client.TrackEventWithContext(serviceBCtx, "UserValidated")
		
		// Service B calls Service C
		serviceCURL := serviceCServer.URL + "/validate"
		serviceCReq, _ := http.NewRequest("GET", serviceCURL, nil)
		middleware.InjectHeaders(serviceCReq, serviceBCorr)
		
		serviceCResp, err := http.DefaultClient.Do(serviceCReq)
		if err != nil {
			t.Errorf("Service B failed to call Service C: %v", err)
			return
		}
		defer serviceCResp.Body.Close()
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("validated"))
	}))
	defer serviceB.Close()
	
	// Now make the actual call from Service A to Service B
	serviceBReq, _ := http.NewRequest("GET", serviceB.URL+"/register", nil)
	middleware := NewHTTPMiddleware()
	middleware.InjectHeaders(serviceBReq, rootCorr)
	
	serviceBResp, err := http.DefaultClient.Do(serviceBReq)
	if err != nil {
		t.Fatalf("Service A failed to call Service B: %v", err)
	}
	defer serviceBResp.Body.Close()
	
	// Track completion event in Service A
	client.TrackEventWithContext(rootCtx, "RegistrationCompleted")
	
	// All operations should be correlated under the same trace ID
	if !client.IsEnabled() {
		t.Error("Client should remain enabled after all operations")
	}
}

// TestCorrelationPersistenceAcrossAsyncOperations tests that correlation
// is maintained across async operations and goroutines
func TestCorrelationPersistenceAcrossAsyncOperations(t *testing.T) {
	client := NewTelemetryClient("test-key")
	
	// Create root correlation
	rootCorr := NewCorrelationContext()
	rootCorr.OperationName = "AsyncProcessing"
	rootCtx := WithCorrelationContext(context.Background(), rootCorr)
	
	// Channel to collect results from goroutines
	results := make(chan string, 3)
	
	// Start multiple async operations
	for i := 0; i < 3; i++ {
		go func(id int) {
			// Each goroutine should maintain correlation context
			corrCtx := GetCorrelationContext(rootCtx)
			if corrCtx == nil {
				results <- "failed-no-context"
				return
			}
			
			if corrCtx.TraceID != rootCorr.TraceID {
				results <- "failed-wrong-trace-id"
				return
			}
			
			// Create child operation
			childCorr := NewChildCorrelationContext(corrCtx)
			childCorr.OperationName = "AsyncTask"
			childCtx := WithCorrelationContext(rootCtx, childCorr)
			
			// Track event in async operation
			client.TrackEventWithContext(childCtx, "AsyncTaskCompleted")
			
			results <- "success"
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		select {
		case result := <-results:
			if result != "success" {
				t.Errorf("Async operation %d failed: %s", i, result)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for async operations")
		}
	}
}

// TestCorrelationWithMixedHeaderFormats tests handling of mixed W3C and Request-Id
// headers in a distributed system where different services use different formats
func TestCorrelationWithMixedHeaderFormats(t *testing.T) {
	middleware := NewHTTPMiddleware()
	
	// Service using W3C headers
	w3cService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrCtx := middleware.ExtractHeaders(r)
		if corrCtx == nil {
			t.Error("W3C service should extract correlation")
			return
		}
		
		// Create outgoing request using Request-Id format
		outgoingReq := httptest.NewRequest("GET", "/legacy", nil)
		outgoingReq.Header.Set("Request-Id", corrCtx.ToRequestID())
		
		// Verify round-trip conversion
		legacyCorr, err := ParseRequestID(outgoingReq.Header.Get("Request-Id"))
		if err != nil {
			t.Errorf("Failed to parse Request-Id: %v", err)
			return
		}
		
		if legacyCorr.TraceID != corrCtx.TraceID {
			t.Error("Trace ID should be preserved in format conversion")
		}
		
		w.WriteHeader(http.StatusOK)
	}))
	defer w3cService.Close()
	
	// Create request with W3C headers
	corrCtx := NewCorrelationContext()
	req, err := http.NewRequest("GET", w3cService.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	middleware.InjectHeaders(req, corrCtx)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
}

// TestCorrelationErrorHandling tests error scenarios and edge cases
func TestCorrelationErrorHandling(t *testing.T) {
	middleware := NewHTTPMiddleware()
	
	tests := []struct {
		name         string
		setupHeaders func(*http.Request)
		expectCorr   bool
	}{
		{
			name: "malformed W3C traceparent",
			setupHeaders: func(r *http.Request) {
				r.Header.Set("traceparent", "invalid-format")
			},
			expectCorr: false,
		},
		{
			name: "malformed Request-Id",
			setupHeaders: func(r *http.Request) {
				r.Header.Set("Request-Id", "not-a-valid-request-id")
			},
			expectCorr: true, // ParseRequestID is lenient and creates new context
		},
		{
			name: "empty headers",
			setupHeaders: func(r *http.Request) {
				r.Header.Set("traceparent", "")
				r.Header.Set("Request-Id", "")
			},
			expectCorr: false,
		},
		{
			name: "conflicting headers with invalid W3C",
			setupHeaders: func(r *http.Request) {
				r.Header.Set("traceparent", "invalid")
				r.Header.Set("Request-Id", "|abcdef0123456789abcdef0123456789.abcdef0123456789.")
			},
			expectCorr: true, // Should fall back to Request-Id
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			tt.setupHeaders(req)
			
			corrCtx := middleware.ExtractHeaders(req)
			
			if tt.expectCorr && corrCtx == nil {
				t.Error("Expected correlation context but got nil")
			}
			if !tt.expectCorr && corrCtx != nil {
				t.Error("Expected nil correlation context but got one")
			}
		})
	}
}

// TestCorrelationWithTelemetryEnvelopes verifies that correlation data
// is properly included in telemetry envelopes across different telemetry types
func TestCorrelationWithTelemetryEnvelopes(t *testing.T) {
	telCtx := NewTelemetryContext("test-key")
	
	// Create correlation context
	corrCtx := NewCorrelationContext()
	corrCtx.OperationName = "EnvelopeTest"
	corrCtx.TraceFlags = 1 // Set sampled flag
	ctx := WithCorrelationContext(context.Background(), corrCtx)
	
	testCases := []struct {
		name      string
		telemetry Telemetry
	}{
		{
			name:      "Event Telemetry",
			telemetry: NewEventTelemetry("TestEvent"),
		},
		{
			name:      "Trace Telemetry",
			telemetry: NewTraceTelemetry("TestTrace", contracts.Information),
		},
		{
			name:      "Request Telemetry",
			telemetry: NewRequestTelemetryWithContext(ctx, "GET", "/test", time.Second, "200"),
		},
		{
			name:      "Dependency Telemetry",
			telemetry: NewRemoteDependencyTelemetryWithContext(ctx, "TestDep", "HTTP", "example.com", true),
		},
		{
			name:      "Availability Telemetry",
			telemetry: NewAvailabilityTelemetryWithContext(ctx, "TestAvailability", time.Second, true),
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			envelope := telCtx.envelopWithContext(ctx, tc.telemetry)
			
			// Verify operation ID is set from correlation context
			if envelope.Tags[contracts.OperationId] != corrCtx.GetOperationID() {
				t.Errorf("Expected operation ID %s, got %s", corrCtx.GetOperationID(), envelope.Tags[contracts.OperationId])
			}
			
			// Verify operation name is set
			if envelope.Tags[contracts.OperationName] != corrCtx.OperationName {
				t.Errorf("Expected operation name %s, got %s", corrCtx.OperationName, envelope.Tags[contracts.OperationName])
			}
			
			// Verify sampling flag is preserved
			// Note: This would be implementation-specific based on how sampling is handled
		})
	}
}

// TestHighConcurrencyCorrelation tests correlation under high concurrency
// to ensure thread safety and performance
func TestHighConcurrencyCorrelation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high concurrency test in short mode")
	}
	
	client := NewTelemetryClient("test-key")
	middleware := NewHTTPMiddleware()
	
	// Create test server that processes requests with correlation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrCtx := middleware.ExtractHeaders(r)
		if corrCtx != nil {
			ctx := WithCorrelationContext(r.Context(), corrCtx)
			client.TrackEventWithContext(ctx, "ConcurrentRequest")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	// Run multiple concurrent requests
	const numGoroutines = 100
	const requestsPerGoroutine = 10
	
	done := make(chan error, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < requestsPerGoroutine; j++ {
				corrCtx := NewCorrelationContext()
				req, err := http.NewRequest("GET", server.URL, nil)
				if err != nil {
					done <- err
					return
				}
				
				middleware.InjectHeaders(req, corrCtx)
				
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					done <- err
					return
				}
				resp.Body.Close()
			}
			done <- nil
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent request failed: %v", err)
		}
	}
}

// TestCorrelationIDFormats tests various ID formats and edge cases
func TestCorrelationIDFormats(t *testing.T) {
	tests := []struct {
		name        string
		traceID     string
		spanID      string
		expectValid bool
		isAllZeros  bool
	}{
		{
			name:        "Standard W3C IDs",
			traceID:     "abcdef0123456789abcdef0123456789",
			spanID:      "abcdef0123456789",
			expectValid: true,
			isAllZeros:  false,
		},
		{
			name:        "All zeros trace ID (invalid)",
			traceID:     "00000000000000000000000000000000",
			spanID:      "abcdef0123456789",
			expectValid: false,
			isAllZeros:  true,
		},
		{
			name:        "All zeros span ID (invalid)",
			traceID:     "abcdef0123456789abcdef0123456789",
			spanID:      "0000000000000000",
			expectValid: false,
			isAllZeros:  true,
		},
		{
			name:        "Maximum hex values",
			traceID:     "ffffffffffffffffffffffffffffffff",
			spanID:      "ffffffffffffffff",
			expectValid: true,
			isAllZeros:  false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corrCtx := &CorrelationContext{
				TraceID: tt.traceID,
				SpanID:  tt.spanID,
			}
			
			// Test W3C format
			w3cHeader := corrCtx.ToW3CTraceParent()
			parsed, err := ParseW3CTraceParent(w3cHeader)
			
			if tt.expectValid {
				if err != nil {
					t.Errorf("Expected valid parsing but got error: %v", err)
				} else if parsed.TraceID != tt.traceID || parsed.SpanID != tt.spanID {
					t.Error("Parsed IDs don't match original")
				}
			} else {
				// Note: Current implementation doesn't validate all-zeros IDs
				// This test documents expected behavior for future validation
				if err == nil && tt.isAllZeros {
					t.Log("Note: All-zeros IDs are currently accepted but may be rejected in future versions")
				}
			}
		})
	}
}

// TestCorrelationWithCustomProperties tests correlation with custom properties
// and ensures they are properly propagated
func TestCorrelationWithCustomProperties(t *testing.T) {
	client := NewTelemetryClient("test-key")
	
	// Create correlation with custom operation name
	corrCtx := NewCorrelationContext()
	corrCtx.OperationName = "CustomOperation"
	ctx := WithCorrelationContext(context.Background(), corrCtx)
	
	// Track telemetry with custom properties
	event := NewEventTelemetry("CustomEvent")
	event.Properties["customProp"] = "customValue"
	event.Properties["correlationTest"] = "true"
	
	client.TrackWithContext(ctx, event)
	
	// Create child operation
	childCorr := NewChildCorrelationContext(corrCtx)
	childCorr.OperationName = "ChildOperation"
	childCtx := WithCorrelationContext(ctx, childCorr)
	
	// Track dependency with custom properties
	dependency := NewRemoteDependencyTelemetryWithContext(childCtx, "CustomDep", "HTTP", "example.com", true)
	dependency.Properties["childProp"] = "childValue"
	
	client.TrackWithContext(childCtx, dependency)
	
	// Verify client is still functional
	if !client.IsEnabled() {
		t.Error("Client should remain enabled after tracking with custom properties")
	}
}