package appinsights

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// DatabaseInstrumentor provides automatic instrumentation for database operations
type DatabaseInstrumentor struct {
	client TelemetryClient
	config AutoCollectionDatabaseConfig
}

// NewDatabaseInstrumentor creates a new database instrumentor
func NewDatabaseInstrumentor(client TelemetryClient, config AutoCollectionDatabaseConfig) *DatabaseInstrumentor {
	return &DatabaseInstrumentor{
		client: client,
		config: config,
	}
}

// WrapDriver wraps a database driver with instrumentation
func (di *DatabaseInstrumentor) WrapDriver(driverName string, d driver.Driver) driver.Driver {
	return &instrumentedDriver{
		driver:        d,
		driverName:    driverName,
		instrumentor:  di,
	}
}

// WrapDB wraps an existing sql.DB with instrumentation
func (di *DatabaseInstrumentor) WrapDB(db *sql.DB, driverName, dataSourceName string) *sql.DB {
	// This is a conceptual wrapper - in practice, we'd intercept at the driver level
	// For now, this returns the original DB but could be extended with a custom wrapper
	return db
}

// instrumentedDriver wraps a database driver
type instrumentedDriver struct {
	driver       driver.Driver
	driverName   string
	instrumentor *DatabaseInstrumentor
}

// Open implements driver.Driver interface
func (id *instrumentedDriver) Open(name string) (driver.Conn, error) {
	conn, err := id.driver.Open(name)
	
	target := id.extractTarget(name)
	success := err == nil
	
	// Track connection as dependency
	if id.instrumentor.config.EnableConnectionTracking {
		id.instrumentor.client.TrackRemoteDependency(
			"Database Connection",
			id.driverName,
			target,
			success,
		)
	}
	
	if err != nil {
		return nil, err
	}
	
	return &instrumentedConn{
		conn:         conn,
		driverName:   id.driverName,
		target:       target,
		instrumentor: id.instrumentor,
	}, nil
}

// extractTarget extracts the database target from connection string
func (id *instrumentedDriver) extractTarget(connectionString string) string {
	// Try to parse as URL first
	if u, err := url.Parse(connectionString); err == nil && u.Host != "" {
		return u.Host
	}
	
	// For other formats, try to extract host information
	// Common patterns: host=localhost, server=localhost, etc.
	hostPatterns := []*regexp.Regexp{
		regexp.MustCompile(`host=([^;\s]+)`),
		regexp.MustCompile(`server=([^;\s]+)`),
		regexp.MustCompile(`hostname=([^;\s]+)`),
		regexp.MustCompile(`address=([^;\s]+)`),
	}
	
	for _, pattern := range hostPatterns {
		if matches := pattern.FindStringSubmatch(connectionString); len(matches) > 1 {
			return matches[1]
		}
	}
	
	// Fallback to driver name if no host found
	return id.driverName
}

// instrumentedConn wraps a database connection
type instrumentedConn struct {
	conn         driver.Conn
	driverName   string
	target       string
	instrumentor *DatabaseInstrumentor
}

// Prepare implements driver.Conn interface
func (ic *instrumentedConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := ic.conn.Prepare(query)
	if err != nil {
		return nil, err
	}
	
	return &instrumentedStmt{
		stmt:         stmt,
		query:        query,
		driverName:   ic.driverName,
		target:       ic.target,
		instrumentor: ic.instrumentor,
	}, nil
}

// Close implements driver.Conn interface
func (ic *instrumentedConn) Close() error {
	return ic.conn.Close()
}

// Begin implements driver.Conn interface
func (ic *instrumentedConn) Begin() (driver.Tx, error) {
	tx, err := ic.conn.Begin()
	if err != nil {
		return nil, err
	}
	
	return &instrumentedTx{
		tx:           tx,
		driverName:   ic.driverName,
		target:       ic.target,
		instrumentor: ic.instrumentor,
	}, nil
}

// instrumentedStmt wraps a database statement
type instrumentedStmt struct {
	stmt         driver.Stmt
	query        string
	driverName   string
	target       string
	instrumentor *DatabaseInstrumentor
}

// Close implements driver.Stmt interface
func (is *instrumentedStmt) Close() error {
	return is.stmt.Close()
}

// NumInput implements driver.Stmt interface
func (is *instrumentedStmt) NumInput() int {
	return is.stmt.NumInput()
}

// Exec implements driver.Stmt interface
func (is *instrumentedStmt) Exec(args []driver.Value) (driver.Result, error) {
	startTime := time.Now()
	result, err := is.stmt.Exec(args)
	duration := time.Since(startTime)
	
	is.trackDatabaseOperation("Exec", duration, err)
	return result, err
}

// Query implements driver.Stmt interface
func (is *instrumentedStmt) Query(args []driver.Value) (driver.Rows, error) {
	startTime := time.Now()
	rows, err := is.stmt.Query(args)
	duration := time.Since(startTime)
	
	is.trackDatabaseOperation("Query", duration, err)
	return rows, err
}

// trackDatabaseOperation tracks a database operation as a dependency
func (is *instrumentedStmt) trackDatabaseOperation(operation string, duration time.Duration, err error) {
	if !is.instrumentor.config.EnableCommandCollection {
		return
	}
	
	success := err == nil
	dependencyName := fmt.Sprintf("%s %s", is.driverName, operation)
	
	// Create dependency telemetry with enhanced information
	dependency := NewRemoteDependencyTelemetry(dependencyName, is.driverName, is.target, success)
	dependency.Duration = duration
	
	if err != nil {
		dependency.ResultCode = "Error"
		dependency.Properties["error"] = err.Error()
	} else {
		dependency.ResultCode = "Success"
	}
	
	// Add query information if enabled
	if is.instrumentor.config.EnableCommandCollection && is.query != "" {
		query := is.query
		if is.instrumentor.config.QuerySanitization {
			query = is.sanitizeQuery(query)
		}
		if is.instrumentor.config.MaxQueryLength > 0 && len(query) > is.instrumentor.config.MaxQueryLength {
			query = query[:is.instrumentor.config.MaxQueryLength] + "..."
		}
		dependency.Data = query
		dependency.Properties["command_type"] = operation
	}
	
	is.instrumentor.client.Track(dependency)
}

// sanitizeQuery removes sensitive information from SQL queries
func (is *instrumentedStmt) sanitizeQuery(query string) string {
	// Convert to uppercase for pattern matching
	upperQuery := strings.ToUpper(query)
	
	// List of sensitive SQL patterns to sanitize
	sensitivePatterns := []*regexp.Regexp{
		// Password patterns
		regexp.MustCompile(`(?i)password\s*=\s*'[^']*'`),
		regexp.MustCompile(`(?i)password\s*=\s*"[^"]*"`),
		regexp.MustCompile(`(?i)pwd\s*=\s*'[^']*'`),
		regexp.MustCompile(`(?i)pwd\s*=\s*"[^"]*"`),
		
		// API key patterns
		regexp.MustCompile(`(?i)api_?key\s*=\s*'[^']*'`),
		regexp.MustCompile(`(?i)api_?key\s*=\s*"[^"]*"`),
		
		// Token patterns
		regexp.MustCompile(`(?i)token\s*=\s*'[^']*'`),
		regexp.MustCompile(`(?i)token\s*=\s*"[^"]*"`),
		
		// Credit card patterns (basic)
		regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`),
		
		// Social security patterns (basic)
		regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
	}
	
	sanitized := query
	for _, pattern := range sensitivePatterns {
		sanitized = pattern.ReplaceAllString(sanitized, "[REDACTED]")
	}
	
	// If this looks like an INSERT/UPDATE with many values, summarize it
	if strings.Contains(upperQuery, "INSERT") || strings.Contains(upperQuery, "UPDATE") {
		if strings.Count(sanitized, "?") > 10 {
			// Replace long parameter lists with summary
			paramPattern := regexp.MustCompile(`\([\s\?,'"]*\)`)
			sanitized = paramPattern.ReplaceAllString(sanitized, "(... parameters ...)")
		}
	}
	
	return sanitized
}

// instrumentedTx wraps a database transaction
type instrumentedTx struct {
	tx           driver.Tx
	driverName   string
	target       string
	instrumentor *DatabaseInstrumentor
}

// Commit implements driver.Tx interface
func (it *instrumentedTx) Commit() error {
	startTime := time.Now()
	err := it.tx.Commit()
	duration := time.Since(startTime)
	
	it.trackTransactionOperation("Commit", duration, err)
	return err
}

// Rollback implements driver.Tx interface
func (it *instrumentedTx) Rollback() error {
	startTime := time.Now()
	err := it.tx.Rollback()
	duration := time.Since(startTime)
	
	it.trackTransactionOperation("Rollback", duration, err)
	return err
}

// trackTransactionOperation tracks a transaction operation
func (it *instrumentedTx) trackTransactionOperation(operation string, duration time.Duration, err error) {
	success := err == nil
	dependencyName := fmt.Sprintf("%s %s", it.driverName, operation)
	
	dependency := NewRemoteDependencyTelemetry(dependencyName, it.driverName, it.target, success)
	dependency.Duration = duration
	
	if err != nil {
		dependency.ResultCode = "Error"
		dependency.Properties["error"] = err.Error()
	} else {
		dependency.ResultCode = "Success"
	}
	
	dependency.Properties["operation_type"] = "Transaction"
	dependency.Properties["transaction_operation"] = operation
	
	it.instrumentor.client.Track(dependency)
}

// RegisterDatabaseDriver registers a database driver with automatic instrumentation
func RegisterDatabaseDriver(client TelemetryClient, config AutoCollectionDatabaseConfig, driverName string, driver driver.Driver) {
	if !config.Enabled {
		sql.Register(driverName, driver)
		return
	}
	
	instrumentor := NewDatabaseInstrumentor(client, config)
	instrumentedDriver := instrumentor.WrapDriver(driverName, driver)
	sql.Register(driverName, instrumentedDriver)
}

// Common database driver name constants for convenience
const (
	DriverMySQL      = "mysql"
	DriverPostgreSQL = "postgres"
	DriverSQLite     = "sqlite3"
	DriverSQLServer  = "sqlserver"
	DriverOracle     = "oracle"
)