# HTTP Client Instrumentation

The Application Insights Go SDK provides comprehensive HTTP client instrumentation for automatic dependency tracking. This feature enables you to track outbound HTTP requests with detailed telemetry including timing, success/failure status, and correlation context.

## Quick Start

```go
package main

import (
    "github.com/microsoft/ApplicationInsights-Go/appinsights"
)

func main() {
    // Create telemetry client
    client := appinsights.NewTelemetryClient("your-instrumentation-key")
    
    // Create instrumented HTTP client
    httpClient := appinsights.NewHTTPClient(client)
    
    // Make requests - automatically tracked as dependencies
    resp, err := httpClient.Get("https://api.example.com/users")
    if err != nil {
        // Handle error
    }
    defer resp.Body.Close()
    
    // Shutdown gracefully
    client.Channel().Close(5 * time.Second)
}
```

## Key Features

### ✅ Automatic Dependency Tracking
- Tracks all HTTP requests as `RemoteDependencyTelemetry`
- Measures precise request timing and duration
- Captures HTTP method, URL, status codes, and success/failure
- Works with GET, POST, PUT, DELETE, and custom HTTP methods

### ✅ URL Sanitization & Security  
- Automatically removes sensitive data from tracked URLs
- Configurable sensitive parameter filtering
- Redacts passwords, API keys, tokens, and other secrets
- Preserves URL structure while protecting sensitive information

### ✅ Correlation Context Support
- Full W3C Trace Context integration
- Automatic correlation header injection
- Child span creation for distributed tracing
- Context-aware telemetry with operation correlation

### ✅ Error Tracking & Monitoring
- Automatic success/failure detection based on HTTP status codes
- Network error tracking and detailed error properties
- Support for custom error handling patterns
- Comprehensive failure telemetry

### ✅ Popular HTTP Library Support
- Standard `http.Client` instrumentation
- Generic instrumentor for any HTTP library using `http.Client`
- Helper functions for popular libraries (Resty, FastHTTP)
- Transport-level instrumentation for maximum compatibility

## Usage Examples

### Basic HTTP Client Wrapper

```go
// Create instrumented HTTP client
client := appinsights.NewTelemetryClient("your-key")
httpClient := appinsights.NewHTTPClient(client)

// All requests are automatically tracked
resp, err := httpClient.Get("https://api.example.com/data")
resp, err = httpClient.Post("https://api.example.com/users", "application/json", userData)
```

### Wrapping Existing HTTP Clients

```go
// Wrap existing http.Client
existingClient := &http.Client{Timeout: 30 * time.Second}
wrappedClient := appinsights.WrapClient(existingClient, telemetryClient)

// Or wrap the default client
wrappedDefault := appinsights.WrapDefaultClient(telemetryClient)
```

### Transport-Level Instrumentation

```go
// Instrument at the transport level
client := &http.Client{
    Transport: appinsights.NewInstrumentedTransport(telemetryClient),
    Timeout:   15 * time.Second,
}
```

### Correlation Context Integration

```go
// Create correlation context
corrCtx := appinsights.NewCorrelationContext()
ctx := appinsights.WithCorrelationContext(context.Background(), corrCtx)

// Requests will include correlation headers
resp, err := httpClient.GetWithContext(ctx, "https://api.example.com/correlated")
```

### URL Sanitization Configuration

```go
httpClient := appinsights.NewHTTPClient(telemetryClient)

// Configure custom sensitive parameters
httpClient.SensitiveQueryParams = []string{
    "secret", "password", "api_key", "token", "auth",
}

// Disable sanitization (for development/debugging)
httpClient.SanitizeURL = false
```

### Generic HTTP Library Instrumentation

```go
// Create generic instrumentor
instrumentor := appinsights.NewHTTPClientInstrumentor(telemetryClient)

// Instrument any http.Client
anyClient := &http.Client{}
instrumentor.InstrumentClient(anyClient)

// Or instrument transports
instrumentedTransport := instrumentor.InstrumentTransport(baseTransport)
```

### Popular HTTP Library Helpers

```go
// Instrument Resty client
restyClient := resty.New()
appinsights.InstrumentRestyClient(restyClient, telemetryClient)

// Generic library instrumentation
instrumentedClient := appinsights.InstrumentHTTPLibrary(func(c *http.Client) {
    // Configure your HTTP library's client
    c.Timeout = 30 * time.Second
}, telemetryClient)
```

## API Reference

### Core Types

#### `HTTPClient`
Instrumented HTTP client wrapper that automatically tracks dependencies.

```go
type HTTPClient struct {
    Client               *http.Client      // Underlying HTTP client
    TelemetryClient      TelemetryClient   // Application Insights client
    SanitizeURL          bool              // Enable URL sanitization (default: true)
    SensitiveQueryParams []string          // Parameters to redact
}
```

#### Methods
- `Do(req *http.Request) (*http.Response, error)` - Execute HTTP request
- `DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error)` - Execute with context
- `Get(url string) (*http.Response, error)` - HTTP GET request
- `GetWithContext(ctx context.Context, url string) (*http.Response, error)` - GET with context  
- `Post(url, contentType string, body interface{}) (*http.Response, error)` - HTTP POST request
- `PostWithContext(ctx context.Context, url, contentType string, body interface{}) (*http.Response, error)` - POST with context

### Constructor Functions

#### `NewHTTPClient(telemetryClient TelemetryClient) *HTTPClient`
Creates new instrumented HTTP client using `http.DefaultClient`.

#### `NewHTTPClientWithClient(client *http.Client, telemetryClient TelemetryClient) *HTTPClient`
Creates instrumented wrapper around existing `http.Client`.

#### `WrapClient(client *http.Client, telemetryClient TelemetryClient) *HTTPClient`
Convenience function to wrap existing HTTP client.

#### `WrapDefaultClient(telemetryClient TelemetryClient) *HTTPClient`
Convenience function to wrap `http.DefaultClient`.

### Transport-Level Functions

#### `NewInstrumentedTransport(telemetryClient TelemetryClient) http.RoundTripper`
Creates instrumented `http.RoundTripper` using `http.DefaultTransport`.

#### `NewInstrumentedTransportWithBase(base http.RoundTripper, telemetryClient TelemetryClient) http.RoundTripper`
Creates instrumented transport wrapping existing `http.RoundTripper`.

### Generic Instrumentation

#### `HTTPClientInstrumentor`
Generic instrumentor for HTTP libraries using `http.Client`.

```go
type HTTPClientInstrumentor struct {
    telemetryClient TelemetryClient
}
```

#### Methods
- `InstrumentClient(client *http.Client)` - Instrument existing client
- `InstrumentTransport(transport http.RoundTripper) http.RoundTripper` - Instrument transport
- `WrapHandlerFunc(handler func(*http.Client)) func(*http.Client)` - Wrap handler function

#### `NewHTTPClientInstrumentor(telemetryClient TelemetryClient) *HTTPClientInstrumentor`
Creates new generic instrumentor.

#### `InstrumentHTTPLibrary(configureClient func(*http.Client), telemetryClient TelemetryClient) *http.Client`
Generic function to instrument any HTTP library that uses `http.Client`.

### Popular Library Helpers

#### `InstrumentRestyClient(restyClient interface{}, telemetryClient TelemetryClient) interface{}`
Instruments Resty HTTP client (maintains interface compatibility).

#### `InstrumentFastHTTPClient(telemetryClient TelemetryClient) func(string, string, interface{}, interface{}) error`
Returns instrumented function for FastHTTP usage patterns.

## Telemetry Properties

The HTTP client instrumentation creates `RemoteDependencyTelemetry` items with the following properties:

### Standard Properties
- **Type**: `"HTTP"` - Identifies this as an HTTP dependency
- **Name**: `"GET /api/users"` - HTTP method + sanitized path
- **Target**: `"api.example.com"` - Host portion of the URL
- **Data**: `"https://api.example.com/api/users?param=value"` - Full URL (sanitized)
- **Duration**: Request execution time
- **Success**: `true/false` based on status code and errors
- **ResultCode**: HTTP status code or `"0"` for network errors
- **Timestamp**: Request start time

### Custom Properties
- **httpMethod**: HTTP method (GET, POST, etc.)
- **httpStatusCode**: HTTP response status code
- **error**: Error message (if request failed)

### Correlation Properties
When correlation context is available:
- **Id**: Correlation span ID
- Operation correlation headers automatically injected

## URL Sanitization

The instrumentation automatically sanitizes URLs to protect sensitive information:

### Default Sensitive Parameters
- `password`, `pwd` 
- `secret`, `key`, `token`
- `api_key`, `apikey`
- `access_token`, `auth`, `authorization`
- `credential`, `credentials`

### Sanitization Behavior
- Sensitive query parameters are replaced with `[REDACTED]`
- User information (username:password) is removed from URLs
- URL fragments are removed
- Normal parameters are preserved

### Configuration
```go
httpClient := appinsights.NewHTTPClient(telemetryClient)

// Add custom sensitive parameters
httpClient.SensitiveQueryParams = append(httpClient.SensitiveQueryParams, "custom_secret")

// Disable sanitization entirely
httpClient.SanitizeURL = false
```

## Error Handling

### Success/Failure Detection
- **Success**: HTTP status codes < 400 or 401 (authentication)
- **Failure**: HTTP status codes >= 400 (except 401), network errors, timeouts

### Error Telemetry
- Network errors set `ResultCode` to `"0"`
- HTTP errors use actual status code as `ResultCode`
- Error messages captured in telemetry properties
- Success flag accurately reflects request outcome

### Error Properties
```go
// Example error telemetry properties
{
    "httpMethod": "GET",
    "httpStatusCode": "500", 
    "error": "connection timeout"
}
```

## Performance Considerations

### Minimal Overhead
- Instrumentation adds minimal latency (< 1ms per request)
- Uses efficient correlation context passing
- Respects existing HTTP client configurations

### Memory Usage
- No request/response body interception
- Lightweight telemetry object creation
- Proper cleanup and resource management

### Scalability
- Works with high-volume HTTP traffic
- Integrates with Application Insights sampling
- Thread-safe for concurrent usage

## Integration with Existing Code

### Drop-in Replacement
```go
// Before
client := http.DefaultClient
resp, err := client.Get("https://api.example.com")

// After  
client := appinsights.WrapDefaultClient(telemetryClient)
resp, err := client.Get("https://api.example.com") // Now tracked!
```

### Library Integration
```go
// Instrument any library using http.Client
func instrumentMyHTTPLibrary(telemetryClient appinsights.TelemetryClient) *MyHTTPLib {
    lib := NewMyHTTPLib()
    
    // Get the underlying http.Client and instrument it
    httpClient := lib.GetHTTPClient() 
    instrumentor := appinsights.NewHTTPClientInstrumentor(telemetryClient)
    instrumentor.InstrumentClient(httpClient)
    
    return lib
}
```

## Complete Example

See [`examples/http_client_example.go`](../examples/http_client_example.go) for a comprehensive example demonstrating all features including:

- Basic HTTP client operations
- Correlation context usage
- URL sanitization
- Error handling
- Library integration patterns
- Best practices

## Best Practices

### 1. Use Correlation Context
Always pass correlation context for distributed tracing:

```go
ctx := appinsights.WithCorrelationContext(context.Background(), corrCtx)
resp, err := httpClient.GetWithContext(ctx, url)
```

### 2. Configure URL Sanitization
Review and configure sensitive parameters for your use case:

```go
httpClient.SensitiveQueryParams = []string{
    "api_key", "secret", "password", "token", 
    "client_secret", "auth_token", // Add your sensitive params
}
```

### 3. Handle Errors Appropriately
The instrumentation tracks errors automatically, but handle them in your code:

```go
resp, err := httpClient.Get(url)
if err != nil {
    // Error is automatically tracked, handle it appropriately
    log.Printf("Request failed: %v", err)
    return err
}
defer resp.Body.Close()
```

### 4. Graceful Shutdown
Ensure telemetry is sent before application shutdown:

```go
// Graceful shutdown
select {
case <-telemetryClient.Channel().Close(5 * time.Second):
    log.Println("Telemetry sent successfully")
case <-time.After(10 * time.Second):
    log.Println("Timeout waiting for telemetry")
}
```

### 5. Library Integration
For HTTP libraries, instrument at the lowest level possible:

```go
// Preferred: Instrument the underlying http.Client
instrumentor.InstrumentClient(library.GetHTTPClient())

// Alternative: Instrument at transport level  
client.Transport = appinsights.NewInstrumentedTransport(telemetryClient)
```

This HTTP client instrumentation provides comprehensive, automatic dependency tracking while maintaining performance and security. It integrates seamlessly with existing Application Insights features and follows established patterns for correlation and telemetry.