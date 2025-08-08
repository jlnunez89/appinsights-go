// +build ignore

package main

// HTTP Middleware Example
// This example uses the "+build ignore" directive to prevent it from being
// built as part of the main package when building with "go build ./...".
//
// To run this example:
//   go run examples/http_middleware_example.go

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
)

func main() {
	// Initialize Application Insights client
	client := appinsights.NewTelemetryClient("your-instrumentation-key")
	
	// Create HTTP middleware
	middleware := appinsights.NewHTTPMiddleware()
	
	// Configure the middleware to use your telemetry client
	middleware.GetClient = func(r *http.Request) appinsights.TelemetryClient {
		return client
	}

	// Create a simple handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate some processing time
		time.Sleep(50 * time.Millisecond)
		
		// Return different status codes based on path
		switch r.URL.Path {
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		case "/notfound":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		default:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Hello, World!"))
		}
	})

	// Wrap handler with middleware for automatic request tracking
	wrappedHandler := middleware.Middleware(handler)

	// Set up routes
	http.Handle("/", wrappedHandler)
	http.Handle("/error", wrappedHandler)
	http.Handle("/notfound", wrappedHandler)

	fmt.Println("Server starting on :8080")
	fmt.Println("Try these endpoints:")
	fmt.Println("  http://localhost:8080/         (200 OK)")
	fmt.Println("  http://localhost:8080/error    (500 Error)")
	fmt.Println("  http://localhost:8080/notfound (404 Not Found)")
	
	// Start server
	log.Fatal(http.ListenAndServe(":8080", nil))
}