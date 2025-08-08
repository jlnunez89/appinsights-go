package appinsights

import (
	"crypto/md5"
	"encoding/binary"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
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

// extractTelemetryTypeFromName is a helper function that can be used by other sampling processors
// to extract telemetry type from envelope names
func extractTelemetryTypeFromName(envelopeName string) TelemetryType {
	// Reuse the logic from PerTypeSamplingProcessor
	processor := &PerTypeSamplingProcessor{}
	return processor.extractTelemetryType(envelopeName)
}

// AdaptiveSamplingConfig holds configuration for adaptive sampling
type AdaptiveSamplingConfig struct {
	// MaxItemsPerSecond is the target maximum items per second across all telemetry types
	MaxItemsPerSecond float64
	
	// EvaluationWindow is how often to evaluate and adjust sampling rates (default: 15 seconds)
	EvaluationWindow time.Duration
	
	// InitialSamplingRate is the initial sampling rate to start with (0-100, default: 100)
	InitialSamplingRate float64
	
	// MinSamplingRate is the minimum sampling rate allowed (0-100, default: 1)
	MinSamplingRate float64
	
	// MaxSamplingRate is the maximum sampling rate allowed (0-100, default: 100)
	MaxSamplingRate float64
	
	// PerTypeConfigs allows setting different limits per telemetry type
	PerTypeConfigs map[TelemetryType]AdaptiveTypeConfig
}

// AdaptiveTypeConfig holds per-type configuration for adaptive sampling
type AdaptiveTypeConfig struct {
	// MaxItemsPerSecond for this specific telemetry type
	MaxItemsPerSecond float64
	
	// MinSamplingRate for this type (overrides global setting)
	MinSamplingRate float64
	
	// MaxSamplingRate for this type (overrides global setting)
	MaxSamplingRate float64
}

// AdaptiveSamplingProcessor implements volume-based adaptive sampling
type AdaptiveSamplingProcessor struct {
	config            AdaptiveSamplingConfig
	mutex             sync.RWMutex
	currentRates      map[TelemetryType]float64 // Current sampling rates per type
	globalRate        float64                   // Global sampling rate
	lastEvaluation    time.Time
	volumeCounters    map[TelemetryType]*VolumeCounter
	globalCounter     *VolumeCounter
	clock             clock.Clock // For testing
}

// VolumeCounter tracks telemetry volume over time
type VolumeCounter struct {
	counts    []int       // Circular buffer of counts per second
	times     []time.Time // Timestamps for each bucket
	index     int         // Current index in circular buffer
	size      int         // Size of the circular buffer
	mutex     sync.RWMutex
}

// NewVolumeCounter creates a new volume counter with specified window size in seconds
func NewVolumeCounter(windowSize int) *VolumeCounter {
	return &VolumeCounter{
		counts: make([]int, windowSize),
		times:  make([]time.Time, windowSize),
		size:   windowSize,
	}
}

// Record records a telemetry item at the current time
func (vc *VolumeCounter) Record(timestamp time.Time) {
	vc.mutex.Lock()
	defer vc.mutex.Unlock()
	
	// Get current second bucket
	currentSecond := timestamp.Truncate(time.Second)
	
	// If this is a new second, advance to next bucket
	if vc.times[vc.index].IsZero() || !vc.times[vc.index].Equal(currentSecond) {
		vc.index = (vc.index + 1) % vc.size
		vc.counts[vc.index] = 0
		vc.times[vc.index] = currentSecond
	}
	
	vc.counts[vc.index]++
}

// GetRate returns the current rate (items per second) over the tracked window
func (vc *VolumeCounter) GetRate(currentTime time.Time) float64 {
	vc.mutex.RLock()
	defer vc.mutex.RUnlock()
	
	cutoff := currentTime.Add(-time.Duration(vc.size) * time.Second)
	totalCount := 0
	validSeconds := 0
	
	for i := 0; i < vc.size; i++ {
		if !vc.times[i].IsZero() && vc.times[i].After(cutoff) {
			totalCount += vc.counts[i]
			validSeconds++
		}
	}
	
	if validSeconds == 0 {
		return 0
	}
	
	return float64(totalCount) / float64(validSeconds)
}

// NewAdaptiveSamplingProcessor creates a new adaptive sampling processor
func NewAdaptiveSamplingProcessor(config AdaptiveSamplingConfig) *AdaptiveSamplingProcessor {
	// Set defaults
	if config.EvaluationWindow <= 0 {
		config.EvaluationWindow = 15 * time.Second
	}
	if config.InitialSamplingRate <= 0 {
		config.InitialSamplingRate = 100
	}
	if config.MinSamplingRate <= 0 {
		config.MinSamplingRate = 1
	}
	if config.MaxSamplingRate <= 0 {
		config.MaxSamplingRate = 100
	}
	if config.MaxItemsPerSecond <= 0 {
		config.MaxItemsPerSecond = 100 // Default to 100 items per second
	}
	
	// Clamp values
	if config.InitialSamplingRate > 100 {
		config.InitialSamplingRate = 100
	}
	if config.MinSamplingRate > 100 {
		config.MinSamplingRate = 100
	}
	if config.MaxSamplingRate > 100 {
		config.MaxSamplingRate = 100
	}
	if config.MinSamplingRate > config.MaxSamplingRate {
		config.MinSamplingRate = config.MaxSamplingRate
	}
	
	windowSize := int(config.EvaluationWindow.Seconds()) + 1 // +1 for safety
	
	processor := &AdaptiveSamplingProcessor{
		config:         config,
		currentRates:   make(map[TelemetryType]float64),
		globalRate:     config.InitialSamplingRate,
		lastEvaluation: time.Time{},
		volumeCounters: make(map[TelemetryType]*VolumeCounter),
		globalCounter:  NewVolumeCounter(windowSize),
		clock:          currentClock,
	}
	
	// Initialize per-type counters and rates
	for telType := range config.PerTypeConfigs {
		processor.volumeCounters[telType] = NewVolumeCounter(windowSize)
		processor.currentRates[telType] = config.InitialSamplingRate
	}
	
	return processor
}

// ShouldSample implements the SamplingProcessor interface with adaptive logic
func (p *AdaptiveSamplingProcessor) ShouldSample(envelope *contracts.Envelope) bool {
	now := p.clock.Now()
	
	// Extract telemetry type
	telType := extractTelemetryTypeFromName(envelope.Name)
	
	// Record this telemetry item for volume tracking
	p.globalCounter.Record(now)
	if counter, exists := p.volumeCounters[telType]; exists {
		counter.Record(now)
	}
	
	// Check if it's time to evaluate and adjust sampling rates
	p.mutex.Lock()
	if p.lastEvaluation.IsZero() || now.Sub(p.lastEvaluation) >= p.config.EvaluationWindow {
		p.evaluateAndAdjustRates(now)
		p.lastEvaluation = now
	}
	p.mutex.Unlock()
	
	// Get current sampling rate for this type
	p.mutex.RLock()
	samplingRate := p.globalRate
	if typeRate, exists := p.currentRates[telType]; exists {
		samplingRate = typeRate
	}
	p.mutex.RUnlock()
	
	// Set sampling metadata
	if samplingRate > 0 {
		envelope.SampleRate = 100.0 / samplingRate
	} else {
		envelope.SampleRate = 0.0
	}
	
	// Apply deterministic hash-based sampling (reuse existing logic)
	if samplingRate >= 100 {
		return true
	}
	if samplingRate <= 0 {
		return false
	}
	
	// Use operation ID for deterministic sampling
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

// evaluateAndAdjustRates adjusts sampling rates based on current volume
// Must be called with write lock held
func (p *AdaptiveSamplingProcessor) evaluateAndAdjustRates(now time.Time) {
	// Get global rate
	globalRate := p.globalCounter.GetRate(now)
	
	// Adjust global rate if needed
	if globalRate > p.config.MaxItemsPerSecond {
		// Too much volume, decrease sampling rate
		targetReduction := globalRate / p.config.MaxItemsPerSecond
		newRate := p.globalRate / targetReduction
		
		// Apply gradual adjustment (maximum 50% change per evaluation)
		maxChange := p.globalRate * 0.5
		if p.globalRate-newRate > maxChange {
			newRate = p.globalRate - maxChange
		}
		
		// Respect minimum
		if newRate < p.config.MinSamplingRate {
			newRate = p.config.MinSamplingRate
		}
		
		p.globalRate = newRate
	} else if globalRate < p.config.MaxItemsPerSecond*0.5 {
		// Low volume, can increase sampling rate
		newRate := p.globalRate * 1.2 // Gradual increase
		
		// Respect maximum
		if newRate > p.config.MaxSamplingRate {
			newRate = p.config.MaxSamplingRate
		}
		
		p.globalRate = newRate
	}
	
	// Adjust per-type rates
	for telType, typeConfig := range p.config.PerTypeConfigs {
		if counter, exists := p.volumeCounters[telType]; exists {
			typeRate := counter.GetRate(now)
			currentSamplingRate := p.currentRates[telType]
			
			if typeRate > typeConfig.MaxItemsPerSecond {
				// Too much volume for this type
				targetReduction := typeRate / typeConfig.MaxItemsPerSecond
				newRate := currentSamplingRate / targetReduction
				
				// Apply gradual adjustment
				maxChange := currentSamplingRate * 0.5
				if currentSamplingRate-newRate > maxChange {
					newRate = currentSamplingRate - maxChange
				}
				
				// Respect per-type minimum
				minRate := typeConfig.MinSamplingRate
				if minRate <= 0 {
					minRate = p.config.MinSamplingRate
				}
				if newRate < minRate {
					newRate = minRate
				}
				
				p.currentRates[telType] = newRate
			} else if typeRate < typeConfig.MaxItemsPerSecond*0.5 {
				// Low volume for this type, can increase
				newRate := currentSamplingRate * 1.2
				
				// Respect per-type maximum
				maxRate := typeConfig.MaxSamplingRate
				if maxRate <= 0 {
					maxRate = p.config.MaxSamplingRate
				}
				if newRate > maxRate {
					newRate = maxRate
				}
				
				p.currentRates[telType] = newRate
			}
		}
	}
}

// GetSamplingRate returns the current global sampling rate
func (p *AdaptiveSamplingProcessor) GetSamplingRate() float64 {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.globalRate
}

// GetSamplingRateForType returns the current sampling rate for a specific type
func (p *AdaptiveSamplingProcessor) GetSamplingRateForType(telType TelemetryType) float64 {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	if rate, exists := p.currentRates[telType]; exists {
		return rate
	}
	return p.globalRate
}

// GetCurrentVolumeRate returns the current volume rate (items per second)
func (p *AdaptiveSamplingProcessor) GetCurrentVolumeRate() float64 {
	now := p.clock.Now()
	return p.globalCounter.GetRate(now)
}

// GetCurrentVolumeRateForType returns the current volume rate for a specific type
func (p *AdaptiveSamplingProcessor) GetCurrentVolumeRateForType(telType TelemetryType) float64 {
	if counter, exists := p.volumeCounters[telType]; exists {
		now := p.clock.Now()
		return counter.GetRate(now)
	}
	return 0
}