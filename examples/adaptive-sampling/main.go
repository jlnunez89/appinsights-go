package main

import (
	"fmt"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

func main() {
	// Create adaptive sampling configuration
	adaptiveConfig := appinsights.AdaptiveSamplingConfig{
		MaxItemsPerSecond:   10, // Target maximum: 10 items per second globally
		EvaluationWindow:    5 * time.Second, // Evaluate and adjust every 5 seconds
		InitialSamplingRate: 100, // Start with 100% sampling
		MinSamplingRate:     10,  // Never go below 10%
		MaxSamplingRate:     100, // Never go above 100%
		
		// Per-type configuration
		PerTypeConfigs: map[appinsights.TelemetryType]appinsights.AdaptiveTypeConfig{
			appinsights.TelemetryTypeEvent: {
				MaxItemsPerSecond: 5,   // Max 5 events per second
				MinSamplingRate:   5,   // Min 5% for events
				MaxSamplingRate:   100, // Max 100% for events
			},
			appinsights.TelemetryTypeTrace: {
				MaxItemsPerSecond: 3,   // Max 3 traces per second
				MinSamplingRate:   1,   // Min 1% for traces
				MaxSamplingRate:   50,  // Max 50% for traces
			},
		},
	}

	// Create telemetry configuration with adaptive sampling
	config := appinsights.NewTelemetryConfiguration("your-instrumentation-key")
	config.SamplingProcessor = appinsights.NewAdaptiveSamplingProcessor(adaptiveConfig)

	// Create telemetry client
	client := appinsights.NewTelemetryClientFromConfig(config)
	defer client.Channel().Close(10 * time.Second)

	fmt.Println("Adaptive Sampling Example")
	fmt.Println("=========================")
	fmt.Printf("Initial sampling rate: %.1f%%\n", config.SamplingProcessor.GetSamplingRate())
	fmt.Printf("Target max items per second: %.0f\n", adaptiveConfig.MaxItemsPerSecond)
	fmt.Println()

	// Phase 1: Normal volume
	fmt.Println("Phase 1: Normal volume (10 items over 5 seconds)")
	for i := 0; i < 10; i++ {
		client.TrackEvent(fmt.Sprintf("normal-event-%d", i))
		client.TrackTrace(fmt.Sprintf("normal-trace-%d", i), contracts.Information)
		time.Sleep(500 * time.Millisecond)
	}
	
	adaptiveProcessor := config.SamplingProcessor.(*appinsights.AdaptiveSamplingProcessor)
	fmt.Printf("Current sampling rate: %.1f%%\n", adaptiveProcessor.GetSamplingRate())
	fmt.Printf("Current volume rate: %.1f items/sec\n", adaptiveProcessor.GetCurrentVolumeRate())
	fmt.Println()

	// Phase 2: High volume to trigger rate reduction
	fmt.Println("Phase 2: High volume (50 items in 2 seconds)")
	for i := 0; i < 50; i++ {
		client.TrackEvent(fmt.Sprintf("high-volume-event-%d", i))
		client.TrackTrace(fmt.Sprintf("high-volume-trace-%d", i), contracts.Warning)
		client.TrackMetric(fmt.Sprintf("high-volume-metric-%d", i), float64(i))
		time.Sleep(40 * time.Millisecond) // Fast rate
	}

	// Wait for evaluation window to allow adjustment
	fmt.Println("Waiting for rate adjustment...")
	time.Sleep(6 * time.Second)

	fmt.Printf("After high volume - Global sampling rate: %.1f%%\n", adaptiveProcessor.GetSamplingRate())
	fmt.Printf("After high volume - Event sampling rate: %.1f%%\n", adaptiveProcessor.GetSamplingRateForType(appinsights.TelemetryTypeEvent))
	fmt.Printf("After high volume - Trace sampling rate: %.1f%%\n", adaptiveProcessor.GetSamplingRateForType(appinsights.TelemetryTypeTrace))
	fmt.Printf("Current volume rate: %.1f items/sec\n", adaptiveProcessor.GetCurrentVolumeRate())
	fmt.Printf("Event volume rate: %.1f items/sec\n", adaptiveProcessor.GetCurrentVolumeRateForType(appinsights.TelemetryTypeEvent))
	fmt.Printf("Trace volume rate: %.1f items/sec\n", adaptiveProcessor.GetCurrentVolumeRateForType(appinsights.TelemetryTypeTrace))
	fmt.Println()

	// Phase 3: Low volume to test recovery
	fmt.Println("Phase 3: Low volume (allowing recovery)")
	for i := 0; i < 5; i++ {
		client.TrackEvent(fmt.Sprintf("recovery-event-%d", i))
		time.Sleep(2 * time.Second) // Very slow rate
	}

	// Wait for another evaluation
	time.Sleep(6 * time.Second)

	fmt.Printf("After recovery period - Global sampling rate: %.1f%%\n", adaptiveProcessor.GetSamplingRate())
	fmt.Printf("After recovery period - Event sampling rate: %.1f%%\n", adaptiveProcessor.GetSamplingRateForType(appinsights.TelemetryTypeEvent))
	fmt.Printf("After recovery period - Trace sampling rate: %.1f%%\n", adaptiveProcessor.GetSamplingRateForType(appinsights.TelemetryTypeTrace))
	fmt.Printf("Current volume rate: %.1f items/sec\n", adaptiveProcessor.GetCurrentVolumeRate())
	fmt.Println()

	fmt.Println("Example completed. Adaptive sampling automatically adjusted rates based on volume!")
}