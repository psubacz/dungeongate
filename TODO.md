# Updated TODO.md - Microservice Architecture Migration

## Completed ✅

- ✅ All configs now live in the configs directory
- ✅ Implemented golang-standards/project-layout structure
- ✅ Session Service fully implemented (SSH server, HTTP handlers, TTY management)
- ✅ Separated build-time and runtime configurations
- ✅ Kubernetes-ready configuration structure
- ✅ External database made optional (SQLite default, PostgreSQL optional)
- ✅ Namespace isolation configuration structure designed

## Easy Tasks 🟢

- update watch menu to look better
- login failure. notify user that user name doest exist
- login failure. notify user that password is incorrect
- login failure. Increase login attempts when failure to login to a max of 3, make it a configurable number
- edit message when exiting that game was saved and display the hash of the save
- Look into inactivity time and automatic logout
- Spectating. We only want to view live stream gameplay. once a user stops playing, clear buffers and remove from menus
- Spectating. windows for the game stream and game messages. window for 
- Logging to file, template in `pkg/log/log.go`
- Logging implementation
- Add TCP socket tuning (TCP_NODELAY, SO_RCVBUF, SO_SNDBUF) for SSH connections
- Replace string concatenation with strings.Builder in query detection (database.go:431-458)
- added messages when session or game servers it at capacity 

## Medium Tasks 🟡
- add intialization functions to loop-fail gracefully if not all componetns are up: database connections, auth service, game service, user
- implment whitelist/blacklist options for incoming connections
- User Service implementation (partial - service layer exists, needs HTTP handlers)
- Implement configuration limits for number of connections to ssh for resource constrained systems (number of connections and games allowed per server)
- Semi-public mode which requires accounts and pre account creation
- Prometheus metrics, not all are displaying as expected
- Add dashboard template configuration for common metrics
- server scoring per user
- global server scores
- Container files for each service
- look into golang object-pooling to reduce allocation churn
- Implement bounded connection pool with worker goroutines for SSH connections to prevent resource exhaustion
- Add sync.Pool for PTY buffer reuse (4096 byte buffers in pty_manager.go:315) to reduce GC pressure
- Add connection rate limiting and backpressure mechanisms to prevent DoS attacks
- Replace unbounded goroutine creation in SSH handler (ssh.go:253, 328, 337) with worker pools
- Implement object pooling for StreamFrame allocation in stream manager (types.go:199) to reduce memory allocations
- Replace mutex-protected counters with atomic operations for frequently updated statistics
- Implement prepared statement caching in database layer to improve query performance
- Pre-allocate slices and maps with known capacity in hot paths to prevent runtime resizing
- Add context-based timeout management for SSH connections to prevent resource leaks
- Implement worker pool for spectator frame distribution (types.go:159) to bound concurrency
- game autosave on exit or ctrl-c from game. make it a user option. enabled by default. store option in database
- add a admin account flag to user profile.
- add `hidden` admin menu for ssh native administation: account management, update and hot reloading configs (if not in k8s)
- look into adding a status webpage
- Session.simplify: break out `internal/session/ssh.go` into smaller files
- game.save: allow users to share a save file( or game config? need to see how seeds are generated). make it an option
- Session.spectating: migrate to a simple pub-sub model between game and session.
- Session.spectating: when a user stops playing, the spectators should return to lobby

## Hard Tasks 🔴

- Game Service implementation (currently stub with health endpoint only). as part of microservice architecture. I want to breakout functionality out of session and into internal/games. lets make this for games running, playing, saving, loading, etc...
- Stream encryption implementation (currently stub returning unencrypted data)
- Namespace isolation implementation (Linux syscalls and container integration)
- Game isolation when multiple players are using the
- Shared game state for nethack "bones" across multiple server/containers of nethack
- Automated password reset and account recovery (should require an sshkey or email)
- look into https://alt.org/nethack/ integration?
- helm charts

## maybe?

### Connection Distribution & Load Balancing 🌐

- Implement PTY tunneling over gRPC between session and game services for container separation
- Add SSH load balancer to distribute connections across multiple session service instances
- Implement game service discovery and load balancing for optimal resource utilization
- Create session proxy pattern to decouple SSH termination from game execution
- Add distributed session state management (Redis/etcd) for session resilience across containers
- Implement intelligent connection routing based on user location, game requirements, and service load
- Add spectator streaming relay services for CDN distribution and global spectating
- Create connection migration support for game service failures and maintenance
- Implement multi-cluster Kubernetes deployment for geographic distribution
- Add service mesh integration (Istio/Linkerd) for secure inter-service communication

### Resilient Connection Handling 🛡️

- Implement connection-level circuit breaker in SSH server (ssh.go:240) to prevent cascading failures
- Add SSH handshake circuit breaker (ssh.go:287) to protect against expensive handshake abuse
- Implement PTY allocation circuit breaker (pty_manager.go:65) to prevent system resource exhaustion
- Add IP-based connection rate limiting with bounded queues for backpressure control
- Implement authentication rate limiting (ssh.go:2130) to prevent brute force attacks
- Add session creation backpressure control when approaching resource limits
- Implement load shedding based on system resource utilization (CPU/memory thresholds)
- Add connection admission control with graceful degradation under high load
- Implement connection queue with timeout-based dropping for overload protection
- Add resource-based load shedding for PTY allocation and session creation
- Enable rate limiting and brute force protection in production configs
- Add connection pool monitoring and alerting for circuit breaker state changes