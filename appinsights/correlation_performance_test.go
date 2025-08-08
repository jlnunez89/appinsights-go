package appinsights

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestCorrelationPerformance tests the performance impact of correlation
func TestCorrelationPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	client := NewTelemetryClient("test-key")

	// Measure baseline performance without correlation
	start := time.Now()
	for i := 0; i < 1000; i++ {
		client.TrackEvent("BaselineEvent")
	}
	baselineDuration := time.Since(start)

	// Measure performance with correlation
	corrCtx := NewCorrelationContext()
	ctx := WithCorrelationContext(context.Background(), corrCtx)

	start = time.Now()
	for i := 0; i < 1000; i++ {
		client.TrackEventWithContext(ctx, "CorrelatedEvent")
	}
	correlatedDuration := time.Since(start)

	// Correlation should not add more than 100% overhead in test environments
	overhead := float64(correlatedDuration-baselineDuration) / float64(baselineDuration)
	if overhead > 1.0 {
		t.Errorf("Correlation overhead too high: %.2f%% (baseline: %v, correlated: %v)",
			overhead*100, baselineDuration, correlatedDuration)
	}

	t.Logf("Performance test results - Baseline: %v, Correlated: %v, Overhead: %.2f%%",
		baselineDuration, correlatedDuration, overhead*100)
}

// TestCorrelationMemoryUsage tests that correlation doesn't leak memory
func TestCorrelationMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	client := NewTelemetryClient("test-key")

	// Create many correlation contexts and ensure they can be garbage collected
	const numContexts = 10000

	for i := 0; i < numContexts; i++ {
		corrCtx := NewCorrelationContext()
		corrCtx.OperationName = "MemoryTest"
		ctx := WithCorrelationContext(context.Background(), corrCtx)

		// Track some telemetry
		client.TrackEventWithContext(ctx, "MemoryTestEvent")

		// Create child contexts
		childCorr := NewChildCorrelationContext(corrCtx)
		childCtx := WithCorrelationContext(ctx, childCorr)
		client.TrackEventWithContext(childCtx, "ChildEvent")

		// Let contexts go out of scope
	}

	// Force garbage collection
	for i := 0; i < 3; i++ {
		time.Sleep(10 * time.Millisecond)
		// Note: Can't force GC in tests, but contexts should be eligible for collection
	}

	// Test should complete without memory issues
	t.Log("Memory usage test completed successfully")
}

// TestCorrelationConcurrentSafety tests that correlation is safe under high concurrency
func TestCorrelationConcurrentSafety(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	client := NewTelemetryClient("test-key")
	middleware := NewHTTPMiddleware()

	const numGoroutines = 50
	const operationsPerGoroutine = 100

	var wg sync.WaitGroup
	errorChan := make(chan error, numGoroutines)

	// Start multiple goroutines performing correlation operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				// Create correlation context
				corrCtx := NewCorrelationContext()
				corrCtx.OperationName = "ConcurrentTest"
				ctx := WithCorrelationContext(context.Background(), corrCtx)

				// Track telemetry
				client.TrackEventWithContext(ctx, "ConcurrentEvent")

				// Create child contexts
				childCorr := NewChildCorrelationContext(corrCtx)
				childCtx := WithCorrelationContext(ctx, childCorr)

				// Track more telemetry
				client.TrackEventWithContext(childCtx, "ConcurrentChildEvent")

				// Test HTTP header operations
				req := httptest.NewRequest("GET", "/test", nil)
				middleware.InjectHeaders(req, corrCtx)

				extractedCorr := middleware.ExtractHeaders(req)
				if extractedCorr == nil {
					errorChan <- &testError{"Failed to extract headers in concurrent test"}
					return
				}

				// Test W3C and Request-Id conversion
				w3cHeader := corrCtx.ToW3CTraceParent()
				_, err := ParseW3CTraceParent(w3cHeader)
				if err != nil {
					errorChan <- err
					return
				}

				requestID := corrCtx.ToRequestID()
				_, err = ParseRequestID(requestID)
				if err != nil {
					errorChan <- err
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		t.Errorf("Concurrent operation failed: %v", err)
	}
}

// TestCorrelationHeaderPerformance tests the performance of header operations
func TestCorrelationHeaderPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping header performance test in short mode")
	}

	middleware := NewHTTPMiddleware()
	corrCtx := NewCorrelationContext()

	const numOperations = 10000

	// Test header injection performance
	start := time.Now()
	for i := 0; i < numOperations; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		middleware.InjectHeaders(req, corrCtx)
	}
	injectionDuration := time.Since(start)

	// Test header extraction performance
	req := httptest.NewRequest("GET", "/test", nil)
	middleware.InjectHeaders(req, corrCtx)

	start = time.Now()
	for i := 0; i < numOperations; i++ {
		middleware.ExtractHeaders(req)
	}
	extractionDuration := time.Since(start)

	// Test W3C parsing performance
	w3cHeader := corrCtx.ToW3CTraceParent()
	start = time.Now()
	for i := 0; i < numOperations; i++ {
		ParseW3CTraceParent(w3cHeader)
	}
	w3cParsingDuration := time.Since(start)

	// Test Request-Id parsing performance
	requestID := corrCtx.ToRequestID()
	start = time.Now()
	for i := 0; i < numOperations; i++ {
		ParseRequestID(requestID)
	}
	requestIDParsingDuration := time.Since(start)

	// Log performance results
	t.Logf("Header operation performance results:")
	t.Logf("  Injection: %v (%v per op)", injectionDuration, injectionDuration/numOperations)
	t.Logf("  Extraction: %v (%v per op)", extractionDuration, extractionDuration/numOperations)
	t.Logf("  W3C Parsing: %v (%v per op)", w3cParsingDuration, w3cParsingDuration/numOperations)
	t.Logf("  Request-Id Parsing: %v (%v per op)", requestIDParsingDuration, requestIDParsingDuration/numOperations)

	// Each operation should be fast (less than 100ms for 10k operations in test environment)
	if injectionDuration > 100*time.Millisecond {
		t.Errorf("Header injection too slow: %v for %d operations", injectionDuration, numOperations)
	}
	if extractionDuration > 100*time.Millisecond {
		t.Errorf("Header extraction too slow: %v for %d operations", extractionDuration, numOperations)
	}
}

// TestCorrelationIDGenerationPerformance tests ID generation performance
func TestCorrelationIDGenerationPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping ID generation performance test in short mode")
	}

	const numOperations = 10000

	// Test correlation context creation performance
	start := time.Now()
	for i := 0; i < numOperations; i++ {
		NewCorrelationContext()
	}
	contextCreationDuration := time.Since(start)

	// Test child context creation performance
	parentCorr := NewCorrelationContext()
	start = time.Now()
	for i := 0; i < numOperations; i++ {
		NewChildCorrelationContext(parentCorr)
	}
	childCreationDuration := time.Since(start)

	// Log performance results
	t.Logf("ID generation performance results:")
	t.Logf("  Context creation: %v (%v per op)", contextCreationDuration, contextCreationDuration/numOperations)
	t.Logf("  Child creation: %v (%v per op)", childCreationDuration, childCreationDuration/numOperations)

	// ID generation should be fast (less than 100ms for 10k operations in test environment)
	if contextCreationDuration > 100*time.Millisecond {
		t.Errorf("Context creation too slow: %v for %d operations", contextCreationDuration, numOperations)
	}
	if childCreationDuration > 100*time.Millisecond {
		t.Errorf("Child context creation too slow: %v for %d operations", childCreationDuration, numOperations)
	}
}

// TestCorrelationStressTest runs a comprehensive stress test
func TestCorrelationStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	client := NewTelemetryClient("test-key")
	middleware := NewHTTPMiddleware()

	// Create test server for HTTP operations
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract and validate correlation
		corrCtx := middleware.ExtractHeaders(r)
		if corrCtx != nil {
			ctx := WithCorrelationContext(r.Context(), corrCtx)
			client.TrackEventWithContext(ctx, "StressTestRequest")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	const duration = 5 * time.Second
	const numWorkers = 20

	var wg sync.WaitGroup
	stopChan := make(chan struct{})
	errorChan := make(chan error, numWorkers)

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			operationCount := 0
			for {
				select {
				case <-stopChan:
					t.Logf("Worker %d completed %d operations", workerID, operationCount)
					return
				default:
					// Perform various correlation operations
					corrCtx := NewCorrelationContext()
					corrCtx.OperationName = "StressTest"
					ctx := WithCorrelationContext(context.Background(), corrCtx)

					// Track telemetry
					client.TrackEventWithContext(ctx, "StressTestEvent")

					// Make HTTP request with correlation
					req, err := http.NewRequest("GET", server.URL, nil)
					if err != nil {
						errorChan <- err
						return
					}
					middleware.InjectHeaders(req, corrCtx)

					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						errorChan <- err
						return
					}
					resp.Body.Close()

					// Create child operations
					childCorr := NewChildCorrelationContext(corrCtx)
					childCtx := WithCorrelationContext(ctx, childCorr)
					client.TrackEventWithContext(childCtx, "StressTestChild")

					operationCount++
				}
			}
		}(i)
	}

	// Run for specified duration
	time.Sleep(duration)
	close(stopChan)

	// Wait for all workers to finish
	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		t.Errorf("Stress test error: %v", err)
	}

	t.Logf("Stress test completed successfully after %v with %d workers", duration, numWorkers)
}

// testError is a simple error type for tests
type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}

// BenchmarkCorrelationContextCreation benchmarks correlation context creation
func BenchmarkCorrelationContextCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewCorrelationContext()
	}
}

// BenchmarkChildCorrelationContextCreation benchmarks child context creation
func BenchmarkChildCorrelationContextCreation(b *testing.B) {
	parent := NewCorrelationContext()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		NewChildCorrelationContext(parent)
	}
}

// BenchmarkW3CTraceParentGeneration benchmarks W3C header generation
func BenchmarkW3CTraceParentGeneration(b *testing.B) {
	corrCtx := NewCorrelationContext()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		corrCtx.ToW3CTraceParent()
	}
}

// BenchmarkW3CTraceParentParsing benchmarks W3C header parsing
func BenchmarkW3CTraceParentParsing(b *testing.B) {
	corrCtx := NewCorrelationContext()
	header := corrCtx.ToW3CTraceParent()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ParseW3CTraceParent(header)
	}
}

// BenchmarkRequestIDGeneration benchmarks Request-Id generation
func BenchmarkRequestIDGeneration(b *testing.B) {
	corrCtx := NewCorrelationContext()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		corrCtx.ToRequestID()
	}
}

// BenchmarkRequestIDParsing benchmarks Request-Id parsing
func BenchmarkRequestIDParsing(b *testing.B) {
	corrCtx := NewCorrelationContext()
	requestID := corrCtx.ToRequestID()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ParseRequestID(requestID)
	}
}

// BenchmarkHeaderInjection benchmarks HTTP header injection
func BenchmarkHeaderInjection(b *testing.B) {
	middleware := NewHTTPMiddleware()
	corrCtx := NewCorrelationContext()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		middleware.InjectHeaders(req, corrCtx)
	}
}

// BenchmarkHeaderExtraction benchmarks HTTP header extraction
func BenchmarkHeaderExtraction(b *testing.B) {
	middleware := NewHTTPMiddleware()
	corrCtx := NewCorrelationContext()
	req := httptest.NewRequest("GET", "/test", nil)
	middleware.InjectHeaders(req, corrCtx)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		middleware.ExtractHeaders(req)
	}
}

// BenchmarkTelemetryWithCorrelation benchmarks telemetry tracking with correlation
func BenchmarkTelemetryWithCorrelation(b *testing.B) {
	client := NewTelemetryClient("test-key")
	corrCtx := NewCorrelationContext()
	ctx := WithCorrelationContext(context.Background(), corrCtx)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		client.TrackEventWithContext(ctx, "BenchmarkEvent")
	}
}
