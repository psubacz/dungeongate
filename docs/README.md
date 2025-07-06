# DungeonGate Documentation

Welcome to the DungeonGate documentation. This directory contains comprehensive documentation for the DungeonGate terminal gaming platform.

## 📖 Documentation Index

### Core Documentation

- **[SSH Service](./ssh-service.md)** - Complete SSH server implementation guide
- **[Spectating System](./SPECTATING.md)** - Real-time spectating with immutable data patterns
- **[Architecture Overview](./ARCHITECTURE.md)** - System architecture and design patterns
- **[Configuration Guide](./CONFIG.md)** - Configuration management and settings
- **[Testing Guide](./TESTING.md)** - Testing strategies and procedures

### Quick Start Guides

- **[Development Setup](../README.md#-getting-started)** - Setting up the development environment
- **[SSH Connection Guide](./ssh-service.md#-usage-examples)** - Connecting and using the SSH interface
- **[Game Configuration](../configs/README.md)** - Setting up games and environments

## 🎯 Key Features Documented

### Session Management
- SSH server implementation with full protocol support
- PTY management and terminal handling
- Session lifecycle and cleanup

### Real-time Spectating
- Immutable data streaming architecture
- Atomic spectator registry management
- Multi-connection support (SSH, WebSocket)
- Performance optimization and scalability

### Configuration System
- Environment-specific configurations
- Database abstraction (SQLite/PostgreSQL)
- Security and authentication settings
- Banner and menu customization

### Development Tools
- **Build System**: Comprehensive Makefile with 40+ targets
- **Testing Framework**: Specialized test suites for each component
- **Quality Assurance**: Automated formatting, linting, and vulnerability scanning
- **Docker Integration**: Development and production containerization
- **Release Management**: Multi-platform builds and automated checks
- **Performance Monitoring**: Benchmarking and profiling tools

## 🏗️ Architecture Overview

DungeonGate follows a microservices architecture pattern:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Session Service │    │ User Service    │    │ Auth Service    │
│                 │    │                 │    │                 │
│ • SSH Server    │    │ • Registration  │    │ • JWT Tokens    │
│ • PTY Manager   │    │ • Profiles      │    │ • Validation    │
│ • Spectating    │    │ • Authentication│    │ • Authorization │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
         ┌─────────────────────────────────────────────┐
         │              Database Layer                 │
         │                                             │
         │ • SQLite (Development)                      │
         │ • PostgreSQL (Production)                   │
         │ • Connection Pooling                        │
         │ • Health Monitoring                         │
         └─────────────────────────────────────────────┘
```

## 🚀 Implementation Status

### ✅ Completed Features

- **SSH Server**: Full SSH-2.0 protocol implementation
- **Real-time Spectating**: Immutable data streaming with atomic operations
- **PTY Management**: Terminal allocation and I/O handling
- **User Registration**: SSH-based user onboarding flow
- **Configuration System**: Environment-specific YAML configurations
- **Database Abstraction**: Dual-mode SQLite/PostgreSQL support

### 🔄 In Progress

- **Auth Service**: Centralized authentication and authorization
- **TTY Recording**: Session recording and playback functionality
- **Game Service**: Game configuration and management

### 📋 Planned Features

- **WebSocket Spectating**: Browser-based real-time spectating
- **Advanced Spectating**: Multi-view and chat functionality  
- **Game Statistics**: Performance tracking and analytics
- **Load Balancing**: Distributed session management

## 🛠️ Development Guides

### Setting Up Development Environment

1. **Prerequisites**: Go 1.21+, Git, SSH client
2. **Clone Repository**: `git clone <repository-url>`
3. **Install Dependencies**: `make deps`
4. **Install Development Tools**: `make deps-tools`
5. **Build Services**: `make build-all`
6. **Start Development Server**: `make dev` (with auto-reload)
7. **Test Connection**: `ssh -p 2222 localhost`

### Alternative Development Workflows

**Quick Testing (Session Service Only)**:
```bash
make test-run          # Start SSH server on port 2222
ssh -p 2222 localhost  # Connect to test
```

**Full System Testing (Auth + Session)**:
```bash
make test-run-all      # Start both services
ssh -p 2222 localhost  # Connect with full auth support
```

**Docker Development**:
```bash
make docker-compose-dev  # Start development environment
make docker-compose-logs # View logs
make docker-compose-down # Stop services
```

### Testing the Platform

**Basic Testing**:
```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run comprehensive test suite
make test-comprehensive
```

**Component-Specific Testing**:
```bash
# Test SSH functionality
make test-ssh

# Test authentication system
make test-auth

# Test spectating system
make test-spectating
```

**Performance Testing**:
```bash
# Run benchmarks
make benchmark

# SSH-specific benchmarks
make benchmark-ssh

# Spectating benchmarks
make benchmark-spectating
```

**Integration Testing**:
```bash
# Start test server
make test-run

# Test SSH connection
make ssh-test-connection

# Check server status
make ssh-check-server
```

## 🔧 Development Commands Reference

### Essential Commands
```bash
# Dependencies and setup
make deps                 # Install Go dependencies
make deps-tools          # Install development tools
make deps-check          # Check dependency status

# Building
make build               # Build session service
make build-auth          # Build auth service
make build-all           # Build all services
make build-debug         # Build with debug symbols
make build-race          # Build with race detection

# Development
make dev                 # Run with auto-reload
make test-run            # Run session service only
make test-run-all        # Run both services
```

### Quality Assurance
```bash
# Code quality
make fmt                 # Format code
make lint                # Run linter
make vet                 # Run go vet
make vuln                # Check vulnerabilities
make verify              # Run all checks

# Testing
make test                # Run all tests
make test-coverage       # Generate coverage report
make test-comprehensive  # Run all test suites
```

### Docker and Deployment
```bash
# Docker
make docker-build-all    # Build all Docker images
make docker-compose-up   # Start services
make docker-compose-dev  # Start development environment

# Database
make db-migrate          # Run migrations
make db-reset            # Reset database

# Release
make release-build       # Build release binaries
make release-check       # Run release checks
```

### Contributing Guidelines

1. **Code Style**: Use `make fmt` and `make lint` before committing
2. **Testing**: Run `make test-comprehensive` to ensure all tests pass
3. **Documentation**: Update relevant documentation for new features
4. **Performance**: Use `make benchmark` to test performance implications
5. **Security**: Run `make vuln` to check for security vulnerabilities
6. **Quality**: Use `make verify` to run all quality checks

## 📊 Performance Characteristics

### Spectating System
- **Frame Processing**: ~100,000 frames/second
- **Spectator Addition**: Sub-microsecond atomic operations
- **Memory Overhead**: ~1KB per active spectator
- **Concurrent Spectators**: Scales linearly with spectator count

### SSH Server
- **Concurrent Connections**: 1000+ simultaneous SSH sessions
- **Session Throughput**: 10,000+ operations per second
- **Memory Usage**: ~2MB per active session
- **Response Time**: Sub-millisecond for menu operations

## 🔍 Troubleshooting

### Common Issues

- **Port Conflicts**: Check if port 2222 is available with `lsof -i :2222`
- **Permission Issues**: Ensure SSH host key has proper permissions (600)
- **Game Launch Failures**: Verify game binaries are installed and accessible
- **Spectating Issues**: Check session service logs for registry errors

### Debug Mode

Enable detailed logging by setting:
```yaml
logging:
  level: "debug"
  format: "text"
  output: "stdout"
```

### Performance Monitoring

- **Service Metrics**: Available at `/metrics` endpoint
- **Health Checks**: Available at `/health` endpoint
- **Session Statistics**: Available through SSH menu system

## 📞 Support and Contributing

- **Issues**: Report bugs and feature requests via GitHub issues
- **Discussions**: Join community discussions for questions and ideas
- **Documentation**: Help improve documentation for better developer experience
- **Code**: Contribute code following the established patterns and guidelines

---

**Note**: This documentation is actively maintained and updated as the platform evolves. For the latest information, always refer to the most recent version in the repository.