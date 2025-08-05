# HTTP Header Propagation Example

This example demonstrates the HTTP header propagation functionality implemented for Application Insights Go SDK.

## Features Demonstrated

- **W3C Trace Context support** - Automatic handling of `traceparent` headers
- **Request-Id header support** - Backward compatibility with Application Insights legacy format
- **HTTP middleware** - Automatic header extraction and injection
- **Correlation propagation** - Maintaining trace relationships across service calls

## Running the Example

1. **Start the server:**
   ```bash
   cd examples/http_correlation
   go run main.go
   ```

2. **Test with W3C Trace Context headers:**
   ```bash
   curl -H 'traceparent: 00-12345678901234567890123456789012-1234567890123456-01' http://localhost:8080/hello
   ```

3. **Test with Request-Id headers:**
   ```bash
   curl -H 'Request-Id: |12345678901234567890123456789012.1234567890123456.' http://localhost:8080/hello
   ```

4. **Test without headers (new correlation will be created):**
   ```bash
   curl http://localhost:8080/hello
   ```

## Key Components

### 1. HTTP Middleware
```go
middleware := appinsights.NewHTTPMiddleware()
handler := middleware.Middleware(mux)
```

### 2. Automatic Header Extraction
The middleware automatically extracts correlation context from:
- `traceparent` header (W3C Trace Context)
- `Request-Id` header (Application Insights legacy)

### 3. Automatic Header Injection
For outgoing requests:
```go
client := &http.Client{
    Transport: middleware.WrapRoundTripper(http.DefaultTransport),
}
```

### 4. Manual Header Operations
```go
// Extract headers manually
corrCtx := middleware.ExtractHeaders(request)

// Inject headers manually
middleware.InjectHeaders(request, corrCtx)
```

## Header Formats

### W3C Trace Context (Preferred)
```
traceparent: 00-12345678901234567890123456789012-1234567890123456-01
```

### Request-Id (Legacy Compatibility)
```
Request-Id: |12345678901234567890123456789012.1234567890123456.
```

## Integration with Application Insights

The middleware seamlessly integrates with Application Insights telemetry:

```go
// Correlation context is automatically available
corrCtx := appinsights.GetCorrelationContext(r.Context())

// Use with telemetry client
client.TrackEventWithContext(ctx, "RequestProcessed")
```

## Benefits

1. **Standards Compliance** - Full W3C Trace Context support
2. **Backward Compatibility** - Request-Id header support for legacy systems
3. **Automatic Propagation** - No manual header management required
4. **Distributed Tracing** - Maintains trace relationships across services
5. **Easy Integration** - Drop-in middleware for existing HTTP services