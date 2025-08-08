package main

import (
	"fmt"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

func main() {
	fmt.Println("Basic Sampling Example")
	fmt.Println("======================")
	fmt.Println()

	// Example 1: No Sampling (100% - default behavior)
	fmt.Println("1. No Sampling (100% rate - default)")
	config1 := appinsights.NewTelemetryConfiguration("00000000-0000-0000-0000-000000000000")
	// No SamplingProcessor set - defaults to 100% sampling
	client1 := appinsights.NewTelemetryClientFromConfig(config1)

	client1.TrackEvent("no-sampling-event")
	client1.TrackTrace("No sampling trace", contracts.Information)
	fmt.Println("   Tracked 2 items with 100% sampling rate")
	fmt.Println()

	// Example 2: Fixed-Rate Sampling (50%)
	fmt.Println("2. Fixed-Rate Sampling (50%)")
	config2 := appinsights.NewTelemetryConfiguration("00000000-0000-0000-0000-000000000000")
	config2.SamplingProcessor = appinsights.NewFixedRateSamplingProcessor(50.0)
	client2 := appinsights.NewTelemetryClientFromConfig(config2)

	fmt.Printf("   Sampling rate: %.1f%%\n", config2.SamplingProcessor.GetSamplingRate())

	// Track several items - statistically about half should be sampled
	for i := 0; i < 10; i++ {
		client2.TrackEvent(fmt.Sprintf("fixed-rate-event-%d", i))
	}
	fmt.Println("   Tracked 10 events - approximately 5 should be sampled")
	fmt.Println()

	// Example 3: Per-Type Sampling
	fmt.Println("3. Per-Type Sampling (different rates for different telemetry types)")
	config3 := appinsights.NewTelemetryConfiguration("00000000-0000-0000-0000-000000000000")

	typeRates := map[appinsights.TelemetryType]float64{
		appinsights.TelemetryTypeEvent:   100, // 100% for events (always sample)
		appinsights.TelemetryTypeTrace:   25,  // 25% for traces
		appinsights.TelemetryTypeMetric:  50,  // 50% for metrics
		appinsights.TelemetryTypeRequest: 100, // 100% for requests
	}

	perTypeProcessor := appinsights.NewPerTypeSamplingProcessor(10, typeRates) // 10% default
	config3.SamplingProcessor = perTypeProcessor
	client3 := appinsights.NewTelemetryClientFromConfig(config3)

	fmt.Printf("   Event sampling rate: %.1f%%\n", perTypeProcessor.GetSamplingRateForType(appinsights.TelemetryTypeEvent))
	fmt.Printf("   Trace sampling rate: %.1f%%\n", perTypeProcessor.GetSamplingRateForType(appinsights.TelemetryTypeTrace))
	fmt.Printf("   Metric sampling rate: %.1f%%\n", perTypeProcessor.GetSamplingRateForType(appinsights.TelemetryTypeMetric))
	fmt.Printf("   Request sampling rate: %.1f%%\n", perTypeProcessor.GetSamplingRateForType(appinsights.TelemetryTypeRequest))

	client3.TrackEvent("per-type-event")         // Should always be sampled (100%)
	client3.TrackTrace("per-type-trace", contracts.Information) // 25% chance
	client3.TrackMetric("per-type-metric", 42.0) // 50% chance
	client3.TrackRequest("GET", "/api/test", 100*time.Millisecond, "200") // Should always be sampled (100%)

	fmt.Println("   Tracked mixed telemetry types with different sampling rates")
	fmt.Println()

	// Example 4: Error Priority (demonstrates intelligent sampling behavior)
	fmt.Println("4. Error Priority Sampling")
	config4 := appinsights.NewTelemetryConfiguration("00000000-0000-0000-0000-000000000000")
	config4.SamplingProcessor = appinsights.NewIntelligentSamplingProcessor(10.0) // 10% default rate
	client4 := appinsights.NewTelemetryClientFromConfig(config4)

	fmt.Printf("   Default sampling rate: %.1f%%\n", config4.SamplingProcessor.GetSamplingRate())

	// These should ALWAYS be sampled regardless of the low default rate
	client4.TrackException("Critical system error")
	client4.TrackTrace("Database connection failed", contracts.Error)
	client4.TrackRequest("POST", "/api/payment", 5*time.Second, "500") // Failed request

	// These will be sampled at the 10% default rate
	client4.TrackEvent("regular-user-action")
	client4.TrackTrace("User logged in", contracts.Information)
	client4.TrackRequest("GET", "/api/health", 50*time.Millisecond, "200") // Successful request

	fmt.Println("   Tracked errors (always sampled) and regular telemetry (10% rate)")
	fmt.Println()

	// Example 5: Dependency-Aware Sampling
	fmt.Println("5. Dependency-Aware Sampling (consistent sampling across operations)")
	config5 := appinsights.NewTelemetryConfiguration("00000000-0000-0000-0000-000000000000")
	config5.SamplingProcessor = appinsights.NewFixedRateSamplingProcessor(50.0)
	client5 := appinsights.NewTelemetryClientFromConfig(config5)

	// Create telemetry items with the same operation ID
	operationId := "sample-operation-12345"

	request := appinsights.NewRequestTelemetry("GET", "/api/order", 200*time.Millisecond, "200")
	request.Tags[contracts.OperationId] = operationId

	dependency := appinsights.NewRemoteDependencyTelemetry("database-query", "SQL", "orders-db", true)
	dependency.Tags[contracts.OperationId] = operationId

	event := appinsights.NewEventTelemetry("order-processed")
	event.Tags[contracts.OperationId] = operationId

	// All these related items will have the same sampling decision
	client5.Track(request)
	client5.Track(dependency)
	client5.Track(event)

	fmt.Println("   Tracked correlated telemetry - all items with same operation ID get same sampling decision")
	fmt.Println()

	// Wait a moment for any async processing
	time.Sleep(100 * time.Millisecond)

	// Close all channels gracefully
	client1.Channel().Flush()
	client2.Channel().Flush()
	client3.Channel().Flush()
	client4.Channel().Flush()
	client5.Channel().Flush()

	fmt.Println("Examples completed!")
	fmt.Println()
	fmt.Println("Key Takeaways:")
	fmt.Println("- Sampling reduces telemetry volume while maintaining statistical accuracy")
	fmt.Println("- Different sampling strategies serve different use cases")
	fmt.Println("- Error telemetry is prioritized and typically always sampled")
	fmt.Println("- Related operations are sampled consistently for complete traces")
	fmt.Println("- Sampling metadata is automatically added to telemetry envelopes")
}