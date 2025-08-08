package appinsights

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"code.cloudfoundry.org/clock"
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

func TestAdaptiveSamplingProcessor_Creation(t *testing.T) {
	config := AdaptiveSamplingConfig{
		MaxItemsPerSecond:   50,
		EvaluationWindow:    10 * time.Second,
		InitialSamplingRate: 100,
		MinSamplingRate:     1,
		MaxSamplingRate:     100,
		PerTypeConfigs: map[TelemetryType]AdaptiveTypeConfig{
			TelemetryTypeEvent: {
				MaxItemsPerSecond: 20,
				MinSamplingRate:   5,
				MaxSamplingRate:   100,
			},
		},
	}
	
	processor := NewAdaptiveSamplingProcessor(config)
	
	if processor.GetSamplingRate() != 100 {
		t.Errorf("GetSamplingRate() = %v, want 100", processor.GetSamplingRate())
	}
	
	if processor.GetSamplingRateForType(TelemetryTypeEvent) != 100 {
		t.Errorf("GetSamplingRateForType(Event) = %v, want 100", processor.GetSamplingRateForType(TelemetryTypeEvent))
	}
}

func TestAdaptiveSamplingProcessor_DefaultConfig(t *testing.T) {
	// Test with minimal config - should set sensible defaults
	config := AdaptiveSamplingConfig{
		MaxItemsPerSecond: 100,
	}
	
	processor := NewAdaptiveSamplingProcessor(config)
	
	if processor.config.EvaluationWindow != 15*time.Second {
		t.Errorf("EvaluationWindow = %v, want 15s", processor.config.EvaluationWindow)
	}
	
	if processor.config.InitialSamplingRate != 100 {
		t.Errorf("InitialSamplingRate = %v, want 100", processor.config.InitialSamplingRate)
	}
	
	if processor.config.MinSamplingRate != 1 {
		t.Errorf("MinSamplingRate = %v, want 1", processor.config.MinSamplingRate)
	}
	
	if processor.config.MaxSamplingRate != 100 {
		t.Errorf("MaxSamplingRate = %v, want 100", processor.config.MaxSamplingRate)
	}
}

func TestAdaptiveSamplingProcessor_VolumeTracking(t *testing.T) {
	config := AdaptiveSamplingConfig{
		MaxItemsPerSecond:   10,
		EvaluationWindow:    5 * time.Second,
		InitialSamplingRate: 100,
	}
	
	processor := NewAdaptiveSamplingProcessor(config)
	
	// Create multiple envelopes and track volume
	for i := 0; i < 5; i++ {
		envelope := &contracts.Envelope{
			Name: "Microsoft.ApplicationInsights.test.Event",
			IKey: "test-key",
			Tags: map[string]string{
				contracts.OperationId: generateTestOperationId(i),
			},
		}
		
		processor.ShouldSample(envelope)
	}
	
	// Check that volume is tracked
	rate := processor.GetCurrentVolumeRate()
	if rate <= 0 {
		t.Errorf("GetCurrentVolumeRate() = %v, want > 0", rate)
	}
}

func TestAdaptiveSamplingProcessor_DeterministicSampling(t *testing.T) {
	config := AdaptiveSamplingConfig{
		MaxItemsPerSecond:   100,
		InitialSamplingRate: 50,
	}
	
	processor := NewAdaptiveSamplingProcessor(config)
	
	// Same operation ID should always produce same result
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

func TestAdaptiveSamplingProcessor_SamplingMetadata(t *testing.T) {
	config := AdaptiveSamplingConfig{
		MaxItemsPerSecond:   1000, // High limit to avoid rate adjustments
		InitialSamplingRate: 25,
		EvaluationWindow:    60 * time.Second, // Long window to avoid evaluations
	}
	
	processor := NewAdaptiveSamplingProcessor(config)
	
	// Set a recent evaluation time to prevent immediate adjustment
	processor.lastEvaluation = processor.clock.Now()
	
	envelope := &contracts.Envelope{
		Name: "Microsoft.ApplicationInsights.test.Event",
		IKey: "test-key",
		Tags: map[string]string{
			contracts.OperationId: "test-operation-id",
		},
	}
	
	processor.ShouldSample(envelope)
	
	expectedSampleRate := 100.0 / 25.0 // 4.0
	if envelope.SampleRate != expectedSampleRate {
		t.Errorf("SampleRate = %v, want %v", envelope.SampleRate, expectedSampleRate)
	}
}

func TestVolumeCounter_Record(t *testing.T) {
	counter := NewVolumeCounter(5)
	now := time.Now()
	
	// Record some items
	counter.Record(now)
	counter.Record(now)
	counter.Record(now.Add(time.Second))
	
	rate := counter.GetRate(now.Add(2 * time.Second))
	if rate <= 0 {
		t.Errorf("GetRate() = %v, want > 0", rate)
	}
}

func TestVolumeCounter_RateCalculation(t *testing.T) {
	counter := NewVolumeCounter(3)
	now := time.Now()
	
	// Record 2 items in first second
	counter.Record(now)
	counter.Record(now)
	
	// Record 3 items in second second
	counter.Record(now.Add(time.Second))
	counter.Record(now.Add(time.Second))
	counter.Record(now.Add(time.Second))
	
	// Get rate after 2 seconds - should be (2+3)/2 = 2.5 items/second
	rate := counter.GetRate(now.Add(2 * time.Second))
	expected := 2.5
	tolerance := 0.1
	
	if rate < expected-tolerance || rate > expected+tolerance {
		t.Errorf("GetRate() = %v, want ~%v", rate, expected)
	}
}

func TestVolumeCounter_EmptyCounter(t *testing.T) {
	counter := NewVolumeCounter(5)
	now := time.Now()
	
	rate := counter.GetRate(now)
	if rate != 0 {
		t.Errorf("GetRate() on empty counter = %v, want 0", rate)
	}
}

func TestTelemetryClientWithAdaptiveSampling(t *testing.T) {
	config := NewTelemetryConfiguration("test-key")
	adaptiveConfig := AdaptiveSamplingConfig{
		MaxItemsPerSecond:   50,
		InitialSamplingRate: 100,
	}
	config.SamplingProcessor = NewAdaptiveSamplingProcessor(adaptiveConfig)
	
	client := NewTelemetryClientFromConfig(config)
	testChannel := &TestTelemetryChannel{}
	tc := client.(*telemetryClient)
	tc.channel = testChannel
	
	// Track some telemetry
	client.TrackEvent("test-event")
	client.TrackTrace("test-trace", contracts.Information)
	client.TrackMetric("test-metric", 42.0)
	
	// All should be sent with 100% initial rate
	if testChannel.getSentCount() != 3 {
		t.Errorf("Expected 3 telemetry items, got %d", testChannel.getSentCount())
	}
	
	// Verify that adaptive sampling processor is being used
	adaptiveProcessor := config.SamplingProcessor.(*AdaptiveSamplingProcessor)
	if adaptiveProcessor.GetCurrentVolumeRate() <= 0 {
		t.Errorf("Expected volume tracking to be working")
	}
}

func TestAdaptiveSamplingProcessor_PerTypeConfiguration(t *testing.T) {
	config := AdaptiveSamplingConfig{
		MaxItemsPerSecond:   100,
		InitialSamplingRate: 50,
		PerTypeConfigs: map[TelemetryType]AdaptiveTypeConfig{
			TelemetryTypeEvent: {
				MaxItemsPerSecond: 20,
				MinSamplingRate:   10,
				MaxSamplingRate:   80,
			},
			TelemetryTypeMetric: {
				MaxItemsPerSecond: 50,
				MinSamplingRate:   5,
				MaxSamplingRate:   100,
			},
		},
	}
	
	processor := NewAdaptiveSamplingProcessor(config)
	
	// Verify per-type rates are initialized
	eventRate := processor.GetSamplingRateForType(TelemetryTypeEvent)
	metricRate := processor.GetSamplingRateForType(TelemetryTypeMetric)
	traceRate := processor.GetSamplingRateForType(TelemetryTypeTrace) // Should use global rate
	
	if eventRate != 50 {
		t.Errorf("Event sampling rate = %v, want 50", eventRate)
	}
	
	if metricRate != 50 {
		t.Errorf("Metric sampling rate = %v, want 50", metricRate)
	}
	
	if traceRate != 50 {
		t.Errorf("Trace sampling rate = %v, want 50 (global)", traceRate)
	}
}

func TestAdaptiveSamplingProcessor_ConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config AdaptiveSamplingConfig
	}{
		{
			name: "Invalid evaluation window",
			config: AdaptiveSamplingConfig{
				MaxItemsPerSecond: 100,
				EvaluationWindow:  -5 * time.Second, // Invalid
			},
		},
		{
			name: "Invalid initial rate",
			config: AdaptiveSamplingConfig{
				MaxItemsPerSecond:   100,
				InitialSamplingRate: -10, // Invalid
			},
		},
		{
			name: "Rate bounds validation",
			config: AdaptiveSamplingConfig{
				MaxItemsPerSecond:   100,
				InitialSamplingRate: 50,
				MinSamplingRate:     80, // Min > Max should be corrected
				MaxSamplingRate:     60,
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewAdaptiveSamplingProcessor(tt.config)
			
			// Should not panic and should have valid configuration
			if processor.config.EvaluationWindow <= 0 {
				t.Errorf("EvaluationWindow should be > 0, got %v", processor.config.EvaluationWindow)
			}
			
			if processor.config.InitialSamplingRate <= 0 {
				t.Errorf("InitialSamplingRate should be > 0, got %v", processor.config.InitialSamplingRate)
			}
			
			if processor.config.MinSamplingRate > processor.config.MaxSamplingRate {
				t.Errorf("MinSamplingRate (%v) should be <= MaxSamplingRate (%v)", 
					processor.config.MinSamplingRate, processor.config.MaxSamplingRate)
			}
		})
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

func TestSamplingMetadataEdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		processor       SamplingProcessor
		expectedSampleRate float64
	}{
		{
			name:            "FixedRate 0% sampling",
			processor:       NewFixedRateSamplingProcessor(0),
			expectedSampleRate: 0.0, // Should not be +Inf
		},
		{
			name:            "PerType 0% sampling",
			processor:       NewPerTypeSamplingProcessor(0, map[TelemetryType]float64{}),
			expectedSampleRate: 0.0, // Should not be +Inf
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
			shouldSample := tt.processor.ShouldSample(envelope)
			
			// 0% sampling should never sample
			if shouldSample {
				t.Errorf("0%% sampling should never sample, got %t", shouldSample)
			}

			if envelope.SampleRate != tt.expectedSampleRate {
				t.Errorf("SampleRate = %v, want %v", envelope.SampleRate, tt.expectedSampleRate)
			}
		})
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

func TestAdaptiveSamplingProcessor_RateAdjustment(t *testing.T) {
	// Use a mock clock for controlled testing
	mockClock := NewMockClock()
	
	config := AdaptiveSamplingConfig{
		MaxItemsPerSecond:   5, // Low limit to trigger adjustments
		EvaluationWindow:    5 * time.Second,
		InitialSamplingRate: 100,
		MinSamplingRate:     10,
		MaxSamplingRate:     100,
	}
	
	processor := NewAdaptiveSamplingProcessor(config)
	processor.clock = mockClock
	
	// Simulate high volume to trigger rate reduction
	baseTime := mockClock.Now()
	
	// Send 10 items per second for 6 seconds (should trigger reduction)
	for second := 0; second < 6; second++ {
		currentTime := baseTime.Add(time.Duration(second) * time.Second)
		mockClock.SetTime(currentTime)
		
		for item := 0; item < 10; item++ {
			envelope := &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.Event",
				IKey: "test-key",
				Tags: map[string]string{
					contracts.OperationId: generateTestOperationId(second*10 + item),
				},
			}
			processor.ShouldSample(envelope)
		}
	}
	
	// After high volume, the sampling rate should be reduced
	finalRate := processor.GetSamplingRate()
	if finalRate >= 100 {
		t.Errorf("Expected sampling rate to be reduced from 100, but got %v", finalRate)
	}
	
	// Verify it's above the minimum
	if finalRate < config.MinSamplingRate {
		t.Errorf("Sampling rate %v is below minimum %v", finalRate, config.MinSamplingRate)
	}
}

func TestAdaptiveSamplingProcessor_PerTypeRateAdjustment(t *testing.T) {
	mockClock := NewMockClock()
	
	config := AdaptiveSamplingConfig{
		MaxItemsPerSecond:   50, // Global limit
		EvaluationWindow:    5 * time.Second,
		InitialSamplingRate: 100,
		PerTypeConfigs: map[TelemetryType]AdaptiveTypeConfig{
			TelemetryTypeEvent: {
				MaxItemsPerSecond: 3, // Lower limit for events
				MinSamplingRate:   5,
				MaxSamplingRate:   100,
			},
		},
	}
	
	processor := NewAdaptiveSamplingProcessor(config)
	processor.clock = mockClock
	
	baseTime := mockClock.Now()
	
	// Send many events to trigger per-type adjustment
	for second := 0; second < 6; second++ {
		currentTime := baseTime.Add(time.Duration(second) * time.Second)
		mockClock.SetTime(currentTime)
		
		// Send 8 events per second (exceeds the 3/sec limit for events)
		for item := 0; item < 8; item++ {
			envelope := &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.Event",
				IKey: "test-key",
				Tags: map[string]string{
					contracts.OperationId: generateTestOperationId(second*10 + item),
				},
			}
			processor.ShouldSample(envelope)
		}
		
		// Send some metrics (should not be affected as much)
		for item := 0; item < 2; item++ {
			envelope := &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.Metric",
				IKey: "test-key",
				Tags: map[string]string{
					contracts.OperationId: generateTestOperationId(second*10 + item + 100),
				},
			}
			processor.ShouldSample(envelope)
		}
	}
	
	// Event sampling rate should be reduced more than global rate
	eventRate := processor.GetSamplingRateForType(TelemetryTypeEvent)
	globalRate := processor.GetSamplingRate()
	
	if eventRate >= 100 {
		t.Errorf("Expected event sampling rate to be reduced from 100, but got %v", eventRate)
	}
	
	// Event rate should be lower than global rate due to per-type limits
	if eventRate > globalRate*1.1 { // Allow small tolerance
		t.Errorf("Expected event rate (%v) to be similar to or lower than global rate (%v)", eventRate, globalRate)
	}
}

func TestAdaptiveSamplingProcessor_VolumeRecovery(t *testing.T) {
	mockClock := NewMockClock()
	
	config := AdaptiveSamplingConfig{
		MaxItemsPerSecond:   10,
		EvaluationWindow:    3 * time.Second,
		InitialSamplingRate: 100,
		MinSamplingRate:     20,
		MaxSamplingRate:     100,
	}
	
	processor := NewAdaptiveSamplingProcessor(config)
	processor.clock = mockClock
	
	baseTime := mockClock.Now()
	
	// Phase 1: High volume to trigger rate reduction
	for second := 0; second < 4; second++ {
		currentTime := baseTime.Add(time.Duration(second) * time.Second)
		mockClock.SetTime(currentTime)
		
		for item := 0; item < 20; item++ { // High volume
			envelope := &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.Event",
				IKey: "test-key",
				Tags: map[string]string{
					contracts.OperationId: generateTestOperationId(second*20 + item),
				},
			}
			processor.ShouldSample(envelope)
		}
	}
	
	reducedRate := processor.GetSamplingRate()
	if reducedRate >= 100 {
		t.Errorf("Expected rate to be reduced after high volume, got %v", reducedRate)
	}
	
	// Phase 2: Low volume to allow recovery - need to advance time sufficiently
	for second := 4; second < 12; second++ { // Longer period for recovery
		currentTime := baseTime.Add(time.Duration(second) * time.Second)
		mockClock.SetTime(currentTime)
		
		// Very low volume - should allow rate to increase
		envelope := &contracts.Envelope{
			Name: "Microsoft.ApplicationInsights.test.Event",
			IKey: "test-key",
			Tags: map[string]string{
				contracts.OperationId: generateTestOperationId(second),
			},
		}
		processor.ShouldSample(envelope)
	}
	
	recoveredRate := processor.GetSamplingRate()
	if recoveredRate <= reducedRate*1.05 { // Allow for at least 5% improvement
		t.Errorf("Expected rate to recover from %v after low volume, but got %v", reducedRate, recoveredRate)
	}
}

// MockClock for testing time-based functionality
type MockClock struct {
	currentTime time.Time
	mutex       sync.RWMutex
}

func NewMockClock() *MockClock {
	return &MockClock{
		currentTime: time.Now(),
	}
}

func (m *MockClock) Now() time.Time {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.currentTime
}

func (m *MockClock) SetTime(t time.Time) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.currentTime = t
}

func (m *MockClock) Sleep(d time.Duration) {
	// For testing, we'll just advance the time
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.currentTime = m.currentTime.Add(d)
}

func (m *MockClock) Since(t time.Time) time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.currentTime.Sub(t)
}

func (m *MockClock) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	go func() {
		time.Sleep(d) // Use real time for simplicity in tests
		ch <- m.Now()
	}()
	return ch
}

func (m *MockClock) NewTimer(d time.Duration) clock.Timer {
	// For testing purposes, delegate to the real clock
	return clock.NewClock().NewTimer(d)
}

func (m *MockClock) NewTicker(d time.Duration) clock.Ticker {
	// For testing purposes, delegate to the real clock
	return clock.NewClock().NewTicker(d)
}

// Tests for Intelligent Sampling features

func TestErrorPrioritySamplingRule_ShouldApply(t *testing.T) {
	rule := NewErrorPrioritySamplingRule()
	
	tests := []struct {
		name     string
		envelope *contracts.Envelope
		expected bool
	}{
		{
			name: "Exception telemetry should apply",
			envelope: &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.Exception",
				IKey: "test-key",
			},
			expected: true,
		},
		{
			name: "Failed request (4xx) should apply",
			envelope: &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.Request",
				IKey: "test-key",
				Data: &contracts.Data{
					BaseData: &contracts.RequestData{
						ResponseCode: "404",
					},
				},
			},
			expected: true,
		},
		{
			name: "Failed request (5xx) should apply",
			envelope: &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.Request",
				IKey: "test-key",
				Data: &contracts.Data{
					BaseData: &contracts.RequestData{
						ResponseCode: "500",
					},
				},
			},
			expected: true,
		},
		{
			name: "Successful request should not apply",
			envelope: &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.Request",
				IKey: "test-key",
				Data: &contracts.Data{
					BaseData: &contracts.RequestData{
						ResponseCode: "200",
						Success:      true,
					},
				},
			},
			expected: false,
		},
		{
			name: "Failed dependency should apply",
			envelope: &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.RemoteDependency",
				IKey: "test-key",
				Data: &contracts.Data{
					BaseData: &contracts.RemoteDependencyData{
						Success: false,
					},
				},
			},
			expected: true,
		},
		{
			name: "Successful dependency should not apply",
			envelope: &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.RemoteDependency",
				IKey: "test-key",
				Data: &contracts.Data{
					BaseData: &contracts.RemoteDependencyData{
						Success: true,
					},
				},
			},
			expected: false,
		},
		{
			name: "Error level trace should apply",
			envelope: &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.Message",
				IKey: "test-key",
				Data: &contracts.Data{
					BaseData: &contracts.MessageData{
						SeverityLevel: contracts.Error,
					},
				},
			},
			expected: true,
		},
		{
			name: "Critical level trace should apply",
			envelope: &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.Message",
				IKey: "test-key",
				Data: &contracts.Data{
					BaseData: &contracts.MessageData{
						SeverityLevel: contracts.Critical,
					},
				},
			},
			expected: true,
		},
		{
			name: "Info level trace should not apply",
			envelope: &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.Message",
				IKey: "test-key",
				Data: &contracts.Data{
					BaseData: &contracts.MessageData{
						SeverityLevel: contracts.Information,
					},
				},
			},
			expected: false,
		},
		{
			name: "Regular event should not apply",
			envelope: &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.Event",
				IKey: "test-key",
			},
			expected: false,
		},
		{
			name: "Nil envelope should not apply",
			envelope: nil,
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.ShouldApply(tt.envelope)
			if result != tt.expected {
				t.Errorf("ShouldApply() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestErrorPrioritySamplingRule_Properties(t *testing.T) {
	rule := NewErrorPrioritySamplingRule()
	
	if rate := rule.GetSamplingRate(); rate != 100.0 {
		t.Errorf("GetSamplingRate() = %v, want 100.0", rate)
	}
	
	if priority := rule.GetPriority(); priority != 1000 {
		t.Errorf("GetPriority() = %v, want 1000", priority)
	}
}

func TestCustomSamplingRule(t *testing.T) {
	// Create a rule that applies to events with "important" in the name
	rule := NewCustomSamplingRule("important-events", 500, 75.0, func(envelope *contracts.Envelope) bool {
		if envelope == nil {
			return false
		}
		telType := extractTelemetryTypeFromName(envelope.Name)
		if telType == TelemetryTypeEvent {
			if envelope.Data != nil {
				if data, ok := envelope.Data.(*contracts.Data); ok && data.BaseData != nil {
					if eventData, ok := data.BaseData.(*contracts.EventData); ok {
						return strings.Contains(eventData.Name, "important")
					}
				}
			}
		}
		return false
	})
	
	tests := []struct {
		name     string
		envelope *contracts.Envelope
		expected bool
	}{
		{
			name: "Important event should apply",
			envelope: &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.Event",
				Data: &contracts.Data{
					BaseData: &contracts.EventData{
						Name: "important-business-event",
					},
				},
			},
			expected: true,
		},
		{
			name: "Regular event should not apply",
			envelope: &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.Event",
				Data: &contracts.Data{
					BaseData: &contracts.EventData{
						Name: "regular-event",
					},
				},
			},
			expected: false,
		},
		{
			name: "Non-event should not apply",
			envelope: &contracts.Envelope{
				Name: "Microsoft.ApplicationInsights.test.Metric",
			},
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.ShouldApply(tt.envelope)
			if result != tt.expected {
				t.Errorf("ShouldApply() = %v, want %v", result, tt.expected)
			}
		})
	}
	
	// Test properties
	if rule.Name() != "important-events" {
		t.Errorf("Name() = %v, want 'important-events'", rule.Name())
	}
	
	if rule.GetPriority() != 500 {
		t.Errorf("GetPriority() = %v, want 500", rule.GetPriority())
	}
	
	if rule.GetSamplingRate() != 75.0 {
		t.Errorf("GetSamplingRate() = %v, want 75.0", rule.GetSamplingRate())
	}
}

func TestCustomSamplingRule_RateClamping(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{-10, 0},   // Negative should be clamped to 0
		{150, 100}, // > 100 should be clamped to 100
		{50, 50},   // Valid value should remain unchanged
	}
	
	for _, tt := range tests {
		rule := NewCustomSamplingRule("test", 100, tt.input, func(envelope *contracts.Envelope) bool {
			return true
		})
		
		if rate := rule.GetSamplingRate(); rate != tt.expected {
			t.Errorf("NewCustomSamplingRule with rate %v: GetSamplingRate() = %v, want %v", 
				tt.input, rate, tt.expected)
		}
	}
}

func TestCustomRuleEngine(t *testing.T) {
	engine := NewCustomRuleEngine(50.0) // 50% default rate
	
	// Add custom rules
	highPriorityRule := NewCustomSamplingRule("high-priority", 800, 100.0, func(envelope *contracts.Envelope) bool {
		return strings.Contains(envelope.Name, "high-priority")
	})
	
	lowPriorityRule := NewCustomSamplingRule("low-priority", 200, 10.0, func(envelope *contracts.Envelope) bool {
		return strings.Contains(envelope.Name, "low-priority")
	})
	
	engine.AddRule(highPriorityRule)
	engine.AddRule(lowPriorityRule)
	
	tests := []struct {
		name          string
		envelopeName  string
		expectedRate  float64
	}{
		{
			name:         "High priority rule should take precedence",
			envelopeName: "Microsoft.ApplicationInsights.high-priority.Event",
			expectedRate: 100.0,
		},
		{
			name:         "Low priority rule should apply when high priority doesn't",
			envelopeName: "Microsoft.ApplicationInsights.low-priority.Event",
			expectedRate: 10.0,
		},
		{
			name:         "Exception should be sampled at 100% (error priority rule)",
			envelopeName: "Microsoft.ApplicationInsights.test.Exception",
			expectedRate: 100.0,
		},
		{
			name:         "Regular envelope should use default rate",
			envelopeName: "Microsoft.ApplicationInsights.test.Event",
			expectedRate: 50.0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope := &contracts.Envelope{
				Name: tt.envelopeName,
				IKey: "test-key",
			}
			
			rate := engine.GetSamplingRate(envelope)
			if rate != tt.expectedRate {
				t.Errorf("GetSamplingRate() = %v, want %v", rate, tt.expectedRate)
			}
		})
	}
}

func TestCustomRuleEngine_RuleManagement(t *testing.T) {
	engine := NewCustomRuleEngine(25.0)
	
	// Add a custom rule
	rule := NewCustomSamplingRule("test-rule", 300, 80.0, func(envelope *contracts.Envelope) bool {
		return strings.Contains(envelope.Name, "test")
	})
	
	engine.AddRule(rule)
	
	// Test that the rule is applied
	envelope := &contracts.Envelope{
		Name: "Microsoft.ApplicationInsights.test.Event",
		IKey: "test-key",
	}
	
	rate := engine.GetSamplingRate(envelope)
	if rate != 80.0 {
		t.Errorf("GetSamplingRate() after adding rule = %v, want 80.0", rate)
	}
	
	// Remove the rule
	engine.RemoveRule("test-rule")
	
	// Test that default rate is used
	rate = engine.GetSamplingRate(envelope)
	if rate != 25.0 {
		t.Errorf("GetSamplingRate() after removing rule = %v, want 25.0", rate)
	}
}

func TestIntelligentSamplingProcessor_Creation(t *testing.T) {
	processor := NewIntelligentSamplingProcessor(60.0)
	
	if processor.GetSamplingRate() != 60.0 {
		t.Errorf("GetSamplingRate() = %v, want 60.0", processor.GetSamplingRate())
	}
	
	// Test that rule engine is properly initialized
	ruleEngine := processor.GetRuleEngine()
	if ruleEngine == nil {
		t.Error("GetRuleEngine() returned nil")
	}
}

func TestIntelligentSamplingProcessor_ErrorPriority(t *testing.T) {
	processor := NewIntelligentSamplingProcessor(10.0) // Low default rate
	
	// Test exception telemetry - should always be sampled
	exceptionEnvelope := &contracts.Envelope{
		Name: "Microsoft.ApplicationInsights.test.Exception",
		IKey: "test-key",
		Tags: map[string]string{
			contracts.OperationId: "test-operation-123",
		},
	}
	
	// Test multiple times to ensure consistency
	for i := 0; i < 10; i++ {
		if !processor.ShouldSample(exceptionEnvelope) {
			t.Errorf("Exception should always be sampled, but got false on iteration %d", i)
		}
		
		// Check that sample rate is set correctly for exceptions
		if exceptionEnvelope.SampleRate != 1.0 {
			t.Errorf("Exception SampleRate = %v, want 1.0", exceptionEnvelope.SampleRate)
		}
	}
	
	// Test failed request - should always be sampled
	failedRequestEnvelope := &contracts.Envelope{
		Name: "Microsoft.ApplicationInsights.test.Request",
		IKey: "test-key",
		Tags: map[string]string{
			contracts.OperationId: "test-operation-456",
		},
		Data: &contracts.Data{
			BaseData: &contracts.RequestData{
				ResponseCode: "500",
			},
		},
	}
	
	if !processor.ShouldSample(failedRequestEnvelope) {
		t.Error("Failed request should always be sampled")
	}
	
	// Test successful request with low sampling rate - may or may not be sampled
	successfulRequestEnvelope := &contracts.Envelope{
		Name: "Microsoft.ApplicationInsights.test.Request",
		IKey: "test-key",
		Tags: map[string]string{
			contracts.OperationId: "test-operation-789",
		},
		Data: &contracts.Data{
			BaseData: &contracts.RequestData{
				ResponseCode: "200",
				Success:      true,
			},
		},
	}
	
	processor.ShouldSample(successfulRequestEnvelope)
	// Should have sample rate reflecting the 10% default rate
	expectedSampleRate := 100.0 / 10.0 // 10.0
	if successfulRequestEnvelope.SampleRate != expectedSampleRate {
		t.Errorf("Successful request SampleRate = %v, want %v", successfulRequestEnvelope.SampleRate, expectedSampleRate)
	}
}

func TestIntelligentSamplingProcessor_DependencyAware(t *testing.T) {
	processor := NewIntelligentSamplingProcessor(50.0)
	
	// Test that envelopes with the same operation ID have consistent sampling decisions
	operationId := "test-operation-consistency"
	
	envelopes := []*contracts.Envelope{
		{
			Name: "Microsoft.ApplicationInsights.test.Request",
			IKey: "test-key",
			Tags: map[string]string{
				contracts.OperationId: operationId,
			},
			Data: &contracts.Data{
				BaseData: &contracts.RequestData{
					ResponseCode: "200",
				},
			},
		},
		{
			Name: "Microsoft.ApplicationInsights.test.RemoteDependency",
			IKey: "test-key",
			Tags: map[string]string{
				contracts.OperationId: operationId,
			},
			Data: &contracts.Data{
				BaseData: &contracts.RemoteDependencyData{
					Success: true,
				},
			},
		},
		{
			Name: "Microsoft.ApplicationInsights.test.Event",
			IKey: "test-key",
			Tags: map[string]string{
				contracts.OperationId: operationId,
			},
		},
	}
	
	// All envelopes with the same operation ID should have the same sampling decision
	firstResult := processor.ShouldSample(envelopes[0])
	
	for i, envelope := range envelopes[1:] {
		result := processor.ShouldSample(envelope)
		if result != firstResult {
			t.Errorf("Envelope %d with same operation ID should have same sampling decision. Expected %v, got %v", 
				i+1, firstResult, result)
		}
	}
}

func TestIntelligentSamplingProcessor_CustomRules(t *testing.T) {
	processor := NewIntelligentSamplingProcessor(20.0)
	
	// Add a custom rule for high-priority events
	customRule := NewCustomSamplingRule("high-priority-events", 600, 90.0, func(envelope *contracts.Envelope) bool {
		if envelope.Data != nil {
			if data, ok := envelope.Data.(*contracts.Data); ok && data.BaseData != nil {
				if eventData, ok := data.BaseData.(*contracts.EventData); ok {
					if eventData.Properties != nil {
						if priority, exists := eventData.Properties["priority"]; exists {
							return priority == "high"
						}
					}
				}
			}
		}
		return false
	})
	
	processor.AddRule(customRule)
	
	// Test high-priority event
	highPriorityEnvelope := &contracts.Envelope{
		Name: "Microsoft.ApplicationInsights.test.Event",
		IKey: "test-key",
		Tags: map[string]string{
			contracts.OperationId: "test-high-priority",
		},
		Data: &contracts.Data{
			BaseData: &contracts.EventData{
				Name: "business-event",
				Properties: map[string]string{
					"priority": "high",
				},
			},
		},
	}
	
	// Should use the custom rule's 90% rate, so sample rate should be 100/90 ≈ 1.11
	processor.ShouldSample(highPriorityEnvelope)
	expectedSampleRate := 100.0 / 90.0
	tolerance := 0.01
	
	if abs(highPriorityEnvelope.SampleRate-expectedSampleRate) > tolerance {
		t.Errorf("High priority event SampleRate = %v, want ~%v", highPriorityEnvelope.SampleRate, expectedSampleRate)
	}
	
	// Test regular event - should use default 20% rate
	regularEnvelope := &contracts.Envelope{
		Name: "Microsoft.ApplicationInsights.test.Event",
		IKey: "test-key",
		Tags: map[string]string{
			contracts.OperationId: "test-regular",
		},
		Data: &contracts.Data{
			BaseData: &contracts.EventData{
				Name: "regular-event",
			},
		},
	}
	
	processor.ShouldSample(regularEnvelope)
	expectedSampleRate = 100.0 / 20.0 // 5.0
	
	if regularEnvelope.SampleRate != expectedSampleRate {
		t.Errorf("Regular event SampleRate = %v, want %v", regularEnvelope.SampleRate, expectedSampleRate)
	}
	
	// Remove the custom rule
	processor.RemoveRule("high-priority-events")
	
	// Test that high-priority event now uses default rate
	processor.ShouldSample(highPriorityEnvelope)
	if highPriorityEnvelope.SampleRate != expectedSampleRate {
		t.Errorf("After removing rule, SampleRate = %v, want %v", highPriorityEnvelope.SampleRate, expectedSampleRate)
	}
}

func TestIntelligentSamplingProcessor_WithFallbackProcessor(t *testing.T) {
	// Create custom rule engine and adaptive processor as fallback
	ruleEngine := NewCustomRuleEngine(40.0)
	adaptiveProcessor := NewAdaptiveSamplingProcessor(AdaptiveSamplingConfig{
		MaxItemsPerSecond:   100,
		InitialSamplingRate: 30.0,
	})
	
	processor := NewIntelligentSamplingProcessorWithFallback(ruleEngine, adaptiveProcessor)
	
	// Test that it uses the adaptive processor for dependency-aware sampling
	if processor.GetSamplingRate() != 30.0 {
		t.Errorf("GetSamplingRate() = %v, want 30.0 (from adaptive processor)", processor.GetSamplingRate())
	}
	
	// Test exception priority still works
	exceptionEnvelope := &contracts.Envelope{
		Name: "Microsoft.ApplicationInsights.test.Exception",
		IKey: "test-key",
		Tags: map[string]string{
			contracts.OperationId: "test-exception",
		},
	}
	
	if !processor.ShouldSample(exceptionEnvelope) {
		t.Error("Exception should always be sampled even with custom fallback processor")
	}
}

func TestTelemetryClientWithIntelligentSampling(t *testing.T) {
	// Test integration with telemetry client
	config := NewTelemetryConfiguration("test-key")
	config.SamplingProcessor = NewIntelligentSamplingProcessor(25.0)
	
	client := NewTelemetryClientFromConfig(config)
	testChannel := &TestTelemetryChannel{}
	tc := client.(*telemetryClient)
	tc.channel = testChannel
	
	// Track an exception - should always be sent
	client.TrackException("test error")
	
	// Track regular telemetry - may or may not be sent based on sampling
	client.TrackEvent("regular-event")
	client.TrackTrace("info message", contracts.Information)
	
	// The exception should definitely be in the sent items
	sentCount := testChannel.getSentCount()
	if sentCount == 0 {
		t.Error("Expected at least the exception to be sent")
	}
	
	// Check that at least one item is the exception
	hasException := false
	for _, envelope := range testChannel.sentItems {
		if strings.Contains(envelope.Name, "Exception") {
			hasException = true
			break
		}
	}
	
	if !hasException {
		t.Error("Exception telemetry should have been sent")
	}
}

// Helper functions for tests

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}