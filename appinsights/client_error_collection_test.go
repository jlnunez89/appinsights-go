package appinsights

import (
	"strings"
	"testing"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

func TestTelemetryClientWithErrorAutoCollection(t *testing.T) {
	mockClock()
	defer resetClock()
	
	config := NewTelemetryConfiguration("test-key")
	config.ErrorAutoCollection = NewErrorAutoCollectionConfig()
	
	client := NewTelemetryClientFromConfig(config)
	
	if client.ErrorAutoCollector() == nil {
		t.Error("Expected client to have an error auto-collector when configured")
	}
	
	if !client.ErrorAutoCollector().IsEnabled() {
		t.Error("Expected error auto-collector to be enabled")
	}
}

func TestTelemetryClientWithoutErrorAutoCollection(t *testing.T) {
	mockClock()
	defer resetClock()
	
	config := NewTelemetryConfiguration("test-key")
	// ErrorAutoCollection is nil by default
	
	client := NewTelemetryClientFromConfig(config)
	
	if client.ErrorAutoCollector() != nil {
		t.Error("Expected client to not have an error auto-collector when not configured")
	}
}

func TestTelemetryClientErrorAutoCollectionEndToEnd(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	// Add error auto-collection to the client
	config := NewErrorAutoCollectionConfig()
	client.(*telemetryClient).errorAutoCollector = NewErrorAutoCollector(client, config)
	
	// Track an error through the auto-collector
	collector := client.ErrorAutoCollector()
	collector.TrackError("test error from auto-collector")
	
	client.Channel().Close()
	req := transmitter.waitForRequest(t)
	
	if !strings.Contains(req.payload, "test error from auto-collector") {
		t.Errorf("Expected payload to contain the tracked error, got: %s", req.payload)
	}
}

func TestTelemetryClientErrorAutoCollectionWithPanicRecovery(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	// Add error auto-collection to the client
	config := NewErrorAutoCollectionConfig()
	client.(*telemetryClient).errorAutoCollector = NewErrorAutoCollector(client, config)
	
	// Test panic recovery
	collector := client.ErrorAutoCollector()
	collector.RecoverPanic(func() {
		panic("test panic recovery")
	})
	
	client.Channel().Close()
	req := transmitter.waitForRequest(t)
	
	if !strings.Contains(req.payload, "test panic recovery") {
		t.Errorf("Expected payload to contain the recovered panic, got: %s", req.payload)
	}
}

func TestTelemetryClientErrorAutoCollectionFiltering(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	// Add error auto-collection to the client with filtering
	config := NewErrorAutoCollectionConfig()
	config.IgnoredErrors = []string{"ignored"}
	client.(*telemetryClient).errorAutoCollector = NewErrorAutoCollector(client, config)
	
	collector := client.ErrorAutoCollector()
	
	// Track an ignored error
	collector.TrackError("this is an ignored error message")
	
	// Track a non-ignored error
	collector.TrackError("this is a tracked error message")
	
	client.Channel().Close()
	req := transmitter.waitForRequest(t)
	
	if strings.Contains(req.payload, "ignored error") {
		t.Error("Expected ignored error to not be in payload")
	}
	
	if !strings.Contains(req.payload, "tracked error") {
		t.Error("Expected tracked error to be in payload")
	}
}

func TestTelemetryClientErrorAutoCollectionDisabled(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	// Add error auto-collection to the client but disabled
	config := NewErrorAutoCollectionConfig()
	config.Enabled = false
	client.(*telemetryClient).errorAutoCollector = NewErrorAutoCollector(client, config)
	
	collector := client.ErrorAutoCollector()
	if collector.IsEnabled() {
		t.Error("Expected error auto-collector to be disabled")
	}
	
	// Try to track an error - should not be sent
	collector.TrackError("should not be tracked")
	
	client.Channel().Close()
	
	// Should not receive any requests
	select {
	case req := <-transmitter.requests:
		if strings.Contains(req.payload, "should not be tracked") {
			t.Error("Expected disabled auto-collector to not track errors")
		}
	default:
		// Expected - no request should be sent
	}
}

func TestTelemetryClientErrorAutoCollectionLegacyCompatibility(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	// Add error auto-collection to the client
	config := NewErrorAutoCollectionConfig()
	client.(*telemetryClient).errorAutoCollector = NewErrorAutoCollector(client, config)
	
	// Test that existing TrackException still works
	client.TrackException("legacy exception tracking")
	
	client.Channel().Close()
	req := transmitter.waitForRequest(t)
	
	if !strings.Contains(req.payload, "legacy exception tracking") {
		t.Errorf("Expected legacy exception tracking to work, got: %s", req.payload)
	}
}

func TestTelemetryClientErrorAutoCollectionWithCustomSanitizer(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	// Add error auto-collection to the client with custom sanitizer
	config := NewErrorAutoCollectionConfig()
	config.ErrorSanitizers = []ErrorSanitizerFunc{
		func(err interface{}, frames []*contracts.StackFrame) (interface{}, []*contracts.StackFrame) {
			errStr := strings.ReplaceAll(err.(string), "sensitive", "[SANITIZED]")
			return errStr, frames
		},
	}
	client.(*telemetryClient).errorAutoCollector = NewErrorAutoCollector(client, config)
	
	collector := client.ErrorAutoCollector()
	collector.TrackError("error with sensitive data")
	
	client.Channel().Close()
	req := transmitter.waitForRequest(t)
	
	if strings.Contains(req.payload, "sensitive") {
		t.Error("Expected sensitive data to be sanitized")
	}
	
	if !strings.Contains(req.payload, "[SANITIZED]") {
		t.Error("Expected sanitized placeholder to be present")
	}
}

func TestTelemetryClientErrorAutoCollectionStackFrameLimit(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	// Add error auto-collection to the client with frame limit
	config := NewErrorAutoCollectionConfig()
	config.MaxStackFrames = 2
	client.(*telemetryClient).errorAutoCollector = NewErrorAutoCollector(client, config)
	
	collector := client.ErrorAutoCollector()
	collector.TrackError("error with limited stack")
	
	client.Channel().Close()
	req := transmitter.waitForRequest(t)
	
	// Count stack frames in payload
	stackFrameCount := strings.Count(req.payload, `"level":`)
	
	if stackFrameCount > 2 {
		t.Errorf("Expected at most 2 stack frames, got %d", stackFrameCount)
	}
}