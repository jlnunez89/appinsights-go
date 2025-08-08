package main

import (
	"fmt"
	"log"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
)

const (
	// memoryAllocationSize is the size of memory allocation for simulation (1MB)
	memoryAllocationSize = 1024 * 1024
)

func main() {
	// Create a telemetry client with your instrumentation key
	client := appinsights.NewTelemetryClient("your-instrumentation-key")

	// Configure performance counter collection
	config := appinsights.PerformanceCounterConfig{
		Enabled:              true,
		CollectionInterval:   5 * time.Second, // Collect every 5 seconds
		EnableSystemMetrics:  true,            // Collect CPU, memory, disk metrics
		EnableRuntimeMetrics: true,            // Collect Go runtime metrics
	}

	// Add a custom performance counter
	customCollector := appinsights.NewCustomPerformanceCounterCollector(
		"Application Metrics",
		func() map[string]float64 {
			// This function is called every collection interval
			// Return custom metrics relevant to your application
			return map[string]float64{
				"custom.active_connections": 42.0,
				"custom.requests_per_second": 150.5,
				"custom.cache_hit_ratio":     0.85,
			}
		},
	)
	config.CustomCollectors = []appinsights.PerformanceCounterCollector{customCollector}

	// Start performance counter collection
	client.StartPerformanceCounterCollection(config)

	fmt.Println("Performance counter collection started!")
	fmt.Println("The following metrics will be collected every 5 seconds:")
	fmt.Println()

	fmt.Println("System Metrics:")
	fmt.Println("  - system.cpu.count")
	fmt.Println("  - system.cpu.usage_percent (Linux only)")
	fmt.Println("  - system.memory.total (Linux only)")
	fmt.Println("  - system.memory.free (Linux only)")
	fmt.Println("  - system.memory.usage_percent (Linux only)")
	fmt.Println("  - system.disk.[device].reads (Linux only)")
	fmt.Println("  - system.disk.[device].writes (Linux only)")
	fmt.Println()

	fmt.Println("Go Runtime Metrics:")
	fmt.Println("  - runtime.memory.alloc")
	fmt.Println("  - runtime.memory.total_alloc")
	fmt.Println("  - runtime.memory.sys")
	fmt.Println("  - runtime.memory.heap_alloc")
	fmt.Println("  - runtime.memory.heap_sys")
	fmt.Println("  - runtime.gc.num_gc")
	fmt.Println("  - runtime.gc.pause_total_ns")
	fmt.Println("  - runtime.goroutines")
	fmt.Println("  - runtime.num_cpu")
	fmt.Println()

	fmt.Println("Custom Metrics:")
	fmt.Println("  - custom.active_connections")
	fmt.Println("  - custom.requests_per_second")
	fmt.Println("  - custom.cache_hit_ratio")
	fmt.Println()

	// Simulate some application work
	fmt.Println("Simulating application work for 20 seconds...")
	
	// Create some goroutines to simulate load
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 1000; j++ {
				// Simulate work that affects memory allocation
				data := make([]byte, memoryAllocationSize) // 1MB allocation
				_ = data
				time.Sleep(100 * time.Millisecond)
			}
		}(i)
	}

	// Let the application run for a while to collect metrics
	time.Sleep(20 * time.Second)

	// Stop performance counter collection
	client.StopPerformanceCounterCollection()
	fmt.Println("Performance counter collection stopped!")

	// Gracefully shutdown the telemetry client
	select {
	case <-client.Channel().Close(5 * time.Second):
		fmt.Println("All telemetry data has been sent successfully!")
	case <-time.After(10 * time.Second):
		fmt.Println("Timeout waiting for telemetry to be sent")
	}
}

// Example of how to create a more sophisticated custom collector
func createDatabaseMetricsCollector() appinsights.PerformanceCounterCollector {
	return appinsights.NewCustomPerformanceCounterCollector(
		"Database Metrics",
		func() map[string]float64 {
			// In a real application, you would query your database
			// connection pool or monitoring system here
			return map[string]float64{
				"database.connections.active": 15.0,
				"database.connections.idle":   5.0,
				"database.query.avg_time_ms":  25.7,
				"database.transactions.rate":  10.2,
			}
		},
	)
}

// Example of how to create an HTTP server metrics collector
func createHTTPServerMetricsCollector() appinsights.PerformanceCounterCollector {
	return appinsights.NewCustomPerformanceCounterCollector(
		"HTTP Server Metrics",
		func() map[string]float64 {
			// In a real application, you would get these from your
			// HTTP server middleware or monitoring system
			return map[string]float64{
				"http.requests.total":       1245.0,
				"http.requests.rate":        42.3,
				"http.response.2xx":         1200.0,
				"http.response.4xx":         30.0,
				"http.response.5xx":         15.0,
				"http.response.avg_time_ms": 125.5,
			}
		},
	)
}

// Example configuration for production use
func createProductionConfig() appinsights.PerformanceCounterConfig {
	return appinsights.PerformanceCounterConfig{
		Enabled:              true,
		CollectionInterval:   60 * time.Second, // Collect every minute in production
		EnableSystemMetrics:  true,
		EnableRuntimeMetrics: true,
		CustomCollectors: []appinsights.PerformanceCounterCollector{
			createDatabaseMetricsCollector(),
			createHTTPServerMetricsCollector(),
		},
	}
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}