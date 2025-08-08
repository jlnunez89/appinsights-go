package appinsights

import (
	"context"
	"testing"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

func TestRequestTelemetryWithCorrelationContext(t *testing.T) {
	// Create correlation context
	corrCtx := NewCorrelationContext()
	ctx := WithCorrelationContext(context.Background(), corrCtx)

	// Create request telemetry with context
	request := NewRequestTelemetryWithContext(ctx, "GET", "/test", time.Second, "200")

	// Verify that the telemetry ID matches the correlation span ID
	if request.Id != corrCtx.SpanID {
		t.Errorf("Expected request ID %s, got %s", corrCtx.SpanID, request.Id)
	}

	// Verify that the telemetry data also has the correct ID
	data := request.TelemetryData().(*contracts.RequestData)
	if data.Id != corrCtx.SpanID {
		t.Errorf("Expected request data ID %s, got %s", corrCtx.SpanID, data.Id)
	}
}

func TestRequestTelemetryWithoutCorrelationContext(t *testing.T) {
	// Create request telemetry without context (should use fallback UUID)
	request := NewRequestTelemetryWithContext(nil, "GET", "/test", time.Second, "200")

	// Verify that an ID was generated
	if request.Id == "" {
		t.Error("Expected non-empty request ID when no correlation context")
	}

	// Verify that the telemetry data also has an ID
	data := request.TelemetryData().(*contracts.RequestData)
	if data.Id == "" {
		t.Error("Expected non-empty request data ID when no correlation context")
	}

	if data.Id != request.Id {
		t.Errorf("Request data ID %s should match request ID %s", data.Id, request.Id)
	}
}

func TestRequestTelemetryWithContextWithoutCorrelation(t *testing.T) {
	// Create request telemetry with context but no correlation
	ctx := context.Background()
	request := NewRequestTelemetryWithContext(ctx, "GET", "/test", time.Second, "200")

	// Verify that an ID was generated
	if request.Id == "" {
		t.Error("Expected non-empty request ID when context has no correlation")
	}

	// Verify that the telemetry data also has an ID
	data := request.TelemetryData().(*contracts.RequestData)
	if data.Id != request.Id {
		t.Errorf("Request data ID %s should match request ID %s", data.Id, request.Id)
	}
}

func TestRemoteDependencyTelemetryWithCorrelationContext(t *testing.T) {
	// Create correlation context
	corrCtx := NewCorrelationContext()
	ctx := WithCorrelationContext(context.Background(), corrCtx)

	// Create dependency telemetry with context
	dependency := NewRemoteDependencyTelemetryWithContext(ctx, "test-dep", "HTTP", "example.com", true)

	// Verify that the telemetry ID matches the correlation span ID
	if dependency.Id != corrCtx.SpanID {
		t.Errorf("Expected dependency ID %s, got %s", corrCtx.SpanID, dependency.Id)
	}

	// Verify that the telemetry data also has the correct ID
	data := dependency.TelemetryData().(*contracts.RemoteDependencyData)
	if data.Id != corrCtx.SpanID {
		t.Errorf("Expected dependency data ID %s, got %s", corrCtx.SpanID, data.Id)
	}
}

func TestRemoteDependencyTelemetryWithoutCorrelationContext(t *testing.T) {
	// Create dependency telemetry without context (should use fallback UUID)
	dependency := NewRemoteDependencyTelemetryWithContext(nil, "test-dep", "HTTP", "example.com", true)

	// Verify that an ID was generated
	if dependency.Id == "" {
		t.Error("Expected non-empty dependency ID when no correlation context")
	}

	// Verify that the telemetry data also has an ID
	data := dependency.TelemetryData().(*contracts.RemoteDependencyData)
	if data.Id != dependency.Id {
		t.Errorf("Dependency data ID %s should match dependency ID %s", data.Id, dependency.Id)
	}
}

func TestAvailabilityTelemetryWithCorrelationContext(t *testing.T) {
	// Create correlation context
	corrCtx := NewCorrelationContext()
	ctx := WithCorrelationContext(context.Background(), corrCtx)

	// Create availability telemetry with context
	availability := NewAvailabilityTelemetryWithContext(ctx, "test-availability", time.Second, true)

	// Verify that the telemetry ID matches the correlation span ID
	if availability.Id != corrCtx.SpanID {
		t.Errorf("Expected availability ID %s, got %s", corrCtx.SpanID, availability.Id)
	}

	// Verify that the telemetry data also has the correct ID
	data := availability.TelemetryData().(*contracts.AvailabilityData)
	if data.Id != corrCtx.SpanID {
		t.Errorf("Expected availability data ID %s, got %s", corrCtx.SpanID, data.Id)
	}
}

func TestAvailabilityTelemetryWithoutCorrelationContext(t *testing.T) {
	// Create availability telemetry without context (should use fallback UUID)
	availability := NewAvailabilityTelemetryWithContext(nil, "test-availability", time.Second, true)

	// Verify that an ID was generated
	if availability.Id == "" {
		t.Error("Expected non-empty availability ID when no correlation context")
	}

	// Verify that the telemetry data also has an ID
	data := availability.TelemetryData().(*contracts.AvailabilityData)
	if data.Id != availability.Id {
		t.Errorf("Availability data ID %s should match availability ID %s", data.Id, availability.Id)
	}
}

func TestTelemetryClientContextAwareTrackingWithCorrelation(t *testing.T) {
	// Create a test client
	client := NewTelemetryClient("test-key")

	// Create correlation context
	corrCtx := NewCorrelationContext()
	corrCtx.OperationName = "test-operation"
	ctx := WithCorrelationContext(context.Background(), corrCtx)

	// Test that context-aware tracking methods don't panic and include correlation
	client.TrackRequestWithContext(ctx, "GET", "/test", time.Second, "200")
	client.TrackRemoteDependencyWithContext(ctx, "test-dep", "HTTP", "example.com", true)
	client.TrackAvailabilityWithContext(ctx, "test-availability", time.Second, true)

	// Verify client is still enabled after tracking
	if !client.IsEnabled() {
		t.Error("Client should remain enabled after context-aware tracking")
	}
}

func TestCorrelationPropagationThroughParentChild(t *testing.T) {
	// Create parent correlation context
	parent := NewCorrelationContext()
	parent.OperationName = "parent-operation"

	// Create child correlation context
	child := NewChildCorrelationContext(parent)
	childCtx := WithCorrelationContext(context.Background(), child)

	// Create telemetry with child context
	request := NewRequestTelemetryWithContext(childCtx, "GET", "/child", time.Second, "200")
	dependency := NewRemoteDependencyTelemetryWithContext(childCtx, "child-dep", "HTTP", "api.example.com", true)

	// Verify that telemetry uses child span ID
	if request.Id != child.SpanID {
		t.Errorf("Request should use child span ID %s, got %s", child.SpanID, request.Id)
	}

	if dependency.Id != child.SpanID {
		t.Errorf("Dependency should use child span ID %s, got %s", child.SpanID, dependency.Id)
	}

	// Verify that both telemetry items have the same ID (same span)
	if request.Id != dependency.Id {
		t.Error("Request and dependency should have the same span ID when created in same context")
	}

	// Create telemetry context and verify envelope correlation
	telCtx := NewTelemetryContext("test-key")
	envelope := telCtx.envelopWithContext(childCtx, request)

	// Verify operation ID is from child (trace ID)
	if envelope.Tags[contracts.OperationId] != child.GetOperationID() {
		t.Errorf("Expected operation ID %s, got %s", child.GetOperationID(), envelope.Tags[contracts.OperationId])
	}

	// Verify parent ID is from child's parent span ID
	if envelope.Tags[contracts.OperationParentId] != child.GetParentID() {
		t.Errorf("Expected parent ID %s, got %s", child.GetParentID(), envelope.Tags[contracts.OperationParentId])
	}
}

func TestBackwardCompatibilityWithExistingConstructors(t *testing.T) {
	// Test that existing constructors still work and don't set IDs by default
	// (except for RequestTelemetry which always had an ID)

	// RemoteDependencyTelemetry should have empty ID with old constructor
	dependency := NewRemoteDependencyTelemetry("test-dep", "HTTP", "example.com", true)
	if dependency.Id != "" {
		t.Errorf("Expected empty dependency ID with old constructor, got %s", dependency.Id)
	}

	// AvailabilityTelemetry should have empty ID with old constructor
	availability := NewAvailabilityTelemetry("test-availability", time.Second, true)
	if availability.Id != "" {
		t.Errorf("Expected empty availability ID with old constructor, got %s", availability.Id)
	}

	// RequestTelemetry should still have UUID with old constructor (existing behavior)
	request := NewRequestTelemetry("GET", "/test", time.Second, "200")
	if request.Id == "" {
		t.Error("Expected non-empty request ID with old constructor (existing behavior)")
	}
}

func TestCorrelationWithMultipleSpansInSameTrace(t *testing.T) {
	// Create parent correlation context
	parent := NewCorrelationContext()
	parent.OperationName = "parent-operation"

	// Create first child
	child1 := NewChildCorrelationContext(parent)
	child1Ctx := WithCorrelationContext(context.Background(), child1)

	// Create second child from same parent
	child2 := NewChildCorrelationContext(parent)
	child2Ctx := WithCorrelationContext(context.Background(), child2)

	// Create telemetry with both child contexts
	request1 := NewRequestTelemetryWithContext(child1Ctx, "GET", "/child1", time.Second, "200")
	request2 := NewRequestTelemetryWithContext(child2Ctx, "GET", "/child2", time.Second, "200")

	// Verify that both requests have the same trace ID but different span IDs
	telCtx := NewTelemetryContext("test-key")
	envelope1 := telCtx.envelopWithContext(child1Ctx, request1)
	envelope2 := telCtx.envelopWithContext(child2Ctx, request2)

	// Both should have the same operation ID (trace ID)
	if envelope1.Tags[contracts.OperationId] != envelope2.Tags[contracts.OperationId] {
		t.Error("Both requests should have the same operation ID (trace ID)")
	}

	// Both should have the same parent ID (parent span ID)
	if envelope1.Tags[contracts.OperationParentId] != envelope2.Tags[contracts.OperationParentId] {
		t.Error("Both requests should have the same parent ID")
	}

	// But should have different span IDs in telemetry data
	if request1.Id == request2.Id {
		t.Error("Different child requests should have different span IDs")
	}
}
