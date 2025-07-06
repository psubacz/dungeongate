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
- edit message when exiting that game was saved and display the hash of the save
- Increase login attempts when failure to login
- Look into inactivity time and automatic logout
- Stream gameplay clear buffers once user exits, only need live game play
- Stream game windows for game messages
- Logging to file, template in `pkg/log/log.go`
- Logging implementation
- Add TCP socket tuning (TCP_NODELAY, SO_RCVBUF, SO_SNDBUF) for SSH connections
- Replace string concatenation with strings.Builder in query detection (database.go:431-458)

## Medium Tasks 🟡
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
- break out `internal/session/ssh.go` into smaller files

## Hard Tasks 🔴

- Auth Service implementation (currently stub with health endpoint only)
- Game Service implementation (currently stub with health endpoint only)
- Stream encryption implementation (currently stub returning unencrypted data)
- Namespace isolation implementation (Linux syscalls and container integration)
- Game isolation when multiple players are using the
- Shared game state for nethack "bones" across multiple server/containers of nethack
- Automated password reset and account recovery (should require an sshkey or email)
- look into https://alt.org/nethack/ integration?
- helm charts