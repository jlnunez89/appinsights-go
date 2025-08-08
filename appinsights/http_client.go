package appinsights

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HTTPClient is a wrapper around http.Client that automatically tracks
// HTTP dependencies using Application Insights telemetry.
type HTTPClient struct {
	// The underlying HTTP client to use for making requests
	Client *http.Client

	// The telemetry client to use for tracking dependencies
	TelemetryClient TelemetryClient

	// SanitizeURL controls whether URLs should be sanitized to remove
	// sensitive information. Defaults to true.
	SanitizeURL bool

	// SensitiveQueryParams is a list of query parameter names that should
	// be removed from URLs when tracking dependencies. Common examples:
	// "password", "key", "token", "secret", "api_key"
	SensitiveQueryParams []string
}

// NewHTTPClient creates a new instrumented HTTP client with the specified
// telemetry client. Uses http.DefaultClient as the underlying client.
func NewHTTPClient(telemetryClient TelemetryClient) *HTTPClient {
	return &HTTPClient{
		Client:          http.DefaultClient,
		TelemetryClient: telemetryClient,
		SanitizeURL:     true,
		SensitiveQueryParams: []string{
			"password", "pwd", "secret", "key", "token", "api_key", "apikey",
			"access_token", "auth", "authorization", "credential", "credentials",
		},
	}
}

// NewHTTPClientWithClient creates a new instrumented HTTP client using
// the provided http.Client as the underlying client.
func NewHTTPClientWithClient(client *http.Client, telemetryClient TelemetryClient) *HTTPClient {
	return &HTTPClient{
		Client:          client,
		TelemetryClient: telemetryClient,
		SanitizeURL:     true,
		SensitiveQueryParams: []string{
			"password", "pwd", "secret", "key", "token", "api_key", "apikey",
			"access_token", "auth", "authorization", "credential", "credentials",
		},
	}
}

// Do executes an HTTP request and automatically tracks it as a dependency.
// This method wraps the underlying client's Do method with telemetry tracking.
func (c *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.DoWithContext(req.Context(), req)
}

// DoWithContext executes an HTTP request with the specified context and 
// automatically tracks it as a dependency with correlation support.
func (c *HTTPClient) DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error) {
	if c.Client == nil {
		c.Client = http.DefaultClient
	}

	// Ensure the request has the provided context
	if ctx != nil {
		req = req.WithContext(ctx)
	}

	// Create an instrumented round tripper if the client doesn't already have one
	transport := c.Client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	// Wrap the transport with our instrumentation
	instrumentedTransport := &instrumentedRoundTripper{
		base:                 transport,
		telemetryClient:     c.TelemetryClient,
		sanitizeURL:         c.SanitizeURL,
		sensitiveQueryParams: c.SensitiveQueryParams,
	}

	// Create a temporary client with the instrumented transport
	tempClient := &http.Client{
		Transport:     instrumentedTransport,
		CheckRedirect: c.Client.CheckRedirect,
		Jar:          c.Client.Jar,
		Timeout:      c.Client.Timeout,
	}

	// Inject correlation headers if correlation context exists
	if corrCtx := GetCorrelationContext(req.Context()); corrCtx != nil {
		middleware := NewHTTPMiddleware()
		childCtx := NewChildCorrelationContext(corrCtx)
		middleware.InjectHeaders(req, childCtx)
	}

	// Execute the request
	return tempClient.Do(req)
}

// Get performs a GET request to the specified URL and tracks it as a dependency.
func (c *HTTPClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// GetWithContext performs a GET request with context and tracks it as a dependency.
func (c *HTTPClient) GetWithContext(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithContext(ctx, req)
}

// Post performs a POST request and tracks it as a dependency.
func (c *HTTPClient) Post(url, contentType string, body interface{}) (*http.Response, error) {
	return c.PostWithContext(context.Background(), url, contentType, body)
}

// PostWithContext performs a POST request with context and tracks it as a dependency.
func (c *HTTPClient) PostWithContext(ctx context.Context, url, contentType string, body interface{}) (*http.Response, error) {
	// Convert body to io.Reader if needed
	var bodyReader io.Reader
	switch v := body.(type) {
	case io.Reader:
		bodyReader = v
	case string:
		bodyReader = strings.NewReader(v)
	case []byte:
		bodyReader = bytes.NewReader(v)
	case nil:
		bodyReader = nil
	default:
		// For other types, try to convert to JSON if possible
		if jsonBytes, err := json.Marshal(v); err == nil {
			bodyReader = bytes.NewReader(jsonBytes)
			if contentType == "" {
				contentType = "application/json"
			}
		} else {
			return nil, fmt.Errorf("unsupported body type: %T", body)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bodyReader)
	if err != nil {
		return nil, err
	}
	
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	
	return c.DoWithContext(ctx, req)
}

// instrumentedRoundTripper implements http.RoundTripper and automatically
// tracks HTTP dependencies.
type instrumentedRoundTripper struct {
	base                 http.RoundTripper
	telemetryClient      TelemetryClient
	sanitizeURL          bool
	sensitiveQueryParams []string
}

// RoundTrip implements the http.RoundTripper interface and tracks the request
// as a dependency telemetry item.
func (rt *instrumentedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.telemetryClient == nil || !rt.telemetryClient.IsEnabled() {
		// If telemetry is disabled, just pass through to the base transport
		base := rt.base
		if base == nil {
			base = http.DefaultTransport
		}
		return base.RoundTrip(req)
	}

	// Record start time
	startTime := time.Now()

	// Execute the request
	base := rt.base
	if base == nil {
		base = http.DefaultTransport
	}
	
	resp, err := base.RoundTrip(req)
	
	// Calculate duration
	duration := time.Since(startTime)

	// Track the dependency
	rt.trackDependency(req, resp, err, startTime, duration)

	return resp, err
}

// trackDependency creates and tracks a RemoteDependencyTelemetry item for the HTTP request.
func (rt *instrumentedRoundTripper) trackDependency(req *http.Request, resp *http.Response, err error, startTime time.Time, duration time.Duration) {
	// Determine success status
	success := err == nil
	var resultCode string
	
	if resp != nil {
		resultCode = strconv.Itoa(resp.StatusCode)
		// Consider only 2xx and 3xx status codes as success, treat all 4xx and 5xx as failures
		success = resp.StatusCode < 400
	} else if err != nil {
		// Network error or other failure
		success = false
		resultCode = "0" // Indicate network failure
	}

	// Sanitize URL for tracking
	sanitizedURL := rt.sanitizeURLForTracking(req.URL)

	// Extract target (host:port)
	target := req.URL.Host
	if target == "" && req.URL.Scheme != "" {
		target = req.URL.Scheme + "://" + req.URL.Host
	}

	// Create dependency name (HTTP method + sanitized path)
	name := req.Method
	if req.URL.Path != "" {
		name += " " + req.URL.Path
	}

	// Create the telemetry item
	var dependency *RemoteDependencyTelemetry
	if req.Context() != nil {
		dependency = NewRemoteDependencyTelemetryWithContext(req.Context(), name, "HTTP", target, success)
	} else {
		dependency = NewRemoteDependencyTelemetry(name, "HTTP", target, success)
	}

	// Set additional properties
	dependency.ResultCode = resultCode
	dependency.Duration = duration
	dependency.Data = sanitizedURL
	dependency.Timestamp = startTime

	// Add HTTP-specific properties
	if dependency.Properties == nil {
		dependency.Properties = make(map[string]string)
	}
	dependency.Properties["httpMethod"] = req.Method
	
	if resp != nil {
		dependency.Properties["httpStatusCode"] = strconv.Itoa(resp.StatusCode)
	}
	
	if err != nil {
		dependency.Properties["error"] = err.Error()
	}

	// Track the dependency
	if req.Context() != nil {
		rt.telemetryClient.TrackWithContext(req.Context(), dependency)
	} else {
		rt.telemetryClient.Track(dependency)
	}
}

// sanitizeURLForTracking removes sensitive information from URLs when tracking dependencies.
func (rt *instrumentedRoundTripper) sanitizeURLForTracking(u *url.URL) string {
	if !rt.sanitizeURL {
		return u.String()
	}

	// Create a copy of the URL to avoid modifying the original
	sanitized := *u

	// Remove user information (username:password)
	sanitized.User = nil

	// Remove or sanitize query parameters
	if rt.sensitiveQueryParams != nil && len(rt.sensitiveQueryParams) > 0 {
		query := sanitized.Query()
		for _, param := range rt.sensitiveQueryParams {
			for key := range query {
				// Case-insensitive matching for common patterns
				if strings.EqualFold(key, param) {
					query.Set(key, "[REDACTED]")
				}
			}
		}
		sanitized.RawQuery = query.Encode()
	}

	// Remove fragment
	sanitized.Fragment = ""

	return sanitized.String()
}

// WrapClient wraps an existing http.Client with Application Insights instrumentation.
// This is a convenience function for adding telemetry to existing clients.
func WrapClient(client *http.Client, telemetryClient TelemetryClient) *HTTPClient {
	return NewHTTPClientWithClient(client, telemetryClient)
}

// WrapDefaultClient wraps the http.DefaultClient with Application Insights instrumentation.
func WrapDefaultClient(telemetryClient TelemetryClient) *HTTPClient {
	return NewHTTPClient(telemetryClient)
}

// NewInstrumentedTransport creates a new http.RoundTripper that automatically
// tracks HTTP dependencies. This can be used to instrument existing http.Client
// instances by setting their Transport field.
func NewInstrumentedTransport(telemetryClient TelemetryClient) http.RoundTripper {
	return &instrumentedRoundTripper{
		base:                 http.DefaultTransport,
		telemetryClient:     telemetryClient,
		sanitizeURL:         true,
		sensitiveQueryParams: []string{
			"password", "pwd", "secret", "key", "token", "api_key", "apikey",
			"access_token", "auth", "authorization", "credential", "credentials",
		},
	}
}

// NewInstrumentedTransportWithBase creates a new http.RoundTripper that wraps
// the provided base transport with Application Insights instrumentation.
func NewInstrumentedTransportWithBase(base http.RoundTripper, telemetryClient TelemetryClient) http.RoundTripper {
	return &instrumentedRoundTripper{
		base:                 base,
		telemetryClient:     telemetryClient,
		sanitizeURL:         true,
		sensitiveQueryParams: []string{
			"password", "pwd", "secret", "key", "token", "api_key", "apikey",
			"access_token", "auth", "authorization", "credential", "credentials",
		},
	}
}

// InstrumentHTTPLibrary provides a generic way to instrument HTTP libraries
// that use http.Client internally. This function takes a function that configures
// an http.Client and returns a configured client with Application Insights instrumentation.
func InstrumentHTTPLibrary(configureClient func(*http.Client), telemetryClient TelemetryClient) *http.Client {
	client := &http.Client{}
	
	// Apply library-specific configuration
	if configureClient != nil {
		configureClient(client)
	}
	
	// Wrap the transport with instrumentation
	if client.Transport == nil {
		client.Transport = http.DefaultTransport
	}
	client.Transport = NewInstrumentedTransportWithBase(client.Transport, telemetryClient)
	
	return client
}

// Popular HTTP library helpers

// InstrumentRestyClient instruments a Resty HTTP client for use with Application Insights.
// This is a convenience function for users of the go-resty/resty library.
// Usage:
//   client := resty.New()
//   instrumentedClient := appinsights.InstrumentRestyClient(client, telemetryClient)
func InstrumentRestyClient(restyClient interface{}, telemetryClient TelemetryClient) interface{} {
	// This is a generic interface approach to avoid importing resty directly
	// Users can cast the result back to *resty.Client
	
	// Use reflection to get the underlying http.Client if available
	// Most HTTP libraries expose GetClient() or similar methods
	if clientGetter, ok := restyClient.(interface{ GetClient() *http.Client }); ok {
		httpClient := clientGetter.GetClient()
		if httpClient.Transport == nil {
			httpClient.Transport = http.DefaultTransport
		}
		httpClient.Transport = NewInstrumentedTransportWithBase(httpClient.Transport, telemetryClient)
	}
	
	return restyClient
}

// InstrumentFastHTTPClient provides a helper for FastHTTP library users.
// Since FastHTTP doesn't use http.Client, this returns a wrapper function
// that can be used to instrument FastHTTP requests.
func InstrumentFastHTTPClient(telemetryClient TelemetryClient) func(string, string, interface{}, interface{}) error {
	return func(method, reqURL string, requestBody, response interface{}) error {
		startTime := time.Now()
		
		// For FastHTTP, users would need to implement their own request logic
		// This is a placeholder that shows the pattern
		
		// Track the dependency
		duration := time.Since(startTime)
		
		// Create dependency telemetry
		name := method
		if u, err := url.Parse(reqURL); err == nil && u.Path != "" {
			name += " " + u.Path
		}
		
		dependency := NewRemoteDependencyTelemetry(name, "HTTP", "", true)
		dependency.Duration = duration
		dependency.Data = reqURL
		dependency.Timestamp = startTime
		
		telemetryClient.Track(dependency)
		
		return nil
	}
}

// Generic HTTP client instrumentor for any library that uses http.Client
type HTTPClientInstrumentor struct {
	telemetryClient TelemetryClient
}

// NewHTTPClientInstrumentor creates a new instrumentor that can be used with
// any HTTP library that exposes its underlying http.Client.
func NewHTTPClientInstrumentor(telemetryClient TelemetryClient) *HTTPClientInstrumentor {
	return &HTTPClientInstrumentor{
		telemetryClient: telemetryClient,
	}
}

// InstrumentClient instruments an http.Client with Application Insights tracking.
func (i *HTTPClientInstrumentor) InstrumentClient(client *http.Client) {
	if client.Transport == nil {
		client.Transport = http.DefaultTransport
	}
	client.Transport = NewInstrumentedTransportWithBase(client.Transport, i.telemetryClient)
}

// InstrumentTransport instruments an http.RoundTripper with Application Insights tracking.
func (i *HTTPClientInstrumentor) InstrumentTransport(transport http.RoundTripper) http.RoundTripper {
	return NewInstrumentedTransportWithBase(transport, i.telemetryClient)
}

// WrapHandlerFunc wraps an http.HandlerFunc to work with instrumented clients.
// This is useful for testing or when you need to pass an instrumented client
// to a function that expects an http.HandlerFunc.
func (i *HTTPClientInstrumentor) WrapHandlerFunc(handler func(*http.Client)) func(*http.Client) {
	return func(client *http.Client) {
		i.InstrumentClient(client)
		handler(client)
	}
}