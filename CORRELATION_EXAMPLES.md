# Correlation Context Usage Examples

This document demonstrates how to use the new correlation context management features in the Application Insights Go SDK.

## Basic Usage

```go
package main

import (
    "context"
    "github.com/microsoft/ApplicationInsights-Go/appinsights"
    "github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

func main() {
    // Create Application Insights client
    client := appinsights.NewTelemetryClient("your-instrumentation-key")

    // Create a correlation context for distributed tracing
    corrCtx := appinsights.NewCorrelationContext()
    corrCtx.OperationName = "ProcessOrder"
    
    // Attach correlation context to Go context
    ctx := appinsights.WithCorrelationContext(context.Background(), corrCtx)
    
    // Track telemetry with correlation context
    client.TrackEventWithContext(ctx, "OrderStarted")
    
    // Pass context to child operations
    processPayment(ctx, client)
    fulfillOrder(ctx, client)
    
    client.TrackEventWithContext(ctx, "OrderCompleted")
}

func processPayment(ctx context.Context, client appinsights.TelemetryClient) {
    // Get existing correlation context and create child
    parentCorr := appinsights.GetCorrelationContext(ctx)
    childCorr := appinsights.NewChildCorrelationContext(parentCorr)
    childCorr.OperationName = "ProcessPayment"
    
    // Create new context with child correlation
    childCtx := appinsights.WithCorrelationContext(ctx, childCorr)
    
    // Track dependency with child correlation
    client.TrackRemoteDependencyWithContext(childCtx, "PaymentAPI", "HTTP", "payment.service.com", true)
}

func fulfillOrder(ctx context.Context, client appinsights.TelemetryClient) {
    // Operations inherit correlation context automatically
    client.TrackTraceWithContext(ctx, "Starting order fulfillment", contracts.Information)
    
    // Create child operation
    parentCorr := appinsights.GetCorrelationContext(ctx)
    childCorr := appinsights.NewChildCorrelationContext(parentCorr)
    childCorr.OperationName = "InventoryCheck"
    
    childCtx := appinsights.WithCorrelationContext(ctx, childCorr)
    client.TrackRemoteDependencyWithContext(childCtx, "InventoryAPI", "HTTP", "inventory.service.com", true)
}
```

## W3C Trace Context Integration

```go
package main

import (
    "context"
    "net/http"
    "github.com/microsoft/ApplicationInsights-Go/appinsights"
)

func httpHandler(w http.ResponseWriter, r *http.Request) {
    client := appinsights.NewTelemetryClient("your-instrumentation-key")
    
    // Extract W3C trace context from incoming request
    traceParent := r.Header.Get("traceparent")
    var ctx context.Context
    
    if traceParent != "" {
        // Parse incoming W3C trace context
        corrCtx, err := appinsights.ParseW3CTraceParent(traceParent)
        if err == nil {
            // Create child span for this operation
            childCorr := appinsights.NewChildCorrelationContext(corrCtx)
            childCorr.OperationName = "HandleRequest"
            ctx = appinsights.WithCorrelationContext(r.Context(), childCorr)
        } else {
            // Invalid trace parent, create new correlation
            ctx = appinsights.WithCorrelationContext(r.Context(), appinsights.NewCorrelationContext())
        }
    } else {
        // No trace parent, start new trace
        ctx = appinsights.WithCorrelationContext(r.Context(), appinsights.NewCorrelationContext())
    }
    
    // Track the request with correlation
    client.TrackRequestWithContext(ctx, r.Method, r.URL.String(), time.Since(start), "200")
    
    // For outgoing requests, add W3C trace context headers
    corrCtx := appinsights.GetCorrelationContext(ctx)
    if corrCtx != nil {
        outgoingReq, _ := http.NewRequest("GET", "https://api.example.com/data", nil)
        outgoingReq.Header.Set("traceparent", corrCtx.ToW3CTraceParent())
        // Make request...
    }
}
```

## Key Features

1. **W3C Trace Context Compliance**: Full support for W3C Trace Context standard with 128-bit trace IDs and 64-bit span IDs
2. **Go Context Integration**: Seamless integration with Go's context.Context for idiomatic correlation propagation
3. **Automatic Parent-Child Relationships**: Easy creation of child spans that maintain trace relationships
4. **Backward Compatibility**: All existing APIs continue to work unchanged
5. **Minimal Performance Overhead**: Efficient ID generation and context management

## API Reference

### Core Types

- `CorrelationContext`: Holds correlation information including trace ID, span ID, parent span ID, and operation name
- `WithCorrelationContext(ctx, corrCtx)`: Attaches correlation context to Go context
- `GetCorrelationContext(ctx)`: Extracts correlation context from Go context
- `GetOrCreateCorrelationContext(ctx)`: Gets existing or creates new correlation context

### Context Creation

- `NewCorrelationContext()`: Creates new root correlation context
- `NewChildCorrelationContext(parent)`: Creates child context inheriting trace ID

### W3C Trace Context

- `ToW3CTraceParent()`: Exports W3C traceparent header value
- `ParseW3CTraceParent(header)`: Parses W3C traceparent header

### Client Methods

- `TrackWithContext(ctx, telemetry)`: Track telemetry with correlation context
- `TrackEventWithContext(ctx, name)`: Track event with correlation
- `TrackTraceWithContext(ctx, message, severity)`: Track trace with correlation
- `TrackRequestWithContext(ctx, method, url, duration, responseCode)`: Track request with correlation
- `TrackRemoteDependencyWithContext(ctx, name, type, target, success)`: Track dependency with correlation