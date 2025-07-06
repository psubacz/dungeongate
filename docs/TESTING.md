# DungeonGate Testing Guide

This guide covers the comprehensive testing framework for DungeonGate, including unit tests, integration tests, and performance benchmarks.

## Quick Start

The fastest way to test the platform:

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run comprehensive test suite
make test-comprehensive

# Start test server and connect
make test-run
ssh -p 2222 localhost
```

## Testing Framework Overview

DungeonGate uses a comprehensive Make-based testing system with specialized test suites:

### Test Categories

**Basic Testing:**
- `make test` - Run all unit tests
- `make test-short` - Run quick tests only
- `make test-race` - Run tests with race detection
- `make test-coverage` - Generate coverage reports

**Component-Specific Testing:**
- `make test-ssh` - SSH server functionality
- `make test-auth` - Authentication system
- `make test-auth-simple` - Core authentication logic
- `make test-auth-functional` - Authentication flows
- `make test-spectating` - Spectating system
- `make test-spectating-full` - Comprehensive spectating tests

**Performance Testing:**
- `make benchmark` - General performance benchmarks
- `make benchmark-ssh` - SSH-specific benchmarks
- `make benchmark-spectating` - Spectating system benchmarks

### What Gets Tested

1. **SSH Server Implementation**
   - SSH-2.0 protocol compliance
   - PTY allocation and management
   - Session handling and cleanup
   - Connection multiplexing

2. **Authentication System**
   - User registration and login flows
   - JWT token generation and validation
   - gRPC service communication
   - Fallback and error handling

3. **Spectating System**
   - Real-time data streaming
   - Atomic spectator registry operations
   - Immutable data structures
   - Concurrent spectator management

4. **Database Operations**
   - SQLite and PostgreSQL compatibility
   - Connection pooling and health checks
   - Query performance and optimization
   - Transaction management

## Testing Environments

### Development Testing

**Session Service Only (Limited Auth):**
```bash
make test-run          # Start SSH server on port 2222
ssh -p 2222 localhost  # Connect to test
```

**Full System with Authentication:**
```bash
make test-run-all      # Start auth + session services
ssh -p 2222 localhost  # Connect with full auth support
```

### Database Testing

The testing framework supports multiple database configurations:

**SQLite (Default for Testing):**
```yaml
database:
  mode: "embedded"
  embedded:
    type: "sqlite"
    path: "./data/sqlite/dungeongate-dev.db"
    migration_path: "./migrations"
    wal_mode: true
```

**PostgreSQL (Production-like Testing):**
```yaml
database:
  mode: "external"
  external:
    type: "postgresql"
    writer_endpoint: "localhost:5432"
    database: "dungeongate_test"
    username: "test_user"
    password: "${TEST_DB_PASSWORD}"
```

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

### Debug and Troubleshooting

**Enable Debug Mode:**
```yaml
session_service:
  logging:
    level: "debug"
    format: "text"
    output: "stdout"
```

**Common Debug Commands:**
```bash
# Check server status
make ssh-check-server

# Test connection
make ssh-test-connection

# Run with debug build
make build-debug
make run-debug

# Check dependencies
make deps-check

# Show project info
make info
```

## Test Environment Management

### Environment Setup

```bash
# Setup test environment
make setup-test-env

# Clean test environment
make clean-test-env

# Full cleanup (build artifacts + test data)
make clean-all
```

### Test Data Management

Test data is managed automatically:

```bash
# Database migrations
make db-migrate          # Run migrations
make db-migrate-down     # Rollback migrations
make db-reset            # Reset database (destructive)

# SSH keys and test data
make setup-test-env      # Creates test SSH keys and directories
make clean-test-env      # Removes all test data
```

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: Test Suite
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Install Dependencies
        run: make deps
        
      - name: Run Quality Checks
        run: make verify
        
      - name: Run Test Suite
        run: make test-comprehensive
        
      - name: Generate Coverage Report
        run: make test-coverage
        
      - name: Run Benchmarks
        run: make benchmark
```

### Docker Testing

```dockerfile
FROM golang:1.21-alpine AS test
WORKDIR /app
COPY . .
RUN make deps
RUN make test-comprehensive
RUN make benchmark
```

```bash
# Docker-based testing
make docker-build-all
make docker-test
```

## Performance Expectations

### Test Performance Targets

**SSH Server:**
- **Concurrent Connections:** 1000+ simultaneous sessions
- **Session Throughput:** 10,000+ operations/second
- **Connection Setup:** < 100ms
- **Memory per Session:** ~2MB

**Spectating System:**
- **Frame Processing:** ~100,000 frames/second
- **Spectator Addition:** Sub-microsecond atomic operations
- **Memory per Spectator:** ~1KB
- **Concurrent Spectators:** Linear scaling

**Database Operations:**
- **SQLite Read Operations:** 1000+ ops/sec
- **PostgreSQL Read Operations:** 500+ ops/sec (network dependent)
- **Connection Setup:** < 200ms
- **Failover Time:** < 5s

### Benchmark Commands

```bash
# General benchmarks
make benchmark

# Component-specific benchmarks
make benchmark-ssh
make benchmark-spectating

# Performance monitoring
make info                    # Show system information
make version                 # Show version and build info
```

## SSH Connection Testing

### Manual Testing

```bash
# Check if SSH server is running
make ssh-check-server

# Test SSH connection
make ssh-test-connection

# Start server and test manually
make test-run
# In another terminal:
ssh -p 2222 localhost
```

### Automated Testing

```bash
# Run all SSH tests
make test-ssh

# Run comprehensive SSH testing
make test-comprehensive
```

## Next Steps

1. **Run the basic test suite** with `make test`
2. **Test specific components** with component-specific make targets
3. **Integrate with your CI/CD pipeline** using the provided examples
4. **Set up Docker testing** for containerized environments
5. **Configure external database testing** for production-like scenarios

For configuration options, see `CONFIG.md`. For architecture details, see `ARCHITECTURE.md`.
