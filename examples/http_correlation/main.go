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
	// Create Application Insights client
	client := appinsights.NewTelemetryClient("your-instrumentation-key")

	// Create HTTP middleware
	middleware := appinsights.NewHTTPMiddleware()

	// Optional: Set up client getter for automatic request tracking
	middleware.GetClient = func(r *http.Request) appinsights.TelemetryClient {
		return client
	}

	// Create HTTP server with middleware
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", helloHandler)
	mux.HandleFunc("/downstream", downstreamHandler)

	// Wrap the handler with correlation middleware
	handler := middleware.Middleware(mux)

	// Start server
	fmt.Println("Server starting on :8080")
	fmt.Println("Try: curl -H 'traceparent: 00-12345678901234567890123456789012-1234567890123456-01' http://localhost:8080/hello")
	fmt.Println("Or:  curl -H 'Request-Id: |12345678901234567890123456789012.1234567890123456.' http://localhost:8080/hello")
	log.Fatal(http.ListenAndServe(":8080", handler))
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	// Get correlation context from request (automatically extracted by middleware)
	corrCtx := appinsights.GetCorrelationContext(r.Context())

	if corrCtx != nil {
		fmt.Fprintf(w, "Hello! Your trace ID is: %s\n", corrCtx.TraceID)
		fmt.Fprintf(w, "Your span ID is: %s\n", corrCtx.SpanID)
		if corrCtx.ParentSpanID != "" {
			fmt.Fprintf(w, "Your parent span ID is: %s\n", corrCtx.ParentSpanID)
		}

		// Make downstream request with automatic header propagation
		makeDownstreamRequest(r.Context())
	} else {
		fmt.Fprintf(w, "Hello! No correlation context found.\n")
	}
}

func downstreamHandler(w http.ResponseWriter, r *http.Request) {
	// This handler would receive correlation headers automatically
	corrCtx := appinsights.GetCorrelationContext(r.Context())

	if corrCtx != nil {
		fmt.Fprintf(w, "Downstream service received trace ID: %s\n", corrCtx.TraceID)
		fmt.Fprintf(w, "Downstream span ID: %s\n", corrCtx.SpanID)
		if corrCtx.ParentSpanID != "" {
			fmt.Fprintf(w, "Downstream parent span ID: %s\n", corrCtx.ParentSpanID)
		}
	} else {
		fmt.Fprintf(w, "Downstream: No correlation context\n")
	}
}

func makeDownstreamRequest(ctx context.Context) {
	// Create HTTP client with correlation support
	middleware := appinsights.NewHTTPMiddleware()
	client := &http.Client{
		Transport: middleware.WrapRoundTripper(http.DefaultTransport),
		Timeout:   5 * time.Second,
	}

	// Create request with context (correlation will be injected automatically)
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:8080/downstream", nil)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return
	}

	// Make request (headers will be injected automatically)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error making downstream request: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("Downstream request completed with status: %s", resp.Status)
}

// Example of manual header injection for outgoing requests
func manualHeaderExample() {
	// Create correlation context
	corrCtx := appinsights.NewCorrelationContext()
	corrCtx.OperationName = "ManualExample"

	// Create HTTP request
	req, _ := http.NewRequest("GET", "http://api.example.com/data", nil)

	// Manually inject headers
	middleware := appinsights.NewHTTPMiddleware()
	middleware.InjectHeaders(req, corrCtx)

	// Now req has both W3C and Request-Id headers
	fmt.Printf("W3C Header: %s\n", req.Header.Get("traceparent"))
	fmt.Printf("Request-Id Header: %s\n", req.Header.Get("Request-Id"))
}

// Example of manual header extraction from incoming requests
func manualExtractionExample(r *http.Request) {
	middleware := appinsights.NewHTTPMiddleware()

	// Extract correlation context from headers
	corrCtx := middleware.ExtractHeaders(r)

	if corrCtx != nil {
		fmt.Printf("Extracted trace ID: %s\n", corrCtx.TraceID)
		fmt.Printf("Extracted span ID: %s\n", corrCtx.SpanID)

		// Use for telemetry tracking
		ctx := appinsights.WithCorrelationContext(r.Context(), corrCtx)
		client := appinsights.NewTelemetryClient("your-key")
		client.TrackEventWithContext(ctx, "RequestProcessed")
	}
}