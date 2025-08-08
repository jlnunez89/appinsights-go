# Adaptive Sampling Example

This example demonstrates the volume-based adaptive sampling feature implemented in the Application Insights Go SDK.

## Overview

Adaptive sampling automatically adjusts sampling rates based on telemetry volume to help maintain optimal performance while preserving important telemetry data.

## Key Features Demonstrated

1. **Volume-based Rate Adjustment**: Automatically reduces sampling rates when telemetry volume exceeds configured limits
2. **Real-time Monitoring**: Tracks telemetry rates in real-time using a circular buffer approach
3. **Per-type Configuration**: Different volume limits and sampling rates for different telemetry types
4. **Rate Recovery**: Automatically increases sampling rates when volume decreases
5. **Deterministic Sampling**: Maintains consistent sampling decisions for correlated operations

## Configuration Options

```go
adaptiveConfig := appinsights.AdaptiveSamplingConfig{
    MaxItemsPerSecond:   10,    // Global volume limit
    EvaluationWindow:    5 * time.Second,  // How often to evaluate and adjust
    InitialSamplingRate: 100,   // Starting sampling rate (%)
    MinSamplingRate:     10,    // Minimum allowed rate (%)
    MaxSamplingRate:     100,   // Maximum allowed rate (%)
    
    // Per-type limits
    PerTypeConfigs: map[appinsights.TelemetryType]appinsights.AdaptiveTypeConfig{
        appinsights.TelemetryTypeEvent: {
            MaxItemsPerSecond: 5,
            MinSamplingRate:   5,
            MaxSamplingRate:   100,
        },
    },
}
```

## Running the Example

```bash
cd examples/adaptive-sampling
go run main.go
```

## Expected Output

The example will show:

1. **Phase 1**: Normal volume with high sampling rates
2. **Phase 2**: High volume triggering rate reduction
3. **Phase 3**: Low volume allowing rate recovery

You'll see real-time sampling rate adjustments and volume measurements throughout the execution.

## Key Concepts

- **Volume Monitoring**: Tracks items per second using a sliding window
- **Gradual Adjustment**: Changes sampling rates gradually (max 50% per evaluation) to avoid dramatic swings
- **Rate Bounds**: Respects minimum and maximum sampling rate limits
- **Type-specific Limits**: Different telemetry types can have their own volume limits and rate bounds
- **Deterministic Behavior**: Same operation IDs always get the same sampling decision

## Integration with Existing Code

To use adaptive sampling in your application:

```go
// Create configuration
config := appinsights.NewTelemetryConfiguration("your-key")
config.SamplingProcessor = appinsights.NewAdaptiveSamplingProcessor(adaptiveConfig)

// Use with existing telemetry client
client := appinsights.NewTelemetryClientFromConfig(config)
```

The adaptive sampling works transparently with all existing telemetry tracking methods.