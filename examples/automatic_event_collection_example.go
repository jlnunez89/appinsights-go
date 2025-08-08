// +build ignore

package main

// Automatic Event Collection Example
// This example uses the "+build ignore" directive to prevent it from being
// built as part of the main package when building with "go build ./...".
//
// To run this example:
//   go run examples/automatic_event_collection_example.go

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
)

func main() {
	fmt.Println("Application Insights - Automatic Event Collection Example")
	fmt.Println("========================================================")
	
	// Create telemetry configuration with automatic event collection
	config := appinsights.NewTelemetryConfiguration("your-instrumentation-key")
	
	// Configure automatic event collection with custom settings
	config.AutoCollection = appinsights.NewAutoCollectionConfig()
	
	// Customize HTTP auto-collection settings
	config.AutoCollection.HTTP.MaxURLLength = 1024
	config.AutoCollection.HTTP.HeaderCollection = []string{"User-Agent", "Referer"}
	
	// Customize database auto-collection settings
	config.AutoCollection.Database.MaxQueryLength = 512
	config.AutoCollection.Database.CollectParameters = false // Keep disabled for security
	
	// Customize error auto-collection settings
	config.AutoCollection.Errors.MaxStackFrames = 20
	config.AutoCollection.Errors.IgnoredErrors = []string{"connection reset", "context canceled"}
	
	// Customize performance counter collection
	config.AutoCollection.PerformanceCounters.CollectionInterval = 30 * time.Second
	
	// Create telemetry client with auto-collection
	client := appinsights.NewTelemetryClientFromConfig(config)
	defer client.Channel().Close()
	
	// Get the auto-collection manager
	autoCollection := client.AutoCollection()
	if autoCollection == nil {
		log.Fatal("Auto-collection not available")
	}
	
	// Start auto-collection (begins performance counter collection)
	autoCollection.Start()
	defer autoCollection.Stop()
	
	fmt.Println("✓ Auto-collection started")
	
	// Example 1: HTTP Server with automatic request tracking
	fmt.Println("\n1. HTTP Server Auto-Collection:")
	runHTTPServerExample(autoCollection)
	
	// Example 2: HTTP Client with automatic dependency tracking
	fmt.Println("\n2. HTTP Client Auto-Collection:")
	runHTTPClientExample(autoCollection)
	
	// Example 3: Database operations with automatic tracking
	fmt.Println("\n3. Database Auto-Collection:")
	runDatabaseExample(autoCollection)
	
	// Example 4: Error auto-collection
	fmt.Println("\n4. Error Auto-Collection:")
	runErrorCollectionExample(autoCollection)
	
	// Example 5: Performance counters (automatic background collection)
	fmt.Println("\n5. Performance Counter Auto-Collection:")
	fmt.Println("   Performance counters are being collected automatically in the background")
	fmt.Println("   Check your Application Insights instance for runtime.* and system.* metrics")
	
	// Wait a bit for all telemetry to be sent
	fmt.Println("\nWaiting for telemetry transmission...")
	time.Sleep(5 * time.Second)
	
	fmt.Println("✓ Automatic event collection example completed")
	fmt.Println("Check your Application Insights instance for the collected telemetry:")
	fmt.Println("  - HTTP requests and dependencies")
	fmt.Println("  - Database operations")
	fmt.Println("  - Error tracking")
	fmt.Println("  - Performance metrics")
}

func runHTTPServerExample(autoCollection *appinsights.AutoCollectionManager) {
	// Create a simple HTTP handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate processing time
		time.Sleep(10 * time.Millisecond)
		
		switch r.URL.Path {
		case "/api/users":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"users": ["alice", "bob"]}`))
		case "/api/error":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		}
	})
	
	// Wrap handler with automatic request tracking
	wrappedHandler := autoCollection.WrapHTTPHandler(handler)
	
	// This would normally start a test server, but for this example we just show the setup
	_ = wrappedHandler // Use the wrapped handler
	
	fmt.Println("   HTTP server wrapped with automatic request tracking")
	fmt.Println("   Requests will be automatically tracked with timing, status codes, and URLs")
}

func runHTTPClientExample(autoCollection *appinsights.AutoCollectionManager) {
	// Create HTTP client with automatic dependency tracking
	httpClient := autoCollection.WrapHTTPClient(&http.Client{
		Timeout: 10 * time.Second,
	})
	
	// Simulate outbound HTTP requests
	simulateHTTPRequest := func(url string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			fmt.Printf("   Error creating request to %s: %v\n", url, err)
			return
		}
		
		resp, err := httpClient.Do(req)
		if err != nil {
			fmt.Printf("   HTTP request to %s failed: %v\n", url, err)
			return
		}
		defer resp.Body.Close()
		
		fmt.Printf("   HTTP request to %s completed: %d\n", url, resp.StatusCode)
	}
	
	// Make some sample requests
	simulateHTTPRequest("https://httpbin.org/status/200")
	simulateHTTPRequest("https://httpbin.org/status/404")
	simulateHTTPRequest("https://httpbin.org/delay/1")
	
	fmt.Println("   HTTP client wrapped with automatic dependency tracking")
	fmt.Println("   Outbound requests are automatically tracked with timing and success status")
}

func runDatabaseExample(autoCollection *appinsights.AutoCollectionManager) {
	// Note: In a real application, you would register the instrumented driver
	// before opening the database connection
	
	// Example of how to register an instrumented database driver:
	// appinsights.RegisterDatabaseDriver(
	//     autoCollection.client,
	//     autoCollection.config.Database,
	//     "mysql_instrumented",
	//     mysql.NewMySQLDriver(),
	// )
	
	// db, err := sql.Open("mysql_instrumented", "user:password@tcp(localhost:3306)/database")
	
	fmt.Println("   Database operations would be automatically tracked:")
	fmt.Println("   - SQL queries with timing and success status")
	fmt.Println("   - Connection events (if enabled)")
	fmt.Println("   - Transaction operations (commit/rollback)")
	fmt.Println("   - Automatic query sanitization for security")
	
	// Simulate what would happen with instrumented database operations
	fmt.Println("   Example tracked operations:")
	fmt.Println("     INSERT INTO users (name, email) VALUES (?, ?) - 5ms - Success")
	fmt.Println("     SELECT * FROM users WHERE id = ? - 2ms - Success")
	fmt.Println("     UPDATE users SET last_login = NOW() WHERE id = ? - 3ms - Success")
	fmt.Println("     DELETE FROM sessions WHERE expired < ? - 8ms - Success")
}

func runErrorCollectionExample(autoCollection *appinsights.AutoCollectionManager) {
	// Example 1: Manual error tracking
	err := fmt.Errorf("user validation failed: invalid email address")
	autoCollection.TrackError(err)
	fmt.Println("   ✓ Tracked manual error")
	
	// Example 2: Error tracking with context
	ctx := context.WithValue(context.Background(), "userID", "12345")
	autoCollection.TrackErrorWithContext(ctx, "database connection timeout")
	fmt.Println("   ✓ Tracked error with context")
	
	// Example 3: Panic recovery
	autoCollection.RecoverPanic(func() {
		// This would normally panic, but is safely recovered
		// panic("simulated panic for testing")
		fmt.Println("   ✓ Function executed safely (panic recovery ready)")
	})
	
	// Example 4: Error with automatic filtering and sanitization
	sensitiveErr := fmt.Errorf("authentication failed for user password=secret123 token=abc123")
	autoCollection.TrackError(sensitiveErr)
	fmt.Println("   ✓ Tracked error with sensitive data (automatically sanitized)")
	
	fmt.Println("   Error auto-collection features:")
	fmt.Println("   - Automatic stack trace collection")
	fmt.Println("   - Sensitive data sanitization")
	fmt.Println("   - Error filtering and categorization")
	fmt.Println("   - Panic recovery and tracking")
}

// Example of custom performance counter collector
func createCustomPerformanceCounters() *appinsights.CustomPerformanceCounterCollector {
	return appinsights.NewCustomPerformanceCounterCollector("Application Metrics", func() map[string]float64 {
		return map[string]float64{
			"app.active_users":    150.0,
			"app.queue_length":    5.0,
			"app.cache_hit_rate":  0.85,
			"app.response_time":   125.5,
		}
	})
}

// Example of HTTP middleware integration with popular frameworks
func exampleFrameworkIntegration(autoCollection *appinsights.AutoCollectionManager) {
	// Gin example
	fmt.Println("Gin Framework Integration:")
	fmt.Println("  router.Use(autoCollection.HTTPMiddleware().GinMiddleware())")
	
	// Echo example  
	fmt.Println("Echo Framework Integration:")
	fmt.Println("  e.Use(autoCollection.HTTPMiddleware().EchoMiddleware())")
	
	// Standard net/http example
	fmt.Println("Standard net/http Integration:")
	fmt.Println("  http.Handle(\"/\", autoCollection.WrapHTTPHandler(handler))")
}