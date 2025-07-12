# TODO.md - Tasks Classified by Service

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

## User Service

### user.core
- User Service implementation (partial - service layer exists, needs HTTP handlers)
- Semi-public mode which requires accounts and pre-account creation

### user.scoring
- Server scoring per user
- Global server scores

## Infrastructure/Platform

### platform.logging
- Logging to file, template in `pkg/log/log.go`
- Logging implementation

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