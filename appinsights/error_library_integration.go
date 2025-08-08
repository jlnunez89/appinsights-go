package appinsights

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

// ErrorLibraryIntegration provides integration with popular Go error handling libraries
type ErrorLibraryIntegration struct {
	collector *ErrorAutoCollector
}

// NewErrorLibraryIntegration creates a new error library integration
func NewErrorLibraryIntegration(collector *ErrorAutoCollector) *ErrorLibraryIntegration {
	return &ErrorLibraryIntegration{
		collector: collector,
	}
}

// ErrorWithStack represents an error with stack trace information
type ErrorWithStack interface {
	error
	StackTrace() []uintptr
}

// ErrorWithCause represents an error that wraps another error
type ErrorWithCause interface {
	error
	Cause() error
}

// ErrorWithUnwrap represents an error that supports Go 1.13+ error unwrapping
type ErrorWithUnwrap interface {
	error
	Unwrap() error
}

// ExtractErrorInfo extracts comprehensive error information including stack traces from various error types
func (eli *ErrorLibraryIntegration) ExtractErrorInfo(err interface{}) (*EnhancedErrorInfo, bool) {
	if err == nil {
		return nil, false
	}

	info := &EnhancedErrorInfo{
		OriginalError: err,
		Message:       eli.getErrorMessage(err),
		Type:          reflect.TypeOf(err).String(),
		StackFrames:   []*contracts.StackFrame{},
		ErrorChain:    []error{},
	}

	// Try to extract stack trace and error chain based on error type
	eli.extractStackTrace(err, info)
	eli.extractErrorChain(err, info)

	return info, true
}

// EnhancedErrorInfo contains comprehensive error information
type EnhancedErrorInfo struct {
	OriginalError interface{}
	Message       string
	Type          string
	StackFrames   []*contracts.StackFrame
	ErrorChain    []error
	SourceFile    string
	SourceLine    int
	Function      string
}

// getErrorMessage extracts the error message from various error types
func (eli *ErrorLibraryIntegration) getErrorMessage(err interface{}) string {
	switch e := err.(type) {
	case error:
		return e.Error()
	case string:
		return e
	case fmt.Stringer:
		return e.String()
	case fmt.GoStringer:
		return e.GoString()
	default:
		return fmt.Sprintf("%v", err)
	}
}

// extractStackTrace attempts to extract stack trace from various error types
func (eli *ErrorLibraryIntegration) extractStackTrace(err interface{}, info *EnhancedErrorInfo) {
	// Try to extract stack trace using different interfaces

	// Check for errors with stack trace (like pkg/errors)
	if stackErr, ok := err.(ErrorWithStack); ok {
		info.StackFrames = eli.convertStackTrace(stackErr.StackTrace())
		return
	}

	// Check for errors with frame information (pkg/errors Frame interface)
	if frameErr := eli.extractFrameInfo(err); frameErr != nil {
		info.StackFrames = frameErr
		return
	}

	// Check for xerrors or Go 1.13+ error with frame info
	if frames := eli.extractXErrorFrames(err); frames != nil {
		info.StackFrames = frames
		return
	}

	// Fall back to current stack trace
	info.StackFrames = eli.collector.getEnhancedCallstack(2)
}

// extractErrorChain extracts the chain of wrapped errors
func (eli *ErrorLibraryIntegration) extractErrorChain(err interface{}, info *EnhancedErrorInfo) {
	var chain []error

	current := err
	for current != nil {
		if e, ok := current.(error); ok {
			chain = append(chain, e)
		}

		// Try different unwrapping methods
		if unwrapper, ok := current.(ErrorWithUnwrap); ok {
			current = unwrapper.Unwrap()
		} else if causer, ok := current.(ErrorWithCause); ok {
			current = causer.Cause()
		} else if e, ok := current.(error); ok {
			// Use Go 1.13+ errors.Unwrap
			current = errors.Unwrap(e)
		} else {
			break
		}
	}

	info.ErrorChain = chain
}

// convertStackTrace converts a stack trace from uintptr slice to StackFrame slice
func (eli *ErrorLibraryIntegration) convertStackTrace(stack []uintptr) []*contracts.StackFrame {
	if len(stack) == 0 {
		return nil
	}

	frames := runtime.CallersFrames(stack)
	var stackFrames []*contracts.StackFrame
	level := 0

	for {
		frame, more := frames.Next()

		stackFrame := &contracts.StackFrame{
			Level:    level,
			FileName: frame.File,
			Line:     frame.Line,
		}

		if frame.Function != "" {
			stackFrame.Method = frame.Function

			// Extract assembly and method names
			lastSlash := strings.LastIndexByte(frame.Function, '/')
			if lastSlash < 0 {
				lastSlash = 0
			}

			firstDot := strings.IndexByte(frame.Function[lastSlash:], '.')
			if firstDot >= 0 {
				stackFrame.Assembly = frame.Function[:lastSlash+firstDot]
				stackFrame.Method = frame.Function[lastSlash+firstDot+1:]
			}
		}

		stackFrames = append(stackFrames, stackFrame)
		level++

		if !more {
			break
		}
	}

	return stackFrames
}

// extractFrameInfo tries to extract frame information using reflection for pkg/errors compatibility
func (eli *ErrorLibraryIntegration) extractFrameInfo(err interface{}) []*contracts.StackFrame {
	// This uses reflection to be compatible with pkg/errors without importing it
	v := reflect.ValueOf(err)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Look for a stack field
	if v.Kind() == reflect.Struct {
		stackField := v.FieldByName("stack")
		if stackField.IsValid() {
			return eli.convertReflectedStack(stackField)
		}
	}

	return nil
}

// extractXErrorFrames extracts frames from xerrors or Go 1.13+ errors
func (eli *ErrorLibraryIntegration) extractXErrorFrames(err interface{}) []*contracts.StackFrame {
	// This is a placeholder for xerrors integration
	// In a real implementation, you might check for specific xerrors interfaces
	// For now, we'll use reflection to look for common frame patterns
	
	v := reflect.ValueOf(err)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		// Look for frame or frames fields
		frameField := v.FieldByName("frame")
		if frameField.IsValid() {
			return eli.convertSingleFrame(frameField)
		}

		framesField := v.FieldByName("frames")
		if framesField.IsValid() {
			return eli.convertMultipleFrames(framesField)
		}
	}

	return nil
}

// convertReflectedStack converts a reflected stack field to StackFrame slice
func (eli *ErrorLibraryIntegration) convertReflectedStack(stackField reflect.Value) []*contracts.StackFrame {
	// This is a generic implementation that works with various stack representations
	if !stackField.CanInterface() {
		return nil
	}

	stackInterface := stackField.Interface()
	
	// Check if it's a slice of uintptr (common in pkg/errors)
	if stack, ok := stackInterface.([]uintptr); ok {
		return eli.convertStackTrace(stack)
	}

	// Check if it implements a String method for debugging
	if stringer, ok := stackInterface.(fmt.Stringer); ok {
		return eli.parseStackString(stringer.String())
	}

	return nil
}

// convertSingleFrame converts a single frame field to StackFrame slice
func (eli *ErrorLibraryIntegration) convertSingleFrame(frameField reflect.Value) []*contracts.StackFrame {
	if !frameField.CanInterface() {
		return nil
	}

	frame := frameField.Interface()
	
	// Use reflection to extract frame information
	v := reflect.ValueOf(frame)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		stackFrame := &contracts.StackFrame{Level: 0}

		// Look for common field names
		if fileField := v.FieldByName("file"); fileField.IsValid() && fileField.Kind() == reflect.String {
			stackFrame.FileName = fileField.String()
		}
		if lineField := v.FieldByName("line"); lineField.IsValid() && lineField.Kind() == reflect.Int {
			stackFrame.Line = int(lineField.Int())
		}
		if funcField := v.FieldByName("name"); funcField.IsValid() && funcField.Kind() == reflect.String {
			stackFrame.Method = funcField.String()
		}

		return []*contracts.StackFrame{stackFrame}
	}

	return nil
}

// convertMultipleFrames converts multiple frames field to StackFrame slice
func (eli *ErrorLibraryIntegration) convertMultipleFrames(framesField reflect.Value) []*contracts.StackFrame {
	if !framesField.CanInterface() || framesField.Kind() != reflect.Slice {
		return nil
	}

	var stackFrames []*contracts.StackFrame
	for i := 0; i < framesField.Len(); i++ {
		frameVal := framesField.Index(i)
		if singleFrame := eli.convertSingleFrame(frameVal); len(singleFrame) > 0 {
			singleFrame[0].Level = i
			stackFrames = append(stackFrames, singleFrame[0])
		}
	}

	return stackFrames
}

// parseStackString parses a stack trace string representation
func (eli *ErrorLibraryIntegration) parseStackString(stackStr string) []*contracts.StackFrame {
	lines := strings.Split(stackStr, "\n")
	var stackFrames []*contracts.StackFrame
	level := 0

	for i := 0; i < len(lines); i += 2 {
		if i+1 >= len(lines) {
			break
		}

		funcLine := strings.TrimSpace(lines[i])
		fileLine := strings.TrimSpace(lines[i+1])

		if funcLine == "" || fileLine == "" {
			continue
		}

		stackFrame := &contracts.StackFrame{
			Level:  level,
			Method: funcLine,
		}

		// Parse file:line format
		if colonIdx := strings.LastIndex(fileLine, ":"); colonIdx > 0 {
			stackFrame.FileName = fileLine[:colonIdx]
			if lineNum := parseInt(fileLine[colonIdx+1:]); lineNum > 0 {
				stackFrame.Line = lineNum
			}
		} else {
			stackFrame.FileName = fileLine
		}

		stackFrames = append(stackFrames, stackFrame)
		level++
	}

	return stackFrames
}

// parseInt safely parses an integer string
func parseInt(s string) int {
	result := 0
	for _, char := range s {
		if char >= '0' && char <= '9' {
			result = result*10 + int(char-'0')
		} else {
			break
		}
	}
	return result
}

// CreateEnhancedExceptionTelemetry creates exception telemetry with enhanced error information
func (eli *ErrorLibraryIntegration) CreateEnhancedExceptionTelemetry(err interface{}) *ExceptionTelemetry {
	info, ok := eli.ExtractErrorInfo(err)
	if !ok {
		return NewExceptionTelemetry(err)
	}

	exception := &ExceptionTelemetry{
		Error:         info.OriginalError,
		Frames:        info.StackFrames,
		SeverityLevel: eli.collector.config.SeverityLevel,
		BaseTelemetry: BaseTelemetry{
			Timestamp:  currentClock.Now(),
			Tags:       make(contracts.ContextTags),
			Properties: make(map[string]string),
		},
		BaseTelemetryMeasurements: BaseTelemetryMeasurements{
			Measurements: make(map[string]float64),
		},
	}

	// Add error chain information to properties
	if len(info.ErrorChain) > 1 {
		for i, chainErr := range info.ErrorChain {
			exception.Properties[fmt.Sprintf("ErrorChain.%d", i)] = chainErr.Error()
			exception.Properties[fmt.Sprintf("ErrorChain.%d.Type", i)] = reflect.TypeOf(chainErr).String()
		}
		exception.Properties["ErrorChain.Length"] = fmt.Sprintf("%d", len(info.ErrorChain))
	}

	// Add source information if available
	if info.SourceFile != "" {
		exception.Properties["Source.File"] = info.SourceFile
	}
	if info.SourceLine > 0 {
		exception.Properties["Source.Line"] = fmt.Sprintf("%d", info.SourceLine)
	}
	if info.Function != "" {
		exception.Properties["Source.Function"] = info.Function
	}

	return exception
}