# DungeonGate Testing Guide

This guide covers how to test the DungeonGate configuration and database setup during development.

## Quick Start

The fastest way to test your configuration:

```bash
# Test database connectivity
go run test-build.go -config configs/testing/sqlite-embedded.yaml -test-db

# Validate configuration only
go run test-build.go -config configs/testing/sqlite-embedded.yaml -validate-only

# Run performance benchmark
go run test-build.go -config configs/testing/sqlite-embedded.yaml -benchmark
```

## Test Build Script

The `test-build.go` script provides several testing modes:

### Command Line Options

- `-config <path>` - Path to configuration file (required)
- `-test-db` - Test database connection and create tables
- `-validate-only` - Only validate configuration, don't test connections
- `-benchmark` - Run basic performance benchmarks

### What Gets Tested

1. **Configuration Loading**
   - YAML parsing and validation
   - Environment variable expansion
   - Configuration structure validation

2. **Database Connectivity**
   - Database connection establishment
   - Connection health checks
   - Table creation and verification
   - Query routing (for external databases)

3. **Performance Benchmarking**
   - Database read performance
   - Connection metrics
   - Failover testing (external databases)

## Configuration Modes

### Embedded Database (SQLite)

Best for development and testing:

```yaml
database:
  mode: "embedded"
  embedded:
    type: "sqlite"
    path: "./test-data/sqlite/users.db"
    migration_path: "./migrations"
    backup_enabled: false
    wal_mode: true
    cache:
      enabled: true
      size: 16
      ttl: "10m"
      type: "memory"
```

**Pros:**
- No external dependencies
- Fast setup and teardown
- Perfect for CI/CD
- Automatic directory creation

**Cons:**
- Single connection limit
- No read/write separation
- Not suitable for production

### External Database (PostgreSQL/MySQL)

For production deployments:

```yaml
database:
  mode: "external"
  external:
    type: "postgresql"
    writer_endpoint: "db-writer:5432"
    reader_use_writer: false
    reader_endpoint: "db-reader:5432"
    database: "dungeongate"
    username: "dungeongate"
    password: "${DB_PASSWORD}"
    ssl_mode: "require"
    max_connections: 100
    max_idle_conns: 10
    failover:
      enabled: true
      health_check_interval: "30s"
      reader_to_writer_fallback: true
```

**Features:**
- Read/write separation
- Connection pooling
- Automatic failover
- Health monitoring
- Production-ready scaling

## Directory Structure

The test system creates and manages these directories:

```
test-data/
├── sqlite/
│   ├── users.db           # SQLite database file
│   ├── ttyrec/           # Session recordings
│   └── tmp/              # Temporary files
└── ssh_keys/
    └── test_host_key     # SSH host key for testing
```

## Environment Variables

Configuration files support environment variable expansion:

```yaml
database:
  external:
    password: "${DB_PASSWORD}"
    host: "${DB_HOST:-localhost}"
```

Common environment variables:
- `DB_PASSWORD` - Database password
- `DB_HOST` - Database host
- `DB_PORT` - Database port
- `SSH_HOST_KEY_PATH` - SSH host key path

## Troubleshooting

### Common Issues

**Database Connection Fails**
```
failed to create database connection: failed to open database: sql: unknown driver
```
**Solution:** Make sure database drivers are imported:
```go
import _ "github.com/mattn/go-sqlite3" // SQLite
import _ "github.com/lib/pq"           // PostgreSQL  
import _ "github.com/go-sql-driver/mysql" // MySQL
```

**Permission Denied on Directories**
```
failed to create database directory: permission denied
```
**Solution:** Ensure the process has write permissions to the target directory, or use a different path.

**Configuration Validation Errors**
```
configuration validation failed: database mode is required
```
**Solution:** Check your YAML syntax and ensure all required fields are present.

### Debug Mode

Enable debug logging for detailed information:

```yaml
session_service:
  logging:
    level: "debug"
    format: "text"
    output: "stdout"
```

## Test Data Management

### Cleanup

Test data is created in `./test-data/`. To clean up:

```bash
# Remove all test data
rm -rf ./test-data/

# Remove only database files
rm -f ./test-data/sqlite/*.db
```

### Backup

For persistent test data:

```bash
# Backup test database
cp ./test-data/sqlite/users.db ./test-data/sqlite/users.db.backup

# Restore from backup
cp ./test-data/sqlite/users.db.backup ./test-data/sqlite/users.db
```

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: Test Configuration
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Test Configuration
        run: |
          go run test-build.go -config configs/testing/sqlite-embedded.yaml -test-db
          
      - name: Benchmark Performance
        run: |
          go run test-build.go -config configs/testing/sqlite-embedded.yaml -benchmark
```

### Docker Testing

```dockerfile
FROM golang:1.21-alpine AS test
WORKDIR /app
COPY . .
RUN go mod download
RUN go run test-build.go -config configs/testing/sqlite-embedded.yaml -test-db
```

## Performance Expectations

### SQLite (Embedded)
- **Read Operations:** 1000+ ops/sec
- **Connection Setup:** < 100ms
- **Table Creation:** < 50ms

### PostgreSQL (External)
- **Read Operations:** 500+ ops/sec (network dependent)
- **Connection Setup:** < 200ms
- **Failover Time:** < 5s

## Next Steps

1. **Run the basic test** to ensure everything works
2. **Create your own test configuration** for your specific setup
3. **Integrate with your CI/CD pipeline**
4. **Set up external database testing** when ready for production

For more advanced configuration options, see `CONFIG.md`.
