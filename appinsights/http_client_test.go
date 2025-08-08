package appinsights

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNewHTTPClient(t *testing.T) {
	client := NewTelemetryClient("test-key")
	httpClient := NewHTTPClient(client)

	if httpClient == nil {
		t.Fatal("NewHTTPClient returned nil")
	}
	if httpClient.Client == nil {
		t.Error("HTTPClient.Client should not be nil")
	}
	if httpClient.TelemetryClient != client {
		t.Error("HTTPClient.TelemetryClient should match provided client")
	}
	if !httpClient.SanitizeURL {
		t.Error("HTTPClient.SanitizeURL should default to true")
	}
	if len(httpClient.SensitiveQueryParams) == 0 {
		t.Error("HTTPClient.SensitiveQueryParams should have default values")
	}
}

func TestNewHTTPClientWithClient(t *testing.T) {
	client := NewTelemetryClient("test-key")
	underlyingClient := &http.Client{Timeout: 30 * time.Second}
	httpClient := NewHTTPClientWithClient(underlyingClient, client)

	if httpClient == nil {
		t.Fatal("NewHTTPClientWithClient returned nil")
	}
	if httpClient.Client != underlyingClient {
		t.Error("HTTPClient.Client should match provided client")
	}
	if httpClient.Client.Timeout != 30*time.Second {
		t.Error("HTTPClient should preserve underlying client properties")
	}
}

func TestHTTPClientDo(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	}))
	defer server.Close()

	// Create a mock telemetry client to capture telemetry
	var capturedTelemetry *RemoteDependencyTelemetry
	telemetryClient := &mockTelemetryClient{
		trackFunc: func(telemetry interface{}) {
			if dep, ok := telemetry.(*RemoteDependencyTelemetry); ok {
				capturedTelemetry = dep
			}
		},
	}

	// Create instrumented HTTP client
	httpClient := NewHTTPClient(telemetryClient)

	// Make a request
	req, err := http.NewRequest("GET", server.URL+"/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	if string(body) != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got %s", string(body))
	}

	// Verify telemetry was captured
	if capturedTelemetry == nil {
		t.Fatal("Expected dependency telemetry to be captured")
	}

	if capturedTelemetry.Type != "HTTP" {
		t.Errorf("Expected dependency type 'HTTP', got %s", capturedTelemetry.Type)
	}
	if !strings.Contains(capturedTelemetry.Name, "GET") {
		t.Errorf("Expected dependency name to contain 'GET', got %s", capturedTelemetry.Name)
	}
	if capturedTelemetry.ResultCode != "200" {
		t.Errorf("Expected result code '200', got %s", capturedTelemetry.ResultCode)
	}
	if !capturedTelemetry.Success {
		t.Error("Expected success to be true")
	}
	if capturedTelemetry.Duration <= 0 {
		t.Error("Expected duration to be > 0")
	}
}

func TestHTTPClientDoWithContext(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify correlation headers are present
		if r.Header.Get(TraceParentHeader) == "" {
			t.Error("Expected W3C traceparent header to be present")
		}
		if r.Header.Get(RequestIDHeader) == "" {
			t.Error("Expected Request-Id header to be present")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create telemetry client
	telemetryClient := NewTelemetryClient("test-key")
	httpClient := NewHTTPClient(telemetryClient)

	// Create correlation context
	corrCtx := NewCorrelationContext()
	ctx := WithCorrelationContext(context.Background(), corrCtx)

	// Make request with context
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := httpClient.DoWithContext(ctx, req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestHTTPClientGet(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("GET response"))
	}))
	defer server.Close()

	// Create instrumented HTTP client
	telemetryClient := NewTelemetryClient("test-key")
	httpClient := NewHTTPClient(telemetryClient)

	// Make GET request
	resp, err := httpClient.Get(server.URL + "/get-test")
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	if string(body) != "GET response" {
		t.Errorf("Expected 'GET response', got %s", string(body))
	}
}

func TestHTTPClientPost(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
		}
		
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(fmt.Sprintf("Received: %s", string(body))))
	}))
	defer server.Close()

	// Create instrumented HTTP client
	telemetryClient := NewTelemetryClient("test-key")
	httpClient := NewHTTPClient(telemetryClient)

	// Test POST with string body
	testData := "test data"
	resp, err := httpClient.Post(server.URL+"/post-test", "text/plain", testData)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	expected := "Received: " + testData
	if string(body) != expected {
		t.Errorf("Expected '%s', got %s", expected, string(body))
	}
}

func TestHTTPClientPostWithJSON(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		
		var data map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			t.Errorf("Failed to decode JSON: %v", err)
		}
		
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"received": data})
	}))
	defer server.Close()

	// Create instrumented HTTP client
	telemetryClient := NewTelemetryClient("test-key")
	httpClient := NewHTTPClient(telemetryClient)

	// Test POST with struct that gets marshaled to JSON
	testData := map[string]interface{}{
		"name":  "test",
		"value": 42,
	}
	
	resp, err := httpClient.Post(server.URL+"/json-test", "", testData)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestHTTPClientErrorHandling(t *testing.T) {
	// Create a test server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	// Create a mock telemetry client to capture telemetry
	var capturedTelemetry *RemoteDependencyTelemetry
	telemetryClient := &mockTelemetryClient{
		trackFunc: func(telemetry interface{}) {
			if dep, ok := telemetry.(*RemoteDependencyTelemetry); ok {
				capturedTelemetry = dep
			}
		},
	}

	// Create instrumented HTTP client
	httpClient := NewHTTPClient(telemetryClient)

	// Make request to endpoint that returns error
	resp, err := httpClient.Get(server.URL + "/error")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}

	// Verify telemetry captured the error
	if capturedTelemetry == nil {
		t.Fatal("Expected dependency telemetry to be captured")
	}

	if capturedTelemetry.ResultCode != "500" {
		t.Errorf("Expected result code '500', got %s", capturedTelemetry.ResultCode)
	}
	if capturedTelemetry.Success {
		t.Error("Expected success to be false for 500 status")
	}
}

func TestHTTPClientNetworkError(t *testing.T) {
	// Create a mock telemetry client to capture telemetry
	var capturedTelemetry *RemoteDependencyTelemetry
	telemetryClient := &mockTelemetryClient{
		trackFunc: func(telemetry interface{}) {
			if dep, ok := telemetry.(*RemoteDependencyTelemetry); ok {
				capturedTelemetry = dep
			}
		},
	}

	// Create instrumented HTTP client
	httpClient := NewHTTPClient(telemetryClient)

	// Make request to non-existent server
	_, err := httpClient.Get("http://localhost:99999/nonexistent")
	if err == nil {
		t.Fatal("Expected network error, but request succeeded")
	}

	// Verify telemetry captured the network error
	if capturedTelemetry == nil {
		t.Fatal("Expected dependency telemetry to be captured")
	}

	if capturedTelemetry.ResultCode != "0" {
		t.Errorf("Expected result code '0' for network error, got %s", capturedTelemetry.ResultCode)
	}
	if capturedTelemetry.Success {
		t.Error("Expected success to be false for network error")
	}
	if capturedTelemetry.Properties["error"] == "" {
		t.Error("Expected error property to be set")
	}
}

func TestURLSanitization(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create a mock telemetry client to capture telemetry
	var capturedTelemetry *RemoteDependencyTelemetry
	telemetryClient := &mockTelemetryClient{
		trackFunc: func(telemetry interface{}) {
			if dep, ok := telemetry.(*RemoteDependencyTelemetry); ok {
				capturedTelemetry = dep
			}
		},
	}

	// Create instrumented HTTP client
	httpClient := NewHTTPClient(telemetryClient)

	// Make request with sensitive data in URL
	urlWithSecrets := server.URL + "/api/test?password=secret123&api_key=abcd1234&normal_param=value"
	resp, err := httpClient.Get(urlWithSecrets)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify telemetry was captured
	if capturedTelemetry == nil {
		t.Fatal("Expected dependency telemetry to be captured")
	}

	// Verify sensitive data was sanitized
	if strings.Contains(capturedTelemetry.Data, "secret123") {
		t.Error("Expected password to be redacted from tracked URL")
	}
	if strings.Contains(capturedTelemetry.Data, "abcd1234") {
		t.Error("Expected api_key to be redacted from tracked URL")
	}
	if !strings.Contains(capturedTelemetry.Data, "normal_param=value") {
		t.Error("Expected normal parameters to be preserved")
	}
	
	// Check for redacted content (URL encoded)
	if !strings.Contains(capturedTelemetry.Data, "%5BREDACTED%5D") && !strings.Contains(capturedTelemetry.Data, "[REDACTED]") {
		t.Errorf("Expected redacted parameters to show [REDACTED] or URL-encoded equivalent, got: %s", capturedTelemetry.Data)
	}
}

func TestURLSanitizationDisabled(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create a mock telemetry client to capture telemetry
	var capturedTelemetry *RemoteDependencyTelemetry
	telemetryClient := &mockTelemetryClient{
		trackFunc: func(telemetry interface{}) {
			if dep, ok := telemetry.(*RemoteDependencyTelemetry); ok {
				capturedTelemetry = dep
			}
		},
	}

	// Create instrumented HTTP client with sanitization disabled
	httpClient := NewHTTPClient(telemetryClient)
	httpClient.SanitizeURL = false

	// Make request with sensitive data in URL
	urlWithSecrets := server.URL + "/api/test?password=secret123&api_key=abcd1234"
	resp, err := httpClient.Get(urlWithSecrets)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify telemetry was captured
	if capturedTelemetry == nil {
		t.Fatal("Expected dependency telemetry to be captured")
	}

	// Verify sensitive data was NOT sanitized
	if !strings.Contains(capturedTelemetry.Data, "password=secret123") {
		t.Error("Expected password to be preserved when sanitization is disabled")
	}
	if !strings.Contains(capturedTelemetry.Data, "api_key=abcd1234") {
		t.Error("Expected api_key to be preserved when sanitization is disabled")
	}
}

func TestCustomSensitiveParams(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create a mock telemetry client to capture telemetry
	var capturedTelemetry *RemoteDependencyTelemetry
	telemetryClient := &mockTelemetryClient{
		trackFunc: func(telemetry interface{}) {
			if dep, ok := telemetry.(*RemoteDependencyTelemetry); ok {
				capturedTelemetry = dep
			}
		},
	}

	// Create instrumented HTTP client with custom sensitive params
	httpClient := NewHTTPClient(telemetryClient)
	httpClient.SensitiveQueryParams = []string{"custom_secret", "private_data"}

	// Make request with custom sensitive data in URL
	urlWithSecrets := server.URL + "/api/test?custom_secret=mysecret&private_data=private&normal_param=value"
	resp, err := httpClient.Get(urlWithSecrets)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify telemetry was captured
	if capturedTelemetry == nil {
		t.Fatal("Expected dependency telemetry to be captured")
	}

	// Verify custom sensitive data was sanitized
	if strings.Contains(capturedTelemetry.Data, "mysecret") {
		t.Error("Expected custom_secret to be redacted from tracked URL")
	}
	if strings.Contains(capturedTelemetry.Data, "private") && !strings.Contains(capturedTelemetry.Data, "private_data=%5BREDACTED%5D") && !strings.Contains(capturedTelemetry.Data, "private_data=[REDACTED]") {
		t.Error("Expected private_data to be redacted from tracked URL")
	}
	if !strings.Contains(capturedTelemetry.Data, "normal_param=value") {
		t.Error("Expected normal parameters to be preserved")
	}
}

func TestWrapClient(t *testing.T) {
	telemetryClient := NewTelemetryClient("test-key")
	underlyingClient := &http.Client{Timeout: 30 * time.Second}
	
	wrappedClient := WrapClient(underlyingClient, telemetryClient)
	
	if wrappedClient == nil {
		t.Fatal("WrapClient returned nil")
	}
	if wrappedClient.Client != underlyingClient {
		t.Error("WrapClient should preserve the underlying client")
	}
	if wrappedClient.TelemetryClient != telemetryClient {
		t.Error("WrapClient should use the provided telemetry client")
	}
}

func TestWrapDefaultClient(t *testing.T) {
	telemetryClient := NewTelemetryClient("test-key")
	
	wrappedClient := WrapDefaultClient(telemetryClient)
	
	if wrappedClient == nil {
		t.Fatal("WrapDefaultClient returned nil")
	}
	if wrappedClient.Client != http.DefaultClient {
		t.Error("WrapDefaultClient should use http.DefaultClient")
	}
	if wrappedClient.TelemetryClient != telemetryClient {
		t.Error("WrapDefaultClient should use the provided telemetry client")
	}
}

func TestNewInstrumentedTransport(t *testing.T) {
	telemetryClient := NewTelemetryClient("test-key")
	
	transport := NewInstrumentedTransport(telemetryClient)
	
	if transport == nil {
		t.Fatal("NewInstrumentedTransport returned nil")
	}
	
	// Test that it implements http.RoundTripper
	var _ http.RoundTripper = transport
}

func TestNewInstrumentedTransportWithBase(t *testing.T) {
	telemetryClient := NewTelemetryClient("test-key")
	baseTransport := &http.Transport{}
	
	transport := NewInstrumentedTransportWithBase(baseTransport, telemetryClient)
	
	if transport == nil {
		t.Fatal("NewInstrumentedTransportWithBase returned nil")
	}
	
	// Test that it implements http.RoundTripper
	var _ http.RoundTripper = transport
	
	// Verify it uses the base transport
	instrumentedRT := transport.(*instrumentedRoundTripper)
	if instrumentedRT.base != baseTransport {
		t.Error("NewInstrumentedTransportWithBase should use the provided base transport")
	}
}

func TestInstrumentedTransportWithDisabledTelemetry(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create disabled telemetry client
	telemetryClient := NewTelemetryClient("test-key")
	telemetryClient.SetIsEnabled(false)

	// Create HTTP client with instrumented transport
	client := &http.Client{
		Transport: NewInstrumentedTransport(telemetryClient),
	}

	// Make request
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response (should work normally even with disabled telemetry)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestDependencyTelemetryProperties(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Create a mock telemetry client to capture telemetry
	var capturedTelemetry *RemoteDependencyTelemetry
	telemetryClient := &mockTelemetryClient{
		trackFunc: func(telemetry interface{}) {
			if dep, ok := telemetry.(*RemoteDependencyTelemetry); ok {
				capturedTelemetry = dep
			}
		},
	}

	// Create instrumented HTTP client
	httpClient := NewHTTPClient(telemetryClient)

	// Make request
	resp, err := httpClient.Get(server.URL + "/api/v1/test")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify telemetry properties
	if capturedTelemetry == nil {
		t.Fatal("Expected dependency telemetry to be captured")
	}

	// Check basic properties
	if capturedTelemetry.Type != "HTTP" {
		t.Errorf("Expected Type 'HTTP', got %s", capturedTelemetry.Type)
	}
	if !strings.Contains(capturedTelemetry.Name, "GET") {
		t.Errorf("Expected Name to contain 'GET', got %s", capturedTelemetry.Name)
	}
	if !strings.Contains(capturedTelemetry.Name, "/api/v1/test") {
		t.Errorf("Expected Name to contain path, got %s", capturedTelemetry.Name)
	}
	if capturedTelemetry.ResultCode != "202" {
		t.Errorf("Expected ResultCode '202', got %s", capturedTelemetry.ResultCode)
	}
	if !capturedTelemetry.Success {
		t.Error("Expected Success to be true for 202 status")
	}

	// Check target (should be the server host)
	parsedURL, _ := url.Parse(server.URL)
	if capturedTelemetry.Target != parsedURL.Host {
		t.Errorf("Expected Target %s, got %s", parsedURL.Host, capturedTelemetry.Target)
	}

	// Check additional properties
	if capturedTelemetry.Properties == nil {
		t.Fatal("Expected Properties to be set")
	}
	if capturedTelemetry.Properties["httpMethod"] != "GET" {
		t.Errorf("Expected httpMethod 'GET', got %s", capturedTelemetry.Properties["httpMethod"])
	}
	if capturedTelemetry.Properties["httpStatusCode"] != "202" {
		t.Errorf("Expected httpStatusCode '202', got %s", capturedTelemetry.Properties["httpStatusCode"])
	}

	// Check timing
	if capturedTelemetry.Duration <= 0 {
		t.Error("Expected Duration > 0")
	}
	if capturedTelemetry.Timestamp.IsZero() {
		t.Error("Expected Timestamp to be set")
	}
}

func TestCorrelationContextIntegration(t *testing.T) {
	// Create a test server that checks for correlation headers
	var receivedTraceParent, receivedRequestID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTraceParent = r.Header.Get(TraceParentHeader)
		receivedRequestID = r.Header.Get(RequestIDHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create a mock telemetry client to capture telemetry
	var capturedTelemetry *RemoteDependencyTelemetry
	telemetryClient := &mockTelemetryClient{
		trackFunc: func(telemetry interface{}) {
			if dep, ok := telemetry.(*RemoteDependencyTelemetry); ok {
				capturedTelemetry = dep
			}
		},
	}

	// Create instrumented HTTP client
	httpClient := NewHTTPClient(telemetryClient)

	// Create correlation context
	corrCtx := NewCorrelationContext()
	ctx := WithCorrelationContext(context.Background(), corrCtx)

	// Make request with correlation context
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := httpClient.DoWithContext(ctx, req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify correlation headers were sent
	if receivedTraceParent == "" {
		t.Error("Expected traceparent header to be sent")
	}
	if receivedRequestID == "" {
		t.Error("Expected Request-Id header to be sent")
	}

	// Verify telemetry includes correlation information
	if capturedTelemetry == nil {
		t.Fatal("Expected dependency telemetry to be captured")
	}
	
	// The dependency should have correlation ID from the child context
	if capturedTelemetry.Id == "" {
		t.Error("Expected dependency Id to be set from correlation context")
	}
}

// Mock HTTP round tripper for testing error scenarios
type errorRoundTripper struct{}

func (rt *errorRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("network connection failed")
}

func TestInstrumentedRoundTripperWithNetworkError(t *testing.T) {
	// Create a mock telemetry client to capture telemetry
	var capturedTelemetry *RemoteDependencyTelemetry
	telemetryClient := &mockTelemetryClient{
		trackFunc: func(telemetry interface{}) {
			if dep, ok := telemetry.(*RemoteDependencyTelemetry); ok {
				capturedTelemetry = dep
			}
		},
	}

	// Create instrumented round tripper with error-prone base
	rt := &instrumentedRoundTripper{
		base:            &errorRoundTripper{},
		telemetryClient: telemetryClient,
		sanitizeURL:     true,
	}

	// Make request that will fail
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	_, err := rt.RoundTrip(req)

	// Verify error is propagated
	if err == nil {
		t.Fatal("Expected error from round tripper")
	}
	if !strings.Contains(err.Error(), "network connection failed") {
		t.Errorf("Expected specific error message, got: %v", err)
	}

	// Verify telemetry captured the error
	if capturedTelemetry == nil {
		t.Fatal("Expected dependency telemetry to be captured")
	}
	if capturedTelemetry.Success {
		t.Error("Expected Success to be false for network error")
	}
	if capturedTelemetry.ResultCode != "0" {
		t.Errorf("Expected ResultCode '0' for network error, got %s", capturedTelemetry.ResultCode)
	}
	if capturedTelemetry.Properties["error"] == "" {
		t.Error("Expected error property to be set")
	}
}

func TestInstrumentHTTPLibrary(t *testing.T) {
	telemetryClient := NewTelemetryClient("test-key")
	
	// Test with no configuration function
	client := InstrumentHTTPLibrary(nil, telemetryClient)
	if client == nil {
		t.Fatal("InstrumentHTTPLibrary returned nil")
	}
	
	// Verify transport is instrumented
	if _, ok := client.Transport.(*instrumentedRoundTripper); !ok {
		t.Error("Expected transport to be instrumented")
	}
	
	// Test with configuration function
	client2 := InstrumentHTTPLibrary(func(c *http.Client) {
		c.Timeout = 30 * time.Second
	}, telemetryClient)
	
	if client2.Timeout != 30*time.Second {
		t.Error("Expected configuration function to be applied")
	}
	if _, ok := client2.Transport.(*instrumentedRoundTripper); !ok {
		t.Error("Expected transport to be instrumented after configuration")
	}
}

func TestNewHTTPClientInstrumentor(t *testing.T) {
	telemetryClient := NewTelemetryClient("test-key")
	instrumentor := NewHTTPClientInstrumentor(telemetryClient)
	
	if instrumentor == nil {
		t.Fatal("NewHTTPClientInstrumentor returned nil")
	}
	if instrumentor.telemetryClient != telemetryClient {
		t.Error("Instrumentor should use provided telemetry client")
	}
}

func TestHTTPClientInstrumentor_InstrumentClient(t *testing.T) {
	telemetryClient := NewTelemetryClient("test-key")
	instrumentor := NewHTTPClientInstrumentor(telemetryClient)
	
	client := &http.Client{Timeout: 10 * time.Second}
	
	instrumentor.InstrumentClient(client)
	
	// Verify transport was instrumented
	if _, ok := client.Transport.(*instrumentedRoundTripper); !ok {
		t.Error("Expected transport to be instrumented")
	}
	
	// Verify original properties are preserved
	if client.Timeout != 10*time.Second {
		t.Error("Expected client properties to be preserved")
	}
}

func TestInstrumentRestyClient(t *testing.T) {
	telemetryClient := NewTelemetryClient("test-key")
	
	// Mock Resty client with GetClient method
	mockRestyClient := &mockRestyClient{
		httpClient: &http.Client{},
	}
	
	result := InstrumentRestyClient(mockRestyClient, telemetryClient)
	
	if result != mockRestyClient {
		t.Error("Expected same client instance to be returned")
	}
	
	// Verify the underlying HTTP client was instrumented
	if _, ok := mockRestyClient.httpClient.Transport.(*instrumentedRoundTripper); !ok {
		t.Error("Expected underlying HTTP client to be instrumented")
	}
}

// Mock Resty client for testing
type mockRestyClient struct {
	httpClient *http.Client
}

func (m *mockRestyClient) GetClient() *http.Client {
	return m.httpClient
}