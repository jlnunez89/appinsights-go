package main

import (
	"fmt"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

func main() {
	// Create a telemetry configuration with intelligent sampling
	config := appinsights.NewTelemetryConfiguration("00000000-0000-0000-0000-000000000000")

	// Create an intelligent sampling processor with 25% default sampling rate
	intelligentSampler := appinsights.NewIntelligentSamplingProcessor(25.0)

	// Add a custom rule for high-priority business events (100% sampling)
	highPriorityRule := appinsights.NewCustomSamplingRule(
		"high-priority-business",
		800,   // High priority
		100.0, // 100% sampling rate
		func(envelope *contracts.Envelope) bool {
			// Sample all events that contain "critical" or "business" in their name
			if envelope.Data != nil {
				if data, ok := envelope.Data.(*contracts.Data); ok && data.BaseData != nil {
					if eventData, ok := data.BaseData.(*contracts.EventData); ok {
						name := eventData.Name
						return len(name) > 0 && (len(name) > 8 && name[:8] == "critical" ||
							len(name) > 8 && name[:8] == "business")
					}
				}
			}
			return false
		},
	)
	intelligentSampler.AddRule(highPriorityRule)

	// Add a custom rule for debug events (5% sampling)
	debugRule := appinsights.NewCustomSamplingRule(
		"debug-events",
		100, // Lower priority
		5.0, // 5% sampling rate
		func(envelope *contracts.Envelope) bool {
			// Sample debug events at lower rate
			if envelope.Data != nil {
				if data, ok := envelope.Data.(*contracts.Data); ok && data.BaseData != nil {
					if eventData, ok := data.BaseData.(*contracts.EventData); ok {
						name := eventData.Name
						return len(name) > 5 && name[:5] == "debug"
					}
				}
			}
			return false
		},
	)
	intelligentSampler.AddRule(debugRule)

	// Set the intelligent sampling processor
	config.SamplingProcessor = intelligentSampler

	// Create telemetry client
	client := appinsights.NewTelemetryClientFromConfig(config)
	defer func() {
		client.Channel().Flush()
		time.Sleep(time.Second)
	}()

	fmt.Println("Intelligent Sampling Example")
	fmt.Println("============================")
	fmt.Printf("Default sampling rate: %.1f%%\n", intelligentSampler.GetSamplingRate())
	fmt.Println()

	// Demonstrate different types of telemetry and their sampling behavior
	fmt.Println("Tracking various telemetry types...")

	// 1. Exception telemetry - should ALWAYS be sampled (error priority)
	fmt.Println("1. Exception telemetry (should always be sampled)")
	client.TrackException("Critical system error occurred")

	// 2. High-priority business event - should ALWAYS be sampled (custom rule)
	fmt.Println("2. Critical business event (should always be sampled)")
	event := appinsights.NewEventTelemetry("critical-order-processing-failure")
	event.Properties["severity"] = "high"
	event.Properties["impact"] = "revenue"
	client.Track(event)

	// 3. Debug event - should be sampled at 5% rate (custom rule)
	fmt.Println("3. Debug event (should be sampled at 5% rate)")
	debugEvent := appinsights.NewEventTelemetry("debug-user-action")
	debugEvent.Properties["user"] = "test-user"
	client.Track(debugEvent)

	// 4. Regular event - should be sampled at 25% default rate
	fmt.Println("4. Regular event (should be sampled at 25% default rate)")
	regularEvent := appinsights.NewEventTelemetry("user-login")
	regularEvent.Properties["source"] = "web"
	client.Track(regularEvent)

	// 5. Error-level trace - should ALWAYS be sampled (error priority)
	fmt.Println("5. Error trace (should always be sampled)")
	client.TrackTrace("Database connection failed", contracts.Error)

	// 6. Info-level trace - should be sampled at 25% default rate
	fmt.Println("6. Info trace (should be sampled at 25% default rate)")
	client.TrackTrace("User session started", contracts.Information)

	// 7. Failed request - should ALWAYS be sampled (error priority)
	fmt.Println("7. Failed HTTP request (should always be sampled)")
	failedRequest := appinsights.NewRequestTelemetry("GET", "/api/orders", 2*time.Second, "500")
	client.Track(failedRequest)

	// 8. Successful request - should be sampled at 25% default rate
	fmt.Println("8. Successful HTTP request (should be sampled at 25% default rate)")
	successRequest := appinsights.NewRequestTelemetry("GET", "/api/status", 100*time.Millisecond, "200")
	client.Track(successRequest)

	// 9. Failed dependency - should ALWAYS be sampled (error priority)
	fmt.Println("9. Failed dependency call (should always be sampled)")
	failedDep := appinsights.NewRemoteDependencyTelemetry("database-query", "SQL", "orders-db", false)
	client.Track(failedDep)

	// 10. Successful dependency - should be sampled at 25% default rate
	fmt.Println("10. Successful dependency call (should be sampled at 25% default rate)")
	successDep := appinsights.NewRemoteDependencyTelemetry("api-call", "HTTP", "payment-service", true)
	client.Track(successDep)

	fmt.Println()
	fmt.Println("Key features demonstrated:")
	fmt.Println("- Error/Exception Priority: Exceptions, failed requests, failed dependencies, and error traces are always sampled")
	fmt.Println("- Custom Sampling Rules: Business-critical events get 100% sampling, debug events get 5% sampling")
	fmt.Println("- Dependency-Aware Sampling: Related operations with the same operation ID are sampled together")
	fmt.Println("- Fallback Sampling: Regular telemetry uses the default 25% sampling rate")

	fmt.Println()
	fmt.Println("Note: In a real application, you would see sampling decisions reflected in your Application Insights data.")
	fmt.Println("The intelligent sampling ensures critical data is never lost while controlling overall telemetry volume.")
}
