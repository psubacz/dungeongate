package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dungeongate/pkg/config"
	_ "github.com/go-sql-driver/mysql" // MySQL driver
	_ "github.com/lib/pq"              // PostgreSQL driver
	_ "github.com/mattn/go-sqlite3"    // SQLite driver
)

// Connection represents a database connection with read/write separation
type Connection struct {
	writer        *sql.DB
	reader        *sql.DB
	config        *config.DatabaseConfig
	metrics       *ConnectionMetrics
	healthMux     sync.RWMutex
	writerHealthy bool
	readerHealthy bool
}

// ConnectionMetrics tracks database connection metrics
type ConnectionMetrics struct {
	mutex             sync.RWMutex
	WriterConnections int64
	ReaderConnections int64
	WriterQueries     int64
	ReaderQueries     int64
	WriterErrors      int64
	ReaderErrors      int64
	FailoverCount     int64
	LastFailover      time.Time
}

// QueryType represents the type of database query
type QueryType int

const (
	QueryTypeRead QueryType = iota
	QueryTypeWrite
	QueryTypeReadWrite // Transactions that need consistent reads
)

// NewConnection creates a new database connection with read/write separation
func NewConnection(cfg *config.DatabaseConfig) (*Connection, error) {
	if cfg == nil {
		return nil, fmt.Errorf("database configuration is nil")
	}

	conn := &Connection{
		config:        cfg,
		metrics:       &ConnectionMetrics{},
		writerHealthy: true,
		readerHealthy: true,
	}

	switch cfg.Mode {
	case config.DatabaseModeEmbedded:
		return conn.initEmbeddedConnection()
	case config.DatabaseModeExternal:
		return conn.initExternalConnection()
	default:
		return nil, fmt.Errorf("unsupported database mode: %s", cfg.Mode)
	}
}

// initEmbeddedConnection initializes an embedded database connection
func (c *Connection) initEmbeddedConnection() (*Connection, error) {
	connStr, err := c.config.GetConnectionString()
	if err != nil {
		return nil, fmt.Errorf("failed to get connection string: %w", err)
	}

	db, err := sql.Open(GetDriverName(c.config.GetDatabaseType()), connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// For embedded databases, both reader and writer use the same connection
	c.writer = db
	c.reader = db

	// Configure connection pool
	c.configureConnectionPool(db, c.config.Embedded)

	return c, nil
}

// NewConnectionFromLegacy creates a new database connection from legacy LegacyDatabaseConfig
// This is for backward compatibility with the legacy LegacyDatabaseConfig type
func NewConnectionFromLegacy(cfg *config.LegacyDatabaseConfig) (*Connection, error) {
	if cfg == nil {
		return nil, fmt.Errorf("database configuration is nil")
	}

	// Convert legacy config to new config for embedded SQLite (most common case)
	newCfg := &config.DatabaseConfig{
		Mode:       config.DatabaseModeEmbedded,
		Type:       cfg.Type,
		Connection: cfg.Connection,
		Pool:       cfg.Pool,
		Embedded: &config.EmbeddedDBConfig{
			Type:          cfg.Type,
			Path:          "./data/default.db", // Default path
			MigrationPath: "./migrations",
			BackupEnabled: false,
			WALMode:       true,
			Cache: &config.CacheConfig{
				Enabled: true,
				Size:    64,
				TTL:     "1h",
				Type:    "memory",
			},
		},
		Settings: &config.DatabaseSettings{
			LogQueries:     false,
			Timeout:        "30s",
			RetryAttempts:  3,
			RetryDelay:     "1s",
			HealthCheck:    true,
			HealthInterval: "30s",
			MetricsEnabled: true,
		},
	}

	// Extract connection string from legacy config if available
	if connStr, ok := cfg.Connection["dsn"]; ok {
		if dsn, ok := connStr.(string); ok {
			newCfg.Embedded.Path = dsn
		}
	}

	return NewConnection(newCfg)
}

// initExternalConnection initializes an external database connection with read/write separation
func (c *Connection) initExternalConnection() (*Connection, error) {
	// Initialize writer connection
	writerConnStr, err := c.config.GetWriterConnectionString()
	if err != nil {
		return nil, fmt.Errorf("failed to get writer connection string: %w", err)
	}

	c.writer, err = sql.Open(GetDriverName(c.config.GetDatabaseType()), writerConnStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open writer database: %w", err)
	}

	if err := c.writer.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping writer database: %w", err)
	}

	// Configure writer connection pool
	c.configureWriterConnectionPool()

	// Initialize reader connection
	if c.config.External.ReaderUseWriter {
		// Use writer connection for reads
		c.reader = c.writer
	} else {
		// Initialize separate reader connection
		readerConnStr, err := c.config.GetReaderConnectionString()
		if err != nil {
			return nil, fmt.Errorf("failed to get reader connection string: %w", err)
		}

		c.reader, err = sql.Open(GetDriverName(c.config.GetDatabaseType()), readerConnStr)
		if err != nil {
			return nil, fmt.Errorf("failed to open reader database: %w", err)
		}

		if err := c.reader.Ping(); err != nil {
			return nil, fmt.Errorf("failed to ping reader database: %w", err)
		}

		// Configure reader connection pool
		c.configureReaderConnectionPool()
	}

	// Start health monitoring if failover is enabled
	if c.config.External.Failover != nil && c.config.External.Failover.Enabled {
		go c.startHealthMonitoring()
	}

	return c, nil
}

// configureConnectionPool configures connection pool for embedded databases
func (c *Connection) configureConnectionPool(db *sql.DB, embeddedConfig *config.EmbeddedDBConfig) {
	if embeddedConfig == nil {
		return
	}

	// Set reasonable defaults for SQLite
	db.SetMaxOpenConns(10) // SQLite doesn't benefit from many connections
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(1 * time.Hour)
}

// configureWriterConnectionPool configures the writer connection pool
func (c *Connection) configureWriterConnectionPool() {
	if c.config.External.MaxConnections > 0 {
		c.writer.SetMaxOpenConns(c.config.External.MaxConnections)
	}
	if c.config.External.MaxIdleConns > 0 {
		c.writer.SetMaxIdleConns(c.config.External.MaxIdleConns)
	}
	if c.config.External.ConnMaxLifetime != "" {
		if lifetime, err := time.ParseDuration(c.config.External.ConnMaxLifetime); err == nil {
			c.writer.SetConnMaxLifetime(lifetime)
		}
	}
}

// configureReaderConnectionPool configures the reader connection pool
func (c *Connection) configureReaderConnectionPool() {
	maxConns := c.config.External.ReaderMaxConnections
	if maxConns == 0 {
		maxConns = c.config.External.MaxConnections / 2 // Default to half of writer connections
	}
	if maxConns > 0 {
		c.reader.SetMaxOpenConns(maxConns)
	}

	maxIdle := c.config.External.ReaderMaxIdleConns
	if maxIdle == 0 {
		maxIdle = c.config.External.MaxIdleConns / 2
	}
	if maxIdle > 0 {
		c.reader.SetMaxIdleConns(maxIdle)
	}

	if c.config.External.ConnMaxLifetime != "" {
		if lifetime, err := time.ParseDuration(c.config.External.ConnMaxLifetime); err == nil {
			c.reader.SetConnMaxLifetime(lifetime)
		}
	}
}

// startHealthMonitoring starts health monitoring for failover support
func (c *Connection) startHealthMonitoring() {
	failover := c.config.External.Failover
	interval := 30 * time.Second

	if failover.HealthCheckInterval != "" {
		if parsed, err := time.ParseDuration(failover.HealthCheckInterval); err == nil {
			interval = parsed
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		c.checkHealth()
	}
}

// checkHealth performs health checks on database connections
func (c *Connection) checkHealth() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check writer health
	writerHealthy := true
	if err := c.writer.PingContext(ctx); err != nil {
		writerHealthy = false
		c.metrics.mutex.Lock()
		c.metrics.WriterErrors++
		c.metrics.mutex.Unlock()
	}

	// Check reader health (if separate)
	readerHealthy := true
	if c.reader != c.writer {
		if err := c.reader.PingContext(ctx); err != nil {
			readerHealthy = false
			c.metrics.mutex.Lock()
			c.metrics.ReaderErrors++
			c.metrics.mutex.Unlock()
		}
	} else {
		readerHealthy = writerHealthy
	}

	// Update health status
	c.healthMux.Lock()
	c.writerHealthy = writerHealthy
	c.readerHealthy = readerHealthy
	c.healthMux.Unlock()
}

// DB returns the appropriate database connection based on query type
func (c *Connection) DB(queryType QueryType) *sql.DB {
	c.healthMux.RLock()
	defer c.healthMux.RUnlock()

	switch queryType {
	case QueryTypeWrite, QueryTypeReadWrite:
		c.metrics.mutex.Lock()
		c.metrics.WriterQueries++
		c.metrics.mutex.Unlock()
		return c.writer

	case QueryTypeRead:
		// Use reader if healthy, otherwise fallback to writer if configured
		if c.readerHealthy {
			c.metrics.mutex.Lock()
			c.metrics.ReaderQueries++
			c.metrics.mutex.Unlock()
			return c.reader
		}

		// Fallback to writer if reader is unhealthy and fallback is enabled
		if c.config.External.Failover != nil && c.config.External.Failover.ReaderToWriterFallback && c.writerHealthy {
			c.metrics.mutex.Lock()
			c.metrics.WriterQueries++
			c.metrics.FailoverCount++
			c.metrics.LastFailover = time.Now()
			c.metrics.mutex.Unlock()
			return c.writer
		}

		// Return reader even if unhealthy (will likely fail, but let caller handle it)
		c.metrics.mutex.Lock()
		c.metrics.ReaderQueries++
		c.metrics.mutex.Unlock()
		return c.reader

	default:
		// Default to writer for unknown query types
		return c.writer
	}
}

// Writer returns the writer database connection
func (c *Connection) Writer() *sql.DB {
	return c.DB(QueryTypeWrite)
}

// Reader returns the reader database connection
func (c *Connection) Reader() *sql.DB {
	return c.DB(QueryTypeRead)
}

// Close closes both database connections
func (c *Connection) Close() error {
	var err error

	if c.writer != nil {
		if writerErr := c.writer.Close(); writerErr != nil {
			err = writerErr
		}
	}

	// Only close reader if it's different from writer
	if c.reader != nil && c.reader != c.writer {
		if readerErr := c.reader.Close(); readerErr != nil {
			if err != nil {
				err = fmt.Errorf("writer close error: %v, reader close error: %v", err, readerErr)
			} else {
				err = readerErr
			}
		}
	}

	return err
}

// Transaction starts a new transaction on the writer database
func (c *Connection) Transaction(ctx context.Context) (*sql.Tx, error) {
	return c.writer.BeginTx(ctx, nil)
}

// Query executes a query and returns rows (uses reader for read queries)
func (c *Connection) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return c.QueryContext(context.Background(), query, args...)
}

// QueryContext executes a query with context and returns rows
func (c *Connection) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	db := c.DB(c.detectQueryType(query))
	return db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query and returns a single row (uses reader for read queries)
func (c *Connection) QueryRow(query string, args ...interface{}) *sql.Row {
	return c.QueryRowContext(context.Background(), query, args...)
}

// QueryRowContext executes a query with context and returns a single row
func (c *Connection) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	db := c.DB(c.detectQueryType(query))
	return db.QueryRowContext(ctx, query, args...)
}

// Exec executes a query without returning rows (uses writer)
func (c *Connection) Exec(query string, args ...interface{}) (sql.Result, error) {
	return c.ExecContext(context.Background(), query, args...)
}

// ExecContext executes a query with context without returning rows
func (c *Connection) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	db := c.DB(QueryTypeWrite) // All exec operations go to writer
	return db.ExecContext(ctx, query, args...)
}

// Prepare prepares a statement (uses writer by default)
func (c *Connection) Prepare(query string) (*sql.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

// PrepareContext prepares a statement with context
func (c *Connection) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	queryType := c.detectQueryType(query)
	db := c.DB(queryType)
	return db.PrepareContext(ctx, query)
}

// detectQueryType attempts to detect the query type based on the SQL statement
func (c *Connection) detectQueryType(query string) QueryType {
	// Simple query type detection based on the first word
	query = strings.TrimSpace(strings.ToUpper(query))

	if strings.HasPrefix(query, "SELECT") ||
		strings.HasPrefix(query, "SHOW") ||
		strings.HasPrefix(query, "DESCRIBE") ||
		strings.HasPrefix(query, "EXPLAIN") {
		return QueryTypeRead
	}

	if strings.HasPrefix(query, "INSERT") ||
		strings.HasPrefix(query, "UPDATE") ||
		strings.HasPrefix(query, "DELETE") ||
		strings.HasPrefix(query, "CREATE") ||
		strings.HasPrefix(query, "DROP") ||
		strings.HasPrefix(query, "ALTER") {
		return QueryTypeWrite
	}

	if strings.HasPrefix(query, "BEGIN") ||
		strings.HasPrefix(query, "START TRANSACTION") {
		return QueryTypeReadWrite
	}

	// Default to write for unknown queries to be safe
	return QueryTypeWrite
}

// GetMetrics returns current connection metrics
func (c *Connection) GetMetrics() *ConnectionMetrics {
	c.metrics.mutex.RLock()
	defer c.metrics.mutex.RUnlock()

	return &ConnectionMetrics{
		WriterConnections: c.metrics.WriterConnections,
		ReaderConnections: c.metrics.ReaderConnections,
		WriterQueries:     c.metrics.WriterQueries,
		ReaderQueries:     c.metrics.ReaderQueries,
		WriterErrors:      c.metrics.WriterErrors,
		ReaderErrors:      c.metrics.ReaderErrors,
		FailoverCount:     c.metrics.FailoverCount,
		LastFailover:      c.metrics.LastFailover,
	}
}

// IsHealthy returns the health status of both connections
func (c *Connection) IsHealthy() (writerHealthy, readerHealthy bool) {
	c.healthMux.RLock()
	defer c.healthMux.RUnlock()
	return c.writerHealthy, c.readerHealthy
}

// Ping pings both database connections
func (c *Connection) Ping() error {
	if err := c.writer.Ping(); err != nil {
		return fmt.Errorf("writer ping failed: %w", err)
	}

	if c.reader != c.writer {
		if err := c.reader.Ping(); err != nil {
			return fmt.Errorf("reader ping failed: %w", err)
		}
	}

	return nil
}

// PingContext pings both database connections with context
func (c *Connection) PingContext(ctx context.Context) error {
	if err := c.writer.PingContext(ctx); err != nil {
		return fmt.Errorf("writer ping failed: %w", err)
	}

	if c.reader != c.writer {
		if err := c.reader.PingContext(ctx); err != nil {
			return fmt.Errorf("reader ping failed: %w", err)
		}
	}

	return nil
}

// Stats returns database statistics for both connections
func (c *Connection) Stats() (writerStats, readerStats sql.DBStats) {
	writerStats = c.writer.Stats()

	if c.reader != c.writer {
		readerStats = c.reader.Stats()
	} else {
		readerStats = writerStats
	}

	return writerStats, readerStats
}

// RunMigrations runs database migrations
func RunMigrations(conn *Connection, cfg *config.DatabaseConfig) error {
	// Use writer connection for migrations
	db := conn.Writer()

	// Create migrations table if it doesn't exist
	createMigrationsTable := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`

	if _, err := db.Exec(createMigrationsTable); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// TODO: Implement actual migration logic
	// This would read migration files from cfg.GetMigrationPath() and apply them

	return nil
}

// CreateTables creates the necessary database tables
func CreateTables(conn *Connection) error {
	// Use writer connection for schema changes
	db := conn.Writer()

	// Determine database type for appropriate SQL syntax
	dbType := conn.config.GetDatabaseType()

	var queries []string

	if dbType == "sqlite" {
		queries = getSQLiteSchema()
	} else {
		queries = getPostgreSQLSchema()
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}

	return nil
}

// getSQLiteSchema returns SQLite-specific schema
func getSQLiteSchema() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username VARCHAR(30) UNIQUE NOT NULL,
			email VARCHAR(80),
			password_hash VARCHAR(255) NOT NULL,
			salt VARCHAR(32) NOT NULL,
			environment TEXT DEFAULT '',
			flags INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_login TIMESTAMP,
			login_count INTEGER DEFAULT 0,
			failed_login_attempts INTEGER DEFAULT 0,
			account_locked BOOLEAN DEFAULT FALSE,
			locked_until TIMESTAMP,
			email_verified BOOLEAN DEFAULT FALSE,
			is_active BOOLEAN DEFAULT TRUE
		)`,
		`CREATE TABLE IF NOT EXISTS user_profiles (
			user_id INTEGER PRIMARY KEY,
			real_name VARCHAR(100),
			location VARCHAR(100),
			website VARCHAR(200),
			bio TEXT,
			avatar_url VARCHAR(500),
			timezone VARCHAR(50) DEFAULT 'UTC',
			language VARCHAR(10) DEFAULT 'en',
			theme VARCHAR(20) DEFAULT 'dark',
			terminal_size VARCHAR(20) DEFAULT '80x24',
			color_mode VARCHAR(20) DEFAULT 'color',
			email_notifications BOOLEAN DEFAULT TRUE,
			public_profile BOOLEAN DEFAULT FALSE,
			allow_spectators BOOLEAN DEFAULT TRUE,
			show_online_status BOOLEAN DEFAULT TRUE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS user_preferences (
			user_id INTEGER,
			key VARCHAR(100),
			value TEXT,
			PRIMARY KEY (user_id, key),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS user_roles (
			user_id INTEGER,
			role VARCHAR(50),
			granted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			granted_by INTEGER,
			PRIMARY KEY (user_id, role),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (granted_by) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS registration_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username VARCHAR(30) NOT NULL,
			email VARCHAR(80),
			ip_address VARCHAR(45),
			user_agent TEXT,
			source VARCHAR(20),
			success BOOLEAN,
			error_message TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_users_active ON users(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_registration_log_created ON registration_log(created_at)`,
	}
}

// getPostgreSQLSchema returns PostgreSQL-specific schema
func getPostgreSQLSchema() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(30) UNIQUE NOT NULL,
			email VARCHAR(80),
			password_hash VARCHAR(255) NOT NULL,
			salt VARCHAR(32) NOT NULL,
			environment TEXT DEFAULT '',
			flags INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_login TIMESTAMP,
			login_count INTEGER DEFAULT 0,
			failed_login_attempts INTEGER DEFAULT 0,
			account_locked BOOLEAN DEFAULT FALSE,
			locked_until TIMESTAMP,
			email_verified BOOLEAN DEFAULT FALSE,
			is_active BOOLEAN DEFAULT TRUE
		)`,
		`CREATE TABLE IF NOT EXISTS user_profiles (
			user_id INTEGER PRIMARY KEY,
			real_name VARCHAR(100),
			location VARCHAR(100),
			website VARCHAR(200),
			bio TEXT,
			avatar_url VARCHAR(500),
			timezone VARCHAR(50) DEFAULT 'UTC',
			language VARCHAR(10) DEFAULT 'en',
			theme VARCHAR(20) DEFAULT 'dark',
			terminal_size VARCHAR(20) DEFAULT '80x24',
			color_mode VARCHAR(20) DEFAULT 'color',
			email_notifications BOOLEAN DEFAULT TRUE,
			public_profile BOOLEAN DEFAULT FALSE,
			allow_spectators BOOLEAN DEFAULT TRUE,
			show_online_status BOOLEAN DEFAULT TRUE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS user_preferences (
			user_id INTEGER,
			key VARCHAR(100),
			value TEXT,
			PRIMARY KEY (user_id, key),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS user_roles (
			user_id INTEGER,
			role VARCHAR(50),
			granted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			granted_by INTEGER,
			PRIMARY KEY (user_id, role),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (granted_by) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS registration_log (
			id SERIAL PRIMARY KEY,
			username VARCHAR(30) NOT NULL,
			email VARCHAR(80),
			ip_address VARCHAR(45),
			user_agent TEXT,
			source VARCHAR(20),
			success BOOLEAN,
			error_message TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_users_active ON users(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_registration_log_created ON registration_log(created_at)`,
	}
}

// Helper function to get database type string for driver registration
func GetDriverName(dbType string) string {
	switch dbType {
	case "postgresql":
		return "postgres"
	case "mysql":
		return "mysql"
	case "sqlite":
		return "sqlite3"
	default:
		return dbType
	}
}
