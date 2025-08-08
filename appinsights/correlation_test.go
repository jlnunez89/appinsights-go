package appinsights

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

func TestNewCorrelationContext(t *testing.T) {
	corrCtx := NewCorrelationContext()

	if corrCtx == nil {
		t.Fatal("NewCorrelationContext returned nil")
	}

	// Validate trace ID format (32 hex characters)
	if len(corrCtx.TraceID) != 32 {
		t.Errorf("Expected trace ID length 32, got %d", len(corrCtx.TraceID))
	}
	if !isValidHex(corrCtx.TraceID) {
		t.Errorf("Trace ID is not valid hex: %s", corrCtx.TraceID)
	}

	// Validate span ID format (16 hex characters)
	if len(corrCtx.SpanID) != 16 {
		t.Errorf("Expected span ID length 16, got %d", len(corrCtx.SpanID))
	}
	if !isValidHex(corrCtx.SpanID) {
		t.Errorf("Span ID is not valid hex: %s", corrCtx.SpanID)
	}

	// Parent span ID should be empty for root context
	if corrCtx.ParentSpanID != "" {
		t.Errorf("Expected empty parent span ID for root context, got %s", corrCtx.ParentSpanID)
	}

	// Default trace flags should be 0
	if corrCtx.TraceFlags != 0 {
		t.Errorf("Expected trace flags 0, got %d", corrCtx.TraceFlags)
	}
}

func TestNewChildCorrelationContext(t *testing.T) {
	parent := NewCorrelationContext()
	parent.OperationName = "test-operation"
	parent.TraceFlags = 1

	child := NewChildCorrelationContext(parent)

	if child == nil {
		t.Fatal("NewChildCorrelationContext returned nil")
	}

	// Child should inherit trace ID
	if child.TraceID != parent.TraceID {
		t.Errorf("Child trace ID %s does not match parent %s", child.TraceID, parent.TraceID)
	}

	// Child should have new span ID
	if child.SpanID == parent.SpanID {
		t.Error("Child span ID should be different from parent")
	}
	if len(child.SpanID) != 16 {
		t.Errorf("Expected child span ID length 16, got %d", len(child.SpanID))
	}

	// Child should have parent's span ID as parent span ID
	if child.ParentSpanID != parent.SpanID {
		t.Errorf("Child parent span ID %s does not match parent span ID %s", child.ParentSpanID, parent.SpanID)
	}

	// Child should inherit trace flags
	if child.TraceFlags != parent.TraceFlags {
		t.Errorf("Child trace flags %d does not match parent %d", child.TraceFlags, parent.TraceFlags)
	}

	// Child should inherit operation name
	if child.OperationName != parent.OperationName {
		t.Errorf("Child operation name %s does not match parent %s", child.OperationName, parent.OperationName)
	}
}

func TestNewChildCorrelationContextWithNilParent(t *testing.T) {
	child := NewChildCorrelationContext(nil)

	if child == nil {
		t.Fatal("NewChildCorrelationContext with nil parent returned nil")
	}

	// Should behave like NewCorrelationContext
	if len(child.TraceID) != 32 {
		t.Errorf("Expected trace ID length 32, got %d", len(child.TraceID))
	}
	if len(child.SpanID) != 16 {
		t.Errorf("Expected span ID length 16, got %d", len(child.SpanID))
	}
	if child.ParentSpanID != "" {
		t.Errorf("Expected empty parent span ID, got %s", child.ParentSpanID)
	}
}

func TestContextIntegration(t *testing.T) {
	ctx := context.Background()
	corrCtx := NewCorrelationContext()

	// Test WithCorrelationContext
	newCtx := WithCorrelationContext(ctx, corrCtx)
	if newCtx == ctx {
		t.Error("WithCorrelationContext should return a new context")
	}

	// Test GetCorrelationContext
	retrieved := GetCorrelationContext(newCtx)
	if retrieved == nil {
		t.Fatal("GetCorrelationContext returned nil")
	}
	if retrieved != corrCtx {
		t.Error("Retrieved correlation context is not the same as stored")
	}

	// Test GetCorrelationContext with context without correlation
	empty := GetCorrelationContext(context.Background())
	if empty != nil {
		t.Errorf("Expected nil correlation context, got %v", empty)
	}
}

func TestGetOrCreateCorrelationContext(t *testing.T) {
	// Test with existing context
	ctx := context.Background()
	existing := NewCorrelationContext()
	ctxWithCorr := WithCorrelationContext(ctx, existing)

	retrieved := GetOrCreateCorrelationContext(ctxWithCorr)
	if retrieved != existing {
		t.Error("GetOrCreateCorrelationContext should return existing context")
	}

	// Test with empty context
	created := GetOrCreateCorrelationContext(context.Background())
	if created == nil {
		t.Fatal("GetOrCreateCorrelationContext returned nil")
	}
	if len(created.TraceID) != 32 {
		t.Errorf("Created trace ID length should be 32, got %d", len(created.TraceID))
	}
}

func TestToW3CTraceParent(t *testing.T) {
	corrCtx := &CorrelationContext{
		TraceID:    "abcdef0123456789abcdef0123456789",
		SpanID:     "abcdef0123456789",
		TraceFlags: 1,
	}

	traceParent := corrCtx.ToW3CTraceParent()
	expected := "00-abcdef0123456789abcdef0123456789-abcdef0123456789-01"

	if traceParent != expected {
		t.Errorf("Expected trace parent %s, got %s", expected, traceParent)
	}
}

func TestParseW3CTraceParent(t *testing.T) {
	tests := []struct {
		name        string
		traceParent string
		expectError bool
		expected    *CorrelationContext
	}{
		{
			name:        "valid traceparent",
			traceParent: "00-abcdef0123456789abcdef0123456789-abcdef0123456789-01",
			expectError: false,
			expected: &CorrelationContext{
				TraceID:    "abcdef0123456789abcdef0123456789",
				SpanID:     "abcdef0123456789",
				TraceFlags: 1,
			},
		},
		{
			name:        "invalid format - too few parts",
			traceParent: "00-abcdef0123456789abcdef0123456789-abcdef0123456789",
			expectError: true,
		},
		{
			name:        "invalid version",
			traceParent: "01-abcdef0123456789abcdef0123456789-abcdef0123456789-01",
			expectError: true,
		},
		{
			name:        "invalid trace ID length",
			traceParent: "00-abcdef-abcdef0123456789-01",
			expectError: true,
		},
		{
			name:        "invalid span ID length",
			traceParent: "00-abcdef0123456789abcdef0123456789-abcdef-01",
			expectError: true,
		},
		{
			name:        "invalid trace flags",
			traceParent: "00-abcdef0123456789abcdef0123456789-abcdef0123456789-zz",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseW3CTraceParent(tt.traceParent)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.TraceID != tt.expected.TraceID {
				t.Errorf("Expected trace ID %s, got %s", tt.expected.TraceID, result.TraceID)
			}
			if result.SpanID != tt.expected.SpanID {
				t.Errorf("Expected span ID %s, got %s", tt.expected.SpanID, result.SpanID)
			}
			if result.TraceFlags != tt.expected.TraceFlags {
				t.Errorf("Expected trace flags %d, got %d", tt.expected.TraceFlags, result.TraceFlags)
			}
		})
	}
}

func TestRoundTripW3CTraceParent(t *testing.T) {
	original := NewCorrelationContext()
	original.TraceFlags = 1

	// Convert to trace parent and back
	traceParent := original.ToW3CTraceParent()
	parsed, err := ParseW3CTraceParent(traceParent)

	if err != nil {
		t.Fatalf("Failed to parse generated trace parent: %v", err)
	}

	if parsed.TraceID != original.TraceID {
		t.Errorf("Trace ID mismatch: expected %s, got %s", original.TraceID, parsed.TraceID)
	}
	if parsed.SpanID != original.SpanID {
		t.Errorf("Span ID mismatch: expected %s, got %s", original.SpanID, parsed.SpanID)
	}
	if parsed.TraceFlags != original.TraceFlags {
		t.Errorf("Trace flags mismatch: expected %d, got %d", original.TraceFlags, parsed.TraceFlags)
	}
}

func TestGetOperationID(t *testing.T) {
	corrCtx := NewCorrelationContext()
	operationID := corrCtx.GetOperationID()

	if operationID != corrCtx.TraceID {
		t.Errorf("Expected operation ID %s, got %s", corrCtx.TraceID, operationID)
	}
}

func TestGetParentID(t *testing.T) {
	parent := NewCorrelationContext()
	child := NewChildCorrelationContext(parent)

	parentID := child.GetParentID()
	if parentID != parent.SpanID {
		t.Errorf("Expected parent ID %s, got %s", parent.SpanID, parentID)
	}

	// Root context should have empty parent ID
	rootParentID := parent.GetParentID()
	if rootParentID != "" {
		t.Errorf("Expected empty parent ID for root context, got %s", rootParentID)
	}
}

func TestIDGeneration(t *testing.T) {
	// Test that generated IDs are unique
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		corrCtx := NewCorrelationContext()

		// Check trace ID uniqueness
		if ids[corrCtx.TraceID] {
			t.Errorf("Duplicate trace ID generated: %s", corrCtx.TraceID)
		}
		ids[corrCtx.TraceID] = true

		// Check span ID uniqueness
		spanIDKey := "span:" + corrCtx.SpanID
		if ids[spanIDKey] {
			t.Errorf("Duplicate span ID generated: %s", corrCtx.SpanID)
		}
		ids[spanIDKey] = true
	}
}

// Helper function to check if a string contains only hexadecimal characters
func isValidHex(s string) bool {
	matched, _ := regexp.MatchString("^[0-9a-f]+$", strings.ToLower(s))
	return matched
}

func TestCorrelationIntegrationWithTelemetryClient(t *testing.T) {
	// Create a client
	client := NewTelemetryClient("test-key")

	// Create correlation context
	corrCtx := NewCorrelationContext()
	corrCtx.OperationName = "test-operation"
	ctx := WithCorrelationContext(context.Background(), corrCtx)

	// Create a test event
	event := NewEventTelemetry("test-event")

	// Track with context
	client.TrackWithContext(ctx, event)

	// Verify that the telemetry was processed (can't easily inspect the envelope
	// without more complex test setup, but this tests the integration path)
	if !client.IsEnabled() {
		t.Error("Client should be enabled")
	}
}

func TestCorrelationContextPropagation(t *testing.T) {
	// Create parent context
	parent := NewCorrelationContext()
	parent.OperationName = "parent-operation"
	parent.TraceFlags = 1

	// Create child context
	child := NewChildCorrelationContext(parent)

	// Verify correlation propagation
	if child.TraceID != parent.TraceID {
		t.Errorf("Child should inherit parent trace ID")
	}

	if child.SpanID == parent.SpanID {
		t.Error("Child should have different span ID")
	}

	if child.ParentSpanID != parent.SpanID {
		t.Error("Child parent span ID should match parent span ID")
	}

	if child.OperationName != parent.OperationName {
		t.Error("Child should inherit operation name")
	}

	if child.TraceFlags != parent.TraceFlags {
		t.Error("Child should inherit trace flags")
	}
}

func TestTelemetryClientContextMethods(t *testing.T) {
	client := NewTelemetryClient("test-key")
	corrCtx := NewCorrelationContext()
	ctx := WithCorrelationContext(context.Background(), corrCtx)

	// Test that context methods don't panic
	client.TrackEventWithContext(ctx, "test-event")
	client.TrackTraceWithContext(ctx, "test-message", contracts.Information)
	client.TrackRequestWithContext(ctx, "GET", "/test", time.Second, "200")
	client.TrackRemoteDependencyWithContext(ctx, "test-dep", "HTTP", "example.com", true)
}

func TestTelemetryContextEnvelopWithCorrelation(t *testing.T) {
	// Create telemetry context
	telCtx := NewTelemetryContext("test-key")

	// Create correlation context
	corrCtx := NewCorrelationContext()
	corrCtx.OperationName = "test-operation"
	corrCtx.TraceFlags = 1
	ctx := WithCorrelationContext(context.Background(), corrCtx)

	// Create test telemetry
	event := NewEventTelemetry("test-event")

	// Envelop with correlation context
	envelope := telCtx.envelopWithContext(ctx, event)

	// Verify operation ID was set from correlation context
	if envelope.Tags[contracts.OperationId] != corrCtx.GetOperationID() {
		t.Errorf("Expected operation ID %s, got %s", corrCtx.GetOperationID(), envelope.Tags[contracts.OperationId])
	}

	// Verify operation name was set
	if envelope.Tags[contracts.OperationName] != corrCtx.OperationName {
		t.Errorf("Expected operation name %s, got %s", corrCtx.OperationName, envelope.Tags[contracts.OperationName])
	}
}

func TestTelemetryContextEnvelopWithCorrelationChild(t *testing.T) {
	// Create telemetry context
	telCtx := NewTelemetryContext("test-key")

	// Create parent and child correlation contexts
	parent := NewCorrelationContext()
	parent.OperationName = "parent-operation"
	child := NewChildCorrelationContext(parent)
	ctx := WithCorrelationContext(context.Background(), child)

	// Create test telemetry
	event := NewEventTelemetry("test-event")

	// Envelop with child correlation context
	envelope := telCtx.envelopWithContext(ctx, event)

	// Verify operation ID is from child (which inherits trace ID from parent)
	if envelope.Tags[contracts.OperationId] != child.GetOperationID() {
		t.Errorf("Expected operation ID %s, got %s", child.GetOperationID(), envelope.Tags[contracts.OperationId])
	}

	// Verify parent ID is set from child's parent span ID
	if envelope.Tags[contracts.OperationParentId] != child.GetParentID() {
		t.Errorf("Expected parent ID %s, got %s", child.GetParentID(), envelope.Tags[contracts.OperationParentId])
	}

	// Verify operation name was inherited
	if envelope.Tags[contracts.OperationName] != parent.OperationName {
		t.Errorf("Expected operation name %s, got %s", parent.OperationName, envelope.Tags[contracts.OperationName])
	}
}

func TestTelemetryContextEnvelopWithoutCorrelation(t *testing.T) {
	// Create telemetry context
	telCtx := NewTelemetryContext("test-key")

	// Create test telemetry
	event := NewEventTelemetry("test-event")

	// Envelop without correlation context
	envelope := telCtx.envelopWithContext(context.Background(), event)

	// Verify operation ID was generated (UUID format)
	operationID := envelope.Tags[contracts.OperationId]
	if operationID == "" {
		t.Error("Operation ID should be generated when no correlation context")
	}

	// Should be UUID format (36 characters with dashes)
	if len(operationID) != 36 {
		t.Errorf("Expected UUID format operation ID, got %s", operationID)
	}

	// Verify no parent ID is set
	if _, exists := envelope.Tags[contracts.OperationParentId]; exists {
		t.Error("Parent ID should not be set when no correlation context")
	}
}

// Request-Id header support tests

func TestToRequestID(t *testing.T) {
	corrCtx := &CorrelationContext{
		TraceID: "abcdef0123456789abcdef0123456789",
		SpanID:  "abcdef0123456789",
	}

	requestID := corrCtx.ToRequestID()
	expected := "|abcdef0123456789abcdef0123456789.abcdef0123456789."

	if requestID != expected {
		t.Errorf("Expected Request-Id %s, got %s", expected, requestID)
	}
}

func TestParseRequestID(t *testing.T) {
	tests := []struct {
		name        string
		requestID   string
		expectError bool
		expectedCtx *CorrelationContext
	}{
		{
			name:        "valid Request-Id format",
			requestID:   "|abcdef0123456789abcdef0123456789.abcdef0123456789.",
			expectError: false,
			expectedCtx: &CorrelationContext{
				TraceID:    "abcdef0123456789abcdef0123456789",
				SpanID:     "abcdef0123456789",
				TraceFlags: 0,
			},
		},
		{
			name:        "valid Request-Id without pipes",
			requestID:   "abcdef0123456789abcdef0123456789.abcdef0123456789",
			expectError: false,
			expectedCtx: &CorrelationContext{
				TraceID:    "abcdef0123456789abcdef0123456789",
				SpanID:     "abcdef0123456789",
				TraceFlags: 0,
			},
		},
		{
			name:        "empty Request-Id",
			requestID:   "",
			expectError: true,
		},
		{
			name:        "invalid format - no dot separator",
			requestID:   "|abcdef0123456789abcdef0123456789abcdef0123456789|",
			expectError: false, // Should create new context
		},
		{
			name:        "legacy short ID",
			requestID:   "|123.456.",
			expectError: false, // Should create new context with generated IDs
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseRequestID(tt.requestID)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Fatal("Result should not be nil")
			}

			if tt.expectedCtx != nil {
				if result.TraceID != tt.expectedCtx.TraceID {
					t.Errorf("Expected trace ID %s, got %s", tt.expectedCtx.TraceID, result.TraceID)
				}
				if result.SpanID != tt.expectedCtx.SpanID {
					t.Errorf("Expected span ID %s, got %s", tt.expectedCtx.SpanID, result.SpanID)
				}
				if result.TraceFlags != tt.expectedCtx.TraceFlags {
					t.Errorf("Expected trace flags %d, got %d", tt.expectedCtx.TraceFlags, result.TraceFlags)
				}
			} else {
				// For cases where we expect new context to be generated
				if len(result.TraceID) != 32 {
					t.Errorf("Expected generated trace ID length 32, got %d", len(result.TraceID))
				}
				if len(result.SpanID) != 16 {
					t.Errorf("Expected generated span ID length 16, got %d", len(result.SpanID))
				}
			}
		})
	}
}

func TestCreateChildRequestID(t *testing.T) {
	// Test with valid parent
	parentRequestID := "|abcdef0123456789abcdef0123456789.abcdef0123456789."
	childRequestID := CreateChildRequestID(parentRequestID)

	if childRequestID == "" {
		t.Fatal("Child Request-Id should not be empty")
	}

	// Parse both to verify relationship
	parentCtx, err := ParseRequestID(parentRequestID)
	if err != nil {
		t.Fatalf("Failed to parse parent Request-Id: %v", err)
	}

	childCtx, err := ParseRequestID(childRequestID)
	if err != nil {
		t.Fatalf("Failed to parse child Request-Id: %v", err)
	}

	// Child should inherit trace ID
	if childCtx.TraceID != parentCtx.TraceID {
		t.Errorf("Child trace ID %s should match parent %s", childCtx.TraceID, parentCtx.TraceID)
	}

	// Child should have different span ID
	if childCtx.SpanID == parentCtx.SpanID {
		t.Error("Child span ID should be different from parent")
	}

	// Test with empty parent
	childOfEmpty := CreateChildRequestID("")
	if childOfEmpty == "" {
		t.Error("Child of empty parent should still generate Request-Id")
	}

	// Test with invalid parent
	childOfInvalid := CreateChildRequestID("invalid")
	if childOfInvalid == "" {
		t.Error("Child of invalid parent should still generate Request-Id")
	}
}

func TestRequestIDRoundTrip(t *testing.T) {
	original := NewCorrelationContext()

	// Convert to Request-Id and back
	requestID := original.ToRequestID()
	parsed, err := ParseRequestID(requestID)

	if err != nil {
		t.Fatalf("Failed to parse generated Request-Id: %v", err)
	}

	if parsed.TraceID != original.TraceID {
		t.Errorf("Trace ID mismatch: expected %s, got %s", original.TraceID, parsed.TraceID)
	}
	if parsed.SpanID != original.SpanID {
		t.Errorf("Span ID mismatch: expected %s, got %s", original.SpanID, parsed.SpanID)
	}
}

func TestW3CToRequestIDCompatibility(t *testing.T) {
	// Create correlation context
	corrCtx := NewCorrelationContext()

	// Get both formats
	w3cTraceParent := corrCtx.ToW3CTraceParent()
	requestID := corrCtx.ToRequestID()

	// Parse both
	w3cParsed, err := ParseW3CTraceParent(w3cTraceParent)
	if err != nil {
		t.Fatalf("Failed to parse W3C trace parent: %v", err)
	}

	requestIDParsed, err := ParseRequestID(requestID)
	if err != nil {
		t.Fatalf("Failed to parse Request-Id: %v", err)
	}

	// Both should have the same trace and span IDs
	if w3cParsed.TraceID != requestIDParsed.TraceID {
		t.Errorf("W3C and Request-Id trace IDs should match: %s vs %s", w3cParsed.TraceID, requestIDParsed.TraceID)
	}
	if w3cParsed.SpanID != requestIDParsed.SpanID {
		t.Errorf("W3C and Request-Id span IDs should match: %s vs %s", w3cParsed.SpanID, requestIDParsed.SpanID)
	}
}

func TestIsValidHexString(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"abcdef0123456789", true},
		{"ABCDEF0123456789", true},
		{"0123456789abcdef", true},
		{"", false},
		{"ghij", false},
		{"123g", false},
		{"123-456", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isValidHexString(tt.input)
			if result != tt.expected {
				t.Errorf("isValidHexString(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}
