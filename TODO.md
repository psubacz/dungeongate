# Updated TODO.md - Microservice Architecture Migration

## Completed ✅

- ✅ All configs now live in the configs directory
- ✅ Implemented golang-standards/project-layout structure
- ✅ Created microservice architecture with separate services:
  - Auth Service (authentication & authorization)
  - User Service (user management & registration)
  - Game Service (game management & process control)
  - Session Service (TTY sessions & recording)
- ✅ Separated build-time and runtime configurations
- ✅ Kubernetes-ready configuration structure
- ✅ Namespace isolation design (replaces traditional chroot)
- ✅ Stream encryption support in session service
- ✅ External database made optional (SQLite default, PostgreSQL optional)

## In Progress 🚧

- logging
- prometheus metrics     ☐ Add dashboard configuration for common metrics

### Core Functionality Implementation
- [ ] **User signup and return to main menu**
  - User service: Complete user registration flow
  - Auth service: Integrate with user service
  - Main menu: Update to use microservices

- [ ] **User login**
  - User service: Password hashing with bcrypt
  - Session management: Token validation
  - hash user credentails

- [ ] **Game Service**
    - launch game
    - connect
    - save
    - load


- [ ] **User watch (spectating)**
  - Session service: Complete spectator functionality
  - Game service: Integration with session service
  - Real-time streaming implementation
  - Circular buffer for replays?

- [ ] **User exit**
  - Session service: Graceful session termination
  - Game service: Process cleanup
  - Resource cleanup implementation

### Technical Improvements
- [ ] **Fix login with SSH to present game menu**
  - Update SSH handler to use microservices
  - Implement service-to-service communication
  - Menu system integration

- [ ] **Ensure credentials are hashed**
  - Implement bcrypt in auth service
  - Migrate existing passwords if needed
  - Add salt generation and storage

- [ ] **Namespace isolation (replaces chroot)**
  - Implement Linux namespace isolation
  - Container-native security model
  - Process separation and resource limits

- [ ] **Stream encryption**
  - Complete encryption implementation in session service
  - Key management and rotation
  - End-to-end encryption for TTY streams

## New Tasks (from Architecture Changes) 📋

### Service Communication
- [ ] Implement gRPC service definitions
- [ ] Add service discovery mechanism
- [ ] Implement health checks for all services
- [ ] Add circuit breaker pattern for resilience

### Database & Storage
- [ ] Complete database migration system
- [ ] Implement PostgreSQL support
- [ ] Add Redis for caching and sessions
- [ ] Database connection pooling

### Security & Authentication
- [ ] Complete JWT implementation with refresh tokens
- [ ] Add role-based access control (RBAC)
- [ ] Implement API rate limiting
- [ ] Add OAuth2/OIDC support for external auth
- [ ] **Server Access Control Models**
  - [ ] Public servers: Allow anonymous user signups
  - [ ] Semi-public servers: Require invitation keys for registration
  - [ ] Private servers: Require preloaded keys for access

### Game Management
- [ ] Complete game process management
- [ ] Implement resource limiting per game
- [ ] Add game configuration hot-reloading
- [ ] Implement game state persistence

### Session Management
- [ ] Complete TTY recording with compression
- [ ] Implement session replay functionality
- [ ] Add real-time spectator streaming
- [ ] Session migration between nodes

### Monitoring & Observability
- [ ] Add Prometheus metrics to all services
- [ ] Implement distributed tracing
- [ ] Add structured logging
- [ ] Health check endpoints

### Deployment & DevOps
- [ ] Complete Kubernetes manifests
- [ ] Add Helm charts
- [ ] Implement CI/CD pipeline
- [ ] Add deployment automation

### Testing
- [ ] Add unit tests for all services
- [ ] Integration tests for service communication
- [ ] End-to-end tests for user flows
- [ ] Performance testing

## Migration Strategy 🔄

### Phase 1: Foundation (Current)
- [x] Project restructure
- [x] Configuration externalization
- [x] Service skeleton implementation
- [ ] Basic service communication

### Phase 2: Core Services
- [ ] Auth service completion
- [ ] User service completion
- [ ] Basic game launching
- [ ] Session management

### Phase 3: Advanced Features
- [ ] Spectating functionality
- [ ] TTY recording and playback
- [ ] Advanced security features
- [ ] Monitoring and observability

### Phase 4: Production Readiness
- [ ] Performance optimization
- [ ] Security hardening
- [ ] Comprehensive testing
- [ ] Documentation completion

## Architecture Benefits 🎯

### Scalability
- Independent scaling of services
- Horizontal scaling support
- Resource optimization per service

### Security
- Namespace isolation (no traditional chroot)
- Service-to-service authentication
- Encrypted communication
- Least privilege principle

### Maintainability
- Clear service boundaries
- Independent deployment
- Technology flexibility per service
- Easier debugging and monitoring

### Operational Excellence
- Kubernetes-native deployment
- Health checks and monitoring
- Automated scaling and recovery
- Configuration management

## Next Steps 🚀

1. **Complete service implementations** - Focus on core functionality
2. **Implement service communication** - gRPC between services
3. **Add comprehensive testing** - Unit, integration, and e2e tests
4. **Database migration** - From monolith to microservices
5. **Security implementation** - Authentication, authorization, encryption
6. **Monitoring setup** - Metrics, logs, traces
7. **Deployment automation** - CI/CD, Kubernetes manifests
8. **Performance optimization** - Load testing, bottleneck identification
9. **Documentation** - API docs, deployment guides, architecture docs
10. **Production deployment** - Staging environment, gradual rollout

## Notes 📝

- The architecture now supports both legacy monolithic deployment and new microservices
- Configuration is externalized and environment-aware
- Docker containers use namespaces instead of traditional chroot for better security
- All services are designed to be stateless and horizontally scalable
- The project follows standard Go project layout for better maintainability
