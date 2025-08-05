package appinsights

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// CorrelationContext holds correlation information for distributed tracing
// following W3C Trace Context standard
type CorrelationContext struct {
	// TraceID is a globally unique identifier for a trace (32-character hex string)
	TraceID string

	// SpanID is a unique identifier for a span within a trace (16-character hex string)
	SpanID string

	// ParentSpanID is the identifier of the parent span (16-character hex string)
	ParentSpanID string

	// TraceFlags contains trace sampling and other flags
	TraceFlags byte

	// OperationName is a human-readable name for the operation
	OperationName string
}

type correlationContextKey struct{}

var correlationKey = correlationContextKey{}

// NewCorrelationContext creates a new correlation context with a new trace ID and span ID
func NewCorrelationContext() *CorrelationContext {
	return &CorrelationContext{
		TraceID:    generateTraceID(),
		SpanID:     generateSpanID(),
		TraceFlags: 0, // Not sampled by default
	}
}

// NewChildCorrelationContext creates a child correlation context that inherits the trace ID
// but generates a new span ID and sets the parent span ID
func NewChildCorrelationContext(parent *CorrelationContext) *CorrelationContext {
	if parent == nil {
		return NewCorrelationContext()
	}

	return &CorrelationContext{
		TraceID:      parent.TraceID,
		SpanID:       generateSpanID(),
		ParentSpanID: parent.SpanID,
		TraceFlags:   parent.TraceFlags,
		OperationName: parent.OperationName,
	}
}

// WithCorrelationContext returns a new context with the correlation context attached
func WithCorrelationContext(ctx context.Context, corrCtx *CorrelationContext) context.Context {
	return context.WithValue(ctx, correlationKey, corrCtx)
}

// GetCorrelationContext extracts the correlation context from the given context
// Returns nil if no correlation context is found
func GetCorrelationContext(ctx context.Context) *CorrelationContext {
	if corrCtx, ok := ctx.Value(correlationKey).(*CorrelationContext); ok {
		return corrCtx
	}
	return nil
}

// GetOrCreateCorrelationContext gets existing correlation context or creates a new one
func GetOrCreateCorrelationContext(ctx context.Context) *CorrelationContext {
	if corrCtx := GetCorrelationContext(ctx); corrCtx != nil {
		return corrCtx
	}
	return NewCorrelationContext()
}

// ToW3CTraceParent returns the W3C traceparent header value
// Format: version-trace_id-span_id-trace_flags
func (c *CorrelationContext) ToW3CTraceParent() string {
	return fmt.Sprintf("00-%s-%s-%02x", c.TraceID, c.SpanID, c.TraceFlags)
}

// ParseW3CTraceParent parses a W3C traceparent header value and returns a CorrelationContext
// Expected format: version-trace_id-span_id-trace_flags
func ParseW3CTraceParent(traceParent string) (*CorrelationContext, error) {
	parts := strings.Split(traceParent, "-")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid traceparent format: expected 4 parts, got %d", len(parts))
	}

	version := parts[0]
	if version != "00" {
		return nil, fmt.Errorf("unsupported traceparent version: %s", version)
	}

	traceID := parts[1]
	if len(traceID) != 32 {
		return nil, fmt.Errorf("invalid trace ID length: expected 32 characters, got %d", len(traceID))
	}

	spanID := parts[2]
	if len(spanID) != 16 {
		return nil, fmt.Errorf("invalid span ID length: expected 16 characters, got %d", len(spanID))
	}

	traceFlags, err := hex.DecodeString(parts[3])
	if err != nil || len(traceFlags) != 1 {
		return nil, fmt.Errorf("invalid trace flags: %s", parts[3])
	}

	return &CorrelationContext{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: traceFlags[0],
	}, nil
}

// generateTraceID generates a random 128-bit trace ID as a 32-character hex string
func generateTraceID() string {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		// Fallback to UUID-based generation if crypto/rand fails
		uuid := newUUID()
		copy(bytes, uuid[:])
	}
	return hex.EncodeToString(bytes)
}

// generateSpanID generates a random 64-bit span ID as a 16-character hex string
func generateSpanID() string {
	bytes := make([]byte, 8)
	_, err := rand.Read(bytes)
	if err != nil {
		// Fallback to UUID-based generation if crypto/rand fails
		uuid := newUUID()
		copy(bytes, uuid[:8])
	}
	return hex.EncodeToString(bytes)
}

// GetOperationID returns the operation ID for Application Insights compatibility
// This uses the trace ID as the operation ID
func (c *CorrelationContext) GetOperationID() string {
	return c.TraceID
}

// GetParentID returns the parent ID for Application Insights compatibility
// This uses the parent span ID if available
func (c *CorrelationContext) GetParentID() string {
	return c.ParentSpanID
}