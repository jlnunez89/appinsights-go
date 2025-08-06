package appinsights

import (
	"context"
	"net/http"
	"time"
)

// SpanContext represents a span with correlation context and telemetry client
type SpanContext struct {
	Context     *CorrelationContext
	Client      TelemetryClient
	StartTime   time.Time
	OperationID string
}

// StartSpan creates a new span with the given operation name
// If a parent context exists, creates a child span; otherwise creates a root span
func StartSpan(ctx context.Context, operationName string, client TelemetryClient) (context.Context, *SpanContext) {
	parentCorr := GetCorrelationContext(ctx)
	
	var corrCtx *CorrelationContext
	if parentCorr != nil {
		corrCtx = NewChildCorrelationContext(parentCorr)
	} else {
		corrCtx = NewCorrelationContext()
	}
	
	corrCtx.OperationName = operationName
	
	spanCtx := &SpanContext{
		Context:     corrCtx,
		Client:      client,
		StartTime:   time.Now(),
		OperationID: corrCtx.GetOperationID(),
	}
	
	newCtx := WithCorrelationContext(ctx, corrCtx)
	return newCtx, spanCtx
}

// FinishSpan completes a span and tracks it as a dependency or request telemetry
func (s *SpanContext) FinishSpan(ctx context.Context, success bool, properties map[string]string) {
	if s == nil || s.Client == nil {
		return
	}
	
	duration := time.Since(s.StartTime)
	
	// Track as a dependency by default
	dependency := NewRemoteDependencyTelemetryWithContext(ctx, s.Context.OperationName, "Internal", "", success)
	dependency.Duration = duration
	dependency.MarkTime(s.StartTime, time.Now())
	
	if properties != nil {
		for k, v := range properties {
			dependency.Properties[k] = v
		}
	}
	
	s.Client.TrackWithContext(ctx, dependency)
}

// WithSpan wraps a function with automatic span creation and completion
// The span will be automatically finished when the function returns
func WithSpan(ctx context.Context, operationName string, client TelemetryClient, fn func(context.Context) error) error {
	spanCtx, span := StartSpan(ctx, operationName, client)
	defer func() {
		if r := recover(); r != nil {
			span.FinishSpan(spanCtx, false, map[string]string{"error": "panic"})
			panic(r) // re-throw the panic
		}
	}()
	
	err := fn(spanCtx)
	success := err == nil
	
	properties := make(map[string]string)
	if err != nil {
		properties["error"] = err.Error()
	}
	
	span.FinishSpan(spanCtx, success, properties)
	return err
}

// StartOperation creates a new operation context with automatic request tracking
// This is useful for HTTP handlers and other operations that should be tracked as requests
func StartOperation(ctx context.Context, operationName string, client TelemetryClient) (context.Context, *OperationContext) {
	parentCorr := GetCorrelationContext(ctx)
	
	var corrCtx *CorrelationContext
	if parentCorr != nil {
		corrCtx = NewChildCorrelationContext(parentCorr)
	} else {
		corrCtx = NewCorrelationContext()
	}
	
	corrCtx.OperationName = operationName
	
	opCtx := &OperationContext{
		Context:       corrCtx,
		Client:        client,
		StartTime:     time.Now(),
		OperationName: operationName,
	}
	
	newCtx := WithCorrelationContext(ctx, corrCtx)
	return newCtx, opCtx
}

// OperationContext represents an operation with automatic request tracking
type OperationContext struct {
	Context       *CorrelationContext
	Client        TelemetryClient
	StartTime     time.Time
	OperationName string
}

// FinishOperation completes an operation and tracks it as a request
func (o *OperationContext) FinishOperation(ctx context.Context, responseCode string, success bool, url string, properties map[string]string) {
	if o == nil || o.Client == nil {
		return
	}
	
	duration := time.Since(o.StartTime)
	
	request := NewRequestTelemetryWithContext(ctx, "OPERATION", url, duration, responseCode)
	request.Success = success
	request.MarkTime(o.StartTime, time.Now())
	
	if properties != nil {
		for k, v := range properties {
			request.Properties[k] = v
		}
	}
	
	o.Client.TrackWithContext(ctx, request)
}

// HTTPRequestCorrelationHelper provides convenient functions for HTTP request correlation
type HTTPRequestCorrelationHelper struct {
	Client TelemetryClient
}

// NewHTTPRequestCorrelationHelper creates a new HTTP request correlation helper
func NewHTTPRequestCorrelationHelper(client TelemetryClient) *HTTPRequestCorrelationHelper {
	return &HTTPRequestCorrelationHelper{
		Client: client,
	}
}

// StartHTTPOperation creates correlation context for an HTTP operation and tracks the request
func (h *HTTPRequestCorrelationHelper) StartHTTPOperation(r *http.Request, operationName string) (context.Context, *HTTPOperationContext) {
	// Extract correlation from headers if present
	middleware := NewHTTPMiddleware()
	corrCtx := middleware.ExtractHeaders(r)
	
	// Create child context if parent exists, otherwise new root
	if corrCtx != nil {
		corrCtx = NewChildCorrelationContext(corrCtx)
	} else {
		corrCtx = NewCorrelationContext()
	}
	
	if operationName != "" {
		corrCtx.OperationName = operationName
	}
	
	ctx := WithCorrelationContext(r.Context(), corrCtx)
	
	httpOpCtx := &HTTPOperationContext{
		Context:       corrCtx,
		Client:        h.Client,
		StartTime:     time.Now(),
		Request:       r,
		OperationName: operationName,
	}
	
	return ctx, httpOpCtx
}

// HTTPOperationContext represents an HTTP operation with correlation context
type HTTPOperationContext struct {
	Context       *CorrelationContext
	Client        TelemetryClient
	StartTime     time.Time
	Request       *http.Request
	OperationName string
}

// FinishHTTPOperation completes an HTTP operation and tracks the request
func (h *HTTPOperationContext) FinishHTTPOperation(ctx context.Context, responseCode string, success bool) {
	if h == nil || h.Client == nil || h.Request == nil {
		return
	}
	
	duration := time.Since(h.StartTime)
	
	h.Client.TrackRequestWithContext(ctx, h.Request.Method, h.Request.URL.String(), duration, responseCode)
}

// InjectHeadersForOutgoingRequest injects correlation headers into an outgoing HTTP request
func (h *HTTPOperationContext) InjectHeadersForOutgoingRequest(outgoingReq *http.Request) {
	if h == nil || h.Context == nil || outgoingReq == nil {
		return
	}
	
	// Create child context for outgoing request
	childCtx := NewChildCorrelationContext(h.Context)
	
	middleware := NewHTTPMiddleware()
	middleware.InjectHeaders(outgoingReq, childCtx)
}

// CorrelationContextBuilder provides a fluent interface for building correlation contexts
type CorrelationContextBuilder struct {
	context *CorrelationContext
}

// NewCorrelationContextBuilder creates a new correlation context builder
func NewCorrelationContextBuilder() *CorrelationContextBuilder {
	return &CorrelationContextBuilder{
		context: NewCorrelationContext(),
	}
}

// NewChildCorrelationContextBuilder creates a new correlation context builder for a child span
func NewChildCorrelationContextBuilder(parent *CorrelationContext) *CorrelationContextBuilder {
	return &CorrelationContextBuilder{
		context: NewChildCorrelationContext(parent),
	}
}

// WithOperationName sets the operation name
func (b *CorrelationContextBuilder) WithOperationName(name string) *CorrelationContextBuilder {
	b.context.OperationName = name
	return b
}

// WithTraceFlags sets the trace flags
func (b *CorrelationContextBuilder) WithTraceFlags(flags byte) *CorrelationContextBuilder {
	b.context.TraceFlags = flags
	return b
}

// WithSampled sets the sampled flag (sets bit 0 of trace flags)
func (b *CorrelationContextBuilder) WithSampled(sampled bool) *CorrelationContextBuilder {
	if sampled {
		b.context.TraceFlags |= 0x01 // Set the sampled bit
	} else {
		b.context.TraceFlags &= 0xFE // Clear the sampled bit
	}
	return b
}

// Build returns the built correlation context
func (b *CorrelationContextBuilder) Build() *CorrelationContext {
	return b.context
}

// BuildWithContext returns a Go context with the built correlation context attached
func (b *CorrelationContextBuilder) BuildWithContext(ctx context.Context) context.Context {
	return WithCorrelationContext(ctx, b.context)
}

// Utility functions for common correlation patterns

// WithNewRootSpan creates a new root correlation context and adds it to the given context
func WithNewRootSpan(ctx context.Context, operationName string) context.Context {
	corrCtx := NewCorrelationContext()
	corrCtx.OperationName = operationName
	return WithCorrelationContext(ctx, corrCtx)
}

// WithChildSpan creates a child correlation context and adds it to the given context
func WithChildSpan(ctx context.Context, operationName string) context.Context {
	parentCorr := GetCorrelationContext(ctx)
	var childCorr *CorrelationContext
	if parentCorr == nil {
		childCorr = NewCorrelationContext()
	} else {
		childCorr = NewChildCorrelationContext(parentCorr)
	}
	if operationName != "" {
		childCorr.OperationName = operationName
	}
	return WithCorrelationContext(ctx, childCorr)
}

// GetOrCreateSpan gets existing correlation context or creates a new one with the given operation name
func GetOrCreateSpan(ctx context.Context, operationName string) (context.Context, *CorrelationContext) {
	corrCtx := GetCorrelationContext(ctx)
	if corrCtx == nil {
		corrCtx = NewCorrelationContext()
		corrCtx.OperationName = operationName
		ctx = WithCorrelationContext(ctx, corrCtx)
	}
	return ctx, corrCtx
}

// CopyCorrelationToRequest copies correlation context from a Go context to HTTP request headers
func CopyCorrelationToRequest(ctx context.Context, req *http.Request) {
	if corrCtx := GetCorrelationContext(ctx); corrCtx != nil {
		middleware := NewHTTPMiddleware()
		middleware.InjectHeaders(req, corrCtx)
	}
}

// TrackDependencyWithSpan is a convenience function to track a dependency with automatic span creation
func TrackDependencyWithSpan(ctx context.Context, client TelemetryClient, name, dependencyType, target string, success bool, fn func(context.Context) error) error {
	return WithSpan(ctx, name, client, func(spanCtx context.Context) error {
		err := fn(spanCtx)
		
		// Track the dependency
		client.TrackRemoteDependencyWithContext(spanCtx, name, dependencyType, target, success && err == nil)
		
		return err
	})
}

// TrackHTTPDependency is a convenience function to track HTTP dependencies with proper correlation
func TrackHTTPDependency(ctx context.Context, client TelemetryClient, req *http.Request, httpClient *http.Client, target string) (*http.Response, error) {
	// Create child span for the HTTP call
	childCtx := WithChildSpan(ctx, "HTTP "+req.Method)
	
	// Inject correlation headers
	CopyCorrelationToRequest(childCtx, req)
	
	start := time.Now()
	resp, err := httpClient.Do(req.WithContext(childCtx))
	duration := time.Since(start)
	
	// Track the dependency
	success := err == nil && resp != nil && resp.StatusCode < 400
	responseCode := ""
	if resp != nil {
		responseCode = strconv.Itoa(resp.StatusCode)
	}
	
	dependency := NewRemoteDependencyTelemetryWithContext(childCtx, req.URL.String(), "HTTP", target, success)
	dependency.Duration = duration
	dependency.ResultCode = responseCode
	dependency.Data = req.Method + " " + req.URL.String()
	
	if err != nil {
		dependency.Properties["error"] = err.Error()
	}
	
	client.TrackWithContext(childCtx, dependency)
	
	return resp, err
}