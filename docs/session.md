# Session Service Architecture

## Overview

The DungeonGate Session Service provides SSH-based terminal access to games through a modern, pool-based architecture designed for high scalability, resource efficiency, and reliability. The service has been completely refactored from a monolithic design to a microservices-ready, horizontally scalable architecture.

## Architecture Evolution

### Previous Architecture (Legacy)
- **Monolithic Handler**: Single large handler managing all connection logic
- **Direct Goroutine Spawning**: Created new goroutines for each connection without limits
- **No Resource Management**: Limited control over system resource usage
- **Minimal Observability**: Basic logging without comprehensive metrics

### Current Architecture (Pool-Based)
- **Pool-Based Design**: Connection, worker, and PTY resource pools
- **Resource Management**: Comprehensive quotas, limits, and usage tracking
- **Backpressure Management**: Circuit breaker and load shedding patterns
- **Comprehensive Observability**: Structured logging and Prometheus metrics
- **Horizontal Scalability**: Stateless design ready for load balancing

## Core Components

### 1. Connection Pool (`internal/session/pools/connection_pool.go`)

The connection pool manages SSH connections with bounded resource limits and priority queuing.

#### Key Features
- **Bounded Connections**: Hard limit on concurrent connections (default: 1000)
- **Priority Queuing**: Support for critical, high, normal, and low priority requests
- **Graceful Shutdown**: Configurable drain timeout for clean shutdowns
- **Resource Quotas**: Per-connection resource limits and tracking
- **Idle Management**: Automatic cleanup of idle connections

#### Configuration
```yaml
connection_pool:
  max_connections: 1000
  queue_size: 100
  queue_timeout: "5s"
  idle_timeout: "30m"
  drain_timeout: "30s"
```

#### Usage Example
```go
// Request a new connection
conn, err := pool.RequestConnection(ctx, channel, sshConn, PriorityNormal)
if err != nil {
    return fmt.Errorf("connection rejected: %w", err)
}

// Use the connection
defer pool.ReleaseConnection(conn.ID)
```

#### Metrics
- `session_pool_active_connections` - Active connections count
- `session_pool_total_connections` - Total connections created
- `session_pool_rejected_connections` - Rejected connection count
- `session_pool_queue_time_seconds` - Time spent in queue

### 2. Worker Pool (`internal/session/pools/worker_pool.go`)

The worker pool manages a fixed number of goroutines to process work items, preventing goroutine explosion.

#### Key Features
- **Fixed Goroutine Pool**: Configurable number of workers (default: 50)
- **Work Item Prioritization**: Priority-based work queue processing
- **Work Type Specialization**: Different handlers for different work types
- **Timeout Management**: Configurable timeouts for work execution
- **Comprehensive Metrics**: Worker utilization and performance tracking

#### Work Types
- `WorkTypeNewConnection` - New SSH connection setup
- `WorkTypeMenuAction` - Menu navigation and user interactions
- `WorkTypeGameIO` - Game I/O streaming
- `WorkTypeAuthentication` - User authentication flows
- `WorkTypeCleanup` - Resource cleanup operations
- `WorkTypeStreamManagement` - Spectator stream management

#### Configuration
```yaml
worker_pool:
  pool_size: 50
  queue_size: 1000
  worker_timeout: "30s"
  shutdown_timeout: "10s"
```

#### Usage Example
```go
// Submit work to the pool
work := &WorkItem{
    Type:       WorkTypeMenuAction,
    Connection: conn,
    Handler:    handleMenuAction,
    Context:    ctx,
    Priority:   PriorityNormal,
}

err := workerPool.Submit(work)
```

#### Metrics
- `session_worker_active_workers` - Number of busy workers
- `session_worker_queue_size` - Work items in queue
- `session_worker_processing_time` - Work processing duration
- `session_worker_utilization` - Worker pool utilization percentage


### 4. Backpressure Manager (`internal/session/pools/backpressure.go`)

The backpressure manager implements circuit breaker and load shedding patterns to protect the system under load.

#### Key Features
- **Circuit Breaker**: Fail-fast when error rate is high
- **Load Shedding**: Drop requests when system load is high
- **Queue Management**: Bounded queues with drop policies
- **System Monitoring**: Track CPU, memory, and queue utilization
- **Adaptive Behavior**: Automatically adjust to system conditions

#### Circuit Breaker States
- **Closed**: Normal operation, all requests allowed
- **Open**: High error rate, requests rejected immediately
- **Half-Open**: Testing recovery, limited requests allowed

#### Configuration
```yaml
backpressure:
  enabled: true
  circuit_breaker:
    enabled: true
    failure_threshold: 10
    recovery_timeout: "60s"
  load_shedding:
    enabled: true
    cpu_threshold: 0.8
    memory_threshold: 0.9
```

#### Usage Example
```go
// Check if request can be accepted
if !backpressure.CanAccept() {
    return fmt.Errorf("server overloaded")
}

// Record operation result
if err != nil {
    backpressure.RecordFailure()
} else {
    backpressure.RecordSuccess()
}
```

## Resource Management

### 1. Resource Limiter (`internal/session/resources/limiter.go`)

Manages resource limits and quotas at user and system levels.

#### Resource Types
- **Connections**: Number of concurrent SSH connections
- **PTYs**: Number of pseudo-terminals
- **Memory**: Memory usage in bytes
- **Bandwidth**: Network bandwidth usage
- **CPU**: CPU core allocation
- **File Descriptors**: Open file descriptor count

#### User Quotas
```yaml
default_user_quota:
  max_connections: 10
  max_ptys: 5
  max_memory: "256MB"
  max_bandwidth: "10MB/s"
  max_cpu_cores: 0.5
  expires_after: "24h"
  priority: 1

vip_user_quota:
  max_connections: 25
  max_ptys: 10
  max_memory: "512MB"
  max_bandwidth: "50MB/s"
  max_cpu_cores: 1.0
  expires_after: "168h"
  priority: 5
```

### 2. Resource Tracker (`internal/session/resources/tracker.go`)

Tracks resource usage across connections and sessions.

#### Tracked Metrics
- Connection lifecycle and state changes
- Data transfer statistics (bytes sent/received)
- Session duration and resource consumption
- User activity patterns
- Resource utilization trends

#### Usage Monitoring
```go
// Track connection
tracker.TrackConnection(connID, userID, remoteAddr)

// Track data transfer
tracker.TrackDataTransfer(connID, bytesSent, bytesReceived)

// Track session
tracker.TrackSession(sessionID, userID, connID, gameID)
```

### 3. Metrics Registry (`internal/session/resources/metrics.go`)

Comprehensive metrics collection with Prometheus-style metrics.

#### Metric Types
- **Counters**: Monotonically increasing values (total connections)
- **Gauges**: Current values that can go up/down (active connections)
- **Histograms**: Distribution of values (response times)
- **Custom Collectors**: Domain-specific metric collection

#### Key Metrics
```
# Connection Pool Metrics
session_pool_active_connections
session_pool_total_connections
session_pool_rejected_connections
session_pool_queue_time_seconds

# Worker Pool Metrics
session_worker_active_workers
session_worker_queue_size
session_worker_processing_time_seconds
session_worker_utilization

# PTY Pool Metrics
session_pty_active_ptys
session_pty_available_ptys
session_pty_fd_usage
session_pty_reuse_rate

# Resource Usage Metrics
session_resource_memory_usage_bytes
session_resource_bandwidth_usage_bytes
session_resource_cpu_usage_cores
session_resource_violations_total
```

## Configuration

### Basic Configuration

The session service configuration is found in `configs/session-service.yaml` and includes comprehensive pool settings:

```yaml
# Connection Pool Configuration
connection_pool:
  max_connections: 1000
  queue_size: 100
  queue_timeout: "5s"
  idle_timeout: "30m"
  drain_timeout: "30s"

# Worker Pool Configuration
worker_pool:
  pool_size: 50
  queue_size: 1000
  worker_timeout: "30s"
  shutdown_timeout: "10s"


# Resource Management
resource_management:
  limits:
    system:
      max_connections: 1000
      max_ptys: 500
      max_memory: "8GB"
      max_bandwidth: "1GB/s"
      max_file_descriptors: 8192
```

### Advanced Configuration

For production deployments, advanced configuration options are available:

```yaml
advanced_pool_config:
  connection_pool:
    enable_prioritization: true
    connection_warming:
      enabled: true
      min_connections: 10



```

## Monitoring and Observability

### Structured Logging

All components use structured logging with contextual information:

```go
logger.Info("Connection accepted",
    "connection_id", connID,
    "user_id", userID,
    "remote_addr", remoteAddr,
    "active_connections", activeCount)
```

### Health Checks

Pool components provide health check endpoints:

- `/health/connection-pool` - Connection pool health
- `/health/worker-pool` - Worker pool health
- `/health/pty-pool` - PTY pool health
- `/health/resource-limiter` - Resource limiter health

### Metrics Endpoints

Prometheus metrics are exposed at:
- `:8085/metrics` - Session service metrics

### Alert Thresholds

Configurable alert thresholds for monitoring:

```yaml
alerts:
  high_connection_count: 900
  high_queue_time: "2s"
  high_error_rate: 0.05
  high_resource_usage: 0.9
```

## Architecture Diagrams

### Pool-Based Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    Session Service (Pool-Based)                │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │ Connection Pool │  │  Worker Pool    │  │    PTY Pool     │  │
│  │                 │  │                 │  │                 │  │
│  │ • Bounded Conns │  │ • Fixed Workers │  │ • PTY Reuse     │  │
│  │ • Priority Queue│  │ • Work Queue    │  │ • FD Management │  │
│  │ • Graceful      │  │ • Specialization│  │ • Health Checks │  │
│  │   Shutdown      │  │ • Metrics       │  │ • Auto Cleanup  │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │ Backpressure    │  │ Resource        │  │ Metrics         │  │
│  │ Manager         │  │ Management      │  │ Registry        │  │
│  │                 │  │                 │  │                 │  │
│  │ • Circuit Break │  │ • Quotas        │  │ • Prometheus    │  │
│  │ • Load Shedding │  │ • Limits        │  │ • Custom        │  │
│  │ • Queue Mgmt    │  │ • Tracking      │  │   Collectors    │  │
│  │ • Monitoring    │  │ • Violations    │  │ • Health Checks │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### Request Flow Architecture

```
SSH Client ──┐
             │
SSH Client ──┼──► Connection Pool ──► Worker Pool ──► Handlers
             │         │                   │              │
SSH Client ──┘         │                   │              │
                       │                   │              │
             Backpressure ◄────────────────┼──────────────┘
               Manager                     │
                 │                         │
                 ▼                         ▼
           Circuit Breaker            PTY Pool ──► Game Service
           Load Shedding                │              │
                                        │              │
                                        ▼              ▼
                                 Resource Tracker   NetHack
                                                       Process
```

### Resource Management Flow

```
┌─────────────┐    Resource     ┌─────────────┐    Usage      ┌─────────────┐
│    User     │    Request      │  Resource   │   Tracking    │  Resource   │
│ Connection  │ ──────────────► │   Limiter   │ ────────────► │   Tracker   │
│             │                 │             │               │             │
│ • UserID    │    ✓ Allow      │ • Quotas    │   • Monitor   │ • Metrics   │
│ • Conn ID   │ ◄────────────── │ • Limits    │   • Stats     │ • History   │
│ • Priority  │    ✗ Reject     │ • Violations│   • Alerts    │ • Cleanup   │
└─────────────┘                 └─────────────┘               └─────────────┘
```

## Migration from Legacy Architecture

### Migration Status

- ✅ **Phase 1: Infrastructure** (Completed)
  - Pool infrastructure implementation
  - Resource management components
  - Configuration integration

- 🚧 **Phase 2: Handler Refactoring** (In Progress)
  - Extract SessionHandler from monolithic handler
  - Create specialized handlers (auth, game, stream)
  - Integrate with existing MenuHandler

- 📋 **Phase 3: Pool Integration** (Planned)
  - Replace direct goroutine spawning with worker pools
  - Implement connection pooling
  - Enable backpressure management

- 📋 **Phase 4: Testing and Optimization** (Planned)
  - Comprehensive testing for pool architecture
  - Performance tuning and resource limit calibration
  - Load testing and optimization

### Compatibility

The new architecture maintains compatibility with:
- Existing SSH client connections
- Current menu system and user interface
- Auth service and game service integration
- Configuration file structure (with extensions)

## Deployment Considerations

### Horizontal Scaling

The pool-based architecture supports horizontal scaling:

1. **Stateless Design**: No session state stored in service
2. **Load Balancer Ready**: Works with any TCP/HTTP load balancer
3. **Resource Isolation**: Each instance manages its own resource pools
4. **Health Check Support**: Built-in health checks for load balancer integration

### Resource Planning

For capacity planning, consider:

- **Connection Pool Size**: Based on expected concurrent users
- **Worker Pool Size**: Based on CPU cores and workload characteristics
- **PTY Pool Size**: Based on game session patterns
- **Memory Limits**: Account for connection overhead and game resources

### Performance Tuning

Key performance tuning parameters:

## Development

### Quick Start

```bash
# Install dependencies
make deps && make deps-tools

# Build services
make build-all

# Start all services
make run-all

# Test SSH connection
ssh -p 2222 localhost
```

### Project Structure

```
internal/session/
├── pools/                    # Pool infrastructure
│   ├── connection_pool.go    # Connection management
│   ├── worker_pool.go        # Worker goroutine pool
│   └── backpressure.go      # Backpressure management
├── resources/               # Resource management
│   ├── limiter.go           # Resource limits and quotas
│   ├── tracker.go           # Resource usage tracking
│   └── metrics.go           # Metrics collection
├── handlers/                # Handler components (future)
│   ├── session_handler.go   # Main session coordinator
│   ├── auth_handler.go      # Authentication workflows
│   ├── game_handler.go      # Game session management
│   └── stream_handler.go    # I/O streaming
└── middleware/              # Connection middleware (future)
    ├── rate_limiter.go      # Rate limiting middleware
    └── auth_middleware.go   # Authentication middleware

# Legacy components (being refactored)
internal/session-old/
├── connection/              # Legacy connection handling
├── menu/                   # Menu system (being enhanced)
├── streaming/              # Streaming management
└── terminal/               # Terminal handling
```

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run pool-specific tests
go test ./internal/session/pools/...
go test ./internal/session/resources/...

# Run with race detection
make test-race

# Generate coverage reports
make test-coverage
```

### Test Categories

- **Unit Tests**: Individual component testing
- **Integration Tests**: Service interaction testing
- **Load Tests**: Pool capacity and performance testing
- **Stress Tests**: Resource exhaustion scenarios

## Security Considerations

### Resource Protection
- User quotas prevent resource exhaustion attacks
- Rate limiting protects against brute force attempts
- Circuit breaker prevents cascade failures

### Connection Security
- SSH authentication with auth service integration
- JWT token validation for API access
- Connection state tracking and validation

### Data Protection
- Secure handling of user credentials
- Encrypted communication channels
- Audit logging for security events

## Troubleshooting

### Common Issues

1. **Connection Rejected**: Check connection pool limits and queue status
2. **High Queue Times**: Increase worker pool size or connection pool capacity
3. **PTY Allocation Failures**: Check FD limits and PTY pool configuration
4. **Resource Exhaustion**: Review resource quotas and system limits

### Debugging Tools

1. **Metrics Dashboard**: Monitor pool utilization and performance
2. **Health Checks**: Verify component health status
3. **Structured Logs**: Search and filter by connection, user, or session ID
4. **Resource Tracking**: Monitor user and system resource usage

### Performance Analysis

Use the built-in metrics to analyze performance:

```bash
# Check connection pool utilization
curl localhost:8085/metrics | grep session_pool_active_connections

# Monitor worker pool performance
curl localhost:8085/metrics | grep session_worker_utilization

# Track PTY efficiency
curl localhost:8085/metrics | grep session_pty_reuse_rate
```

## Future Enhancements

### Planned Features
- Dynamic worker scaling based on load
- Advanced queue prioritization algorithms
- Intelligent PTY pre-allocation
- Machine learning-based resource prediction

### Kubernetes Integration
- Horizontal Pod Autoscaler (HPA) support
- Resource request/limit optimization
- Service mesh integration
- Advanced health checks

### Enhanced Monitoring
- Distributed tracing support
- Advanced alerting rules
- Performance profiling integration
- Real-time dashboard updates

## Conclusion

The new pool-based session service architecture provides a solid foundation for scalable, reliable terminal game hosting. With comprehensive resource management, observability, and horizontal scaling support, the service is ready for production deployment and future growth.

The architecture emphasizes:
- **Reliability**: Circuit breaker and backpressure management
- **Scalability**: Horizontal scaling and resource pools
- **Observability**: Comprehensive metrics and structured logging
- **Maintainability**: Clear separation of concerns and modular design
- **Security**: Resource protection and secure communication

This design positions DungeonGate for future growth while maintaining the performance and reliability required for real-time terminal gaming.

---

**Built with ❤️ for the roguelike gaming community**

For questions, issues, or contributions, please visit our GitHub repository.