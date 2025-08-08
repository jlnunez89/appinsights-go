package appinsights

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

// Mock TelemetryClient for testing
type mockTelemetryClientForPC struct {
	metrics map[string]float64
	mu      sync.RWMutex
}

func newMockTelemetryClientForPC() *mockTelemetryClientForPC {
	return &mockTelemetryClientForPC{
		metrics: make(map[string]float64),
	}
}

func (m *mockTelemetryClientForPC) Context() *TelemetryContext                     { return nil }
func (m *mockTelemetryClientForPC) InstrumentationKey() string                     { return "test-key" }
func (m *mockTelemetryClientForPC) Channel() TelemetryChannel                      { return nil }
func (m *mockTelemetryClientForPC) IsEnabled() bool                                { return true }
func (m *mockTelemetryClientForPC) SetIsEnabled(enabled bool)                      {}
func (m *mockTelemetryClientForPC) Track(telemetry Telemetry)                      {}
func (m *mockTelemetryClientForPC) TrackWithContext(ctx context.Context, telemetry Telemetry) {}
func (m *mockTelemetryClientForPC) TrackEvent(name string)                         {}
func (m *mockTelemetryClientForPC) TrackTrace(name string, severity contracts.SeverityLevel) {}
func (m *mockTelemetryClientForPC) TrackRequest(method, url string, duration time.Duration, responseCode string) {}
func (m *mockTelemetryClientForPC) TrackRemoteDependency(name, dependencyType, target string, success bool) {}
func (m *mockTelemetryClientForPC) TrackAvailability(name string, duration time.Duration, success bool) {}
func (m *mockTelemetryClientForPC) TrackException(err interface{})                 {}
func (m *mockTelemetryClientForPC) TrackEventWithContext(ctx context.Context, name string) {}
func (m *mockTelemetryClientForPC) TrackTraceWithContext(ctx context.Context, message string, severity contracts.SeverityLevel) {}
func (m *mockTelemetryClientForPC) TrackRequestWithContext(ctx context.Context, method, url string, duration time.Duration, responseCode string) {}
func (m *mockTelemetryClientForPC) TrackRemoteDependencyWithContext(ctx context.Context, name, dependencyType, target string, success bool) {}
func (m *mockTelemetryClientForPC) TrackAvailabilityWithContext(ctx context.Context, name string, duration time.Duration, success bool) {}
func (m *mockTelemetryClientForPC) StartPerformanceCounterCollection(config PerformanceCounterConfig) {}
func (m *mockTelemetryClientForPC) StopPerformanceCounterCollection()              {}
func (m *mockTelemetryClientForPC) IsPerformanceCounterCollectionEnabled() bool    { return false }

func (m *mockTelemetryClientForPC) TrackMetric(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics[name] = value
}

func (m *mockTelemetryClientForPC) getMetric(name string) (float64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, exists := m.metrics[name]
	return value, exists
}

func (m *mockTelemetryClientForPC) getMetrics() map[string]float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]float64)
	for k, v := range m.metrics {
		result[k] = v
	}
	return result
}

func (m *mockTelemetryClientForPC) clearMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = make(map[string]float64)
}

func TestRuntimeMetricsCollector(t *testing.T) {
	client := newMockTelemetryClientForPC()
	collector := NewRuntimeMetricsCollector()
	
	// Test collector name
	if collector.Name() != "Runtime Metrics" {
		t.Errorf("Expected collector name 'Runtime Metrics', got '%s'", collector.Name())
	}
	
	// Collect metrics
	collector.Collect(client)
	
	// Verify expected metrics were collected
	expectedMetrics := []string{
		"runtime.memory.alloc",
		"runtime.memory.total_alloc",
		"runtime.memory.sys",
		"runtime.memory.heap_alloc",
		"runtime.memory.heap_sys",
		"runtime.memory.heap_idle",
		"runtime.memory.heap_inuse",
		"runtime.memory.heap_released",
		"runtime.memory.heap_objects",
		"runtime.memory.stack_inuse",
		"runtime.memory.stack_sys",
		"runtime.gc.num_gc",
		"runtime.gc.pause_total_ns",
		"runtime.gc.pause_ns",
		"runtime.gc.cpu_fraction",
		"runtime.goroutines",
		"runtime.num_cpu",
		"runtime.cgocall",
	}
	
	for _, metric := range expectedMetrics {
		if _, exists := client.getMetric(metric); !exists {
			t.Errorf("Expected metric '%s' was not collected", metric)
		}
	}
	
	// Verify specific values make sense
	if goroutines, exists := client.getMetric("runtime.goroutines"); exists {
		if goroutines <= 0 {
			t.Errorf("Expected positive number of goroutines, got %f", goroutines)
		}
	}
	
	if numCPU, exists := client.getMetric("runtime.num_cpu"); exists {
		if numCPU != float64(runtime.NumCPU()) {
			t.Errorf("Expected %d CPUs, got %f", runtime.NumCPU(), numCPU)
		}
	}
}

func TestSystemMetricsCollector(t *testing.T) {
	client := newMockTelemetryClientForPC()
	collector := NewSystemMetricsCollector()
	
	// Test collector name
	if collector.Name() != "System Metrics" {
		t.Errorf("Expected collector name 'System Metrics', got '%s'", collector.Name())
	}
	
	// Collect metrics
	collector.Collect(client)
	
	// At minimum, CPU count should be collected on all platforms
	if _, exists := client.getMetric("system.cpu.count"); !exists {
		t.Error("Expected 'system.cpu.count' metric was not collected")
	}
	
	if cpuCount, exists := client.getMetric("system.cpu.count"); exists {
		if cpuCount != float64(runtime.NumCPU()) {
			t.Errorf("Expected %d CPUs, got %f", runtime.NumCPU(), cpuCount)
		}
	}
}

func TestCustomPerformanceCounterCollector(t *testing.T) {
	client := newMockTelemetryClientForPC()
	
	// Create a custom collector
	customCollector := NewCustomPerformanceCounterCollector("Test Collector", func() map[string]float64 {
		return map[string]float64{
			"custom.metric1": 100.0,
			"custom.metric2": 200.0,
		}
	})
	
	// Test collector name
	if customCollector.Name() != "Test Collector" {
		t.Errorf("Expected collector name 'Test Collector', got '%s'", customCollector.Name())
	}
	
	// Collect metrics
	customCollector.Collect(client)
	
	// Verify custom metrics were collected
	if value, exists := client.getMetric("custom.metric1"); !exists || value != 100.0 {
		t.Errorf("Expected custom.metric1 = 100.0, got %f (exists: %t)", value, exists)
	}
	
	if value, exists := client.getMetric("custom.metric2"); !exists || value != 200.0 {
		t.Errorf("Expected custom.metric2 = 200.0, got %f (exists: %t)", value, exists)
	}
}

func TestCustomPerformanceCounterCollector_NilCollector(t *testing.T) {
	client := newMockTelemetryClientForPC()
	
	// Create a custom collector with nil function
	customCollector := NewCustomPerformanceCounterCollector("Nil Collector", nil)
	
	// This should not panic and should not collect any metrics
	customCollector.Collect(client)
	
	metrics := client.getMetrics()
	if len(metrics) != 0 {
		t.Errorf("Expected no metrics from nil collector, got %d metrics", len(metrics))
	}
}

func TestPerformanceCounterConfig(t *testing.T) {
	// Test default configuration
	config := PerformanceCounterConfig{
		Enabled:              true,
		EnableSystemMetrics:  true,
		EnableRuntimeMetrics: true,
	}
	
	if !config.Enabled {
		t.Error("Expected config to be enabled")
	}
	
	if !config.EnableSystemMetrics {
		t.Error("Expected system metrics to be enabled")
	}
	
	if !config.EnableRuntimeMetrics {
		t.Error("Expected runtime metrics to be enabled")
	}
}

func TestPerformanceCounterManager_Creation(t *testing.T) {
	client := newMockTelemetryClientForPC()
	
	config := PerformanceCounterConfig{
		Enabled:              true,
		CollectionInterval:   5 * time.Second,
		EnableSystemMetrics:  true,
		EnableRuntimeMetrics: true,
	}
	
	manager := NewPerformanceCounterManager(client, config)
	
	if manager == nil {
		t.Fatal("Expected non-nil performance counter manager")
	}
	
	if manager.config.CollectionInterval != 5*time.Second {
		t.Errorf("Expected collection interval 5s, got %v", manager.config.CollectionInterval)
	}
}

func TestPerformanceCounterManager_DefaultInterval(t *testing.T) {
	client := newMockTelemetryClientForPC()
	
	config := PerformanceCounterConfig{
		Enabled:              true,
		EnableSystemMetrics:  true,
		EnableRuntimeMetrics: true,
		// CollectionInterval not set - should default to 60 seconds
	}
	
	manager := NewPerformanceCounterManager(client, config)
	
	if manager.config.CollectionInterval != 60*time.Second {
		t.Errorf("Expected default collection interval 60s, got %v", manager.config.CollectionInterval)
	}
}

func TestPerformanceCounterManager_CustomCollectors(t *testing.T) {
	client := newMockTelemetryClientForPC()
	
	customCollector := NewCustomPerformanceCounterCollector("Test", func() map[string]float64 {
		return map[string]float64{"test.metric": 123.0}
	})
	
	config := PerformanceCounterConfig{
		Enabled:              true,
		CollectionInterval:   100 * time.Millisecond,
		EnableSystemMetrics:  false,
		EnableRuntimeMetrics: false,
		CustomCollectors:     []PerformanceCounterCollector{customCollector},
	}
	
	manager := NewPerformanceCounterManager(client, config)
	
	// Start collection
	manager.Start()
	defer manager.Stop()
	
	// Wait for at least one collection cycle
	time.Sleep(150 * time.Millisecond)
	
	// Verify custom metric was collected
	if value, exists := client.getMetric("test.metric"); !exists || value != 123.0 {
		t.Errorf("Expected test.metric = 123.0, got %f (exists: %t)", value, exists)
	}
}

func TestPerformanceCounterManager_StartStop(t *testing.T) {
	client := newMockTelemetryClientForPC()
	
	config := PerformanceCounterConfig{
		Enabled:              true,
		CollectionInterval:   50 * time.Millisecond,
		EnableSystemMetrics:  false,
		EnableRuntimeMetrics: true,
		CustomCollectors:     nil,
	}
	
	manager := NewPerformanceCounterManager(client, config)
	
	// Start collection
	manager.Start()
	
	// Wait for collection to happen
	time.Sleep(100 * time.Millisecond)
	
	// Verify some metrics were collected
	metrics := client.getMetrics()
	if len(metrics) == 0 {
		t.Error("Expected some metrics to be collected")
	}
	
	// Stop collection
	manager.Stop()
	
	// Clear metrics and wait a bit more
	client.clearMetrics()
	time.Sleep(100 * time.Millisecond)
	
	// Should not collect new metrics after stop
	metricsAfterStop := client.getMetrics()
	if len(metricsAfterStop) != 0 {
		t.Errorf("Expected no new metrics after stop, got %d metrics", len(metricsAfterStop))
	}
}

func TestPerformanceCounterManager_DisabledConfig(t *testing.T) {
	client := newMockTelemetryClientForPC()
	
	config := PerformanceCounterConfig{
		Enabled:              false, // Disabled
		CollectionInterval:   50 * time.Millisecond,
		EnableSystemMetrics:  true,
		EnableRuntimeMetrics: true,
	}
	
	manager := NewPerformanceCounterManager(client, config)
	
	// Start should not actually start collection when disabled
	manager.Start()
	
	// Wait for what would be collection time
	time.Sleep(100 * time.Millisecond)
	
	// Should not collect any metrics
	metrics := client.getMetrics()
	if len(metrics) != 0 {
		t.Errorf("Expected no metrics when disabled, got %d metrics", len(metrics))
	}
	
	manager.Stop()
}

func TestTelemetryClient_PerformanceCounterMethods(t *testing.T) {
	client := NewTelemetryClient("test-key")
	
	// Initially, performance counter collection should be disabled
	if client.IsPerformanceCounterCollectionEnabled() {
		t.Error("Expected performance counter collection to be initially disabled")
	}
	
	// Start performance counter collection
	config := PerformanceCounterConfig{
		Enabled:              true,
		CollectionInterval:   100 * time.Millisecond,
		EnableSystemMetrics:  true,
		EnableRuntimeMetrics: true,
	}
	
	client.StartPerformanceCounterCollection(config)
	
	// Now it should be enabled
	if !client.IsPerformanceCounterCollectionEnabled() {
		t.Error("Expected performance counter collection to be enabled after start")
	}
	
	// Stop performance counter collection
	client.StopPerformanceCounterCollection()
	
	// Should be disabled again
	if client.IsPerformanceCounterCollectionEnabled() {
		t.Error("Expected performance counter collection to be disabled after stop")
	}
}

func TestTelemetryClient_PerformanceCounterRestart(t *testing.T) {
	client := NewTelemetryClient("test-key")
	
	config1 := PerformanceCounterConfig{
		Enabled:              true,
		CollectionInterval:   100 * time.Millisecond,
		EnableSystemMetrics:  true,
		EnableRuntimeMetrics: false,
	}
	
	// Start first configuration
	client.StartPerformanceCounterCollection(config1)
	if !client.IsPerformanceCounterCollectionEnabled() {
		t.Error("Expected performance counter collection to be enabled")
	}
	
	config2 := PerformanceCounterConfig{
		Enabled:              true,
		CollectionInterval:   200 * time.Millisecond,
		EnableSystemMetrics:  false,
		EnableRuntimeMetrics: true,
	}
	
	// Start second configuration (should replace first)
	client.StartPerformanceCounterCollection(config2)
	if !client.IsPerformanceCounterCollectionEnabled() {
		t.Error("Expected performance counter collection to remain enabled")
	}
	
	// Clean up
	client.StopPerformanceCounterCollection()
}