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
- Build scripts and automation
- Testing frameworks and procedures
- Debugging and monitoring tools

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
4. **Build Service**: `make build`
5. **Start Development Server**: `make test-run`
6. **Test Connection**: `ssh -p 2222 localhost`

### Testing the Platform

```bash
# Run unit tests
make test

# Start test server
make test-run

# Test SSH functionality
ssh -p 2222 localhost

# Test watch functionality
# 1. Connect via SSH
# 2. Select 'w' for watch
# 3. Choose a test session
# 4. Verify real-time streaming
```

### Contributing Guidelines

1. **Code Style**: Follow Go conventions and run `gofmt`
2. **Testing**: Write comprehensive unit and integration tests
3. **Documentation**: Update relevant documentation for new features
4. **Performance**: Consider performance implications, especially for spectating features
5. **Security**: Follow security best practices for SSH and authentication

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