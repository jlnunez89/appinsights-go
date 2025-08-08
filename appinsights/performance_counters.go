package appinsights

import (
	"context"
	"runtime"
	"sync"
	"time"
)

const (
	// gcPauseCircularBufferSize is the size of the circular buffer for GC pause times
	gcPauseCircularBufferSize = 256
	// gcPauseIndexMask is used to calculate the index in the circular buffer for the most recent GC pause
	gcPauseIndexMask = 255
)

// PerformanceCounterCollector represents a collector that gathers performance metrics
type PerformanceCounterCollector interface {
	// Collect gathers metrics and sends them via the provided telemetry client
	Collect(client TelemetryClient)
	
	// Name returns the name of this collector
	Name() string
}

// PerformanceCounterConfig configures performance counter collection
type PerformanceCounterConfig struct {
	// Enabled controls whether performance counter collection is active
	Enabled bool
	
	// CollectionInterval specifies how often to collect performance counters
	CollectionInterval time.Duration
	
	// EnableSystemMetrics controls collection of CPU, memory, and disk metrics
	EnableSystemMetrics bool
	
	// EnableRuntimeMetrics controls collection of Go runtime metrics
	EnableRuntimeMetrics bool
	
	// CustomCollectors allows registration of custom performance counter collectors
	CustomCollectors []PerformanceCounterCollector
}

// PerformanceCounterManager manages periodic collection of performance counters
type PerformanceCounterManager struct {
	config    PerformanceCounterConfig
	client    TelemetryClient
	collectors []PerformanceCounterCollector
	
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	mu sync.RWMutex
}

// NewPerformanceCounterManager creates a new performance counter manager
func NewPerformanceCounterManager(client TelemetryClient, config PerformanceCounterConfig) *PerformanceCounterManager {
	if config.CollectionInterval == 0 {
		config.CollectionInterval = 60 * time.Second // Default to 1 minute
	}
	
	pcm := &PerformanceCounterManager{
		config: config,
		client: client,
	}
	
	pcm.setupCollectors()
	return pcm
}

// setupCollectors initializes the built-in collectors based on configuration
func (pcm *PerformanceCounterManager) setupCollectors() {
	pcm.collectors = make([]PerformanceCounterCollector, 0)
	
	if pcm.config.EnableSystemMetrics {
		pcm.collectors = append(pcm.collectors, NewSystemMetricsCollector())
	}
	
	if pcm.config.EnableRuntimeMetrics {
		pcm.collectors = append(pcm.collectors, NewRuntimeMetricsCollector())
	}
	
	// Add custom collectors
	pcm.collectors = append(pcm.collectors, pcm.config.CustomCollectors...)
}

// Start begins periodic collection of performance counters
func (pcm *PerformanceCounterManager) Start() {
	pcm.mu.Lock()
	defer pcm.mu.Unlock()
	
	if !pcm.config.Enabled || pcm.cancel != nil {
		return // Not enabled or already running
	}
	
	pcm.ctx, pcm.cancel = context.WithCancel(context.Background())
	
	pcm.wg.Add(1)
	go pcm.collectLoop()
}

// Stop halts performance counter collection
func (pcm *PerformanceCounterManager) Stop() {
	pcm.mu.Lock()
	cancel := pcm.cancel
	pcm.cancel = nil
	pcm.mu.Unlock()
	
	if cancel != nil {
		cancel()
		pcm.wg.Wait()
	}
}

// collectLoop runs the periodic collection of performance counters
func (pcm *PerformanceCounterManager) collectLoop() {
	defer pcm.wg.Done()
	
	ticker := time.NewTicker(pcm.config.CollectionInterval)
	defer ticker.Stop()
	
	// Collect immediately on start
	pcm.collectMetrics()
	
	for {
		select {
		case <-pcm.ctx.Done():
			return
		case <-ticker.C:
			pcm.collectMetrics()
		}
	}
}

// collectMetrics runs all registered collectors
func (pcm *PerformanceCounterManager) collectMetrics() {
	pcm.mu.RLock()
	collectors := pcm.collectors
	pcm.mu.RUnlock()
	
	for _, collector := range collectors {
		collector.Collect(pcm.client)
	}
}

// RuntimeMetricsCollector collects Go runtime metrics
type RuntimeMetricsCollector struct{}

// NewRuntimeMetricsCollector creates a new runtime metrics collector
func NewRuntimeMetricsCollector() *RuntimeMetricsCollector {
	return &RuntimeMetricsCollector{}
}

// Name returns the collector name
func (r *RuntimeMetricsCollector) Name() string {
	return "Runtime Metrics"
}

// Collect gathers Go runtime metrics
func (r *RuntimeMetricsCollector) Collect(client TelemetryClient) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	// Memory metrics
	client.TrackMetric("runtime.memory.alloc", float64(m.Alloc))
	client.TrackMetric("runtime.memory.total_alloc", float64(m.TotalAlloc))
	client.TrackMetric("runtime.memory.sys", float64(m.Sys))
	client.TrackMetric("runtime.memory.heap_alloc", float64(m.HeapAlloc))
	client.TrackMetric("runtime.memory.heap_sys", float64(m.HeapSys))
	client.TrackMetric("runtime.memory.heap_idle", float64(m.HeapIdle))
	client.TrackMetric("runtime.memory.heap_inuse", float64(m.HeapInuse))
	client.TrackMetric("runtime.memory.heap_released", float64(m.HeapReleased))
	client.TrackMetric("runtime.memory.heap_objects", float64(m.HeapObjects))
	client.TrackMetric("runtime.memory.stack_inuse", float64(m.StackInuse))
	client.TrackMetric("runtime.memory.stack_sys", float64(m.StackSys))
	
	// GC metrics
	client.TrackMetric("runtime.gc.num_gc", float64(m.NumGC))
	client.TrackMetric("runtime.gc.pause_total_ns", float64(m.PauseTotalNs))
	client.TrackMetric("runtime.gc.pause_ns", float64(m.PauseNs[(m.NumGC+gcPauseIndexMask)%gcPauseCircularBufferSize]))
	client.TrackMetric("runtime.gc.cpu_fraction", m.GCCPUFraction)
	
	// Goroutine metrics
	client.TrackMetric("runtime.goroutines", float64(runtime.NumGoroutine()))
	client.TrackMetric("runtime.num_cpu", float64(runtime.NumCPU()))
	client.TrackMetric("runtime.cgocall", float64(runtime.NumCgoCall()))
}

// CustomPerformanceCounterCollector allows users to define custom performance counters
type CustomPerformanceCounterCollector struct {
	name      string
	collector func() map[string]float64
}

// NewCustomPerformanceCounterCollector creates a custom performance counter collector
func NewCustomPerformanceCounterCollector(name string, collector func() map[string]float64) *CustomPerformanceCounterCollector {
	return &CustomPerformanceCounterCollector{
		name:      name,
		collector: collector,
	}
}

// Name returns the collector name
func (c *CustomPerformanceCounterCollector) Name() string {
	return c.name
}

// Collect gathers custom metrics
func (c *CustomPerformanceCounterCollector) Collect(client TelemetryClient) {
	if c.collector == nil {
		return
	}
	
	metrics := c.collector()
	for name, value := range metrics {
		client.TrackMetric(name, value)
	}
}