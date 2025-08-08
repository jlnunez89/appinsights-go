package appinsights

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// HTTP header constants
const (
	// W3C Trace Context headers
	TraceParentHeader = "traceparent"
	TraceStateHeader  = "tracestate"

	// Request-Id header for backward compatibility
	RequestIDHeader = "Request-Id"

	// Application Insights specific headers
	RequestContextHeader         = "Request-Context"
	RequestContextCorrelationKey = "appId"
)

// responseWriter wraps http.ResponseWriter to capture status code and response size
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

// newResponseWriter creates a new response writer wrapper
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     200, // Default to 200 if WriteHeader is not called
	}
}

// WriteHeader captures the status code and calls the underlying WriteHeader
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the response size and calls the underlying Write
func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// Status returns the HTTP status code
func (rw *responseWriter) Status() int {
	return rw.statusCode
}

// Size returns the number of bytes written
func (rw *responseWriter) Size() int64 {
	return rw.written
}

// HTTPMiddleware provides HTTP middleware for automatic header injection and extraction
type HTTPMiddleware struct {
	// Optional callback to get the telemetry client for requests
	GetClient func(*http.Request) TelemetryClient
}

// NewHTTPMiddleware creates a new HTTP middleware instance
func NewHTTPMiddleware() *HTTPMiddleware {
	return &HTTPMiddleware{}
}

// ExtractHeaders extracts correlation context from HTTP request headers
// Supports both W3C Trace Context and Request-Id headers
func (m *HTTPMiddleware) ExtractHeaders(r *http.Request) *CorrelationContext {
	// Try W3C Trace Context first (preferred)
	if traceParent := r.Header.Get(TraceParentHeader); traceParent != "" {
		if corrCtx, err := ParseW3CTraceParent(traceParent); err == nil {
			// TODO: Handle tracestate header if needed in the future
			return corrCtx
		}
	}

	// Fall back to Request-Id header for backward compatibility
	if requestID := r.Header.Get(RequestIDHeader); requestID != "" {
		if corrCtx, err := ParseRequestID(requestID); err == nil {
			return corrCtx
		}
	}

	// No correlation headers found, return nil
	return nil
}

// InjectHeaders injects correlation headers into an HTTP request
// Adds both W3C Trace Context and Request-Id headers for compatibility
func (m *HTTPMiddleware) InjectHeaders(r *http.Request, corrCtx *CorrelationContext) {
	if corrCtx == nil {
		return
	}

	// Set W3C Trace Context header (primary)
	r.Header.Set(TraceParentHeader, corrCtx.ToW3CTraceParent())

	// Set Request-Id header for backward compatibility
	r.Header.Set(RequestIDHeader, corrCtx.ToRequestID())

	// TODO: Handle tracestate header if needed in the future
}

// Middleware returns an HTTP middleware function that automatically handles correlation
// and request tracking. This middleware extracts correlation context from incoming requests,
// makes it available in the request context, and tracks request telemetry with accurate
// timing, status codes, and URL information.
func (m *HTTPMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Record start time for request duration tracking
		startTime := time.Now()

		// Extract correlation context from headers
		corrCtx := m.ExtractHeaders(r)

		// If no correlation context found, create a new one for this request
		if corrCtx == nil {
			corrCtx = NewCorrelationContext()
		} else {
			// Create a child context for this request to maintain trace hierarchy
			corrCtx = NewChildCorrelationContext(corrCtx)
		}

		// Add correlation context to request context
		ctx := WithCorrelationContext(r.Context(), corrCtx)
		r = r.WithContext(ctx)

		// Wrap response writer to capture status code and response size
		rw := newResponseWriter(w)

		// Set correlation headers in response for client visibility
		m.setResponseHeaders(rw, corrCtx)

		// Call the next handler
		next.ServeHTTP(rw, r)

		// Track the request telemetry after completion if client getter is provided
		if m.GetClient != nil {
			if client := m.GetClient(r); client != nil {
				// Calculate request duration
				duration := time.Since(startTime)
				
				// Get status code as string
				responseCode := strconv.Itoa(rw.Status())
				
				// Track the completed request with accurate timing and status
				client.TrackRequestWithContext(ctx, r.Method, r.URL.String(), duration, responseCode)
			}
		}
	})
}

// setResponseHeaders sets correlation headers in the HTTP response
func (m *HTTPMiddleware) setResponseHeaders(w http.ResponseWriter, corrCtx *CorrelationContext) {
	if corrCtx == nil {
		return
	}

	// Set Request-Id header in response for client correlation
	w.Header().Set(RequestIDHeader, corrCtx.ToRequestID())
}

// WrapRoundTripper wraps an http.RoundTripper to automatically inject correlation headers
// into outgoing HTTP requests. This enables distributed tracing across service boundaries.
func (m *HTTPMiddleware) WrapRoundTripper(rt http.RoundTripper) http.RoundTripper {
	return &correlationRoundTripper{
		base:       rt,
		middleware: m,
	}
}

// correlationRoundTripper implements http.RoundTripper with automatic header injection
type correlationRoundTripper struct {
	base       http.RoundTripper
	middleware *HTTPMiddleware
}

// RoundTrip implements http.RoundTripper interface
func (rt *correlationRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Extract correlation context from request context
	if corrCtx := GetCorrelationContext(req.Context()); corrCtx != nil {
		// Create a child context for the outgoing request
		childCtx := NewChildCorrelationContext(corrCtx)

		// Inject headers into the outgoing request
		rt.middleware.InjectHeaders(req, childCtx)
	}

	// Use base round tripper to make the actual request
	base := rt.base
	if base == nil {
		base = http.DefaultTransport
	}

	return base.RoundTrip(req)
}

// ContextExtractor is a helper function to extract correlation context from HTTP requests
// This can be used in HTTP handlers to get correlation context for telemetry tracking
func ContextExtractor(r *http.Request) context.Context {
	middleware := NewHTTPMiddleware()
	corrCtx := middleware.ExtractHeaders(r)

	if corrCtx != nil {
		return WithCorrelationContext(r.Context(), corrCtx)
	}

	return r.Context()
}

// GetOrCreateCorrelationFromRequest extracts or creates correlation context from an HTTP request
// This is a convenience function for HTTP handlers that need correlation context
func GetOrCreateCorrelationFromRequest(r *http.Request) *CorrelationContext {
	middleware := NewHTTPMiddleware()
	corrCtx := middleware.ExtractHeaders(r)

	if corrCtx == nil {
		corrCtx = NewCorrelationContext()
	}

	return corrCtx
}

// GinMiddleware returns a Gin middleware function for automatic request tracking
// This middleware integrates with the Gin framework to provide correlation context
// and automatic telemetry tracking with proper timing and status codes.
//
// Usage:
//   middleware := appinsights.NewHTTPMiddleware()
//   middleware.GetClient = func(*http.Request) TelemetryClient { return client }
//   router.Use(middleware.GinMiddleware())
func (m *HTTPMiddleware) GinMiddleware() interface{} {
	// Return a function that matches Gin's middleware signature: func(*gin.Context)
	// We use interface{} to avoid importing gin in this core package
	return func(c interface{}) {
		// This will be called by Gin with a *gin.Context
		// We need to extract the http.Request and http.ResponseWriter from it
		ginContext := c.(interface {
			Request() *http.Request
			Writer() http.ResponseWriter
			Next()
			Set(string, interface{})
			Get(string) (interface{}, bool)
			SetRequest(*http.Request)
		})

		req := ginContext.Request()
		w := ginContext.Writer()
		
		// Record start time for request duration tracking
		startTime := time.Now()

		// Extract correlation context from headers
		corrCtx := m.ExtractHeaders(req)

		// If no correlation context found, create a new one for this request
		if corrCtx == nil {
			corrCtx = NewCorrelationContext()
		} else {
			// Create a child context for this request to maintain trace hierarchy
			corrCtx = NewChildCorrelationContext(corrCtx)
		}

		// Add correlation context to request context and Gin context
		ctx := WithCorrelationContext(req.Context(), corrCtx)
		ginContext.SetRequest(req.WithContext(ctx))
		ginContext.Set("appinsights_correlation", corrCtx)

		// Set correlation headers in response for client visibility
		m.setResponseHeaders(w, corrCtx)

		// Call the next middleware/handler
		ginContext.Next()

		// Track the request telemetry after completion if client getter is provided
		if m.GetClient != nil {
			if client := m.GetClient(req); client != nil {
				// Calculate request duration
				duration := time.Since(startTime)
				
				// Get status code - for Gin we need to get it from the writer
				statusCode := 200 // Default
				if rw, ok := w.(interface{ Status() int }); ok {
					statusCode = rw.Status()
				}
				responseCode := strconv.Itoa(statusCode)
				
				// Track the completed request with accurate timing and status
				client.TrackRequestWithContext(ctx, req.Method, req.URL.String(), duration, responseCode)
			}
		}
	}
}

// EchoMiddleware returns an Echo middleware function for automatic request tracking
// This middleware integrates with the Echo framework to provide correlation context
// and automatic telemetry tracking with proper timing and status codes.
//
// Usage:
//   middleware := appinsights.NewHTTPMiddleware()
//   middleware.GetClient = func(*http.Request) TelemetryClient { return client }
//   e.Use(middleware.EchoMiddleware())
func (m *HTTPMiddleware) EchoMiddleware() interface{} {
	// Return a function that matches Echo's middleware signature: func(echo.HandlerFunc) echo.HandlerFunc
	// We use interface{} to avoid importing echo in this core package
	return func(next interface{}) interface{} {
		// Return handler function that matches echo.HandlerFunc signature
		return func(c interface{}) error {
			// This will be called by Echo with an echo.Context
			echoContext := c.(interface {
				Request() *http.Request
				Response() interface {
					Status() int
					Writer() http.ResponseWriter
				}
				Set(string, interface{})
				Get(string) interface{}
				SetRequest(*http.Request)
			})

			req := echoContext.Request()
			res := echoContext.Response()
			
			// Record start time for request duration tracking
			startTime := time.Now()

			// Extract correlation context from headers
			corrCtx := m.ExtractHeaders(req)

			// If no correlation context found, create a new one for this request
			if corrCtx == nil {
				corrCtx = NewCorrelationContext()
			} else {
				// Create a child context for this request to maintain trace hierarchy
				corrCtx = NewChildCorrelationContext(corrCtx)
			}

			// Add correlation context to request context and Echo context
			ctx := WithCorrelationContext(req.Context(), corrCtx)
			echoContext.SetRequest(req.WithContext(ctx))
			echoContext.Set("appinsights_correlation", corrCtx)

			// Set correlation headers in response for client visibility
			m.setResponseHeaders(res.Writer(), corrCtx)

			// Call the next handler
			nextHandler := next.(func(interface{}) error)
			err := nextHandler(c)

			// Track the request telemetry after completion if client getter is provided
			if m.GetClient != nil {
				if client := m.GetClient(req); client != nil {
					// Calculate request duration
					duration := time.Since(startTime)
					
					// Get status code from Echo response
					responseCode := strconv.Itoa(res.Status())
					
					// Track the completed request with accurate timing and status
					client.TrackRequestWithContext(ctx, req.Method, req.URL.String(), duration, responseCode)
				}
			}

			return err
		}
	}
}
