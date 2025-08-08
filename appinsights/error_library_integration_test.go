package appinsights

import (
	"errors"
	"runtime"
	"strings"
	"testing"
)

func TestErrorLibraryIntegration_Creation(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	integration := NewErrorLibraryIntegration(collector)
	
	if integration.collector != collector {
		t.Error("Expected integration to have the provided collector")
	}
}

func TestErrorLibraryIntegration_ExtractErrorInfo_BasicError(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	integration := NewErrorLibraryIntegration(collector)
	
	testErr := errors.New("basic error")
	info, ok := integration.ExtractErrorInfo(testErr)
	
	if !ok {
		t.Error("Expected ExtractErrorInfo to return true for valid error")
	}
	
	if info.Message != "basic error" {
		t.Errorf("Expected message to be 'basic error', got: %s", info.Message)
	}
	
	if info.Type != "*errors.errorString" {
		t.Errorf("Expected type to be '*errors.errorString', got: %s", info.Type)
	}
	
	if len(info.StackFrames) == 0 {
		t.Error("Expected stack frames to be captured")
	}
}

func TestErrorLibraryIntegration_ExtractErrorInfo_NilError(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	integration := NewErrorLibraryIntegration(collector)
	
	info, ok := integration.ExtractErrorInfo(nil)
	
	if ok {
		t.Error("Expected ExtractErrorInfo to return false for nil error")
	}
	
	if info != nil {
		t.Error("Expected info to be nil for nil error")
	}
}

func TestErrorLibraryIntegration_ExtractErrorInfo_ErrorWithStack(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	integration := NewErrorLibraryIntegration(collector)
	
	// Create a mock error with stack trace
	pc := make([]uintptr, 10)
	n := runtime.Callers(1, pc)
	stackErr := &testErrorWithStack{
		msg:   "error with stack",
		stack: pc[:n],
	}
	
	info, ok := integration.ExtractErrorInfo(stackErr)
	
	if !ok {
		t.Error("Expected ExtractErrorInfo to return true for error with stack")
	}
	
	if info.Message != "error with stack" {
		t.Errorf("Expected message to be 'error with stack', got: %s", info.Message)
	}
	
	if len(info.StackFrames) == 0 {
		t.Error("Expected stack frames to be extracted from error")
	}
}

func TestErrorLibraryIntegration_ExtractErrorInfo_ErrorWithCause(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	integration := NewErrorLibraryIntegration(collector)
	
	rootCause := errors.New("root cause")
	wrappedErr := &testErrorWithCause{
		msg:   "wrapped error",
		cause: rootCause,
	}
	
	info, ok := integration.ExtractErrorInfo(wrappedErr)
	
	if !ok {
		t.Error("Expected ExtractErrorInfo to return true for error with cause")
	}
	
	if len(info.ErrorChain) < 2 {
		t.Errorf("Expected error chain to have at least 2 errors, got: %d", len(info.ErrorChain))
	}
	
	if info.ErrorChain[0].Error() != "wrapped error" {
		t.Errorf("Expected first error in chain to be 'wrapped error', got: %s", info.ErrorChain[0].Error())
	}
	
	if info.ErrorChain[1].Error() != "root cause" {
		t.Errorf("Expected second error in chain to be 'root cause', got: %s", info.ErrorChain[1].Error())
	}
}

func TestErrorLibraryIntegration_ExtractErrorInfo_ErrorWithUnwrap(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	integration := NewErrorLibraryIntegration(collector)
	
	rootCause := errors.New("root cause")
	wrappedErr := &testErrorWithUnwrap{
		msg:     "wrapped error",
		wrapped: rootCause,
	}
	
	info, ok := integration.ExtractErrorInfo(wrappedErr)
	
	if !ok {
		t.Error("Expected ExtractErrorInfo to return true for error with unwrap")
	}
	
	if len(info.ErrorChain) < 2 {
		t.Errorf("Expected error chain to have at least 2 errors, got: %d", len(info.ErrorChain))
	}
	
	if info.ErrorChain[0].Error() != "wrapped error" {
		t.Errorf("Expected first error in chain to be 'wrapped error', got: %s", info.ErrorChain[0].Error())
	}
	
	if info.ErrorChain[1].Error() != "root cause" {
		t.Errorf("Expected second error in chain to be 'root cause', got: %s", info.ErrorChain[1].Error())
	}
}

func TestErrorLibraryIntegration_GetErrorMessage_DifferentTypes(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	integration := NewErrorLibraryIntegration(collector)
	
	tests := []struct {
		name     string
		err      interface{}
		expected string
	}{
		{"error type", errors.New("test error"), "test error"},
		{"string type", "test string", "test string"},
		{"stringer type", &myStringer{}, "My stringer error"},
		{"go stringer type", &myGoStringer{}, "My go stringer error"},
		{"other type", 42, "42"},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := integration.getErrorMessage(test.err)
			if result != test.expected {
				t.Errorf("Expected message '%s', got '%s'", test.expected, result)
			}
		})
	}
}

func TestErrorLibraryIntegration_ConvertStackTrace(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	integration := NewErrorLibraryIntegration(collector)
	
	// Get current stack
	pc := make([]uintptr, 10)
	n := runtime.Callers(1, pc)
	stack := pc[:n]
	
	frames := integration.convertStackTrace(stack)
	
	if len(frames) == 0 {
		t.Error("Expected stack frames to be generated")
	}
	
	// Check that the first frame contains this test function
	found := false
	for _, frame := range frames {
		if strings.Contains(frame.Method, "TestErrorLibraryIntegration_ConvertStackTrace") {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Expected to find test function in stack trace")
	}
}

func TestErrorLibraryIntegration_ConvertStackTrace_EmptyStack(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	integration := NewErrorLibraryIntegration(collector)
	
	frames := integration.convertStackTrace([]uintptr{})
	
	if frames != nil {
		t.Error("Expected nil frames for empty stack")
	}
}

func TestErrorLibraryIntegration_ParseStackString(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	integration := NewErrorLibraryIntegration(collector)
	
	stackStr := `main.main
	/path/to/main.go:15
runtime.main
	/usr/local/go/src/runtime/proc.go:250`
	
	frames := integration.parseStackString(stackStr)
	
	if len(frames) != 2 {
		t.Errorf("Expected 2 frames, got %d", len(frames))
	}
	
	if frames[0].Method != "main.main" {
		t.Errorf("Expected first frame method to be 'main.main', got: %s", frames[0].Method)
	}
	
	if frames[0].FileName != "/path/to/main.go" {
		t.Errorf("Expected first frame file to be '/path/to/main.go', got: %s", frames[0].FileName)
	}
	
	if frames[0].Line != 15 {
		t.Errorf("Expected first frame line to be 15, got: %d", frames[0].Line)
	}
	
	if frames[1].Method != "runtime.main" {
		t.Errorf("Expected second frame method to be 'runtime.main', got: %s", frames[1].Method)
	}
}

func TestErrorLibraryIntegration_ParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"123", 123},
		{"0", 0},
		{"999", 999},
		{"123abc", 123}, // Should stop at non-digit
		{"abc123", 0},   // Should return 0 for non-numeric start
		{"", 0},         // Should return 0 for empty string
	}
	
	for _, test := range tests {
		result := parseInt(test.input)
		if result != test.expected {
			t.Errorf("parseInt(%q) = %d, expected %d", test.input, result, test.expected)
		}
	}
}

func TestErrorLibraryIntegration_CreateEnhancedExceptionTelemetry(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	integration := NewErrorLibraryIntegration(collector)
	
	rootCause := errors.New("root cause")
	wrappedErr := &testErrorWithCause{
		msg:   "wrapped error",
		cause: rootCause,
	}
	
	exception := integration.CreateEnhancedExceptionTelemetry(wrappedErr)
	
	if exception.Error != wrappedErr {
		t.Error("Expected exception to contain the original error")
	}
	
	if len(exception.Properties) == 0 {
		t.Error("Expected exception to have properties with error chain information")
	}
	
	// Check for error chain properties
	if exception.Properties["ErrorChain.0"] != "wrapped error" {
		t.Errorf("Expected ErrorChain.0 to be 'wrapped error', got: %s", exception.Properties["ErrorChain.0"])
	}
	
	if exception.Properties["ErrorChain.1"] != "root cause" {
		t.Errorf("Expected ErrorChain.1 to be 'root cause', got: %s", exception.Properties["ErrorChain.1"])
	}
	
	if exception.Properties["ErrorChain.Length"] != "2" {
		t.Errorf("Expected ErrorChain.Length to be '2', got: %s", exception.Properties["ErrorChain.Length"])
	}
}

func TestErrorLibraryIntegration_CreateEnhancedExceptionTelemetry_BasicError(t *testing.T) {
	mockClock()
	defer resetClock()
	
	client, transmitter := newTestChannelServer()
	defer transmitter.Close()
	
	collector := NewErrorAutoCollector(client, NewErrorAutoCollectionConfig())
	integration := NewErrorLibraryIntegration(collector)
	
	basicErr := errors.New("basic error")
	exception := integration.CreateEnhancedExceptionTelemetry(basicErr)
	
	if exception.Error != basicErr {
		t.Error("Expected exception to contain the original error")
	}
	
	// Should not have error chain properties for basic error
	if len(exception.Properties) > 0 {
		// Check that there are no ErrorChain properties
		for key := range exception.Properties {
			if strings.HasPrefix(key, "ErrorChain.") {
				t.Errorf("Expected no error chain properties for basic error, found: %s", key)
			}
		}
	}
}