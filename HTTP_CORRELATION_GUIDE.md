# HTTP Header Propagation Usage Guide

This guide demonstrates how to use the HTTP header propagation features added to the Application Insights Go SDK.

## Overview

The SDK now supports automatic HTTP header propagation for distributed tracing:

- **W3C Trace Context** (traceparent/tracestate headers) - Industry standard
- **Request-Id headers** - Application Insights legacy format for backward compatibility
- **HTTP middleware** - Automatic header extraction and injection
- **Round-tripper wrapper** - Automatic correlation for outgoing HTTP requests

## Quick Start

### 1. Basic HTTP Server with Correlation

```go
package main

import (
    "net/http"
    "github.com/microsoft/ApplicationInsights-Go/appinsights"
)

func main() {
    // Create middleware
    middleware := appinsights.NewHTTPMiddleware()
    
    // Your HTTP handlers
    mux := http.NewServeMux()
    mux.HandleFunc("/api", apiHandler)
    
    // Wrap with correlation middleware
    handler := middleware.Middleware(mux)
    
    http.ListenAndServe(":8080", handler)
}

func apiHandler(w http.ResponseWriter, r *http.Request) {
    // Correlation context is automatically available
    corrCtx := appinsights.GetCorrelationContext(r.Context())
    if corrCtx != nil {
        // Use trace ID for logging, etc.
        log.Printf("Processing request with trace ID: %s", corrCtx.TraceID)
    }
}
```

### 2. HTTP Client with Automatic Header Injection

```go
// Create HTTP client with correlation support
middleware := appinsights.NewHTTPMiddleware()
client := &http.Client{
    Transport: middleware.WrapRoundTripper(http.DefaultTransport),
}

// Make request - headers will be injected automatically
req, _ := http.NewRequestWithContext(ctx, "GET", "http://api.example.com", nil)
resp, err := client.Do(req)
```

### 3. Integration with Application Insights Telemetry

```go
func handler(w http.ResponseWriter, r *http.Request) {
    client := appinsights.NewTelemetryClient("your-key")
    
    // Correlation context is automatically used
    client.TrackEventWithContext(r.Context(), "RequestProcessed")
    client.TrackRequestWithContext(r.Context(), r.Method, r.URL.String(), duration, "200")
}
```

## Advanced Usage

### Manual Header Operations

```go
middleware := appinsights.NewHTTPMiddleware()

// Extract correlation from incoming request
corrCtx := middleware.ExtractHeaders(request)

// Inject correlation into outgoing request
middleware.InjectHeaders(outgoingRequest, corrCtx)

// Create child correlation for sub-operations
childCtx := appinsights.NewChildCorrelationContext(corrCtx)
```

### Working with Correlation Context

```go
// Create new correlation context
corrCtx := appinsights.NewCorrelationContext()
corrCtx.OperationName = "ProcessPayment"

// Add to Go context
ctx := appinsights.WithCorrelationContext(context.Background(), corrCtx)

// Extract from Go context
corrCtx = appinsights.GetCorrelationContext(ctx)

// Get or create if not present
corrCtx = appinsights.GetOrCreateCorrelationContext(ctx)
```

### Header Format Conversion

```go
corrCtx := appinsights.NewCorrelationContext()

// Generate W3C Trace Context header
w3cHeader := corrCtx.ToW3CTraceParent()
// Result: "00-abcd...32chars...-abcd...16chars...-01"

// Generate Request-Id header  
requestIDHeader := corrCtx.ToRequestID()
// Result: "|abcd...32chars....abcd...16chars..."

// Parse headers back to correlation context
w3cCtx, _ := appinsights.ParseW3CTraceParent(w3cHeader)
reqIdCtx, _ := appinsights.ParseRequestID(requestIDHeader)
```

## Supported Header Formats

### W3C Trace Context (Preferred)
```
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
tracestate: congo=ucfJifl5GOE,rojo=00f067aa0ba902b7
```

### Request-Id (Legacy Compatibility)
```
Request-Id: |4bf92f3577b34da6a3ce929d0e0e4736.00f067aa0ba902b7.
```

## Middleware Options

### With Telemetry Client Integration

```go
middleware := appinsights.NewHTTPMiddleware()
middleware.GetClient = func(r *http.Request) appinsights.TelemetryClient {
    // Return appropriate client based on request
    return getClientForRequest(r)
}
```

### Response Headers

The middleware automatically sets correlation headers in responses:
- `Request-Id` header is included in all responses
- Helps clients correlate their requests with your responses

## Best Practices

1. **Use W3C Headers**: Prefer W3C Trace Context for new integrations
2. **Support Both Formats**: Include both headers for maximum compatibility
3. **Create Child Contexts**: Use child contexts for sub-operations to maintain hierarchy
4. **Propagate Context**: Always pass Go context through your application layers
5. **Handle Errors**: Check for parsing errors when manually handling headers

## Error Handling

```go
// Header parsing can fail with invalid input
corrCtx, err := appinsights.ParseW3CTraceParent(header)
if err != nil {
    // Fall back to creating new context or using Request-Id
    corrCtx = appinsights.NewCorrelationContext()
}

// ParseRequestID is more lenient and generates new context on failures
corrCtx, _ := appinsights.ParseRequestID(header) // Never returns error for invalid input
```

## Migration Guide

### From Manual Correlation
```go
// Before: Manual correlation handling
operationID := generateOperationID()
setOperationIDInContext(ctx, operationID)

// After: Automatic correlation
ctx := appinsights.WithCorrelationContext(ctx, appinsights.NewCorrelationContext())
```

### From Legacy Request-Id Only
```go
// Before: Only Request-Id support
requestID := r.Header.Get("Request-Id")
// manual parsing...

// After: Automatic support for both formats
corrCtx := middleware.ExtractHeaders(r)
// Works with both W3C and Request-Id headers
```

For more examples, see the `examples/http_correlation` directory.