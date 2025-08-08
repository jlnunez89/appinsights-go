package appinsights

import (
	"context"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

// Application Insights telemetry client provides interface to track telemetry
// items.
type TelemetryClient interface {
	// Gets the telemetry context for this client. Values found on this
	// context will get written out to every telemetry item tracked by
	// this client.
	Context() *TelemetryContext

	// Gets the instrumentation key assigned to this telemetry client.
	InstrumentationKey() string

	// Gets the telemetry channel used to submit data to the backend.
	Channel() TelemetryChannel

	// Gets whether this client is enabled and will accept telemetry.
	IsEnabled() bool

	// Enables or disables the telemetry client. When disabled, telemetry
	// is silently swallowed by the client. Defaults to enabled.
	SetIsEnabled(enabled bool)

	// Submits the specified telemetry item.
	Track(telemetry Telemetry)

	// Submits the specified telemetry item with correlation context support.
	TrackWithContext(ctx context.Context, telemetry Telemetry)

	// Log a user action with the specified name
	TrackEvent(name string)

	// Log a numeric value that is not specified with a specific event.
	// Typically used to send regular reports of performance indicators.
	TrackMetric(name string, value float64)

	// Log a trace message with the specified severity level.
	TrackTrace(name string, severity contracts.SeverityLevel)

	// Log an HTTP request with the specified method, URL, duration and
	// response code.
	TrackRequest(method, url string, duration time.Duration, responseCode string)

	// Log a dependency with the specified name, type, target, and
	// success status.
	TrackRemoteDependency(name, dependencyType, target string, success bool)

	// Log an availability test result with the specified test name,
	// duration, and success status.
	TrackAvailability(name string, duration time.Duration, success bool)

	// Log an exception with the specified error, which may be a string,
	// error or Stringer. The current callstack is collected
	// automatically.
	TrackException(err interface{})

	// Gets the error auto-collector for this client (if enabled)
	ErrorAutoCollector() *ErrorAutoCollector

	// Context-aware tracking methods for improved correlation support

	// Log a user action with the specified name and correlation context
	TrackEventWithContext(ctx context.Context, name string)

	// Log a trace message with the specified severity level and correlation context
	TrackTraceWithContext(ctx context.Context, message string, severity contracts.SeverityLevel)

	// Log an HTTP request with correlation context
	TrackRequestWithContext(ctx context.Context, method, url string, duration time.Duration, responseCode string)

	// Log a dependency with correlation context
	TrackRemoteDependencyWithContext(ctx context.Context, name, dependencyType, target string, success bool)

	// Log an availability test result with correlation context
	TrackAvailabilityWithContext(ctx context.Context, name string, duration time.Duration, success bool)

	// Performance counter management methods

	// StartPerformanceCounterCollection begins periodic collection of performance counters
	StartPerformanceCounterCollection(config PerformanceCounterConfig)

	// StopPerformanceCounterCollection halts performance counter collection
	StopPerformanceCounterCollection()

	// IsPerformanceCounterCollectionEnabled returns true if performance counter collection is active
	IsPerformanceCounterCollectionEnabled() bool
}

type telemetryClient struct {
	channel            TelemetryChannel
	context            *TelemetryContext
	isEnabled          bool
	samplingProcessor  SamplingProcessor
	performanceManager *PerformanceCounterManager
	errorAutoCollector *ErrorAutoCollector
}

// Creates a new telemetry client instance that submits telemetry with the
// specified instrumentation key.
func NewTelemetryClient(iKey string) TelemetryClient {
	return NewTelemetryClientFromConfig(NewTelemetryConfiguration(iKey))
}

// Creates a new telemetry client instance configured by the specified
// TelemetryConfiguration object.
func NewTelemetryClientFromConfig(config *TelemetryConfiguration) TelemetryClient {
	samplingProcessor := config.SamplingProcessor
	if samplingProcessor == nil {
		// Default to no sampling (100% rate) for backward compatibility
		samplingProcessor = NewDisabledSamplingProcessor()
	}

	client := &telemetryClient{
		channel:           NewInMemoryChannel(config),
		context:           config.setupContext(),
		isEnabled:         true,
		samplingProcessor: samplingProcessor,
	}

	// Initialize error auto-collection if configured
	if config.ErrorAutoCollection != nil {
		client.errorAutoCollector = NewErrorAutoCollector(client, config.ErrorAutoCollection)
	}

	return client
}

// Gets the telemetry context for this client.  Values found on this context
// will get written out to every telemetry item tracked by this client.
func (tc *telemetryClient) Context() *TelemetryContext {
	return tc.context
}

// Gets the telemetry channel used to submit data to the backend.
func (tc *telemetryClient) Channel() TelemetryChannel {
	return tc.channel
}

// Gets the instrumentation key assigned to this telemetry client.
func (tc *telemetryClient) InstrumentationKey() string {
	return tc.context.InstrumentationKey()
}

// Gets whether this client is enabled and will accept telemetry.
func (tc *telemetryClient) IsEnabled() bool {
	return tc.isEnabled
}

// Enables or disables the telemetry client.  When disabled, telemetry is
// silently swallowed by the client.  Defaults to enabled.
func (tc *telemetryClient) SetIsEnabled(isEnabled bool) {
	tc.isEnabled = isEnabled
}

// Submits the specified telemetry item.
func (tc *telemetryClient) Track(item Telemetry) {
	if tc.isEnabled && item != nil {
		envelope := tc.context.envelop(item)
		if tc.samplingProcessor.ShouldSample(envelope) {
			tc.channel.Send(envelope)
		}
	}
}

// Submits the specified telemetry item with correlation context support.
func (tc *telemetryClient) TrackWithContext(ctx context.Context, item Telemetry) {
	if tc.isEnabled && item != nil {
		envelope := tc.context.envelopWithContext(ctx, item)
		if tc.samplingProcessor.ShouldSample(envelope) {
			tc.channel.Send(envelope)
		}
	}
}

// Log a user action with the specified name
func (tc *telemetryClient) TrackEvent(name string) {
	tc.Track(NewEventTelemetry(name))
}

// Log a numeric value that is not specified with a specific event.
// Typically used to send regular reports of performance indicators.
func (tc *telemetryClient) TrackMetric(name string, value float64) {
	tc.Track(NewMetricTelemetry(name, value))
}

// Log a trace message with the specified severity level.
func (tc *telemetryClient) TrackTrace(message string, severity contracts.SeverityLevel) {
	tc.Track(NewTraceTelemetry(message, severity))
}

// Log an HTTP request with the specified method, URL, duration and response
// code.
func (tc *telemetryClient) TrackRequest(method, url string, duration time.Duration, responseCode string) {
	tc.Track(NewRequestTelemetry(method, url, duration, responseCode))
}

// Log a dependency with the specified name, type, target, and success
// status.
func (tc *telemetryClient) TrackRemoteDependency(name, dependencyType, target string, success bool) {
	tc.Track(NewRemoteDependencyTelemetry(name, dependencyType, target, success))
}

// Log an availability test result with the specified test name, duration,
// and success status.
func (tc *telemetryClient) TrackAvailability(name string, duration time.Duration, success bool) {
	tc.Track(NewAvailabilityTelemetry(name, duration, success))
}

// Log an exception with the specified error, which may be a string, error
// or Stringer.  The current callstack is collected automatically.
func (tc *telemetryClient) TrackException(err interface{}) {
	tc.Track(newExceptionTelemetry(err, 1))
}

// Context-aware tracking methods for improved correlation support

// Log a user action with the specified name and correlation context
func (tc *telemetryClient) TrackEventWithContext(ctx context.Context, name string) {
	tc.TrackWithContext(ctx, NewEventTelemetry(name))
}

// Log a trace message with the specified severity level and correlation context
func (tc *telemetryClient) TrackTraceWithContext(ctx context.Context, message string, severity contracts.SeverityLevel) {
	tc.TrackWithContext(ctx, NewTraceTelemetry(message, severity))
}

// Log an HTTP request with correlation context
func (tc *telemetryClient) TrackRequestWithContext(ctx context.Context, method, url string, duration time.Duration, responseCode string) {
	tc.TrackWithContext(ctx, NewRequestTelemetryWithContext(ctx, method, url, duration, responseCode))
}

// Log a dependency with correlation context
func (tc *telemetryClient) TrackRemoteDependencyWithContext(ctx context.Context, name, dependencyType, target string, success bool) {
	tc.TrackWithContext(ctx, NewRemoteDependencyTelemetryWithContext(ctx, name, dependencyType, target, success))
}

// Log an availability test result with correlation context
func (tc *telemetryClient) TrackAvailabilityWithContext(ctx context.Context, name string, duration time.Duration, success bool) {
	tc.TrackWithContext(ctx, NewAvailabilityTelemetryWithContext(ctx, name, duration, success))
}

// StartPerformanceCounterCollection begins periodic collection of performance counters
func (tc *telemetryClient) StartPerformanceCounterCollection(config PerformanceCounterConfig) {
	if tc.performanceManager != nil {
		tc.performanceManager.Stop()
	}
	
	tc.performanceManager = NewPerformanceCounterManager(tc, config)
	tc.performanceManager.Start()
}

// StopPerformanceCounterCollection halts performance counter collection
func (tc *telemetryClient) StopPerformanceCounterCollection() {
	if tc.performanceManager != nil {
		tc.performanceManager.Stop()
		tc.performanceManager = nil
	}
}

// IsPerformanceCounterCollectionEnabled returns true if performance counter collection is active
func (tc *telemetryClient) IsPerformanceCounterCollectionEnabled() bool {
	return tc.performanceManager != nil
}

// Gets the error auto-collector for this client (if enabled)
func (tc *telemetryClient) ErrorAutoCollector() *ErrorAutoCollector {
	return tc.errorAutoCollector
}
