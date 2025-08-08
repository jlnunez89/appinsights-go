// Package examples demonstrates how to use the HTTP Client Instrumentation
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
)

func main() {
	// Create a telemetry client with your instrumentation key
	client := appinsights.NewTelemetryClient("your-instrumentation-key-here")
	
	// Example 1: Basic HTTP Client Wrapper
	fmt.Println("=== Example 1: Basic HTTP Client Wrapper ===")
	httpClient := appinsights.NewHTTPClient(client)
	
	// Make requests - they will be automatically tracked
	resp, err := httpClient.Get("https://httpbin.org/get?param=value")
	if err != nil {
		log.Printf("Request failed: %v", err)
	} else {
		resp.Body.Close()
		fmt.Printf("GET request completed with status: %d\n", resp.StatusCode)
	}
	
	// Example 2: HTTP Client with Correlation Context
	fmt.Println("\n=== Example 2: HTTP Client with Correlation Context ===")
	corrCtx := appinsights.NewCorrelationContext()
	ctx := appinsights.WithCorrelationContext(context.Background(), corrCtx)
	
	resp, err = httpClient.GetWithContext(ctx, "https://httpbin.org/headers")
	if err != nil {
		log.Printf("Request with context failed: %v", err)
	} else {
		resp.Body.Close()
		fmt.Printf("GET request with correlation completed with status: %d\n", resp.StatusCode)
	}
	
	// Example 3: POST request
	fmt.Println("\n=== Example 3: POST Request ===")
	data := map[string]interface{}{
		"name": "Application Insights",
		"type": "HTTP Client Instrumentation",
	}
	
	resp, err = httpClient.Post("https://httpbin.org/post", "application/json", data)
	if err != nil {
		log.Printf("POST request failed: %v", err)
	} else {
		resp.Body.Close()
		fmt.Printf("POST request completed with status: %d\n", resp.StatusCode)
	}
	
	// Example 4: Wrapping an existing HTTP client
	fmt.Println("\n=== Example 4: Wrapping Existing HTTP Client ===")
	existingClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	wrappedClient := appinsights.WrapClient(existingClient, client)
	resp, err = wrappedClient.Get("https://httpbin.org/delay/1")
	if err != nil {
		log.Printf("Wrapped client request failed: %v", err)
	} else {
		resp.Body.Close()
		fmt.Printf("Wrapped client request completed with status: %d\n", resp.StatusCode)
	}
	
	// Example 5: Instrumenting transport directly
	fmt.Println("\n=== Example 5: Instrumenting Transport ===")
	directClient := &http.Client{
		Transport: appinsights.NewInstrumentedTransport(client),
		Timeout:   15 * time.Second,
	}
	
	resp, err = directClient.Get("https://httpbin.org/status/200")
	if err != nil {
		log.Printf("Direct transport client request failed: %v", err)
	} else {
		resp.Body.Close()
		fmt.Printf("Direct transport client request completed with status: %d\n", resp.StatusCode)
	}
	
	// Example 6: URL Sanitization
	fmt.Println("\n=== Example 6: URL Sanitization ===")
	sanitizedClient := appinsights.NewHTTPClient(client)
	// Configure custom sensitive parameters
	sanitizedClient.SensitiveQueryParams = []string{"secret", "password", "api_key", "token"}
	
	// This request will have sensitive data redacted in telemetry
	resp, err = sanitizedClient.Get("https://httpbin.org/get?secret=mysecret&normal=value")
	if err != nil {
		log.Printf("Sanitized request failed: %v", err)
	} else {
		resp.Body.Close()
		fmt.Printf("Sanitized request completed with status: %d\n", resp.StatusCode)
	}
	
	// Example 7: Generic HTTP Library Instrumentation
	fmt.Println("\n=== Example 7: Generic HTTP Library Instrumentation ===")
	instrumentor := appinsights.NewHTTPClientInstrumentor(client)
	
	// Instrument any http.Client
	anyClient := &http.Client{}
	instrumentor.InstrumentClient(anyClient)
	
	resp, err = anyClient.Get("https://httpbin.org/json")
	if err != nil {
		log.Printf("Instrumented client request failed: %v", err)
	} else {
		resp.Body.Close()
		fmt.Printf("Instrumented client request completed with status: %d\n", resp.StatusCode)
	}
	
	// Example 8: Error tracking
	fmt.Println("\n=== Example 8: Error Tracking ===")
	// This will track a failed dependency
	resp, err = httpClient.Get("https://httpbin.org/status/500")
	if err != nil {
		log.Printf("Error request failed: %v", err)
	} else {
		resp.Body.Close()
		fmt.Printf("Error request completed with status: %d (tracked as failed dependency)\n", resp.StatusCode)
	}
	
	// Wait a moment for telemetry to be sent
	fmt.Println("\nWaiting for telemetry to be sent...")
	time.Sleep(2 * time.Second)
	
	// Gracefully shut down the telemetry client
	select {
	case <-client.Channel().Close(5 * time.Second):
		fmt.Println("Telemetry sent successfully")
	case <-time.After(10 * time.Second):
		fmt.Println("Timeout waiting for telemetry to be sent")
	}
	
	fmt.Println("\nDone! Check your Application Insights dashboard for the tracked HTTP dependencies.")
}