// +build ignore

package main

// Example of using AppInsights middleware with Echo framework
// This example uses the "+build ignore" directive to prevent it from being
// built as part of the main package (since it requires external dependencies).
//
// To run this example:
//   go mod init echo-example
//   go get github.com/labstack/echo/v4
//   go get github.com/microsoft/ApplicationInsights-Go/appinsights
//   go run echo_middleware_example.go

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/microsoft/ApplicationInsights-Go/appinsights"
)

// EchoContext adapter to make it work with our middleware
type echoContextAdapter struct {
	echo.Context
}

func (e *echoContextAdapter) Request() *http.Request {
	return e.Context.Request()
}

func (e *echoContextAdapter) Response() interface{
	Status() int
	Writer() http.ResponseWriter
} {
	return &echoResponseAdapter{e.Context.Response()}
}

func (e *echoContextAdapter) Set(key string, value interface{}) {
	e.Context.Set(key, value)
}

func (e *echoContextAdapter) Get(key string) interface{} {
	return e.Context.Get(key)
}

func (e *echoContextAdapter) SetRequest(req *http.Request) {
	e.Context.SetRequest(req)
}

type echoResponseAdapter struct {
	*echo.Response
}

func (r *echoResponseAdapter) Status() int {
	return r.Response.Status
}

func (r *echoResponseAdapter) Writer() http.ResponseWriter {
	return r.Response.Writer
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

	// Create Echo instance
	e := echo.New()

	// Add AppInsights middleware using our Echo adapter
	echoMiddlewareFactory := middleware.EchoMiddleware().(func(interface{}) interface{})
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		echoHandler := echoMiddlewareFactory(func(c interface{}) error {
			echoCtx := c.(echo.Context)
			return next(echoCtx)
		}).(func(interface{}) error)

		return func(c echo.Context) error {
			adapter := &echoContextAdapter{Context: c}
			return echoHandler(adapter)
		}
	})

	// Define routes
	e.GET("/", func(c echo.Context) error {
		time.Sleep(30 * time.Millisecond) // Simulate processing
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "Hello from Echo!",
			"time":    time.Now(),
		})
	})

	e.GET("/error", func(c echo.Context) error {
		time.Sleep(80 * time.Millisecond) // Simulate processing
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Something went wrong",
		})
	})

	e.PUT("/update/:id", func(c echo.Context) error {
		time.Sleep(60 * time.Millisecond) // Simulate processing
		id := c.Param("id")
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "Resource updated successfully",
			"id":      id,
		})
	})

	fmt.Println("Echo server starting on :8080")
	fmt.Println("Try these endpoints:")
	fmt.Println("  GET  http://localhost:8080/          (200 OK)")
	fmt.Println("  GET  http://localhost:8080/error     (500 Error)")
	fmt.Println("  PUT  http://localhost:8080/update/42 (200 OK)")
	
	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}