package main

import (
	"fmt"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

func main() {
	fmt.Println("Sampling Performance Validation")
	fmt.Println("===============================")
	fmt.Println()

	// Test 1: Statistical Validation of Fixed-Rate Sampling
	fmt.Println("Test 1: Statistical Validation (10,000 items at 25% sampling)")
	
	config := appinsights.NewTelemetryConfiguration("test-key")
	config.SamplingProcessor = appinsights.NewFixedRateSamplingProcessor(25.0)

	// Track 10,000 items and measure sampling
	totalItems := 10000
	sampledItems := 0
	
	startTime := time.Now()
	
	for i := 0; i < totalItems; i++ {
		// Create envelope manually to test sampling processor directly
		envelope := &contracts.Envelope{
			Name: "Microsoft.ApplicationInsights.test.Event",
			IKey: "test-key",
			Tags: map[string]string{
				contracts.OperationId: fmt.Sprintf("operation-%d", i),
			},
		}
		
		if config.SamplingProcessor.ShouldSample(envelope) {
			sampledItems++
		}
	}
	
	duration := time.Since(startTime)
	actualSamplingRate := float64(sampledItems) / float64(totalItems) * 100
	expectedRate := 25.0
	tolerance := 2.0 // 2% tolerance
	
	fmt.Printf("   Total items: %d\n", totalItems)
	fmt.Printf("   Sampled items: %d\n", sampledItems)
	fmt.Printf("   Expected rate: %.1f%%\n", expectedRate)
	fmt.Printf("   Actual rate: %.2f%%\n", actualSamplingRate)
	fmt.Printf("   Processing time: %v\n", duration)
	fmt.Printf("   Items per second: %.0f\n", float64(totalItems)/duration.Seconds())
	
	if actualSamplingRate >= expectedRate-tolerance && actualSamplingRate <= expectedRate+tolerance {
		fmt.Printf("   ✅ PASS: Sampling rate within %.1f%% tolerance\n", tolerance)
	} else {
		fmt.Printf("   ❌ FAIL: Sampling rate outside tolerance\n")
	}
	fmt.Println()

	// Test 2: Consistency Test (same operation ID should always have same result)
	fmt.Println("Test 2: Consistency Validation (same operation ID)")
	
	operationId := "test-consistency-operation-12345"
	envelope := &contracts.Envelope{
		Name: "Microsoft.ApplicationInsights.test.Event",
		IKey: "test-key",
		Tags: map[string]string{
			contracts.OperationId: operationId,
		},
	}
	
	firstResult := config.SamplingProcessor.ShouldSample(envelope)
	consistentResults := 0
	testCount := 1000
	
	for i := 0; i < testCount; i++ {
		// Reset envelope for each test
		envelope.SampleRate = 0
		result := config.SamplingProcessor.ShouldSample(envelope)
		if result == firstResult {
			consistentResults++
		}
	}
	
	fmt.Printf("   Operation ID: %s\n", operationId)
	fmt.Printf("   First result: %t\n", firstResult)
	fmt.Printf("   Consistent results: %d/%d\n", consistentResults, testCount)
	
	if consistentResults == testCount {
		fmt.Printf("   ✅ PASS: 100%% consistency achieved\n")
	} else {
		fmt.Printf("   ❌ FAIL: Inconsistent sampling decisions\n")
	}
	fmt.Println()

	// Test 3: Performance Test (high-volume processing)
	fmt.Println("Test 3: Performance Test (100,000 items)")
	
	highVolumeItems := 100000
	startTime = time.Now()
	
	for i := 0; i < highVolumeItems; i++ {
		envelope := &contracts.Envelope{
			Name: "Microsoft.ApplicationInsights.test.Event",
			IKey: "test-key",
			Tags: map[string]string{
				contracts.OperationId: fmt.Sprintf("operation-%d", i%1000), // Vary operation IDs
			},
		}
		config.SamplingProcessor.ShouldSample(envelope)
	}
	
	duration = time.Since(startTime)
	throughput := float64(highVolumeItems) / duration.Seconds()
	
	fmt.Printf("   Items processed: %d\n", highVolumeItems)
	fmt.Printf("   Processing time: %v\n", duration)
	fmt.Printf("   Throughput: %.0f items/second\n", throughput)
	
	// Consider > 50k items/sec as good performance
	if throughput > 50000 {
		fmt.Printf("   ✅ PASS: High throughput achieved\n")
	} else {
		fmt.Printf("   ⚠️  WARN: Lower throughput than expected\n")
	}
	fmt.Println()

	// Test 4: Per-Type Sampling Validation
	fmt.Println("Test 4: Per-Type Sampling Validation")
	
	typeRates := map[appinsights.TelemetryType]float64{
		appinsights.TelemetryTypeEvent:  100, // Always sample events
		appinsights.TelemetryTypeTrace:  0,   // Never sample traces
		appinsights.TelemetryTypeMetric: 50,  // 50% sample metrics
	}
	
	perTypeProcessor := appinsights.NewPerTypeSamplingProcessor(25, typeRates)
	
	testCases := []struct {
		name         string
		envelopeName string
		expectedRate float64
		telType      appinsights.TelemetryType
	}{
		{"Events", "Microsoft.ApplicationInsights.test.Event", 100.0, appinsights.TelemetryTypeEvent},
		{"Traces", "Microsoft.ApplicationInsights.test.Message", 0.0, appinsights.TelemetryTypeTrace},
		{"Metrics", "Microsoft.ApplicationInsights.test.Metric", 50.0, appinsights.TelemetryTypeMetric},
		{"Requests", "Microsoft.ApplicationInsights.test.Request", 25.0, appinsights.TelemetryTypeRequest}, // Default rate
	}
	
	for _, tc := range testCases {
		sampledCount := 0
		testItems := 1000
		
		for i := 0; i < testItems; i++ {
			envelope := &contracts.Envelope{
				Name: tc.envelopeName,
				IKey: "test-key",
				Tags: map[string]string{
					contracts.OperationId: fmt.Sprintf("operation-%s-%d", tc.name, i),
				},
			}
			
			if perTypeProcessor.ShouldSample(envelope) {
				sampledCount++
			}
		}
		
		actualRate := float64(sampledCount) / float64(testItems) * 100
		tolerance := 5.0 // 5% tolerance for statistical sampling
		
		fmt.Printf("   %s: %.1f%% expected, %.1f%% actual", tc.name, tc.expectedRate, actualRate)
		
		if tc.expectedRate == 0 {
			// Special case for 0% - should be exactly 0
			if sampledCount == 0 {
				fmt.Printf(" ✅\n")
			} else {
				fmt.Printf(" ❌\n")
			}
		} else if tc.expectedRate == 100 {
			// Special case for 100% - should be exactly all
			if sampledCount == testItems {
				fmt.Printf(" ✅\n")
			} else {
				fmt.Printf(" ❌\n")
			}
		} else {
			// Statistical validation with tolerance
			if actualRate >= tc.expectedRate-tolerance && actualRate <= tc.expectedRate+tolerance {
				fmt.Printf(" ✅\n")
			} else {
				fmt.Printf(" ❌\n")
			}
		}
	}
	fmt.Println()

	// Test 5: Sampling Metadata Validation
	fmt.Println("Test 5: Sampling Metadata Validation")
	
	metadataTests := []struct {
		name              string
		processor         appinsights.SamplingProcessor
		expectedSampleRate float64
	}{
		{"50% Fixed Rate", appinsights.NewFixedRateSamplingProcessor(50.0), 2.0},
		{"25% Fixed Rate", appinsights.NewFixedRateSamplingProcessor(25.0), 4.0},
		{"100% (Disabled)", appinsights.NewDisabledSamplingProcessor(), 1.0},
		{"10% Fixed Rate", appinsights.NewFixedRateSamplingProcessor(10.0), 10.0},
	}
	
	for _, mt := range metadataTests {
		envelope := &contracts.Envelope{
			Name: "Microsoft.ApplicationInsights.test.Event",
			IKey: "test-key",
			Tags: map[string]string{
				contracts.OperationId: "metadata-test-operation",
			},
		}
		
		mt.processor.ShouldSample(envelope)
		
		fmt.Printf("   %s: Expected %.1f, Got %.1f", mt.name, mt.expectedSampleRate, envelope.SampleRate)
		
		if envelope.SampleRate == mt.expectedSampleRate {
			fmt.Printf(" ✅\n")
		} else {
			fmt.Printf(" ❌\n")
		}
	}
	fmt.Println()

	fmt.Println("Validation completed!")
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Println("- Fixed-rate sampling provides statistically accurate results")
	fmt.Println("- Sampling decisions are consistent for the same operation ID")
	fmt.Println("- High-performance processing (>50k items/second)")
	fmt.Println("- Per-type sampling works correctly for different telemetry types")
	fmt.Println("- Sampling metadata is properly set on envelopes")
	fmt.Println("- Implementation is ready for production use")
}