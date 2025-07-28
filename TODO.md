# TODO.md - Tasks Classified by Service

DungeonGate Prometheus Metrics Integration Prompt

  Objective

  Add comprehensive Prometheus metrics to all DungeonGate services for monitoring, alerting, and observability. Integrate metrics collection with the existing
  structured logging (slog) system to provide a complete observability solution.

  Current Architecture Analysis

  Services to Instrument:

  1. Auth Service (cmd/auth-service/main.go)
    - gRPC server on port 8082
    - HTTP health endpoint on port 8081
    - Handles user authentication, JWT tokens, user registration
  2. Session Service (cmd/session-service/main.go)
    - SSH server on port 2222
    - HTTP server on port 8083
    - gRPC server on port 9093
    - Manages terminal sessions, PTY bridging, spectating
  3. Game Service (cmd/game-service/main.go)
    - gRPC server on port 50051
    - HTTP server on port 8085
    - Manages game instances, session lifecycle, save files

  Existing Infrastructure:

  - Logging: Unified slog-based structured logging in pkg/logging/slog.go
  - Configuration: YAML-based config system in pkg/config/
  - Health Checks: Basic HTTP health endpoints exist
  - Metrics Stub: pkg/metrics/prometheus.go exists but needs implementation

  Implementation Requirements

  1. Core Metrics Package (pkg/metrics/prometheus.go)

  Standard Service Metrics:

  // HTTP Metrics
  http_requests_total{service, method, status_code, endpoint}
  http_request_duration_seconds{service, method, endpoint}
  http_requests_in_flight{service}

  // gRPC Metrics  
  grpc_server_requests_total{service, method, status_code}
  grpc_server_request_duration_seconds{service, method}
  grpc_server_requests_in_flight{service}

  // General Service Metrics
  service_up{service, version}
  service_start_time_seconds{service}
  service_build_info{service, version, commit, build_time}

  Auth Service Specific Metrics:

  // Authentication Metrics
  auth_login_attempts_total{service, status, user_type}
  auth_login_duration_seconds{service}
  auth_token_generations_total{service, token_type}
  auth_token_validations_total{service, status}
  auth_failed_logins_total{service, reason}
  auth_lockouts_total{service}

  // User Management
  auth_users_registered_total{service}
  auth_active_sessions{service}
  auth_jwt_tokens_issued_total{service, type}
  auth_jwt_tokens_expired_total{service}

  // Database Operations
  auth_database_operations_total{service, operation, status}
  auth_database_operation_duration_seconds{service, operation}

  Session Service Specific Metrics:

  // SSH Connection Metrics
  ssh_connections_total{service, status}
  ssh_connections_active{service}
  ssh_connection_duration_seconds{service}
  ssh_authentication_attempts_total{service, method, status}

  // Terminal Session Metrics  
  terminal_sessions_total{service, status}
  terminal_sessions_active{service}
  terminal_session_duration_seconds{service}
  terminal_pty_operations_total{service, operation}

  // Spectating Metrics
  spectator_connections_total{service}
  spectator_connections_active{service}
  spectating_sessions_active{service}

  // Data Transfer
  ssh_bytes_transferred_total{service, direction}
  terminal_data_transferred_total{service, direction}

  Game Service Specific Metrics:

  // Game Instance Metrics
  game_instances_total{service, game_id, status}
  game_instances_active{service, game_id}
  game_instance_duration_seconds{service, game_id}
  game_process_spawns_total{service, game_id, status}

  // Save Management
  game_saves_total{service, game_id, operation, status}
  game_save_size_bytes{service, game_id}
  game_save_operations_duration_seconds{service, operation}

  // Session Lifecycle
  game_sessions_started_total{service, game_id}
  game_sessions_ended_total{service, game_id, reason}
  game_session_crashes_total{service, game_id}

  // Resource Usage
  game_process_memory_bytes{service, game_id, pid}
  game_process_cpu_seconds_total{service, game_id, pid}

  2. Metrics Integration Strategy

  Middleware Pattern:

  // HTTP Middleware
  func HTTPMetricsMiddleware(serviceName string) func(http.Handler) http.Handler

  // gRPC Interceptors  
  func UnaryServerInterceptor(serviceName string) grpc.UnaryServerInterceptor
  func StreamServerInterceptor(serviceName string) grpc.StreamServerInterceptor

  Service Integration Points:

  - HTTP Handlers: Wrap all HTTP endpoints with metrics middleware
  - gRPC Services: Add interceptors to all gRPC servers
  - SSH Server: Instrument connection lifecycle in session service
  - Database Operations: Add metrics to repository pattern
  - Business Logic: Instrument key operations (auth, game starts, saves)

  3. Configuration Integration

  Update existing config files:

  # Add to common.yaml
  metrics:
    enabled: true
    port: 9090
    path: "/metrics"
    collect_interval: "15s"

  # Service-specific overrides in service configs
  metrics:
    port: 9091  # auth-service
    # port: 9092  # session-service  
    # port: 9093  # game-service

  Configuration Struct:

  // In pkg/config/config.go
  type MetricsConfig struct {
      Enabled         bool   `yaml:"enabled"`
      Port            int    `yaml:"port"`
      Path            string `yaml:"path"`
      CollectInterval string `yaml:"collect_interval"`
  }

  4. Implementation Steps

  Phase 1: Core Metrics Infrastructure

  1. Implement pkg/metrics/prometheus.go:
    - Create metric registry and standard metrics
    - Implement HTTP and gRPC middleware
    - Add service-specific metric factories
    - Integrate with existing structured logging
  2. Update Configuration System:
    - Add MetricsConfig to all service configs
    - Update YAML files with metrics configuration
    - Add environment variable support

  Phase 2: Auth Service Integration

  1. Add metrics HTTP endpoint to auth service
  2. Instrument gRPC authentication methods:
    - Login attempts and duration
    - Token generation and validation
    - User registration flows
  3. Add database operation metrics
  4. Instrument JWT token lifecycle

  Phase 3: Session Service Integration

  1. Add metrics HTTP endpoint to session service
  2. Instrument SSH server:
    - Connection lifecycle metrics
    - Authentication attempt tracking
    - Data transfer metrics
  3. Instrument terminal session management:
    - PTY creation and lifecycle
    - Session duration tracking
  4. Add spectating metrics

  Phase 4: Game Service Integration

  1. Add metrics HTTP endpoint to game service
  2. Instrument game instance management:
    - Process spawning and lifecycle
    - Resource usage tracking
  3. Add save file operation metrics
  4. Instrument session lifecycle events

  Phase 5: Advanced Metrics

  1. Business Logic Metrics:
    - User behavior patterns
    - Game popularity and usage
    - Error rate analysis
  2. Performance Metrics:
    - Memory and CPU usage
    - Network I/O patterns
    - Database query performance
  3. SLA Metrics:
    - Service availability
    - Response time percentiles
    - Error budgets

  5. Integration with Logging

  Correlation Strategy:

  // Correlate metrics with structured logs
  func InstrumentWithLogging(logger *slog.Logger, metricName string) {
      // Record metric
      metric.With(labels).Inc()

      // Log with correlation
      logger.Info("Operation completed",
          "metric", metricName,
          "labels", labels,
          "value", value,
      )
  }

  Error Correlation:

  - Automatically increment error metrics when errors are logged
  - Add tracing IDs to correlate logs with metrics
  - Include metric context in error logs

  6. Monitoring and Alerting Setup

  Key Alerts to Configure:

  # Service Availability
  service_up{service="auth-service"} == 0
  service_up{service="session-service"} == 0
  service_up{service="game-service"} == 0

  # High Error Rates
  rate(auth_login_attempts_total{status="failed"}[5m]) > 10
  rate(ssh_connections_total{status="failed"}[5m]) > 5
  rate(game_instances_total{status="crashed"}[5m]) > 1

  # Performance Degradation
  http_request_duration_seconds{quantile="0.95"} > 1.0
  grpc_server_request_duration_seconds{quantile="0.95"} > 0.5
  game_instance_duration_seconds{quantile="0.95"} > 3600

  # Resource Exhaustion
  ssh_connections_active > 950  # Near max connections
  game_instances_active > 450   # Near max games

  Dashboard Metrics:

  - Service health and uptime
  - Request rates and latencies
  - Error rates and types
  - Active connections and sessions
  - Resource utilization
  - Business metrics (user activity, game popularity)

  7. Testing Strategy

  Unit Tests:

  - Metric collection accuracy
  - Label consistency
  - Middleware functionality
  - Configuration parsing

  Integration Tests:

  - End-to-end metric collection
  - HTTP endpoints return valid Prometheus format
  - Metric correlation with actual operations
  - Performance impact measurement

  Load Tests:

  - Metric collection under high load
  - Memory usage with many active metrics
  - Performance overhead assessment

  Success Criteria

  1. Comprehensive Coverage: All critical operations instrumented with appropriate metrics
  2. Performance: <1% performance overhead from metrics collection
  3. Reliability: Metrics collection doesn't impact service functionality
  4. Observability: Clear correlation between logs, metrics, and system behavior
  5. Operational: Easy to set up monitoring and alerting based on collected metrics
  6. Standards Compliance: Metrics follow Prometheus naming conventions and best practices

  Example Implementation Patterns

  Service Startup with Metrics:

  // In main.go
  func main() {
      // ... existing setup ...

      // Initialize metrics
      metricsRegistry := metrics.NewRegistry("auth-service", version, buildTime, gitCommit)

      // Add metrics middleware to HTTP server
      httpHandler := metrics.HTTPMiddleware(metricsRegistry)(existingHandler)

      // Add metrics interceptor to gRPC server  
      grpcServer := grpc.NewServer(
          grpc.UnaryInterceptor(metrics.UnaryServerInterceptor(metricsRegistry)),
      )

      // Start metrics endpoint
      go func() {
          metricsServer := &http.Server{
              Addr: fmt.Sprintf(":%d", cfg.Metrics.Port),
              Handler: promhttp.Handler(),
          }
          metricsServer.ListenAndServe()
      }()
  }

  Business Logic Instrumentation:

  // In auth service
  func (s *AuthService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
      timer := s.metrics.LoginDuration.Start()
      defer timer.ObserveDuration()

      s.metrics.LoginAttempts.With("status", "started").Inc()

      result, err := s.authenticateUser(req.Username, req.Password)
      if err != nil {
          s.metrics.LoginAttempts.With("status", "failed", "reason", err.Error()).Inc()
          return nil, err
      }

      s.metrics.LoginAttempts.With("status", "success").Inc()
      s.metrics.ActiveSessions.Inc()

      return result, nil
  }

This 

## Session Service

### session.spectating
- Update watch menu to look better
- Clear buffers and remove from menus when user stops playing
- Add windows for game stream and game messages
- Migrate to a simple pub-sub model between game and session
- When a user stops playing, spectators should return to lobby
- Add spectator streaming relay services for CDN distribution and global spectating
- Implement worker pool for spectator frame distribution to bound concurrency

### session.gameSelect
- Edit message when exiting that game was saved and display the hash of the save

### session.connection
- Add TCP socket tuning (TCP_NODELAY, SO_RCVBUF, SO_SNDBUF) for SSH connections
- Implement bounded connection pool with worker goroutines for SSH connections
- Replace unbounded goroutine creation in SSH handler with worker pools
- Add context-based timeout management for SSH connections
- Add SSH handshake circuit breaker to protect against handshake abuse
- Implement connection-level circuit breaker to prevent cascading failures
- Add IP-based connection rate limiting with bounded queues
- Add connection admission control with graceful degradation under high load
- Implement connection queue with timeout-based dropping for overload protection
- Add SSH load balancer to distribute connections across multiple session service instances

### session.pty
- Add sync.Pool for PTY buffer reuse (4096 byte buffers) to reduce GC pressure
- Implement PTY allocation circuit breaker to prevent system resource exhaustion
- Add resource-based load shedding for PTY allocation
- Implement PTY tunneling over gRPC between session and game services

### session.stream
- Implement object pooling for StreamFrame allocation to reduce memory allocations
- Stream encryption implementation (currently stub returning unencrypted data)

### session.admin
- Add hidden admin menu for SSH native administration
- Account management functionality
- Update and hot reloading configs (if not in k8s)

### session.general
- Look into inactivity time and automatic logout
- Add messages when session server is at capacity
- Implement whitelist/blacklist options for incoming connections
- Implement configuration limits for number of SSH connections
- Add connection rate limiting and backpressure mechanisms to prevent DoS
- Add session creation backpressure control when approaching resource limits
- Implement load shedding based on system resource utilization
- Create session proxy pattern to decouple SSH termination from game execution
- Add distributed session state management (Redis/etcd) for session resilience
- Implement connection migration support for failures and maintenance

## Auth Service

### auth.login
- Notify user when username doesn't exist
- Notify user when password is incorrect
- Implement authentication rate limiting to prevent brute force attacks
- Enable rate limiting and brute force protection in production configs

### auth.recovery
- Automated password reset and account recovery (require SSH key or email)

### auth.admin
- Add admin account flag to user profile

## User Service

### user.core
- User Service implementation (partial - service layer exists, needs HTTP handlers)
- Semi-public mode which requires accounts and pre-account creation

### user.scoring
- Server scoring per user
- Global server scores

## Game Service

### game.core
- Game Service implementation (currently stub with health endpoint only)
- Breakout functionality from session into internal/games
- Handle games running, playing, saving, loading

### game.save
- Game autosave on exit or ctrl-c (make it a user option, enabled by default)
- Store autosave option in database
- Allow users to share a save file (or game config/seeds)

### game.isolation
- Game isolation when multiple players are using the same service
- Shared game state for nethack "bones" across multiple servers/containers

### game.integration
- Look into https://alt.org/nethack/ integration

### game.discovery
- Implement game service discovery and load balancing
- Add intelligent connection routing based on user location and requirements

## Infrastructure/Platform

### platform.monitoring
- Prometheus metrics (not all displaying as expected)
- Add dashboard template configuration for common metrics
- Add connection pool monitoring and alerting for circuit breaker state changes
- Add status webpage

### platform.database
- Replace string concatenation with strings.Builder in query detection
- Implement prepared statement caching to improve query performance
- Pre-allocate slices and maps with known capacity in hot paths

### platform.performance
- Look into golang object-pooling to reduce allocation churn
- Replace mutex-protected counters with atomic operations for statistics

### platform.initialization
- Add initialization functions to loop-fail gracefully if not all components are up
- Database connections, auth service, game service, user service

### platform.deployment
- Container files for each service
- Helm charts
- Implement multi-cluster Kubernetes deployment for geographic distribution
- Add service mesh integration (Istio/Linkerd) for secure inter-service communication

## Hard Tasks 🔴

- Game Service implementation (major revision, no backwards compatibility needed)
- Stream encryption implementation
- Game isolation for multiple players
- Shared game state for nethack "bones"
- Automated password reset and account recovery
- https://alt.org/nethack/ integration
- Helm charts

## Maybe/Future Considerations 🤔

### Connection Distribution & Load Balancing
- PTY tunneling over gRPC
- SSH load balancer
- Game service discovery and load balancing
- Session proxy patterni do
- Distributed session state management
- Intelligent connection routing
- Spectator streaming relay services
- Connection migration support
- Multi-cluster Kubernetes deployment
- Service mesh integration

### Resilient Connection Handling
- Various circuit breakers (connection, SSH handshake, PTY allocation)
- Rate limiting implementations
- Backpressure control mechanisms
- Load shedding strategies
- Connection admission control
- Resource-based protections
- Production config hardening
