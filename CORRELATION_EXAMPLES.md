# Correlation Context Usage Examples

This document demonstrates how to use the correlation context management features in the Application Insights Go SDK, including the new helper functions for easier span and operation management.

## Basic Usage

```go
package main

import (
    "context"
    "github.com/microsoft/ApplicationInsights-Go/appinsights"
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
    client.TrackTraceWithContext(ctx, "Starting order fulfillment", appinsights.Information)
    
    // Create child operation
    parentCorr := appinsights.GetCorrelationContext(ctx)
    childCorr := appinsights.NewChildCorrelationContext(parentCorr)
    childCorr.OperationName = "InventoryCheck"
    
    childCtx := appinsights.WithCorrelationContext(ctx, childCorr)
    client.TrackRemoteDependencyWithContext(childCtx, "InventoryAPI", "HTTP", "inventory.service.com", true)
}
```

## New Helper Functions for Easier Correlation Management

The SDK now provides convenience functions that make correlation management much simpler:

### Span Management Helpers

```go
package main

import (
    "context"
    "errors"
    "github.com/microsoft/ApplicationInsights-Go/appinsights"
)

func main() {
    client := appinsights.NewTelemetryClient("your-instrumentation-key")
    ctx := context.Background()

    // Method 1: Using WithSpan for automatic span management
    err := appinsights.WithSpan(ctx, "ProcessOrder", client, func(spanCtx context.Context) error {
        // All operations in this function are automatically tracked as part of the span
        client.TrackEventWithContext(spanCtx, "OrderValidated")
        
        // Call other operations with the span context
        return processOrderSteps(spanCtx, client)
    })
    
    if err != nil {
        // Error is automatically tracked in the span
        log.Printf("Order processing failed: %v", err)
    }

    // Method 2: Manual span management for more control
    spanCtx, span := appinsights.StartSpan(ctx, "ManualOperation", client)
    defer span.FinishSpan(spanCtx, true, map[string]string{"custom": "property"})
    
    // Do work within the span
    client.TrackEventWithContext(spanCtx, "ManualEvent")
}

func processOrderSteps(ctx context.Context, client appinsights.TelemetryClient) error {
    // Each step can be its own span
    err := appinsights.WithSpan(ctx, "ValidateInventory", client, func(spanCtx context.Context) error {
        // Validate inventory
        return checkInventory(spanCtx, client)
    })
    
    if err != nil {
        return err
    }
    
    // Another step
    return appinsights.WithSpan(ctx, "ChargePayment", client, func(spanCtx context.Context) error {
        return chargePayment(spanCtx, client)
    })
}
```

### HTTP Operation Helpers

```go
func httpHandler(w http.ResponseWriter, r *http.Request) {
    client := appinsights.NewTelemetryClient("your-instrumentation-key")
    
    // Create HTTP operation helper
    helper := appinsights.NewHTTPRequestCorrelationHelper(client)
    
    // Start HTTP operation (automatically extracts correlation from headers)
    ctx, httpOp := helper.StartHTTPOperation(r, "HandleAPIRequest")
    defer httpOp.FinishHTTPOperation(ctx, "200", true)
    
    // All operations within this handler will be correlated
    client.TrackEventWithContext(ctx, "RequestReceived")
    
    // Make outgoing HTTP requests with automatic correlation
    outgoingReq, _ := http.NewRequest("GET", "https://api.example.com/data", nil)
    httpOp.InjectHeadersForOutgoingRequest(outgoingReq)
    
    // Send the request (headers are automatically injected)
    resp, err := http.DefaultClient.Do(outgoingReq)
    if err != nil {
        http.Error(w, "Internal Server Error", 500)
        return
    }
    defer resp.Body.Close()
    
    w.WriteHeader(200)
    w.Write([]byte("Success"))
}
```

### Context Builder Pattern

```go
func advancedCorrelationExample() {
    client := appinsights.NewTelemetryClient("your-instrumentation-key")
    
    // Use builder pattern for complex correlation setup
    ctx := appinsights.NewCorrelationContextBuilder().
        WithOperationName("ComplexOperation").
        WithSampled(true).
        BuildWithContext(context.Background())
    
    // Or create child contexts
    parentCorr := appinsights.GetCorrelationContext(ctx)
    childCtx := appinsights.NewChildCorrelationContextBuilder(parentCorr).
        WithOperationName("ChildOperation").
        BuildWithContext(ctx)
    
    client.TrackEventWithContext(childCtx, "ChildEvent")
}
```

### Convenience Functions

```go
func convenienceFunctionExamples() {
    client := appinsights.NewTelemetryClient("your-instrumentation-key")
    ctx := context.Background()
    
    // Quick span creation
    ctx = appinsights.WithNewRootSpan(ctx, "RootOperation")
    ctx = appinsights.WithChildSpan(ctx, "ChildOperation")
    
    // Get or create correlation
    ctx, corrCtx := appinsights.GetOrCreateSpan(ctx, "EnsureOperation")
    
    // Copy correlation to HTTP requests
    req, _ := http.NewRequest("GET", "https://api.example.com", nil)
    appinsights.CopyCorrelationToRequest(ctx, req)
    
    // Track dependencies with automatic span creation
    err := appinsights.TrackDependencyWithSpan(ctx, client, "DatabaseQuery", "SQL", "mydb.server.com", true, func(spanCtx context.Context) error {
        // Database operation here
        return nil
    })
    
    // Track HTTP dependencies with correlation
    httpClient := &http.Client{}
    resp, err := appinsights.TrackHTTPDependency(ctx, client, req, httpClient, "api.example.com")
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