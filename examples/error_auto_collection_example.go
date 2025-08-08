package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
)

func main() {
	// Create telemetry client with error auto-collection
	config := appinsights.NewTelemetryConfiguration("your-instrumentation-key")
	
	// Configure error auto-collection
	config.ErrorAutoCollection = appinsights.NewErrorAutoCollectionConfig()
	config.ErrorAutoCollection.MaxStackFrames = 10
	config.ErrorAutoCollection.IgnoredErrors = []string{"ignored error"}
	
	// Add custom error filter
	config.ErrorAutoCollection.ErrorFilters = []appinsights.ErrorFilterFunc{
		func(err interface{}) bool {
			errStr := fmt.Sprintf("%v", err)
			return len(errStr) > 5 // Only track errors with messages longer than 5 chars
		},
	}
	
	// Add custom sanitizer
	config.ErrorAutoCollection.ErrorSanitizers = []appinsights.ErrorSanitizerFunc{
		appinsights.DefaultErrorSanitizer,
		appinsights.FilePathSanitizer,
	}
	
	client := appinsights.NewTelemetryClientFromConfig(config)
	defer client.Channel().Close()
	
	// Get the error auto-collector
	errorCollector := client.ErrorAutoCollector()
	if errorCollector == nil {
		log.Fatal("Error auto-collector not available")
	}
	
	fmt.Println("Error Auto-Collection Example")
	fmt.Println("=============================")
	
	// Example 1: Manual error tracking
	fmt.Println("1. Manual error tracking:")
	err := errors.New("This is a manually tracked error")
	errorCollector.TrackError(err)
	fmt.Printf("   Tracked error: %v\n", err)
	
	// Example 2: Error tracking with context
	fmt.Println("2. Error tracking with context:")
	ctx := context.Background()
	errorCollector.TrackErrorWithContext(ctx, "Error with context information")
	fmt.Println("   Tracked error with context")
	
	// Example 3: Panic recovery
	fmt.Println("3. Panic recovery:")
	errorCollector.RecoverPanic(func() {
		panic("This panic will be recovered and tracked")
	})
	fmt.Println("   Panic recovered and tracked")
	
	// Example 4: Function wrapping
	fmt.Println("4. Function wrapping:")
	wrappedFunc := errorCollector.WrapFunction(func() {
		panic("This panic in wrapped function will be tracked")
	})
	wrappedFunc()
	fmt.Println("   Wrapped function panic recovered and tracked")
	
	// Example 5: Error filtering (this error will be ignored)
	fmt.Println("5. Error filtering:")
	errorCollector.TrackError("ignored error message")
	fmt.Println("   Attempted to track ignored error (filtered out)")
	
	// Example 6: Error sanitization
	fmt.Println("6. Error sanitization:")
	sensitiveErr := errors.New("Database connection failed: password=secret123")
	errorCollector.TrackError(sensitiveErr)
	fmt.Println("   Tracked error with sensitive data (sanitized)")
	
	// Example 7: Chained errors (simulating error wrapping)
	fmt.Println("7. Chained errors:")
	rootErr := errors.New("root cause error")
	wrappedErr := fmt.Errorf("wrapped error: %w", rootErr)
	errorCollector.TrackError(wrappedErr)
	fmt.Printf("   Tracked wrapped error: %v\n", wrappedErr)
	
	// Example 8: Manual exception tracking (legacy compatibility)
	fmt.Println("8. Legacy exception tracking:")
	client.TrackException("Legacy exception tracking still works")
	fmt.Println("   Tracked exception using legacy method")
	
	// Wait a bit for telemetry to be sent
	time.Sleep(2 * time.Second)
	
	fmt.Println("\nError auto-collection examples completed.")
	fmt.Println("Check your Application Insights instance for the tracked errors.")
}