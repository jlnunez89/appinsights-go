# Automatic Event Collection

The Application Insights Go SDK provides comprehensive automatic event collection capabilities that reduce instrumentation burden by automatically capturing common telemetry without explicit code changes.

## Overview

Automatic event collection includes:

- **HTTP Auto-Collection**: Automatic tracking of HTTP requests and dependencies
- **Error Auto-Collection**: Automatic error and panic tracking with filtering
- **Performance Counter Collection**: System and runtime metrics collection

## Quick Start

### Basic Setup

```go
import "github.com/microsoft/ApplicationInsights-Go/appinsights"

// Create configuration with auto-collection
config := appinsights.NewTelemetryConfiguration("your-instrumentation-key")
config.AutoCollection = appinsights.NewAutoCollectionConfig()

// Create client with auto-collection enabled
client := appinsights.NewTelemetryClientFromConfig(config)
defer client.Channel().Close()

// Get auto-collection manager
autoCollection := client.AutoCollection()
autoCollection.Start()
defer autoCollection.Stop()
```

### HTTP Server Auto-Collection

Automatically track incoming HTTP requests:

```go
// Standard net/http
handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("Hello, World!"))
})

wrappedHandler := autoCollection.WrapHTTPHandler(handler)
http.Handle("/", wrappedHandler)
```

### HTTP Client Auto-Collection

Automatically track outgoing HTTP requests:

```go
// Wrap HTTP client for automatic dependency tracking
httpClient := autoCollection.WrapHTTPClient(&http.Client{
    Timeout: 10 * time.Second,
})

// All requests are automatically tracked
resp, err := httpClient.Get("https://api.example.com/users")
```

### Framework Integration

#### Gin Framework

```go
import "github.com/gin-gonic/gin"

router := gin.Default()
router.Use(autoCollection.HTTPMiddleware().GinMiddleware())
```

#### Echo Framework

```go
import "github.com/labstack/echo/v4"

e := echo.New()
e.Use(autoCollection.HTTPMiddleware().EchoMiddleware())
```

## Configuration Options

### HTTP Auto-Collection Settings

```go
config.AutoCollection.HTTP.Enabled = true
config.AutoCollection.HTTP.EnableRequestTracking = true
config.AutoCollection.HTTP.EnableDependencyTracking = true
config.AutoCollection.HTTP.URLSanitization = true
config.AutoCollection.HTTP.MaxURLLength = 2048
config.AutoCollection.HTTP.HeaderCollection = []string{"User-Agent"}
```

### Error Auto-Collection Settings

```go
config.AutoCollection.Errors.Enabled = true
config.AutoCollection.Errors.EnablePanicRecovery = true
config.AutoCollection.Errors.MaxStackFrames = 50
config.AutoCollection.Errors.IgnoredErrors = []string{"connection reset"}

// Add custom error filters
config.AutoCollection.Errors.ErrorFilters = []appinsights.ErrorFilterFunc{
    func(err interface{}) bool {
        // Only track errors with messages longer than 5 characters
        return len(fmt.Sprintf("%v", err)) > 5
    },
}

// Add custom sanitizers
config.AutoCollection.Errors.ErrorSanitizers = []appinsights.ErrorSanitizerFunc{
    appinsights.DefaultErrorSanitizer,
    appinsights.FilePathSanitizer,
}
```

### Performance Counter Settings

```go
config.AutoCollection.PerformanceCounters.Enabled = true
config.AutoCollection.PerformanceCounters.CollectionInterval = 60 * time.Second
config.AutoCollection.PerformanceCounters.EnableSystemMetrics = true
config.AutoCollection.PerformanceCounters.EnableRuntimeMetrics = true

// Add custom collectors
config.AutoCollection.PerformanceCounters.CustomCollectors = []appinsights.PerformanceCounterCollector{
    appinsights.NewCustomPerformanceCounterCollector("App Metrics", func() map[string]float64 {
        return map[string]float64{
            "app.active_users": 150.0,
            "app.queue_length": 5.0,
        }
    }),
}
```



## Error Auto-Collection

### Manual Error Tracking

```go
// Track errors manually
autoCollection.TrackError(err)

// Track with context
ctx := context.WithValue(context.Background(), "userID", "12345")
autoCollection.TrackErrorWithContext(ctx, err)
```

### Panic Recovery

```go
// Automatic panic recovery
autoCollection.RecoverPanic(func() {
    // Code that might panic
    panic("something went wrong")
})

// With context
autoCollection.RecoverPanicWithContext(ctx, func() {
    // Code that might panic
})
```

### Automatic Features

- Stack trace collection with configurable depth
- Sensitive data sanitization (passwords, tokens, etc.)
- Error filtering and categorization
- Integration with popular error libraries
- File path sanitization for privacy

## Performance Counter Collection

### Collected Metrics

#### System Metrics (Linux)
- `system.cpu.usage_percent` - CPU utilization percentage
- `system.memory.usage_percent` - Memory utilization percentage
- `system.memory.total` - Total system memory
- `system.memory.available` - Available memory
- `system.disk.*.read_bytes` - Disk read bytes per device
- `system.disk.*.write_bytes` - Disk write bytes per device

#### Go Runtime Metrics
- `runtime.memory.alloc` - Currently allocated memory
- `runtime.memory.heap_alloc` - Heap allocated memory
- `runtime.gc.num_gc` - Number of GC cycles
- `runtime.gc.pause_ns` - Latest GC pause time
- `runtime.goroutines` - Number of active goroutines
- `runtime.num_cpu` - Number of CPUs

### Custom Performance Counters

```go
customCollector := appinsights.NewCustomPerformanceCounterCollector(
    "Business Metrics",
    func() map[string]float64 {
        return map[string]float64{
            "business.active_sessions": getActiveSessions(),
            "business.orders_per_minute": getOrderRate(),
            "business.revenue_per_hour": getRevenueRate(),
        }
    },
)

config.AutoCollection.PerformanceCounters.CustomCollectors = append(
    config.AutoCollection.PerformanceCounters.CustomCollectors,
    customCollector,
)
```

## Best Practices

### Security Considerations

1. **URL Sanitization**: Keep `URLSanitization` enabled for HTTP operations
2. **Error Sanitization**: Use built-in sanitizers to remove sensitive data

```go
// Secure configuration
config.AutoCollection.HTTP.URLSanitization = true
config.AutoCollection.Errors.ErrorSanitizers = []appinsights.ErrorSanitizerFunc{
    appinsights.DefaultErrorSanitizer,
    appinsights.FilePathSanitizer,
}
```

### Performance Optimization

1. **Collection Intervals**: Adjust performance counter collection frequency
2. **Stack Frame Limits**: Limit stack trace depth for error collection
3. **URL Length Limits**: Set reasonable URL length limits

```go
// Performance-optimized configuration
config.AutoCollection.PerformanceCounters.CollectionInterval = 60 * time.Second
config.AutoCollection.Errors.MaxStackFrames = 20
config.AutoCollection.HTTP.MaxURLLength = 1024
```

### Filtering and Customization

1. **Error Filtering**: Filter out noise and irrelevant errors
2. **Ignored Errors**: Configure patterns for errors to ignore
3. **Custom Metrics**: Add business-specific performance counters

```go
// Custom filtering
config.AutoCollection.Errors.IgnoredErrors = []string{
    "connection reset by peer",
    "context canceled",
    "EOF",
}

config.AutoCollection.Errors.ErrorFilters = []appinsights.ErrorFilterFunc{
    func(err interface{}) bool {
        errStr := fmt.Sprintf("%v", err)
        // Only track errors from specific packages
        return strings.Contains(errStr, "myapp/") || strings.Contains(errStr, "business/")
    },
}
```

## Migration Guide

### From Manual Instrumentation

Before (manual):
```go
// Manual HTTP tracking
client.TrackRequest("GET", r.URL.String(), duration, responseCode)

// Manual error tracking
client.TrackException(err)

// Manual dependency tracking
client.TrackRemoteDependency("HTTP", "GET", "api.example.com", success)
```

After (automatic):
```go
// HTTP auto-collection setup (once)
handler = autoCollection.WrapHTTPHandler(handler)
httpClient = autoCollection.WrapHTTPClient(httpClient)

// Error auto-collection setup (once)
autoCollection.RecoverPanic(func() {
    // Your code here - errors automatically tracked
})

// All telemetry is now collected automatically
```

### Backward Compatibility

Auto-collection is fully backward compatible:
- Existing manual telemetry calls continue to work
- No breaking API changes
- Auto-collection can be disabled per component
- Gradual migration is supported

## Troubleshooting

### Common Issues

1. **Missing Telemetry**: Ensure auto-collection is started
2. **High Volume**: Adjust collection intervals and limits
3. **Sensitive Data**: Verify sanitization settings are enabled
4. **Performance Impact**: Monitor collection overhead

### Debugging

Enable diagnostics to troubleshoot auto-collection:

```go
appinsights.NewDiagnosticsMessageListener(func(msg string) error {
    fmt.Printf("[DEBUG] %s\n", msg)
    return nil
})
```

### Metrics to Monitor

- Collection overhead (< 5% recommended)
- Telemetry volume and sampling rates
- Error rates and types
- Performance counter accuracy

## Examples

See complete examples:
- [Automatic Event Collection Example](./examples/automatic_event_collection_example.go)
- [HTTP Middleware Examples](./examples/http_middleware_example.go)
- [Error Auto-Collection Example](./examples/error_auto_collection_example.go)
- [Performance Counters Example](./examples/performance_counters_example.go)