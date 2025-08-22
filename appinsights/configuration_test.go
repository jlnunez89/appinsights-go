package appinsights

import "testing"

func TestTelemetryConfiguration(t *testing.T) {
	// Connection string now required
	testKey := "test"
	defaultEndpoint := DefaultIngestionEndpoint

	config := NewTelemetryConfiguration("InstrumentationKey=" + testKey)

	if config.InstrumentationKey != testKey {
		t.Errorf("InstrumentationKey is %s, want %s", config.InstrumentationKey, testKey)
	}

	if config.EndpointUrl != defaultEndpoint {
		t.Errorf("EndpointUrl is %s, want %s", config.EndpointUrl, defaultEndpoint)
	}

	if config.Client != nil {
		t.Errorf("Client is not nil, want nil")
	}

	// Test that SamplingProcessor is nil by default
	if config.SamplingProcessor != nil {
		t.Errorf("SamplingProcessor is not nil, want nil")
	}
}

func TestTelemetryConfigurationWithSampling(t *testing.T) {
	// Connection string now required
	testKey := "test-sampling"

	config := NewTelemetryConfiguration("InstrumentationKey=" + testKey)

	// Test setting a sampling processor
	processor := NewFixedRateSamplingProcessor(50.0)
	config.SamplingProcessor = processor

	if config.SamplingProcessor == nil {
		t.Errorf("SamplingProcessor is nil after setting")
	}

	if config.SamplingProcessor.GetSamplingRate() != 50.0 {
		t.Errorf("Sampling rate is %v, want 50.0", config.SamplingProcessor.GetSamplingRate())
	}

	// Test creating client with sampling configuration
	client := NewTelemetryClientFromConfig(config)
	if client == nil {
		t.Errorf("Client creation failed with sampling processor")
	}
}
