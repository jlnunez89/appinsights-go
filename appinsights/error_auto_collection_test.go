package appinsights

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

// Test error types for error library integration testing
type testErrorWithStack struct {
	msg   string
	stack []uintptr
}

func (e *testErrorWithStack) Error() string {
	return e.msg
}

func (e *testErrorWithStack) StackTrace() []uintptr {
	return e.stack
}

type testErrorWithCause struct {
	msg   string
	cause error
}

func (e *testErrorWithCause) Error() string {
	return e.msg
}

func (e *testErrorWithCause) Cause() error {
	return e.cause
}

type testErrorWithUnwrap struct {
	msg    string
	wrapped error
}

func (e *testErrorWithUnwrap) Error() string {
	return e.msg
}

func (e *testErrorWithUnwrap) Unwrap() error {
	return e.wrapped
}

func TestNewErrorAutoCollectionConfig(t *testing.T) {
	config := NewErrorAutoCollectionConfig()
	
	if !config.Enabled {
		t.Error("Expected Enabled to be true")
	}
	if !config.EnablePanicRecovery {
		t.Error("Expected EnablePanicRecovery to be true")
	}
	if !config.EnableErrorLibraryIntegration {
		t.Error("Expected EnableErrorLibraryIntegration to be true")
	}
	if config.MaxStackFrames != 50 {
		t.Errorf("Expected MaxStackFrames to be 50, got %d", config.MaxStackFrames)
	}
	if config.SeverityLevel != Error {
		t.Errorf("Expected SeverityLevel to be Error, got %v", config.SeverityLevel)
	}
}

func TestErrorAutoCollector_Creation(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	config := NewErrorAutoCollectionConfig()
	collector := NewErrorAutoCollector(client, config)
	
	if collector.client != client {
		t.Error("Expected collector to have the provided client")
	}
	if collector.config != config {
		t.Error("Expected collector to have the provided config")
	}
	if !collector.IsEnabled() {
		t.Error("Expected collector to be enabled")
	}
}

func TestErrorAutoCollector_CreationWithNilConfig(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, nil)
	
	if collector.config == nil {
		t.Error("Expected collector to have default config when nil is provided")
	}
	if !collector.IsEnabled() {
		t.Error("Expected collector to be enabled with default config")
	}
}

func TestErrorAutoCollector_Enable(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	
	collector.Enable(false)
	if collector.IsEnabled() {
		t.Error("Expected collector to be disabled")
	}
	
	collector.Enable(true)
	if !collector.IsEnabled() {
		t.Error("Expected collector to be enabled")
	}
}

func TestErrorAutoCollector_TrackError(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	
	testErr := errors.New("test error")
	collector.TrackError(testErr)
	
	client.Channel().Close()
	req := transmitter.waitForRequest(t)
	
	if !strings.Contains(req.payload, "test error") {
		t.Errorf("Expected payload to contain 'test error', got: %s", req.payload)
	}
}

func TestErrorAutoCollector_TrackErrorWithContext(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	
	ctx := context.Background()
	testErr := errors.New("test error with context")
	collector.TrackErrorWithContext(ctx, testErr)
	
	client.Channel().Close()
	req := transmitter.waitForRequest(t)
	
	if !strings.Contains(req.payload, "test error with context") {
		t.Errorf("Expected payload to contain 'test error with context', got: %s", req.payload)
	}
}

func TestErrorAutoCollector_TrackErrorDisabled(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	config := NewErrorAutoCollectionConfig()
	config.Enabled = false
	collector := NewErrorAutoCollector(client, config)
	
	testErr := errors.New("test error")
	collector.TrackError(testErr)
	
	client.Channel().Close()
	
	// Should not receive any requests when disabled
	select {
	case <-transmitter.requests:
		t.Error("Expected no telemetry when error collection is disabled")
	case <-time.After(100 * time.Millisecond):
		// Expected - no request should be sent
	}
}

func TestErrorAutoCollector_ErrorFiltering(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	config := NewErrorAutoCollectionConfig()
	config.IgnoredErrors = []string{"ignored error"}
	collector := NewErrorAutoCollector(client, config)
	
	// This error should be ignored
	ignoredErr := errors.New("this is an ignored error")
	collector.TrackError(ignoredErr)
	
	// This error should be tracked
	trackedErr := errors.New("this is a tracked error")
	collector.TrackError(trackedErr)
	
	client.Channel().Close()
	req := transmitter.waitForRequest(t)
	
	if strings.Contains(req.payload, "ignored error") {
		t.Error("Expected ignored error to not be tracked")
	}
	if !strings.Contains(req.payload, "tracked error") {
		t.Error("Expected tracked error to be present")
	}
}

func TestErrorAutoCollector_CustomErrorFilter(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	config := NewErrorAutoCollectionConfig()
	config.ErrorFilters = []ErrorFilterFunc{
		func(err interface{}) bool {
			errStr := fmt.Sprintf("%v", err)
			return !strings.Contains(errStr, "filtered")
		},
	}
	collector := NewErrorAutoCollector(client, config)
	
	// This error should be filtered out
	filteredErr := errors.New("this is a filtered error")
	collector.TrackError(filteredErr)
	
	// This error should be tracked
	allowedErr := errors.New("this is an allowed error")
	collector.TrackError(allowedErr)
	
	client.Channel().Close()
	req := transmitter.waitForRequest(t)
	
	if strings.Contains(req.payload, "filtered error") {
		t.Error("Expected filtered error to not be tracked")
	}
	if !strings.Contains(req.payload, "allowed error") {
		t.Error("Expected allowed error to be present")
	}
}

func TestErrorAutoCollector_ErrorSanitization(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	config := NewErrorAutoCollectionConfig()
	config.ErrorSanitizers = []ErrorSanitizerFunc{
		func(err interface{}, frames []*contracts.StackFrame) (interface{}, []*contracts.StackFrame) {
			errStr := fmt.Sprintf("%v", err)
			sanitized := strings.ReplaceAll(errStr, "password=secret123", "password=[REDACTED]")
			return sanitized, frames
		},
	}
	collector := NewErrorAutoCollector(client, config)
	
	sensitiveErr := errors.New("login failed: password=secret123")
	collector.TrackError(sensitiveErr)
	
	client.Channel().Close()
	req := transmitter.waitForRequest(t)
	
	if strings.Contains(req.payload, "secret123") {
		t.Error("Expected sensitive data to be sanitized")
	}
	if !strings.Contains(req.payload, "[REDACTED]") {
		t.Error("Expected sanitized placeholder to be present")
	}
}

func TestErrorAutoCollector_RecoverPanic(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	
	collector.RecoverPanic(func() {
		panic("test panic")
	})
	
	client.Channel().Close()
	req := transmitter.waitForRequest(t)
	
	if !strings.Contains(req.payload, "test panic") {
		t.Errorf("Expected payload to contain 'test panic', got: %s", req.payload)
	}
}

func TestErrorAutoCollector_RecoverPanicWithContext(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	
	ctx := context.Background()
	collector.RecoverPanicWithContext(ctx, func() {
		panic("test panic with context")
	})
	
	client.Channel().Close()
	req := transmitter.waitForRequest(t)
	
	if !strings.Contains(req.payload, "test panic with context") {
		t.Errorf("Expected payload to contain 'test panic with context', got: %s", req.payload)
	}
}

func TestErrorAutoCollector_WrapFunction(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	
	wrappedFunc := collector.WrapFunction(func() {
		panic("wrapped function panic")
	})
	
	wrappedFunc()
	
	client.Channel().Close()
	req := transmitter.waitForRequest(t)
	
	if !strings.Contains(req.payload, "wrapped function panic") {
		t.Errorf("Expected payload to contain 'wrapped function panic', got: %s", req.payload)
	}
}

func TestErrorAutoCollector_MaxStackFrames(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	config := NewErrorAutoCollectionConfig()
	config.MaxStackFrames = 3
	collector := NewErrorAutoCollector(client, config)
	
	testErr := errors.New("test error")
	collector.TrackError(testErr)
	
	client.Channel().Close()
	req := transmitter.waitForRequest(t)
	
	// Count the number of stack frames in the payload
	stackFrameCount := strings.Count(req.payload, `"level":`)
	
	if stackFrameCount > 3 {
		t.Errorf("Expected at most 3 stack frames, got %d", stackFrameCount)
	}
}

func TestDefaultErrorFilter(t *testing.T) {
	if !DefaultErrorFilter(errors.New("test")) {
		t.Error("Expected DefaultErrorFilter to return true for non-nil error")
	}
	
	if DefaultErrorFilter(nil) {
		t.Error("Expected DefaultErrorFilter to return false for nil error")
	}
}

func TestSeverityErrorFilter(t *testing.T) {
	filter := SeverityErrorFilter([]string{"critical", "fatal"})
	
	if !filter(errors.New("This is a critical error")) {
		t.Error("Expected filter to match 'critical' keyword")
	}
	
	if !filter(errors.New("Fatal system failure")) {
		t.Error("Expected filter to match 'fatal' keyword")
	}
	
	if filter(errors.New("This is a warning")) {
		t.Error("Expected filter to not match non-keyword error")
	}
}

func TestDefaultErrorSanitizer(t *testing.T) {
	originalErr := "login failed: password=secret123 token=abc456 key=xyz789"
	sanitizedErr, _ := DefaultErrorSanitizer(originalErr, nil)
	
	sanitizedStr := fmt.Sprintf("%v", sanitizedErr)
	
	if strings.Contains(sanitizedStr, "secret123") ||
		strings.Contains(sanitizedStr, "abc456") ||
		strings.Contains(sanitizedStr, "xyz789") {
		t.Errorf("Expected sensitive data to be sanitized, got: %s", sanitizedStr)
	}
	
	if !strings.Contains(sanitizedStr, "[REDACTED]") {
		t.Error("Expected [REDACTED] placeholder to be present")
	}
}

func TestFilePathSanitizer(t *testing.T) {
	frames := []*contracts.StackFrame{
		{
			FileName: "/very/long/sensitive/path/to/user/home/project/file.go",
			Line:     123,
		},
		{
			FileName: "/short/path.go",
			Line:     456,
		},
	}
	
	_, sanitizedFrames := FilePathSanitizer(nil, frames)
	
	if !strings.HasPrefix(sanitizedFrames[0].FileName, ".../") {
		t.Errorf("Expected long path to be sanitized with ..., got: %s", sanitizedFrames[0].FileName)
	}
	
	if sanitizedFrames[1].FileName != "/short/path.go" {
		t.Errorf("Expected short path to remain unchanged, got: %s", sanitizedFrames[1].FileName)
	}
}

func TestErrorAutoCollector_PanicRecoveryDisabled(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	config := NewErrorAutoCollectionConfig()
	config.EnablePanicRecovery = false
	collector := NewErrorAutoCollector(client, config)
	
	// Should panic and not be recovered since panic recovery is disabled
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic to not be recovered when panic recovery is disabled")
		}
	}()
	
	collector.RecoverPanic(func() {
		panic("should not be recovered")
	})
}