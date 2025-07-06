# DungeonGate Architecture

This document outlines the high-level architecture of the DungeonGate platform, focusing on the microservices design and key implementation patterns.

## 🏗️ System Overview

DungeonGate is a microservices-based platform for hosting terminal games, built with modern Go patterns and designed for scalability.

```
┌─────────────────────────────────────────────────────────────────┐
│                       DungeonGate Platform                      │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   Client    │  │   Gateway   │  │      Microservices      │  │
│  │             │  │             │  │                         │  │
│  │ • SSH       │→→│ • Session   │→→│ • Session Service ✅    │  │
│  │ • Terminal  │  │   Service   │  │ • User Service ✅       │  │
│  │             │  │ • Load      │  │ • Auth Service 🔄       │  │
│  │             │  │   Balancer  │  │ • Game Service 📋       │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │  Database   │  │   Storage   │  │      Infrastructure     │  │
│  │             │  │             │  │                         │  │
│  │ • SQLite    │  │ • TTY Rec   │  │ • Kubernetes            │  │
│  │ • PostgreSQL│  │ • Save Data │  │ • Monitoring            │  │
│  │ • Redis     │  │ • Logs      │  │ • Load Balancing        │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## 🎯 Core Services

### Session Service (Primary Implementation)

The session service is the main entry point for users, handling SSH connections and game sessions.

**Key Components:**
- **SSH Server**: Full SSH-2.0 protocol implementation
- **PTY Manager**: Pseudo-terminal allocation and management
- **Session Manager**: Game session lifecycle and state management
- **Spectating System**: Real-time terminal streaming with immutable data patterns

**Architecture Patterns:**
- **Immutable Data Sharing**: Lock-free concurrent programming for spectator management
- **Atomic Operations**: `atomic.Pointer[T]` for thread-safe registry updates
- **Stream Processing**: Buffered channel-based frame distribution
- **Copy-on-Write**: Efficient memory usage for spectator lists

### User Service (Implemented)

Handles user registration, authentication, and profile management.

**Features:**
- User registration flow via SSH terminal
- Database-backed user storage
- Profile management
- Authentication integration

### Auth Service (Planned)

Centralized authentication and authorization service.

**Planned Features:**
- JWT token management
- Multi-factor authentication
- Session validation
- Role-based access control

### Game Service (Planned)

Game configuration, launching, and management service.

**Planned Features:**
- Game configuration management
- Process lifecycle management
- Save file handling
- Game statistics tracking

## 🔄 Data Flow

### SSH Connection Flow

```mermaid
sequenceDiagram
    participant User
    participant SSH Server
    participant Session Service
    participant Game Process
    participant Database

    User->>SSH Server: SSH Connection
    SSH Server->>Session Service: Create Session
    Session Service->>Database: Store Session
    SSH Server->>User: Display Menu
    User->>SSH Server: Select Game
    SSH Server->>Session Service: Launch Game
    Session Service->>Game Process: Start PTY
    Game Process->>SSH Server: Terminal Output
    SSH Server->>User: Stream Output
```

### Spectating Data Flow

```mermaid
sequenceDiagram
    participant Game
    participant StreamManager
    participant Registry
    participant Spectator1
    participant Spectator2

    Game->>StreamManager: Terminal Output
    StreamManager->>StreamManager: Create Immutable Frame
    StreamManager->>Registry: Get Spectator List
    Registry->>StreamManager: Return Spectators
    par
        StreamManager->>Spectator1: Send Frame
    and
        StreamManager->>Spectator2: Send Frame
    end
```

## 🏛️ Design Patterns

### Immutable Data Architecture

The spectating system demonstrates modern Go concurrency patterns:

**Core Principles:**
1. **Immutability**: Data structures are never modified in place
2. **Atomic Updates**: State changes use atomic compare-and-swap operations
3. **Copy-on-Write**: New versions created for each update
4. **Lock-Free Design**: No mutexes in hot paths

**Implementation:**
```go
// Immutable spectator registry
type SpectatorRegistry struct {
    Spectators map[string]*Spectator
    Version    uint64
}

// Atomic registry management
registry := &atomic.Pointer[SpectatorRegistry]{}
registry.Store(NewSpectatorRegistry())

// Lock-free updates
for {
    old := registry.Load()
    new := old.AddSpectator(spectator)
    if registry.CompareAndSwap(old, new) {
        break // Success
    }
    // Retry on conflict
}
```

### Microservices Communication

**gRPC Services:**
- High-performance internal communication
- Type-safe protocol buffers
- Streaming support for real-time features

**HTTP APIs:**
- REST endpoints for web integration
- Health checks and metrics
- Configuration management

### Database Abstraction

**Dual-Mode Support:**
- **Embedded**: SQLite for development and small deployments
- **External**: PostgreSQL for production with read/write separation

**Features:**
- Connection pooling and lifecycle management
- Health monitoring with automatic failover
- Query logging and performance metrics
- Migration support

## 🔧 Configuration Architecture

### Environment-Specific Configs

```yaml
# Development
database:
  mode: "embedded"
  type: "sqlite"
  embedded:
    path: "./data/sqlite/dungeongate-dev.db"

# Production  
database:
  mode: "external"
  type: "postgresql"
  external:
    writer_endpoint: "postgres-writer:5432"
    reader_endpoint: "postgres-reader:5432"
```

### Configuration Validation

- **Schema Validation**: YAML structure validation
- **Environment Variables**: Secure secret injection
- **Default Values**: Comprehensive fallback configuration
- **Hot Reloading**: Runtime configuration updates (planned)

## 🚀 Deployment Architecture

### Development Deployment

```bash
# Single-process deployment
./dungeongate-session-service -config=./configs/development/local.yaml
```

### Production Deployment (Planned)

```yaml
# Kubernetes deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dungeongate-session-service
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: session-service
        image: dungeongate/session-service:latest
        ports:
        - containerPort: 22    # SSH
        - containerPort: 8083  # HTTP API
        - containerPort: 9093  # gRPC
```

## 📊 Monitoring and Observability

### Metrics (Planned)

- **Service Metrics**: Request rates, response times, error rates
- **Business Metrics**: Active sessions, user registrations, game launches
- **Infrastructure Metrics**: Resource usage, database performance
- **Custom Metrics**: Spectator counts, frame processing rates

### Logging

- **Structured Logging**: JSON format with contextual fields
- **Correlation IDs**: Request tracing across services
- **Log Levels**: Configurable verbosity
- **Security Logging**: Authentication and authorization events

### Health Checks

- **Service Health**: Individual service status
- **Dependency Health**: Database, external service connectivity
- **Business Health**: Core functionality validation

## 🔒 Security Architecture

### Authentication Flow

1. **SSH Layer**: Basic SSH protocol authentication
2. **Application Layer**: Menu-driven user authentication
3. **Service Layer**: JWT token validation between services

### Security Controls

- **Rate Limiting**: Connection and request throttling
- **Brute Force Protection**: Failed attempt monitoring
- **Session Security**: Secure token generation and validation
- **Network Security**: TLS for inter-service communication
- **Input Validation**: Comprehensive request sanitization

## 🎯 Performance Characteristics

### Scalability Targets

- **Concurrent Users**: 1000+ simultaneous SSH connections
- **Session Throughput**: 10,000+ session operations per second
- **Spectator Scale**: 100+ spectators per game session
- **Database Performance**: Sub-millisecond query response times

### Optimization Strategies

- **Connection Pooling**: Efficient resource utilization
- **Async Processing**: Non-blocking I/O operations
- **Caching**: Redis for session and configuration data
- **Load Balancing**: Horizontal scaling with session affinity

## 🔄 Future Architecture Evolution

### Planned Enhancements

1. **Service Mesh**: Istio for advanced networking and security
2. **Event Sourcing**: Immutable event logs for audit and replay
3. **CQRS**: Command-Query Responsibility Segregation for read/write optimization
4. **Streaming**: Apache Kafka for real-time event processing
5. **Edge Computing**: CDN-like game session distribution

### Technology Roadmap

- **Go 1.22+**: Latest language features and performance improvements
- **gRPC-Web**: Browser-native gRPC communication
- **WebAssembly**: Client-side game logic execution
- **Container Orchestration**: Advanced Kubernetes patterns
- **Multi-Cloud**: Cloud-agnostic deployment strategies

---

This architecture provides a solid foundation for scalable, maintainable terminal gaming infrastructure while maintaining the simplicity and performance characteristics required for real-time terminal applications.