package appinsights

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"net/http"
	"sync"
	"time"
)

// AutoCollectionConfig configures all automatic event collection features
type AutoCollectionConfig struct {
	// HTTP auto-collection settings
	HTTP AutoCollectionHTTPConfig

	// Error auto-collection settings  
	Errors *ErrorAutoCollectionConfig

	// Performance counter collection settings
	PerformanceCounters PerformanceCounterConfig

	// Database auto-instrumentation settings
	Database AutoCollectionDatabaseConfig

	// Message queue auto-instrumentation settings (future extension)
	MessageQueue AutoCollectionMessageQueueConfig
}

// AutoCollectionHTTPConfig configures HTTP auto-collection
type AutoCollectionHTTPConfig struct {
	// Enabled controls whether HTTP auto-collection is active
	Enabled bool

	// EnableRequestTracking enables automatic request telemetry
	EnableRequestTracking bool

	// EnableDependencyTracking enables automatic dependency telemetry for outbound HTTP calls
	EnableDependencyTracking bool

	// URLSanitization enables automatic URL sanitization to remove sensitive data
	URLSanitization bool

	// HeaderCollection specifies which headers to collect (empty = none, "*" = all, or specific header names)
	HeaderCollection []string

	// MaxURLLength limits the maximum URL length recorded (0 = no limit)
	MaxURLLength int
}

// AutoCollectionDatabaseConfig configures database auto-instrumentation
type AutoCollectionDatabaseConfig struct {
	// Enabled controls whether database auto-instrumentation is active
	Enabled bool

	// EnableCommandCollection enables collection of SQL commands and parameters
	EnableCommandCollection bool

	// EnableConnectionTracking enables tracking of database connection events
	EnableConnectionTracking bool

	// QuerySanitization enables automatic sanitization of SQL queries
	QuerySanitization bool

	// MaxQueryLength limits the maximum query length recorded (0 = no limit)
	MaxQueryLength int

	// CollectParameters enables collection of SQL parameters (security sensitive)
	CollectParameters bool
}

// AutoCollectionMessageQueueConfig configures message queue auto-instrumentation  
type AutoCollectionMessageQueueConfig struct {
	// Enabled controls whether message queue auto-instrumentation is active
	Enabled bool

	// EnableProducerTracking enables tracking of message publishing
	EnableProducerTracking bool

	// EnableConsumerTracking enables tracking of message consumption
	EnableConsumerTracking bool

	// CollectMessageContent enables collection of message content (security sensitive)
	CollectMessageContent bool

	// MaxMessageSize limits the maximum message size recorded (0 = no limit)
	MaxMessageSize int
}

// NewAutoCollectionConfig creates a new configuration with recommended defaults
func NewAutoCollectionConfig() *AutoCollectionConfig {
	return &AutoCollectionConfig{
		HTTP: AutoCollectionHTTPConfig{
			Enabled:                  true,
			EnableRequestTracking:    true,
			EnableDependencyTracking: true,
			URLSanitization:          true,
			HeaderCollection:         []string{}, // No headers by default for security
			MaxURLLength:             2048,
		},
		Errors: NewErrorAutoCollectionConfig(),
		PerformanceCounters: PerformanceCounterConfig{
			Enabled:              true,
			CollectionInterval:   60 * time.Second,
			EnableSystemMetrics:  true,
			EnableRuntimeMetrics: true,
			CustomCollectors:     []PerformanceCounterCollector{},
		},
		Database: AutoCollectionDatabaseConfig{
			Enabled:                 true,
			EnableCommandCollection: true,
			EnableConnectionTracking: false, // Disabled by default as it can be noisy
			QuerySanitization:       true,
			MaxQueryLength:          1024,
			CollectParameters:       false, // Disabled by default for security
		},
		MessageQueue: AutoCollectionMessageQueueConfig{
			Enabled:                false, // Disabled by default until implemented
			EnableProducerTracking: true,
			EnableConsumerTracking: true,
			CollectMessageContent:  false, // Disabled by default for security
			MaxMessageSize:         1024,
		},
	}
}

// AutoCollectionManager coordinates all automatic event collection features
type AutoCollectionManager struct {
	client TelemetryClient
	config *AutoCollectionConfig

	// Component managers
	httpMiddleware        *HTTPMiddleware
	errorCollector        *ErrorAutoCollector
	performanceManager    *PerformanceCounterManager
	databaseInstrumentor  *DatabaseInstrumentor

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
}

// NewAutoCollectionManager creates a new auto-collection manager
func NewAutoCollectionManager(client TelemetryClient, config *AutoCollectionConfig) *AutoCollectionManager {
	if config == nil {
		config = NewAutoCollectionConfig()
	}

	acm := &AutoCollectionManager{
		client: client,
		config: config,
	}

	acm.setupComponents()
	return acm
}

// setupComponents initializes all auto-collection components
func (acm *AutoCollectionManager) setupComponents() {
	// HTTP middleware
	if acm.config.HTTP.Enabled {
		acm.httpMiddleware = NewHTTPMiddleware()
		acm.httpMiddleware.GetClient = func(r *http.Request) TelemetryClient {
			return acm.client
		}
	}

	// Error auto-collection
	if acm.config.Errors != nil && acm.config.Errors.Enabled {
		acm.errorCollector = NewErrorAutoCollector(acm.client, acm.config.Errors)
	}

	// Performance counters
	if acm.config.PerformanceCounters.Enabled {
		acm.performanceManager = NewPerformanceCounterManager(acm.client, acm.config.PerformanceCounters)
	}

	// Database instrumentation
	if acm.config.Database.Enabled {
		acm.databaseInstrumentor = NewDatabaseInstrumentor(acm.client, acm.config.Database)
	}
}

// Start begins all enabled auto-collection features
func (acm *AutoCollectionManager) Start() {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	if acm.cancel != nil {
		return // Already started
	}

	acm.ctx, acm.cancel = context.WithCancel(context.Background())

	// Start performance counter collection
	if acm.performanceManager != nil {
		acm.performanceManager.Start()
	}

	// Error collection and HTTP middleware are passive and don't need starting
}

// Stop halts all auto-collection features
func (acm *AutoCollectionManager) Stop() {
	acm.mu.Lock()
	cancel := acm.cancel
	acm.cancel = nil
	acm.mu.Unlock()

	if cancel != nil {
		cancel()

		// Stop performance counter collection
		if acm.performanceManager != nil {
			acm.performanceManager.Stop()
		}
	}
}

// HTTPMiddleware returns the HTTP middleware for request tracking (if enabled)
func (acm *AutoCollectionManager) HTTPMiddleware() *HTTPMiddleware {
	acm.mu.RLock()
	defer acm.mu.RUnlock()
	return acm.httpMiddleware
}

// ErrorCollector returns the error auto-collector (if enabled)
func (acm *AutoCollectionManager) ErrorCollector() *ErrorAutoCollector {
	acm.mu.RLock()
	defer acm.mu.RUnlock()
	return acm.errorCollector
}

// DatabaseInstrumentor returns the database instrumentor (if enabled)
func (acm *AutoCollectionManager) DatabaseInstrumentor() *DatabaseInstrumentor {
	acm.mu.RLock()
	defer acm.mu.RUnlock()
	return acm.databaseInstrumentor
}

// WrapHTTPHandler wraps an HTTP handler with automatic request tracking
func (acm *AutoCollectionManager) WrapHTTPHandler(handler http.Handler) http.Handler {
	if acm.httpMiddleware == nil || !acm.config.HTTP.EnableRequestTracking {
		return handler
	}
	return acm.httpMiddleware.Middleware(handler)
}

// WrapHTTPClient wraps an HTTP client with automatic dependency tracking
func (acm *AutoCollectionManager) WrapHTTPClient(client *http.Client) *http.Client {
	if acm.httpMiddleware == nil || !acm.config.HTTP.EnableDependencyTracking {
		return client
	}

	if client == nil {
		client = &http.Client{}
	}

	// Clone the client to avoid modifying the original
	wrappedClient := *client
	wrappedClient.Transport = acm.httpMiddleware.WrapRoundTripper(client.Transport)
	return &wrappedClient
}

// WrapDatabase wraps a database driver with automatic dependency tracking
func (acm *AutoCollectionManager) WrapDatabase(driverName string, driver driver.Driver) driver.Driver {
	if acm.databaseInstrumentor == nil {
		return driver
	}
	return acm.databaseInstrumentor.WrapDriver(driverName, driver)
}

// WrapSQLDB wraps an existing sql.DB with instrumentation
func (acm *AutoCollectionManager) WrapSQLDB(db *sql.DB, driverName, dataSourceName string) *sql.DB {
	if acm.databaseInstrumentor == nil {
		return db
	}
	return acm.databaseInstrumentor.WrapDB(db, driverName, dataSourceName)
}

// TrackError tracks an error using the auto-collector (if enabled)
func (acm *AutoCollectionManager) TrackError(err interface{}) {
	if acm.errorCollector != nil {
		acm.errorCollector.TrackError(err)
	}
}

// TrackErrorWithContext tracks an error with context using the auto-collector (if enabled)
func (acm *AutoCollectionManager) TrackErrorWithContext(ctx context.Context, err interface{}) {
	if acm.errorCollector != nil {
		acm.errorCollector.TrackErrorWithContext(ctx, err)
	}
}

// RecoverPanic executes a function and recovers from panics using the auto-collector (if enabled)
func (acm *AutoCollectionManager) RecoverPanic(fn func()) {
	if acm.errorCollector != nil {
		acm.errorCollector.RecoverPanic(fn)
	} else {
		fn()
	}
}

// RecoverPanicWithContext executes a function and recovers from panics with context (if enabled)
func (acm *AutoCollectionManager) RecoverPanicWithContext(ctx context.Context, fn func()) {
	if acm.errorCollector != nil {
		acm.errorCollector.RecoverPanicWithContext(ctx, fn)
	} else {
		fn()
	}
}