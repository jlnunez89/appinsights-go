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
	return true
}

// GetSamplingRate returns 100% (no sampling)
func (p *DisabledSamplingProcessor) GetSamplingRate() float64 {
	return 100.0
}