# DungeonGate Scripts Directory

This directory contains build, test, and maintenance scripts for the DungeonGate project.

## 🚀 Quick Start

For most development tasks, use the unified script:

```bash
# Setup development environment
./scripts/build-and-test.sh setup

# Build and test
./scripts/build-and-test.sh verify

# Start test server
./scripts/build-and-test.sh start
```

## 📁 Script Overview

### Primary Scripts

#### `build-and-test.sh` ⭐
**The main unified script for all build and test operations.**

```bash
# Environment setup
./scripts/build-and-test.sh setup          # Setup dev environment
./scripts/build-and-test.sh clean          # Clean artifacts
./scripts/build-and-test.sh deps           # Install dependencies

# Building
./scripts/build-and-test.sh build          # Build release binary
./scripts/build-and-test.sh build debug    # Build with debug symbols
./scripts/build-and-test.sh build-all      # Build all variants

# Testing
./scripts/build-and-test.sh test           # Run all tests
./scripts/build-and-test.sh test ssh       # Run SSH tests only
./scripts/build-and-test.sh test spectating # Run spectating tests
./scripts/build-and-test.sh benchmark      # Run benchmarks

# Quality checks
./scripts/build-and-test.sh verify         # Run all verification checks
./scripts/build-and-test.sh lint           # Run linter
./scripts/build-and-test.sh security       # Security checks

# Server operations
./scripts/build-and-test.sh start          # Start test server
./scripts/build-and-test.sh status         # Check server status

# Workflows
./scripts/build-and-test.sh ci             # CI pipeline
./scripts/build-and-test.sh release-check  # Release readiness
```

#### `test-integration.sh`
**Comprehensive integration testing script.**

```bash
# Run all integration tests
./scripts/test-integration.sh

# Tests include:
# - SSH connectivity
# - HTTP API endpoints  
# - Configuration validation
# - Database connectivity
# - Spectating system
# - Basic stress testing
```

#### `migrate.sh`
**Database migration management.**

```bash
# Migration commands
./scripts/migrate.sh up                    # Apply migrations
./scripts/migrate.sh down [count]          # Rollback migrations
./scripts/migrate.sh reset                 # Reset database (DESTRUCTIVE)
./scripts/migrate.sh status                # Show migration status
./scripts/migrate.sh create <name>         # Create new migration
./scripts/migrate.sh validate              # Validate migrations
```

### Legacy Scripts (Maintained for Compatibility)

#### `build-ssh.sh` 
**Deprecated - redirects to `build-and-test.sh`**

```bash
# These commands now redirect to the unified script:
./scripts/build-ssh.sh build              # → build-and-test.sh build
./scripts/build-ssh.sh test               # → build-and-test.sh test ssh
./scripts/build-ssh.sh benchmark          # → build-and-test.sh benchmark ssh
```

#### `dev-setup.sh`
**Development environment setup - still functional**

```bash
# Setup development environment (alternative to build-and-test.sh setup)
./scripts/dev-setup.sh
```

#### `test-config.sh`
**Configuration testing - enhanced to use new system**

```bash
# Test configuration files
./scripts/test-config.sh [config-file] [options]
```

### Utility Scripts

#### `clean-test-data.sh`
**Clean test data and temporary files**

#### `make-executable.sh`
**Make scripts executable**

#### `setup-databases.py`
**Python script for database setup**

#### `test-database-configs.sh`
**Test various database configurations**

## 🔧 Makefile Integration

The scripts are integrated with the Makefile for consistent usage:

```bash
# Environment
make setup                    # → scripts/build-and-test.sh setup
make deps                     # Install dependencies
make clean                    # Clean artifacts

# Building  
make build                    # Build release binary
make build-debug              # Build with debug symbols
make build-race               # Build with race detection

# Testing
make test                     # Run all tests
make test-ssh                 # Run SSH tests
make test-spectating          # Run spectating tests
make test-coverage            # Run tests with coverage
make benchmark                # Run benchmarks

# Quality
make verify                   # All verification checks
make lint                     # Run linter
make fmt                      # Format code
make vuln                     # Security checks

# Server
make test-run                 # Start test server
make ssh-check-server         # Check server status
make ssh-test-connection      # Test SSH connection

# Integration
make test-integration         # → scripts/test-integration.sh

# Database
make db-migrate               # → scripts/migrate.sh up
make db-reset                 # → scripts/migrate.sh reset
```

## 🎯 Common Workflows

### Development Setup
```bash
# First time setup
make setup                    # or ./scripts/build-and-test.sh setup
make build
make test-run
```

### Daily Development
```bash
# Quick verification
make verify                   # Format, lint, test

# Test specific functionality
make test-ssh                 # Test SSH functionality
make test-spectating          # Test spectating system

# Run with live reload
make dev                      # Start development server with auto-restart
```

### Testing the Server
```bash
# Terminal 1: Start server
make test-run

# Terminal 2: Test connection
make ssh-test-connection      # or ssh -p 2222 localhost

# Terminal 3: Run integration tests
make test-integration
```

### Pre-commit Workflow
```bash
# Run all checks before committing
make verify                   # Format, lint, security, test
make test-coverage            # Ensure good test coverage
make test-integration         # Integration tests
```

### Release Preparation
```bash
# Complete release checks
make release-check            # All quality checks + benchmarks
make release-build            # Build release binaries
```

## 🔍 Troubleshooting

### Common Issues

#### Port Conflicts
```bash
# Check what's using port 2222
make ssh-check-server

# Kill existing processes
lsof -ti:2222 | xargs kill
```

#### Build Issues
```bash
# Clean everything and rebuild
make clean-all
make setup
make build
```

#### Test Failures
```bash
# Run tests with verbose output
./scripts/build-and-test.sh test all true

# Run specific test categories
make test-ssh                 # SSH functionality
make test-spectating          # Spectating system
```

#### Permission Issues
```bash
# Fix script permissions
./scripts/make-executable.sh

# Fix SSH key permissions
chmod 600 test-data/ssh_keys/test_host_key
```

### Debug Mode

```bash
# Build and run with debug symbols
make build-debug
make run-debug

# Run tests with race detection
make test-race
```

### Performance Issues

```bash
# Run benchmarks to identify bottlenecks
make benchmark
make benchmark-ssh
make benchmark-spectating

# Profile with race detection
make build-race
# Run race build for testing
```

## 📊 Script Dependencies

### Required Tools
- **Go** (1.21+) - Core language
- **Git** - Version control
- **SSH** - For SSH key generation and testing
- **Make** - Build automation

### Optional Tools (Enhanced Development)
- **golangci-lint** - Code linting
- **govulncheck** - Security scanning  
- **air** - Live reload development server
- **netcat (nc)** - Network connectivity testing
- **curl** - HTTP API testing
- **python3** - YAML validation

Install optional tools:
```bash
make deps-tools
```

## 🚀 Performance Characteristics

### Build Performance
- **Release build**: ~5-10 seconds
- **Debug build**: ~8-15 seconds  
- **Race build**: ~10-20 seconds

### Test Performance
- **Unit tests**: ~10-30 seconds
- **Integration tests**: ~30-60 seconds
- **Full test suite**: ~1-2 minutes

### Benchmark Results
- **SSH connections**: 1000+ concurrent
- **Spectating performance**: 100K+ frames/second
- **Memory usage**: ~2MB per session

## 📝 Contributing

### Adding New Scripts

1. Place scripts in the `/scripts` directory
2. Make them executable: `chmod +x script-name.sh`
3. Follow the existing naming convention
4. Add proper usage documentation
5. Integrate with Makefile if appropriate
6. Update this README

### Script Standards

- Use `#!/bin/bash` shebang
- Include error handling with `set -e`
- Provide colored output for better UX
- Include help/usage functions
- Add proper error messages
- Use consistent formatting

### Testing Scripts

```bash
# Test script functionality
./scripts/build-and-test.sh verify

# Test integration
make test-integration

# Test with different configurations
./scripts/test-config.sh configs/testing/sqlite-embedded.yaml
```

---

**For questions or issues with build scripts, check the troubleshooting section above or refer to the main project documentation.**