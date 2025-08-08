package appinsights

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

func TestFixedRateSamplingProcessor_ShouldSample(t *testing.T) {
	tests := []struct {
		name         string
		samplingRate float64
		expectedRate float64
		testCount    int
	}{
		{
			name:         "0% sampling rate",
			samplingRate: 0,
			expectedRate: 0,
			testCount:    100,
		},
		{
			name:         "100% sampling rate",
			samplingRate: 100,
			expectedRate: 100,
			testCount:    100,
		},
		{
			name:         "50% sampling rate",
			samplingRate: 50,
			expectedRate: 50,
			testCount:    1000,
		},
		{
			name:         "10% sampling rate",
			samplingRate: 10,
			expectedRate: 10,
			testCount:    1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewFixedRateSamplingProcessor(tt.samplingRate)
			
			if processor.GetSamplingRate() != tt.expectedRate {
				t.Errorf("GetSamplingRate() = %v, want %v", processor.GetSamplingRate(), tt.expectedRate)
			}

			sampled := 0
			for i := 0; i < tt.testCount; i++ {
				// Create envelope with unique operation ID
				envelope := &contracts.Envelope{
					Name: "test",
					IKey: "test-key",
					Tags: map[string]string{
						contracts.OperationId: generateTestOperationId(i),
					},
				}

				if processor.ShouldSample(envelope) {
					sampled++
				}
			}

			actualRate := float64(sampled) / float64(tt.testCount) * 100
			tolerance := 5.0 // 5% tolerance for statistical variation

			if tt.expectedRate == 0 && sampled != 0 {
				t.Errorf("Expected 0%% sampling but got %d samples out of %d", sampled, tt.testCount)
			} else if tt.expectedRate == 100 && sampled != tt.testCount {
				t.Errorf("Expected 100%% sampling but got %d samples out of %d", sampled, tt.testCount)
			} else if tt.expectedRate > 0 && tt.expectedRate < 100 {
				if actualRate < tt.expectedRate-tolerance || actualRate > tt.expectedRate+tolerance {
					t.Errorf("Sampling rate %v%% is outside tolerance. Expected ~%v%%, got %v%% (%d/%d)", 
						tt.samplingRate, tt.expectedRate, actualRate, sampled, tt.testCount)
				}
			}
		})
	}
}

func TestFixedRateSamplingProcessor_DeterministicSampling(t *testing.T) {
	processor := NewFixedRateSamplingProcessor(50)
	
	// Same operation ID should always produce same result
	envelope := &contracts.Envelope{
		Name: "test",
		IKey: "test-key",
		Tags: map[string]string{
			contracts.OperationId: "test-operation-id-123",
		},
	}

	firstResult := processor.ShouldSample(envelope)
	for i := 0; i < 10; i++ {
		result := processor.ShouldSample(envelope)
		if result != firstResult {
			t.Errorf("Sampling decision changed for same operation ID. Expected consistent result.")
		}
	}
}

func TestFixedRateSamplingProcessor_NoOperationId(t *testing.T) {
	processor := NewFixedRateSamplingProcessor(50)
	
	// Envelope without operation ID should use envelope name + ikey
	envelope := &contracts.Envelope{
		Name: "test-envelope",
		IKey: "test-key",
		Tags: map[string]string{},
	}

	// Should still make a sampling decision
	result1 := processor.ShouldSample(envelope)
	result2 := processor.ShouldSample(envelope)
	
	// Should be deterministic
	if result1 != result2 {
		t.Errorf("Sampling decision should be deterministic even without operation ID")
	}
}

func TestFixedRateSamplingProcessor_InvalidRates(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{-10, 0},   // Negative rates should be clamped to 0
		{150, 100}, // Rates > 100 should be clamped to 100
		{0, 0},     // Zero should remain zero
		{100, 100}, // 100 should remain 100
		{50, 50},   // Valid rates should remain unchanged
	}

	for _, tt := range tests {
		processor := NewFixedRateSamplingProcessor(tt.input)
		if processor.GetSamplingRate() != tt.expected {
			t.Errorf("NewFixedRateSamplingProcessor(%v) rate = %v, want %v", 
				tt.input, processor.GetSamplingRate(), tt.expected)
		}
	}
}

func TestDisabledSamplingProcessor(t *testing.T) {
	processor := NewDisabledSamplingProcessor()
	
	if processor.GetSamplingRate() != 100.0 {
		t.Errorf("DisabledSamplingProcessor.GetSamplingRate() = %v, want 100.0", processor.GetSamplingRate())
	}

	envelope := &contracts.Envelope{
		Name: "test",
		IKey: "test-key",
		Tags: map[string]string{
			contracts.OperationId: "test-operation-id",
		},
	}

	if !processor.ShouldSample(envelope) {
		t.Errorf("DisabledSamplingProcessor.ShouldSample() = false, want true")
	}
}

func TestCalculateSamplingHash(t *testing.T) {
	// Test empty string
	if hash := calculateSamplingHash(""); hash != 0 {
		t.Errorf("calculateSamplingHash(\"\") = %v, want 0", hash)
	}

	// Test that same operation ID produces same hash
	operationId := "test-operation-id-consistent"
	hash1 := calculateSamplingHash(operationId)
	hash2 := calculateSamplingHash(operationId)
	if hash1 != hash2 {
		t.Errorf("calculateSamplingHash should be deterministic. Got %v and %v for same input", hash1, hash2)
	}

	// Test that normalization works (case and dashes)
	normalizedId := "testoperationidconsistent"
	uppercaseId := "TEST-OPERATION-ID-CONSISTENT"
	mixedId := "Test-Operation-Id-Consistent"
	
	hashNormal := calculateSamplingHash(normalizedId)
	hashUpper := calculateSamplingHash(uppercaseId)
	hashMixed := calculateSamplingHash(mixedId)
	
	if hashNormal != hashUpper {
		t.Errorf("Hash should be case-insensitive. Normal: %v, Upper: %v", hashNormal, hashUpper)
	}
	if hashNormal != hashMixed {
		t.Errorf("Hash should be case-insensitive. Normal: %v, Mixed: %v", hashNormal, hashMixed)
	}

	// Test that different operation IDs produce different hashes
	hash3 := calculateSamplingHash("different-operation-id")
	hash4 := calculateSamplingHash("another-different-id")
	if hash3 == hash4 {
		t.Errorf("Different operation IDs should produce different hashes (collision detected)")
	}
}

func TestTelemetryClientWithSampling(t *testing.T) {
	// Test that telemetry client properly uses sampling processor
	config := NewTelemetryConfiguration("test-key")
	config.SamplingProcessor = NewFixedRateSamplingProcessor(0) // 0% sampling
	
	client := NewTelemetryClientFromConfig(config)
	
	// Create a test channel to verify no telemetry is sent
	testChannel := &TestTelemetryChannel{}
	tc := client.(*telemetryClient)
	tc.channel = testChannel
	
	// Track some telemetry
	client.TrackEvent("test-event")
	client.TrackTrace("test-trace", contracts.Information)
	
	// Verify no telemetry was sent due to 0% sampling
	if testChannel.getSentCount() != 0 {
		t.Errorf("Expected 0 telemetry items with 0%% sampling, got %d", testChannel.getSentCount())
	}
	
	// Now test with 100% sampling
	config.SamplingProcessor = NewFixedRateSamplingProcessor(100)
	client = NewTelemetryClientFromConfig(config)
	tc = client.(*telemetryClient)
	tc.channel = testChannel
	testChannel.reset()
	
	client.TrackEvent("test-event")
	client.TrackTrace("test-trace", contracts.Information)
	
	// Verify all telemetry was sent
	if testChannel.getSentCount() != 2 {
		t.Errorf("Expected 2 telemetry items with 100%% sampling, got %d", testChannel.getSentCount())
	}
}

func TestTelemetryClientWithContextSampling(t *testing.T) {
	// Test that context-aware tracking also respects sampling
	config := NewTelemetryConfiguration("test-key")
	config.SamplingProcessor = NewFixedRateSamplingProcessor(0) // 0% sampling
	
	client := NewTelemetryClientFromConfig(config)
	testChannel := &TestTelemetryChannel{}
	tc := client.(*telemetryClient)
	tc.channel = testChannel
	
	ctx := context.Background()
	client.TrackEventWithContext(ctx, "test-event")
	client.TrackTraceWithContext(ctx, "test-trace", contracts.Information)
	
	if testChannel.getSentCount() != 0 {
		t.Errorf("Expected 0 telemetry items with 0%% sampling, got %d", testChannel.getSentCount())
	}
}

func TestTelemetryClientDefaultSampling(t *testing.T) {
	// Test that client without explicit sampling processor uses disabled sampling (100%)
	config := NewTelemetryConfiguration("test-key")
	// Don't set SamplingProcessor - should default to disabled
	
	client := NewTelemetryClientFromConfig(config)
	testChannel := &TestTelemetryChannel{}
	tc := client.(*telemetryClient)
	tc.channel = testChannel
	
	client.TrackEvent("test-event")
	
	if testChannel.getSentCount() != 1 {
		t.Errorf("Expected 1 telemetry item with default sampling, got %d", testChannel.getSentCount())
	}
}

func TestSamplingMetadata(t *testing.T) {
	tests := []struct {
		name            string
		processor       SamplingProcessor
		expectedSampleRate float64
	}{
		{
			name:            "FixedRate 50% sampling",
			processor:       NewFixedRateSamplingProcessor(50),
			expectedSampleRate: 2.0, // 100/50 = 2.0 (each item represents 2 items)
		},
		{
			name:            "FixedRate 10% sampling",
			processor:       NewFixedRateSamplingProcessor(10),
			expectedSampleRate: 10.0, // 100/10 = 10.0
		},
		{
			name:            "FixedRate 100% sampling",
			processor:       NewFixedRateSamplingProcessor(100),
			expectedSampleRate: 1.0, // 100/100 = 1.0
		},
		{
			name:            "Disabled sampling",
			processor:       NewDisabledSamplingProcessor(),
			expectedSampleRate: 1.0, // No sampling, each item represents itself
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope := &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.Event",
				IKey: "test-key",
				Tags: map[string]string{
					contracts.OperationId: "test-operation-id",
				},
			}

			// Call ShouldSample to set metadata
			tt.processor.ShouldSample(envelope)

			if envelope.SampleRate != tt.expectedSampleRate {
				t.Errorf("SampleRate = %v, want %v", envelope.SampleRate, tt.expectedSampleRate)
			}
		})
	}
}

func TestPerTypeSamplingProcessor_Creation(t *testing.T) {
	typeRates := map[TelemetryType]float64{
		TelemetryTypeEvent:   50,
		TelemetryTypeMetric:  25,
		TelemetryTypeRequest: 100,
	}
	
	processor := NewPerTypeSamplingProcessor(10, typeRates)
	
	if processor.GetSamplingRate() != 10 {
		t.Errorf("GetSamplingRate() = %v, want 10", processor.GetSamplingRate())
	}
	
	if processor.GetSamplingRateForType(TelemetryTypeEvent) != 50 {
		t.Errorf("GetSamplingRateForType(Event) = %v, want 50", processor.GetSamplingRateForType(TelemetryTypeEvent))
	}
	
	if processor.GetSamplingRateForType(TelemetryTypeTrace) != 10 {
		t.Errorf("GetSamplingRateForType(Trace) = %v, want 10 (default)", processor.GetSamplingRateForType(TelemetryTypeTrace))
	}
}

func TestPerTypeSamplingProcessor_InvalidRates(t *testing.T) {
	typeRates := map[TelemetryType]float64{
		TelemetryTypeEvent:  -10,  // Should be clamped to 0
		TelemetryTypeMetric: 150,  // Should be clamped to 100
	}
	
	processor := NewPerTypeSamplingProcessor(-5, typeRates) // Default should be clamped to 0
	
	if processor.GetSamplingRate() != 0 {
		t.Errorf("GetSamplingRate() = %v, want 0 (clamped)", processor.GetSamplingRate())
	}
	
	if processor.GetSamplingRateForType(TelemetryTypeEvent) != 0 {
		t.Errorf("GetSamplingRateForType(Event) = %v, want 0 (clamped)", processor.GetSamplingRateForType(TelemetryTypeEvent))
	}
	
	if processor.GetSamplingRateForType(TelemetryTypeMetric) != 100 {
		t.Errorf("GetSamplingRateForType(Metric) = %v, want 100 (clamped)", processor.GetSamplingRateForType(TelemetryTypeMetric))
	}
}

func TestPerTypeSamplingProcessor_TelemetryTypeExtraction(t *testing.T) {
	processor := NewPerTypeSamplingProcessor(50, map[TelemetryType]float64{})
	
	tests := []struct {
		envelopeName string
		expectedType TelemetryType
	}{
		{
			envelopeName: "Microsoft.ApplicationInsights.test-key.Event",
			expectedType: TelemetryTypeEvent,
		},
		{
			envelopeName: "Microsoft.ApplicationInsights.Event",
			expectedType: TelemetryTypeEvent,
		},
		{
			envelopeName: "Microsoft.ApplicationInsights.test-key.Message",
			expectedType: TelemetryTypeTrace,
		},
		{
			envelopeName: "Microsoft.ApplicationInsights.test-key.Metric",
			expectedType: TelemetryTypeMetric,
		},
		{
			envelopeName: "Microsoft.ApplicationInsights.test-key.Request",
			expectedType: TelemetryTypeRequest,
		},
		{
			envelopeName: "Microsoft.ApplicationInsights.test-key.RemoteDependency",
			expectedType: TelemetryTypeRemoteDependency,
		},
		{
			envelopeName: "Microsoft.ApplicationInsights.test-key.Exception",
			expectedType: TelemetryTypeException,
		},
		{
			envelopeName: "Microsoft.ApplicationInsights.test-key.Availability",
			expectedType: TelemetryTypeAvailability,
		},
		{
			envelopeName: "Microsoft.ApplicationInsights.test-key.PageView",
			expectedType: TelemetryTypePageView,
		},
		{
			envelopeName: "Microsoft.ApplicationInsights.test-key.Unknown",
			expectedType: TelemetryType(""),
		},
		{
			envelopeName: "InvalidEnvelopeName",
			expectedType: TelemetryType(""),
		},
		{
			envelopeName: "",
			expectedType: TelemetryType(""),
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.envelopeName, func(t *testing.T) {
			actualType := processor.extractTelemetryType(tt.envelopeName)
			if actualType != tt.expectedType {
				t.Errorf("extractTelemetryType(%q) = %v, want %v", tt.envelopeName, actualType, tt.expectedType)
			}
		})
	}
}

func TestPerTypeSamplingProcessor_SamplingBehavior(t *testing.T) {
	typeRates := map[TelemetryType]float64{
		TelemetryTypeEvent:   0,   // 0% sampling - should never sample
		TelemetryTypeMetric:  100, // 100% sampling - should always sample
		TelemetryTypeRequest: 50,  // 50% sampling
	}
	
	processor := NewPerTypeSamplingProcessor(10, typeRates) // 10% default
	
	tests := []struct {
		name         string
		envelopeName string
		expectedRate float64
		testCount    int
		shouldSample []bool // For 0% and 100% cases
	}{
		{
			name:         "Event - 0% sampling",
			envelopeName: "Microsoft.ApplicationInsights.test.Event",
			expectedRate: 0,
			testCount:    100,
			shouldSample: []bool{false}, // All should be false
		},
		{
			name:         "Metric - 100% sampling",
			envelopeName: "Microsoft.ApplicationInsights.test.Metric",
			expectedRate: 100,
			testCount:    100,
			shouldSample: []bool{true}, // All should be true
		},
		{
			name:         "Request - 50% sampling",
			envelopeName: "Microsoft.ApplicationInsights.test.Request",
			expectedRate: 50,
			testCount:    1000,
		},
		{
			name:         "Trace - default 10% sampling",
			envelopeName: "Microsoft.ApplicationInsights.test.Message",
			expectedRate: 10,
			testCount:    1000,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampled := 0
			
			for i := 0; i < tt.testCount; i++ {
				envelope := &contracts.Envelope{
					Name: tt.envelopeName,
					IKey: "test-key",
					Tags: map[string]string{
						contracts.OperationId: generateTestOperationId(i),
					},
				}
				
				if processor.ShouldSample(envelope) {
					sampled++
				}
				
				// Check that sampling metadata is set correctly
				expectedSampleRate := 100.0 / tt.expectedRate
				if tt.expectedRate == 0 {
					expectedSampleRate = 100.0 / 0.001 // Avoid division by zero, but this should not happen in practice
				}
				if envelope.SampleRate != expectedSampleRate && tt.expectedRate > 0 {
					t.Errorf("SampleRate = %v, want %v", envelope.SampleRate, expectedSampleRate)
					break
				}
			}
			
			actualRate := float64(sampled) / float64(tt.testCount) * 100
			tolerance := 5.0 // 5% tolerance for statistical variation
			
			if tt.expectedRate == 0 && sampled != 0 {
				t.Errorf("Expected 0%% sampling but got %d samples out of %d", sampled, tt.testCount)
			} else if tt.expectedRate == 100 && sampled != tt.testCount {
				t.Errorf("Expected 100%% sampling but got %d samples out of %d", sampled, tt.testCount)
			} else if tt.expectedRate > 0 && tt.expectedRate < 100 {
				if actualRate < tt.expectedRate-tolerance || actualRate > tt.expectedRate+tolerance {
					t.Errorf("Sampling rate %v%% is outside tolerance. Expected ~%v%%, got %v%% (%d/%d)", 
						tt.expectedRate, tt.expectedRate, actualRate, sampled, tt.testCount)
				}
			}
		})
	}
}

func TestPerTypeSamplingProcessor_DeterministicSampling(t *testing.T) {
	typeRates := map[TelemetryType]float64{
		TelemetryTypeEvent: 50,
	}
	processor := NewPerTypeSamplingProcessor(25, typeRates)
	
	// Same operation ID should always produce same result for same type
	envelope := &contracts.Envelope{
		Name: "Microsoft.ApplicationInsights.test.Event",
		IKey: "test-key",
		Tags: map[string]string{
			contracts.OperationId: "test-operation-id-123",
		},
	}
	
	firstResult := processor.ShouldSample(envelope)
	for i := 0; i < 10; i++ {
		envelope.SampleRate = 0 // Reset to test metadata setting
		result := processor.ShouldSample(envelope)
		if result != firstResult {
			t.Errorf("Sampling decision changed for same operation ID. Expected consistent result.")
		}
	}
}

func TestTelemetryClientWithPerTypeSampling(t *testing.T) {
	// Test that telemetry client properly uses per-type sampling processor
	typeRates := map[TelemetryType]float64{
		TelemetryTypeEvent: 0,   // Block all events
		TelemetryTypeTrace: 100, // Allow all traces
	}
	
	config := NewTelemetryConfiguration("test-key")
	config.SamplingProcessor = NewPerTypeSamplingProcessor(50, typeRates)
	
	client := NewTelemetryClientFromConfig(config)
	testChannel := &TestTelemetryChannel{}
	tc := client.(*telemetryClient)
	tc.channel = testChannel
	
	// Track some telemetry
	client.TrackEvent("test-event")          // Should be blocked (0%)
	client.TrackTrace("test-trace", contracts.Information) // Should pass (100%)
	
	// Verify correct filtering happened
	if testChannel.getSentCount() != 1 {
		t.Errorf("Expected 1 telemetry item (trace only), got %d", testChannel.getSentCount())
	}
	
	// Verify the sent item is the trace
	if len(testChannel.sentItems) > 0 {
		envelope := testChannel.sentItems[0]
		if !strings.Contains(envelope.Name, "Message") {
			t.Errorf("Expected Message telemetry, got %s", envelope.Name)
		}
	}
}

// Helper function to generate test operation IDs
func generateTestOperationId(seed int) string {
	return newUUID().String()
}

// TestTelemetryChannel for testing purposes
type TestTelemetryChannel struct {
	sentItems []*contracts.Envelope
}

func (tc *TestTelemetryChannel) EndpointAddress() string {
	return "test://endpoint"
}

func (tc *TestTelemetryChannel) Send(item *contracts.Envelope) {
	tc.sentItems = append(tc.sentItems, item)
}

func (tc *TestTelemetryChannel) Flush() {}

func (tc *TestTelemetryChannel) Stop() {}

func (tc *TestTelemetryChannel) IsThrottled() bool {
	return false
}

func (tc *TestTelemetryChannel) Close(retryTimeout ...time.Duration) <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (tc *TestTelemetryChannel) getSentCount() int {
	return len(tc.sentItems)
}

func (tc *TestTelemetryChannel) reset() {
	tc.sentItems = nil
}