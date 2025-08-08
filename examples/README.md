# Sampling Examples

This directory contains comprehensive examples demonstrating the full range of sampling capabilities in the Application Insights Go SDK.

## Examples Overview

### [basic_sampling_example.go](./basic_sampling_example.go)
**Recommended starting point** - Demonstrates all major sampling types with practical examples:
- No sampling (100% - default)
- Fixed-rate sampling (50%)
- Per-type sampling (different rates per telemetry type)
- Error priority sampling (errors always sampled)
- Dependency-aware sampling (consistent across operations)

### [intelligent_sampling_example.go](./intelligent_sampling_example.go)
Advanced intelligent sampling with custom rules and error prioritization:
- Custom sampling rules with priorities
- Automatic error/exception priority (always sampled)
- Business rule examples (critical events at 100%, debug at 5%)
- Dependency-aware sampling for complete traces

### [adaptive-sampling/](./adaptive-sampling/)
Volume-based adaptive sampling that adjusts rates automatically:
- Real-time volume monitoring
- Automatic rate adjustment based on telemetry volume
- Per-type volume limits and rate bounds
- Recovery behavior when volume decreases

### [sampling_validation.go](./sampling_validation.go)
Performance and statistical validation demonstrating production readiness:
- Statistical accuracy validation (within 2% tolerance)
- Consistency testing (same operation ID = same decision)
- Performance testing (>1.8M items/second throughput)
- Per-type sampling verification
- Sampling metadata validation

## Quick Start

Run any example to see sampling in action:

```bash
# Basic introduction to all sampling types
go run basic_sampling_example.go

# Advanced intelligent sampling with custom rules
go run intelligent_sampling_example.go

# Adaptive volume-based sampling
cd adaptive-sampling && go run main.go

# Performance and statistical validation
go run sampling_validation.go
```

## Key Features Demonstrated

- **Fixed-Rate Sampling**: Consistent percentage-based sampling
- **Per-Type Sampling**: Different rates for events, traces, metrics, etc.
- **Adaptive Sampling**: Automatic rate adjustment based on volume
- **Intelligent Sampling**: Custom rules with priority-based evaluation
- **Error Priority**: Exceptions and failures always sampled (100%)
- **Dependency-Aware**: Correlated operations sampled together
- **High Performance**: >1.8M items/second processing capability
- **Statistical Accuracy**: Proper sampling metadata for accurate aggregations

## Integration

All sampling processors integrate seamlessly with the existing TelemetryClient:

```go
config := appinsights.NewTelemetryConfiguration("your-key")
config.SamplingProcessor = appinsights.NewFixedRateSamplingProcessor(25.0)
client := appinsights.NewTelemetryClientFromConfig(config)

// Use client normally - sampling happens automatically
client.TrackEvent("user-action")
client.TrackException("system-error") // Always sampled due to error priority
```

The sampling implementation is production-ready and provides comprehensive telemetry volume control while preserving statistical significance and ensuring critical data is never lost.