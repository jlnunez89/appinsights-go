package appinsights

import (
	"testing"
	"time"
)

// Integration test to verify performance counters work end-to-end
func TestPerformanceCounters_Integration(t *testing.T) {
	// Use mock client that captures metrics
	mockClient := newMockTelemetryClientForPC()
	
	// Configure performance counter collection
	pcConfig := PerformanceCounterConfig{
		Enabled:              true,
		CollectionInterval:   100 * time.Millisecond, // Fast for testing
		EnableSystemMetrics:  true,
		EnableRuntimeMetrics: true,
		CustomCollectors: []PerformanceCounterCollector{
			NewCustomPerformanceCounterCollector("Test", func() map[string]float64 {
				return map[string]float64{
					"test.custom.metric": 123.45,
				}
			}),
		},
	}
	
	// Create and start performance counter manager
	manager := NewPerformanceCounterManager(mockClient, pcConfig)
	manager.Start()
	
	// Wait for several collection cycles
	time.Sleep(350 * time.Millisecond)
	
	// Stop collection
	manager.Stop()
	
	// Verify metrics were collected
	metrics := mockClient.getMetrics()
	if len(metrics) == 0 {
		t.Fatal("Expected metrics to be collected, but got none")
	}
	
	// Check for expected metric types
	foundSystemMetric := false
	foundRuntimeMetric := false
	foundCustomMetric := false
	
	for metricName, metricValue := range metrics {
		switch metricName {
		case "system.cpu.count":
			foundSystemMetric = true
			if metricValue <= 0 {
				t.Errorf("Expected positive CPU count, got %f", metricValue)
			}
		case "runtime.goroutines":
			foundRuntimeMetric = true
			if metricValue <= 0 {
				t.Errorf("Expected positive goroutine count, got %f", metricValue)
			}
		case "test.custom.metric":
			foundCustomMetric = true
			if metricValue != 123.45 {
				t.Errorf("Expected custom metric value 123.45, got %f", metricValue)
			}
		}
	}
	
	if !foundSystemMetric {
		t.Error("Expected to find system.cpu.count metric")
	}
	if !foundRuntimeMetric {
		t.Error("Expected to find runtime.goroutines metric")
	}
	if !foundCustomMetric {
		t.Error("Expected to find test.custom.metric")
	}
	
	// Verify we collected metrics multiple times (should have at least 3 collections)
	if len(metrics) < 10 {
		t.Errorf("Expected at least 10 different metrics collected, got %d", len(metrics))
	}
}

// Test that performance counters respect disabled configuration
func TestPerformanceCounters_DisabledIntegration(t *testing.T) {
	// Use mock client that captures metrics
	mockClient := newMockTelemetryClientForPC()
	
	// Configure performance counter collection as DISABLED
	pcConfig := PerformanceCounterConfig{
		Enabled:              false, // DISABLED
		CollectionInterval:   50 * time.Millisecond,
		EnableSystemMetrics:  true,
		EnableRuntimeMetrics: true,
	}
	
	// Create and start performance counter manager
	manager := NewPerformanceCounterManager(mockClient, pcConfig)
	manager.Start() // Should not actually start because disabled
	
	// Wait for what would be collection time
	time.Sleep(150 * time.Millisecond)
	
	// Stop collection
	manager.Stop()
	
	// Verify no metrics were collected
	metrics := mockClient.getMetrics()
	if len(metrics) != 0 {
		t.Errorf("Expected no metrics when disabled, but got %d items", len(metrics))
	}
}

// Test performance counter collection with rapid start/stop
func TestPerformanceCounters_RapidStartStop(t *testing.T) {
	client := NewTelemetryClient("test-key")
	
	config := PerformanceCounterConfig{
		Enabled:              true,
		CollectionInterval:   10 * time.Millisecond,
		EnableSystemMetrics:  false,
		EnableRuntimeMetrics: true,
	}
	
	// Rapidly start and stop multiple times
	for i := 0; i < 10; i++ {
		client.StartPerformanceCounterCollection(config)
		if !client.IsPerformanceCounterCollectionEnabled() {
			t.Errorf("Expected performance counter collection to be enabled on iteration %d", i)
		}
		
		time.Sleep(1 * time.Millisecond) // Brief pause
		
		client.StopPerformanceCounterCollection()
		if client.IsPerformanceCounterCollectionEnabled() {
			t.Errorf("Expected performance counter collection to be disabled on iteration %d", i)
		}
	}
}

// Test that multiple collectors work correctly together
func TestPerformanceCounters_MultipleCollectors(t *testing.T) {
	mockClient := newMockTelemetryClientForPC()
	
	// Create multiple custom collectors
	collector1 := NewCustomPerformanceCounterCollector("Collector1", func() map[string]float64 {
		return map[string]float64{
			"collector1.metric1": 100.0,
			"collector1.metric2": 200.0,
		}
	})
	
	collector2 := NewCustomPerformanceCounterCollector("Collector2", func() map[string]float64 {
		return map[string]float64{
			"collector2.metric1": 300.0,
			"collector2.metric2": 400.0,
		}
	})
	
	config := PerformanceCounterConfig{
		Enabled:              true,
		CollectionInterval:   100 * time.Millisecond,
		EnableSystemMetrics:  false,
		EnableRuntimeMetrics: false,
		CustomCollectors:     []PerformanceCounterCollector{collector1, collector2},
	}
	
	manager := NewPerformanceCounterManager(mockClient, config)
	manager.Start()
	
	// Wait for collection
	time.Sleep(150 * time.Millisecond)
	
	manager.Stop()
	
	// Verify all custom metrics were collected
	metrics := mockClient.getMetrics()
	
	expectedMetrics := map[string]float64{
		"collector1.metric1": 100.0,
		"collector1.metric2": 200.0,
		"collector2.metric1": 300.0,
		"collector2.metric2": 400.0,
	}
	
	for expectedName, expectedValue := range expectedMetrics {
		if actualValue, exists := metrics[expectedName]; !exists {
			t.Errorf("Expected metric %s was not found", expectedName)
		} else if actualValue != expectedValue {
			t.Errorf("Expected %s = %f, got %f", expectedName, expectedValue, actualValue)
		}
	}
}