package appinsights

import (
	"crypto/md5"
	"encoding/binary"
	"strings"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

// SamplingProcessor defines the interface for making sampling decisions
type SamplingProcessor interface {
	// ShouldSample returns true if the telemetry item should be sampled (kept)
	ShouldSample(envelope *contracts.Envelope) bool
	
	// GetSamplingRate returns the current sampling rate as a percentage (0-100)
	GetSamplingRate() float64
}

// FixedRateSamplingProcessor implements a simple fixed-rate sampling strategy
type FixedRateSamplingProcessor struct {
	samplingRate float64 // Sampling rate as a percentage (0-100)
}

// NewFixedRateSamplingProcessor creates a new fixed-rate sampling processor
// samplingRate should be between 0 and 100 (percentage)
func NewFixedRateSamplingProcessor(samplingRate float64) *FixedRateSamplingProcessor {
	if samplingRate < 0 {
		samplingRate = 0
	}
	if samplingRate > 100 {
		samplingRate = 100
	}
	
	return &FixedRateSamplingProcessor{
		samplingRate: samplingRate,
	}
}

// ShouldSample implements deterministic hash-based sampling for consistency
func (p *FixedRateSamplingProcessor) ShouldSample(envelope *contracts.Envelope) bool {
	// Set sampling metadata in envelope
	if p.samplingRate > 0 {
		envelope.SampleRate = 100.0 / p.samplingRate
	} else {
		// For 0% sampling, no items are actually sent, so this value won't be used
		// but we set it to a reasonable value to avoid +Inf
		envelope.SampleRate = 0.0
	}
	
	if p.samplingRate >= 100 {
		return true
	}
	if p.samplingRate <= 0 {
		return false
	}
	
	// Use operation ID for deterministic sampling across correlated operations
	operationId := ""
	if envelope.Tags != nil {
		if opId, exists := envelope.Tags[contracts.OperationId]; exists {
			operationId = opId
		}
	}
	
	// Fall back to envelope name + ikey if no operation ID
	if operationId == "" {
		operationId = envelope.Name + envelope.IKey
	}
	
	// Calculate hash-based sampling decision
	hash := calculateSamplingHash(operationId)
	threshold := uint32((p.samplingRate / 100.0) * 0xFFFFFFFF)
	
	return hash < threshold
}

// GetSamplingRate returns the current sampling rate
func (p *FixedRateSamplingProcessor) GetSamplingRate() float64 {
	return p.samplingRate
}

// calculateSamplingHash creates a deterministic hash from the operation ID
// that's evenly distributed across the uint32 range
func calculateSamplingHash(operationId string) uint32 {
	if operationId == "" {
		return 0
	}
	
	// Normalize the operation ID (remove dashes, convert to lowercase)
	normalized := strings.ToLower(strings.ReplaceAll(operationId, "-", ""))
	
	// Calculate MD5 hash
	hasher := md5.New()
	hasher.Write([]byte(normalized))
	hashBytes := hasher.Sum(nil)
	
	// Take first 4 bytes and convert to uint32
	return binary.BigEndian.Uint32(hashBytes[:4])
}

// DisabledSamplingProcessor is a no-op processor that samples everything (100% rate)
type DisabledSamplingProcessor struct{}

// NewDisabledSamplingProcessor creates a sampling processor that doesn't filter anything
func NewDisabledSamplingProcessor() *DisabledSamplingProcessor {
	return &DisabledSamplingProcessor{}
}

// ShouldSample always returns true (no sampling)
func (p *DisabledSamplingProcessor) ShouldSample(envelope *contracts.Envelope) bool {
	// Set sampling metadata - no sampling means each item represents itself (1:1 ratio)
	envelope.SampleRate = 1.0
	return true
}

// GetSamplingRate returns 100% (no sampling)
func (p *DisabledSamplingProcessor) GetSamplingRate() float64 {
	return 100.0
}

// TelemetryType represents the type of telemetry data
type TelemetryType string

const (
	TelemetryTypeEvent            TelemetryType = "Event"
	TelemetryTypeTrace            TelemetryType = "Message"
	TelemetryTypeMetric           TelemetryType = "Metric"
	TelemetryTypeRequest          TelemetryType = "Request"
	TelemetryTypeRemoteDependency TelemetryType = "RemoteDependency"
	TelemetryTypeException        TelemetryType = "Exception"
	TelemetryTypeAvailability     TelemetryType = "Availability"
	TelemetryTypePageView         TelemetryType = "PageView"
)

// PerTypeSamplingProcessor implements per-telemetry-type sampling strategy
type PerTypeSamplingProcessor struct {
	typeRates   map[TelemetryType]float64 // Sampling rates per telemetry type
	defaultRate float64                   // Default rate for unknown types
}

// NewPerTypeSamplingProcessor creates a new per-type sampling processor
func NewPerTypeSamplingProcessor(defaultRate float64, typeRates map[TelemetryType]float64) *PerTypeSamplingProcessor {
	// Clamp default rate
	if defaultRate < 0 {
		defaultRate = 0
	}
	if defaultRate > 100 {
		defaultRate = 100
	}
	
	// Clamp all type-specific rates
	normalizedRates := make(map[TelemetryType]float64)
	for telType, rate := range typeRates {
		if rate < 0 {
			rate = 0
		}
		if rate > 100 {
			rate = 100
		}
		normalizedRates[telType] = rate
	}
	
	return &PerTypeSamplingProcessor{
		typeRates:   normalizedRates,
		defaultRate: defaultRate,
	}
}

// ShouldSample implements per-type deterministic hash-based sampling
func (p *PerTypeSamplingProcessor) ShouldSample(envelope *contracts.Envelope) bool {
	// Determine telemetry type from envelope name
	telType := p.extractTelemetryType(envelope.Name)
	
	// Get sampling rate for this type
	samplingRate := p.defaultRate
	if typeRate, exists := p.typeRates[telType]; exists {
		samplingRate = typeRate
	}
	
	// Set sampling metadata in envelope
	if samplingRate > 0 {
		envelope.SampleRate = 100.0 / samplingRate
	} else {
		// For 0% sampling, no items are actually sent, so this value won't be used
		// but we set it to a reasonable value to avoid +Inf
		envelope.SampleRate = 0.0
	}
	
	if samplingRate >= 100 {
		return true
	}
	if samplingRate <= 0 {
		return false
	}
	
	// Use operation ID for deterministic sampling across correlated operations
	operationId := ""
	if envelope.Tags != nil {
		if opId, exists := envelope.Tags[contracts.OperationId]; exists {
			operationId = opId
		}
	}
	
	// Fall back to envelope name + ikey if no operation ID
	if operationId == "" {
		operationId = envelope.Name + envelope.IKey
	}
	
	// Calculate hash-based sampling decision
	hash := calculateSamplingHash(operationId)
	threshold := uint32((samplingRate / 100.0) * 0xFFFFFFFF)
	
	return hash < threshold
}

// GetSamplingRate returns the default sampling rate
func (p *PerTypeSamplingProcessor) GetSamplingRate() float64 {
	return p.defaultRate
}

// GetSamplingRateForType returns the sampling rate for a specific telemetry type
func (p *PerTypeSamplingProcessor) GetSamplingRateForType(telType TelemetryType) float64 {
	if rate, exists := p.typeRates[telType]; exists {
		return rate
	}
	return p.defaultRate
}

// extractTelemetryType determines the telemetry type from envelope name
// Envelope names follow pattern: "Microsoft.ApplicationInsights.{key}.{Type}"
// or "Microsoft.ApplicationInsights.{Type}" when key is empty
func (p *PerTypeSamplingProcessor) extractTelemetryType(envelopeName string) TelemetryType {
	// Handle the different envelope name patterns
	if envelopeName == "" {
		return TelemetryType("")
	}
	
	// Find the last dot and extract the type
	lastDot := -1
	for i := len(envelopeName) - 1; i >= 0; i-- {
		if envelopeName[i] == '.' {
			lastDot = i
			break
		}
	}
	
	if lastDot == -1 || lastDot == len(envelopeName)-1 {
		return TelemetryType("")
	}
	
	typeName := envelopeName[lastDot+1:]
	
	// Map to known telemetry types
	switch typeName {
	case "Event":
		return TelemetryTypeEvent
	case "Message":
		return TelemetryTypeTrace
	case "Metric":
		return TelemetryTypeMetric
	case "Request":
		return TelemetryTypeRequest
	case "RemoteDependency":
		return TelemetryTypeRemoteDependency
	case "Exception":
		return TelemetryTypeException
	case "Availability":
		return TelemetryTypeAvailability
	case "PageView":
		return TelemetryTypePageView
	default:
		return TelemetryType("")
	}
}