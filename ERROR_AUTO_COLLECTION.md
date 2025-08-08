# Error Auto-Collection

Application Insights Go SDK now supports comprehensive error auto-collection functionality. This feature provides automatic panic recovery, error tracking, filtering, and sanitization capabilities.

## Quick Start

```go
import "github.com/microsoft/ApplicationInsights-Go/appinsights"

// Create configuration with error auto-collection
config := appinsights.NewTelemetryConfiguration("your-instrumentation-key")
config.ErrorAutoCollection = appinsights.NewErrorAutoCollectionConfig()

// Create client
client := appinsights.NewTelemetryClientFromConfig(config)
errorCollector := client.ErrorAutoCollector()
```

## Key Features

### 1. Automatic Panic Recovery

Automatically recover from panics and track them as exceptions:

```go
// Recover from panics in a function
errorCollector.RecoverPanic(func() {
    panic("This will be tracked automatically")
})

// Wrap functions for automatic panic recovery
wrappedFunc := errorCollector.WrapFunction(riskyFunction)
wrappedFunc() // Any panic will be tracked
```

### 2. Error Filtering

Filter which errors should be tracked:

```go
config.ErrorAutoCollection.IgnoredErrors = []string{"connection timeout", "not found"}

// Custom error filters
config.ErrorAutoCollection.ErrorFilters = []appinsights.ErrorFilterFunc{
    func(err interface{}) bool {
        errStr := fmt.Sprintf("%v", err)
        return !strings.Contains(errStr, "temporary")
    },
}
```

### 3. Error Sanitization

Automatically sanitize sensitive information from error messages:

```go
// Use built-in sanitizers
config.ErrorAutoCollection.ErrorSanitizers = []appinsights.ErrorSanitizerFunc{
    appinsights.DefaultErrorSanitizer,     // Removes passwords, tokens, etc.
    appinsights.FilePathSanitizer,         // Sanitizes file paths
}

// Custom sanitizer
customSanitizer := func(err interface{}, frames []*contracts.StackFrame) (interface{}, []*contracts.StackFrame) {
    errStr := strings.ReplaceAll(fmt.Sprintf("%v", err), "secret", "[REDACTED]")
    return errStr, frames
}
config.ErrorAutoCollection.ErrorSanitizers = append(config.ErrorAutoCollection.ErrorSanitizers, customSanitizer)
```

### 4. Enhanced Stack Traces

Configure stack trace collection:

```go
config.ErrorAutoCollection.MaxStackFrames = 50
config.ErrorAutoCollection.IncludeSourceCode = true
```

### 5. Error Library Integration

Automatic support for popular Go error libraries:

```go
// Works with pkg/errors stack traces
err := errors.WithStack(errors.New("error with stack"))
errorCollector.TrackError(err)

// Works with error wrapping
rootErr := errors.New("root cause")
wrappedErr := fmt.Errorf("wrapped: %w", rootErr)
errorCollector.TrackError(wrappedErr) // Tracks entire error chain
```

## Configuration Options

```go
config := appinsights.NewErrorAutoCollectionConfig()

// Enable/disable features
config.Enabled = true
config.EnablePanicRecovery = true
config.EnableErrorLibraryIntegration = true

// Stack trace options
config.MaxStackFrames = 50
config.IncludeSourceCode = false

// Error filtering
config.IgnoredErrors = []string{"ignored pattern"}
config.ErrorFilters = []appinsights.ErrorFilterFunc{/* custom filters */}

// Error sanitization
config.ErrorSanitizers = []appinsights.ErrorSanitizerFunc{/* custom sanitizers */}

// Severity level for auto-collected errors
config.SeverityLevel = appinsights.Error
```

## Manual Error Tracking

You can still manually track errors with enhanced features:

```go
// Basic error tracking
errorCollector.TrackError(err)

// Error tracking with context
ctx := context.Background()
errorCollector.TrackErrorWithContext(ctx, err)
```

## Backward Compatibility

All existing exception tracking methods continue to work unchanged:

```go
// Legacy methods still work
client.TrackException(err)
appinsights.TrackPanic(client, false)
```

## Built-in Error Filters

- `DefaultErrorFilter`: Excludes nil errors
- `SeverityErrorFilter`: Filters based on keyword matching

## Built-in Error Sanitizers

- `DefaultErrorSanitizer`: Removes common sensitive patterns (password, token, key, secret, auth)
- `FilePathSanitizer`: Shortens long file paths for privacy

## Error Library Support

The auto-collector automatically detects and integrates with:

- Standard Go errors
- `pkg/errors` stack traces
- Go 1.13+ error wrapping (`fmt.Errorf` with `%w`)
- Custom error types implementing stack trace interfaces

## Performance Considerations

- Error filtering happens before stack trace collection for efficiency
- Stack frame limits prevent excessive memory usage
- Sanitization is applied only to tracked errors
- Panic recovery has minimal overhead when errors don't occur

## Thread Safety

All error auto-collection operations are thread-safe and can be used concurrently across goroutines.

## Example

See `examples/error_auto_collection_example.go` for a comprehensive demonstration of all features.