package appinsights

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

// ErrorFilterFunc is a function that determines whether an error should be tracked
type ErrorFilterFunc func(err interface{}) bool

// ErrorSanitizerFunc is a function that sanitizes error messages and stack traces
type ErrorSanitizerFunc func(err interface{}, stackFrames []*contracts.StackFrame) (interface{}, []*contracts.StackFrame)

// ErrorAutoCollectionConfig configures automatic error collection behavior
type ErrorAutoCollectionConfig struct {
	// Enabled controls whether automatic error collection is active
	Enabled bool

	// EnablePanicRecovery enables automatic panic recovery in goroutines
	EnablePanicRecovery bool

	// EnableErrorLibraryIntegration enables integration with popular error libraries
	EnableErrorLibraryIntegration bool

	// MaxStackFrames limits the number of stack frames collected (0 = no limit)
	MaxStackFrames int

	// IncludeSourceCode includes source code context in stack traces when available
	IncludeSourceCode bool

	// ErrorFilters are functions to determine which errors should be tracked
	ErrorFilters []ErrorFilterFunc

	// ErrorSanitizers are functions to sanitize error data before tracking
	ErrorSanitizers []ErrorSanitizerFunc

	// IgnoredErrors contains error types/messages that should not be tracked
	IgnoredErrors []string

	// SeverityLevel sets the default severity level for auto-collected errors
	SeverityLevel contracts.SeverityLevel
}

// NewErrorAutoCollectionConfig creates a new configuration with default values
func NewErrorAutoCollectionConfig() *ErrorAutoCollectionConfig {
	return &ErrorAutoCollectionConfig{
		Enabled:                       true,
		EnablePanicRecovery:           true,
		EnableErrorLibraryIntegration: true,
		MaxStackFrames:                50,
		IncludeSourceCode:             false,
		ErrorFilters:                  []ErrorFilterFunc{},
		ErrorSanitizers:               []ErrorSanitizerFunc{},
		IgnoredErrors:                 []string{},
		SeverityLevel:                 Error,
	}
}

// ErrorAutoCollector handles automatic error collection and tracking
type ErrorAutoCollector struct {
	client    TelemetryClient
	config    *ErrorAutoCollectionConfig
	mu        sync.RWMutex
	isEnabled bool
}

// NewErrorAutoCollector creates a new error auto-collector
func NewErrorAutoCollector(client TelemetryClient, config *ErrorAutoCollectionConfig) *ErrorAutoCollector {
	if config == nil {
		config = NewErrorAutoCollectionConfig()
	}
	
	return &ErrorAutoCollector{
		client:    client,
		config:    config,
		isEnabled: config.Enabled,
	}
}

// Enable enables or disables the error auto-collector
func (eac *ErrorAutoCollector) Enable(enabled bool) {
	eac.mu.Lock()
	defer eac.mu.Unlock()
	eac.isEnabled = enabled
}

// IsEnabled returns whether the error auto-collector is enabled
func (eac *ErrorAutoCollector) IsEnabled() bool {
	eac.mu.RLock()
	defer eac.mu.RUnlock()
	return eac.isEnabled && eac.config.Enabled
}

// TrackError tracks an error with automatic filtering and sanitization
func (eac *ErrorAutoCollector) TrackError(err interface{}) {
	eac.TrackErrorWithContext(context.Background(), err)
}

// TrackErrorWithContext tracks an error with context, applying filtering and sanitization
func (eac *ErrorAutoCollector) TrackErrorWithContext(ctx context.Context, err interface{}) {
	if !eac.IsEnabled() || err == nil {
		return
	}

	// Apply error filters
	if !eac.shouldTrackError(err) {
		return
	}

	// Create exception telemetry with enhanced stack trace
	exceptionTelemetry := eac.createExceptionTelemetry(err, 2)
	
	// Apply sanitizers
	exceptionTelemetry = eac.sanitizeException(exceptionTelemetry)
	
	// Track the exception
	if ctx == context.Background() {
		eac.client.Track(exceptionTelemetry)
	} else {
		eac.client.TrackWithContext(ctx, exceptionTelemetry)
	}
}

// WrapFunction wraps a function with automatic panic recovery
func (eac *ErrorAutoCollector) WrapFunction(fn func()) func() {
	return func() {
		eac.RecoverPanic(fn)
	}
}

// WrapFunctionWithContext wraps a function with automatic panic recovery and context
func (eac *ErrorAutoCollector) WrapFunctionWithContext(ctx context.Context, fn func()) func() {
	return func() {
		eac.RecoverPanicWithContext(ctx, fn)
	}
}

// RecoverPanic executes a function and recovers from any panics
func (eac *ErrorAutoCollector) RecoverPanic(fn func()) {
	eac.RecoverPanicWithContext(context.Background(), fn)
}

// RecoverPanicWithContext executes a function and recovers from any panics with context
func (eac *ErrorAutoCollector) RecoverPanicWithContext(ctx context.Context, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			if eac.IsEnabled() && eac.config.EnablePanicRecovery {
				eac.TrackErrorWithContext(ctx, r)
			} else {
				// Re-panic if panic recovery is disabled
				panic(r)
			}
		}
	}()
	
	fn()
}

// shouldTrackError determines if an error should be tracked based on filters and ignored errors
func (eac *ErrorAutoCollector) shouldTrackError(err interface{}) bool {
	// Check ignored errors
	if eac.isIgnoredError(err) {
		return false
	}

	// Apply custom filters
	for _, filter := range eac.config.ErrorFilters {
		if !filter(err) {
			return false
		}
	}

	return true
}

// isIgnoredError checks if the error matches any ignored error patterns
func (eac *ErrorAutoCollector) isIgnoredError(err interface{}) bool {
	errStr := eac.getErrorString(err)
	errType := reflect.TypeOf(err).String()

	for _, ignored := range eac.config.IgnoredErrors {
		if strings.Contains(errStr, ignored) || strings.Contains(errType, ignored) {
			return true
		}
	}

	return false
}

// getErrorString extracts a string representation from an error
func (eac *ErrorAutoCollector) getErrorString(err interface{}) string {
	if err == nil {
		return ""
	}

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

// createExceptionTelemetry creates enhanced exception telemetry with better stack traces
func (eac *ErrorAutoCollector) createExceptionTelemetry(err interface{}, skip int) *ExceptionTelemetry {
	frames := eac.getEnhancedCallstack(skip + 1)
	
	return &ExceptionTelemetry{
		Error:         err,
		Frames:        frames,
		SeverityLevel: eac.config.SeverityLevel,
		BaseTelemetry: BaseTelemetry{
			Timestamp:  currentClock.Now(),
			Tags:       make(contracts.ContextTags),
			Properties: make(map[string]string),
		},
		BaseTelemetryMeasurements: BaseTelemetryMeasurements{
			Measurements: make(map[string]float64),
		},
	}
}

// getEnhancedCallstack generates an enhanced callstack with configurable limits
func (eac *ErrorAutoCollector) getEnhancedCallstack(skip int) []*contracts.StackFrame {
	if skip < 0 {
		skip = 0
	}

	maxFrames := eac.config.MaxStackFrames
	if maxFrames <= 0 {
		maxFrames = 64 // Default limit
	}

	stack := make([]uintptr, maxFrames+skip)
	depth := runtime.Callers(skip+1, stack)
	if depth == 0 {
		return nil
	}

	frames := runtime.CallersFrames(stack[:depth])
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

		if !more || (eac.config.MaxStackFrames > 0 && level >= eac.config.MaxStackFrames) {
			break
		}
	}

	return stackFrames
}

// sanitizeException applies configured sanitizers to exception data
func (eac *ErrorAutoCollector) sanitizeException(exception *ExceptionTelemetry) *ExceptionTelemetry {
	sanitizedErr := exception.Error
	sanitizedFrames := exception.Frames

	// Apply all configured sanitizers
	for _, sanitizer := range eac.config.ErrorSanitizers {
		sanitizedErr, sanitizedFrames = sanitizer(sanitizedErr, sanitizedFrames)
	}

	exception.Error = sanitizedErr
	exception.Frames = sanitizedFrames
	
	return exception
}

// Common error filters

// DefaultErrorFilter is a basic filter that excludes nil errors
func DefaultErrorFilter(err interface{}) bool {
	return err != nil
}

// SeverityErrorFilter creates a filter based on error message content
func SeverityErrorFilter(keywords []string) ErrorFilterFunc {
	return func(err interface{}) bool {
		errStr := strings.ToLower(fmt.Sprintf("%v", err))
		for _, keyword := range keywords {
			if strings.Contains(errStr, strings.ToLower(keyword)) {
				return true
			}
		}
		return false
	}
}

// Common error sanitizers

// DefaultErrorSanitizer removes sensitive information from error messages
func DefaultErrorSanitizer(err interface{}, stackFrames []*contracts.StackFrame) (interface{}, []*contracts.StackFrame) {
	if err == nil {
		return err, stackFrames
	}

	errStr := fmt.Sprintf("%v", err)
	
	// Remove potential sensitive information using regex
	sensitivePatterns := []*regexp.Regexp{
		regexp.MustCompile(`password=[^&\s]*`),
		regexp.MustCompile(`token=[^&\s]*`),
		regexp.MustCompile(`key=[^&\s]*`),
		regexp.MustCompile(`secret=[^&\s]*`),
		regexp.MustCompile(`auth=[^&\s]*`),
	}

	for _, pattern := range sensitivePatterns {
		errStr = pattern.ReplaceAllString(errStr, "[REDACTED]")
	}

	return errStr, stackFrames
}

// FilePathSanitizer removes sensitive file paths from stack traces
func FilePathSanitizer(err interface{}, stackFrames []*contracts.StackFrame) (interface{}, []*contracts.StackFrame) {
	if stackFrames == nil {
		return err, stackFrames
	}

	for _, frame := range stackFrames {
		if frame.FileName != "" {
			// Keep only the last few path components for privacy
			parts := strings.Split(frame.FileName, "/")
			if len(parts) > 3 {
				frame.FileName = ".../" + strings.Join(parts[len(parts)-3:], "/")
			}
		}
	}

	return err, stackFrames
}