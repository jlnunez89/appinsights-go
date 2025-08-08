package appinsights

import (
	"context"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAutoCollectionConfig(t *testing.T) {
	config := NewAutoCollectionConfig()
	
	// Verify default values
	if !config.HTTP.Enabled {
		t.Error("HTTP auto-collection should be enabled by default")
	}
	
	if !config.HTTP.EnableRequestTracking {
		t.Error("HTTP request tracking should be enabled by default")
	}
	
	if !config.HTTP.EnableDependencyTracking {
		t.Error("HTTP dependency tracking should be enabled by default")
	}
	
	if !config.PerformanceCounters.Enabled {
		t.Error("Performance counters should be enabled by default")
	}
	
	if !config.Database.Enabled {
		t.Error("Database auto-collection should be enabled by default")
	}
	
	if config.MessageQueue.Enabled {
		t.Error("Message queue auto-collection should be disabled by default")
	}
	
	if config.HTTP.MaxURLLength != 2048 {
		t.Errorf("Expected MaxURLLength to be 2048, got %d", config.HTTP.MaxURLLength)
	}
	
	if config.Database.MaxQueryLength != 1024 {
		t.Errorf("Expected MaxQueryLength to be 1024, got %d", config.Database.MaxQueryLength)
	}
}

func TestAutoCollectionManager_Creation(t *testing.T) {
	client := NewTelemetryClient("test-key")
	config := NewAutoCollectionConfig()
	
	manager := NewAutoCollectionManager(client, config)
	
	if manager == nil {
		t.Fatal("Auto-collection manager should not be nil")
	}
	
	// Verify components are created when enabled
	if manager.HTTPMiddleware() == nil {
		t.Error("HTTP middleware should be created when enabled")
	}
	
	if manager.ErrorCollector() == nil {
		t.Error("Error collector should be created when enabled")
	}
	
	if manager.DatabaseInstrumentor() == nil {
		t.Error("Database instrumentor should be created when enabled")
	}
}

func TestAutoCollectionManager_DisabledComponents(t *testing.T) {
	client := NewTelemetryClient("test-key")
	config := NewAutoCollectionConfig()
	
	// Disable all components
	config.HTTP.Enabled = false
	config.Errors.Enabled = false
	config.Database.Enabled = false
	
	manager := NewAutoCollectionManager(client, config)
	
	if manager.HTTPMiddleware() != nil {
		t.Error("HTTP middleware should not be created when disabled")
	}
	
	if manager.ErrorCollector() != nil {
		t.Error("Error collector should not be created when disabled")
	}
	
	if manager.DatabaseInstrumentor() != nil {
		t.Error("Database instrumentor should not be created when disabled")
	}
}

func TestAutoCollectionManager_StartStop(t *testing.T) {
	client := NewTelemetryClient("test-key")
	config := NewAutoCollectionConfig()
	
	manager := NewAutoCollectionManager(client, config)
	
	// Start should not panic
	manager.Start()
	
	// Stop should not panic
	manager.Stop()
	
	// Multiple starts/stops should be safe
	manager.Start()
	manager.Start()
	manager.Stop()
	manager.Stop()
}

func TestAutoCollectionManager_HTTPWrapping(t *testing.T) {
	client := NewTelemetryClient("test-key")
	config := NewAutoCollectionConfig()
	
	manager := NewAutoCollectionManager(client, config)
	
	// Test HTTP handler wrapping
	originalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})
	
	wrappedHandler := manager.WrapHTTPHandler(originalHandler)
	if wrappedHandler == nil {
		t.Fatal("Wrapped handler should not be nil")
	}
	
	// Test with disabled request tracking
	config.HTTP.EnableRequestTracking = false
	manager = NewAutoCollectionManager(client, config)
	
	wrappedHandler = manager.WrapHTTPHandler(originalHandler)
	// Since we can't compare functions directly, just ensure it's not nil
	if wrappedHandler == nil {
		t.Error("Wrapped handler should not be nil even when request tracking is disabled")
	}
}

func TestAutoCollectionManager_HTTPClientWrapping(t *testing.T) {
	client := NewTelemetryClient("test-key")
	config := NewAutoCollectionConfig()
	
	manager := NewAutoCollectionManager(client, config)
	
	// Test HTTP client wrapping
	originalClient := &http.Client{Timeout: 10 * time.Second}
	wrappedClient := manager.WrapHTTPClient(originalClient)
	
	if wrappedClient == nil {
		t.Fatal("Wrapped client should not be nil")
	}
	
	if wrappedClient == originalClient {
		t.Error("Client should be wrapped, not the same instance")
	}
	
	if wrappedClient.Timeout != originalClient.Timeout {
		t.Error("Wrapped client should preserve original timeout")
	}
	
	// Test with disabled dependency tracking
	config.HTTP.EnableDependencyTracking = false
	manager = NewAutoCollectionManager(client, config)
	
	wrappedClient = manager.WrapHTTPClient(originalClient)
	if wrappedClient != originalClient {
		t.Error("Client should not be wrapped when dependency tracking is disabled")
	}
}

func TestAutoCollectionManager_ErrorTracking(t *testing.T) {
	client := NewTelemetryClient("test-key")
	config := NewAutoCollectionConfig()
	
	manager := NewAutoCollectionManager(client, config)
	
	// Test error tracking
	testErr := "test error"
	manager.TrackError(testErr) // Should not panic
	
	// Test error tracking with context
	ctx := context.Background()
	manager.TrackErrorWithContext(ctx, testErr) // Should not panic
	
	// Test panic recovery
	recovered := false
	manager.RecoverPanic(func() {
		recovered = true
		// Normal execution, no panic
	})
	
	if !recovered {
		t.Error("Function should have been executed")
	}
	
	// Test with disabled error collection
	config.Errors.Enabled = false
	manager = NewAutoCollectionManager(client, config)
	
	// These should not panic even with disabled error collection
	manager.TrackError(testErr)
	manager.TrackErrorWithContext(ctx, testErr)
	manager.RecoverPanic(func() {})
}

func TestTelemetryClientWithAutoCollection(t *testing.T) {
	config := NewTelemetryConfiguration("test-key")
	config.AutoCollection = NewAutoCollectionConfig()
	
	client := NewTelemetryClientFromConfig(config)
	
	autoCollection := client.AutoCollection()
	if autoCollection == nil {
		t.Fatal("Auto-collection manager should be available")
	}
	
	// Verify components are accessible
	if autoCollection.HTTPMiddleware() == nil {
		t.Error("HTTP middleware should be available")
	}
	
	if autoCollection.ErrorCollector() == nil {
		t.Error("Error collector should be available")
	}
	
	if autoCollection.DatabaseInstrumentor() == nil {
		t.Error("Database instrumentor should be available")
	}
}

func TestTelemetryClientWithoutAutoCollection(t *testing.T) {
	client := NewTelemetryClient("test-key")
	
	autoCollection := client.AutoCollection()
	if autoCollection != nil {
		t.Error("Auto-collection manager should be nil when not configured")
	}
}

func TestAutoCollectionHTTPIntegration(t *testing.T) {
	client := NewTelemetryClient("test-key")
	config := NewAutoCollectionConfig()
	
	manager := NewAutoCollectionManager(client, config)
	manager.Start()
	defer manager.Stop()
	
	// Create test server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	})
	
	wrappedHandler := manager.WrapHTTPHandler(handler)
	server := httptest.NewServer(wrappedHandler)
	defer server.Close()
	
	// Create instrumented client
	httpClient := manager.WrapHTTPClient(&http.Client{Timeout: 5 * time.Second})
	
	// Make request
	resp, err := httpClient.Get(server.URL)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestDatabaseInstrumentorBasic(t *testing.T) {
	client := NewTelemetryClient("test-key")
	config := AutoCollectionDatabaseConfig{
		Enabled:                 true,
		EnableCommandCollection: true,
		QuerySanitization:       true,
		MaxQueryLength:          1024,
	}
	
	instrumentor := NewDatabaseInstrumentor(client, config)
	if instrumentor == nil {
		t.Fatal("Database instrumentor should not be nil")
	}
	
	// Test driver wrapping
	testDriver := &testDatabaseDriver{}
	wrappedDriver := instrumentor.WrapDriver("test", testDriver)
	
	if wrappedDriver == nil {
		t.Fatal("Wrapped driver should not be nil")
	}
	
	if wrappedDriver == testDriver {
		t.Error("Driver should be wrapped, not the same instance")
	}
}

// Mock database driver for testing
type testDatabaseDriver struct{}

func (d *testDatabaseDriver) Open(name string) (driver.Conn, error) {
	return &testDatabaseConn{}, nil
}

type testDatabaseConn struct{}

func (c *testDatabaseConn) Prepare(query string) (driver.Stmt, error) {
	return &testDatabaseStmt{query: query}, nil
}

func (c *testDatabaseConn) Close() error {
	return nil
}

func (c *testDatabaseConn) Begin() (driver.Tx, error) {
	return &testDatabaseTx{}, nil
}

type testDatabaseStmt struct {
	query string
}

func (s *testDatabaseStmt) Close() error {
	return nil
}

func (s *testDatabaseStmt) NumInput() int {
	return 0
}

func (s *testDatabaseStmt) Exec(args []driver.Value) (driver.Result, error) {
	return &testResult{}, nil
}

func (s *testDatabaseStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &testRows{}, nil
}

type testDatabaseTx struct{}

func (tx *testDatabaseTx) Commit() error {
	return nil
}

func (tx *testDatabaseTx) Rollback() error {
	return nil
}

type testResult struct{}

func (r *testResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (r *testResult) RowsAffected() (int64, error) {
	return 1, nil
}

type testRows struct{}

func (r *testRows) Columns() []string {
	return []string{"id", "name"}
}

func (r *testRows) Close() error {
	return nil
}

func (r *testRows) Next(dest []driver.Value) error {
	return nil
}

func TestQuerySanitization(t *testing.T) {
	client := NewTelemetryClient("test-key")
	config := AutoCollectionDatabaseConfig{
		Enabled:           true,
		QuerySanitization: true,
	}
	
	instrumentor := NewDatabaseInstrumentor(client, config)
	testStmt := &instrumentedStmt{
		query:        "SELECT * FROM users WHERE password = 'secret123'",
		instrumentor: instrumentor,
	}
	
	sanitized := testStmt.sanitizeQuery(testStmt.query)
	
	if sanitized == testStmt.query {
		t.Error("Query should have been sanitized")
	}
	
	if !contains(sanitized, "[REDACTED]") {
		t.Error("Sanitized query should contain [REDACTED]")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func strings_Contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}