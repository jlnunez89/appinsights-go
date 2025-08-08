package main

// Example of using AppInsights middleware with Gin framework
// To run this example:
//   go mod init gin-example
//   go get github.com/gin-gonic/gin
//   go get github.com/microsoft/ApplicationInsights-Go/appinsights
//   go run gin_example.go

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/microsoft/ApplicationInsights-Go/appinsights"
)

// GinContext adapter to make it work with our middleware
type ginContextAdapter struct {
	*gin.Context
}

func (g *ginContextAdapter) Request() *http.Request {
	return g.Context.Request
}

func (g *ginContextAdapter) Writer() http.ResponseWriter {
	return g.Context.Writer
}

func (g *ginContextAdapter) Next() {
	g.Context.Next()
}

func (g *ginContextAdapter) Set(key string, value interface{}) {
	g.Context.Set(key, value)
}

func (g *ginContextAdapter) Get(key string) (interface{}, bool) {
	return g.Context.Get(key)
}

func main() {
	// Initialize Application Insights client
	client := appinsights.NewTelemetryClient("your-instrumentation-key")
	
	// Create HTTP middleware
	middleware := appinsights.NewHTTPMiddleware()
	
	// Configure the middleware to use your telemetry client
	middleware.GetClient = func(r *http.Request) appinsights.TelemetryClient {
		return client
	}

	// Create Gin router
	r := gin.Default()

	// Add AppInsights middleware using our Gin adapter
	ginMiddleware := middleware.GinMiddleware().(func(interface{}))
	r.Use(func(c *gin.Context) {
		adapter := &ginContextAdapter{Context: c}
		ginMiddleware(adapter)
	})

	// Define routes
	r.GET("/", func(c *gin.Context) {
		time.Sleep(25 * time.Millisecond) // Simulate processing
		c.JSON(http.StatusOK, gin.H{
			"message": "Hello from Gin!",
			"time":    time.Now(),
		})
	})

	r.GET("/error", func(c *gin.Context) {
		time.Sleep(100 * time.Millisecond) // Simulate processing
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong",
		})
	})

	r.POST("/data", func(c *gin.Context) {
		time.Sleep(75 * time.Millisecond) // Simulate processing
		c.JSON(http.StatusCreated, gin.H{
			"message": "Data created successfully",
			"id":      "12345",
		})
	})

	fmt.Println("Gin server starting on :8080")
	fmt.Println("Try these endpoints:")
	fmt.Println("  GET  http://localhost:8080/         (200 OK)")
	fmt.Println("  GET  http://localhost:8080/error    (500 Error)")
	fmt.Println("  POST http://localhost:8080/data     (201 Created)")
	
	// Start server
	r.Run(":8080")
}