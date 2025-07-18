Connection-Optimized Refactor Design

  Core Architecture: Pool-Based Resource Management

  internal/session/
  ├── pools/                    # Worker pool infrastructure
  │   ├── connection_pool.go    # Connection pool manager
  │   ├── worker_pool.go        # Worker goroutine pool
  │   ├── pty_pool.go          # PTY resource pool
  │   └── backpressure.go      # Queue & backpressure management
  ├── handlers/                 # Refactored handler components
  │   ├── session_handler.go    # Main session coordinator
  │   ├── auth_handler.go       # Authentication workflows
  │   ├── game_handler.go       # Game session management
  │   ├── menu_handler.go       # Menu actions (enhanced existing)
  │   └── stream_handler.go     # I/O streaming
  ├── resources/               # Resource management
  │   ├── limiter.go           # Resource limits & quotas
  │   ├── tracker.go           # Connection & resource tracking
  │   └── metrics.go           # Resource utilization metrics
  └── middleware/              # Connection middleware
      ├── rate_limiter.go      # Rate limiting middleware
      ├── auth_middleware.go   # Authentication middleware
      └── logging_middleware.go # Request logging

  1. Connection Pool Architecture

  pools/connection_pool.go
  type ConnectionPool struct {
      maxConnections     int
      activeConnections  map[string]*Connection
      connectionQueue    chan *ConnectionRequest
      workerPool         *WorkerPool
      ptyPool           *PTYPool
      backpressure      *BackpressureManager
      rateLimiter       *RateLimiter
      metrics           *Metrics
      logger            *slog.Logger
      mu                sync.RWMutex
  }

  type Connection struct {
      ID            string
      SSHChannel    ssh.Channel
      UserID        string
      CreatedAt     time.Time
      LastActivity  time.Time
      State         ConnectionState
      ResourceQuota *ResourceQuota
  }

  type ConnectionRequest struct {
      SSHChannel   ssh.Channel
      SSHConn      *ssh.ServerConn
      ResponseChan chan *ConnectionResponse
      Priority     Priority
  }

  Key Features:
  - Bounded Connection Pool: Hard limits on concurrent connections
  - Queue Management: FIFO queue with priority support
  - Resource Tracking: Per-connection resource quotas
  - Graceful Degradation: Reject new connections when at capacity

  2. Worker Pool Architecture

  pools/worker_pool.go
  type WorkerPool struct {
      workers        []*Worker
      workQueue      chan WorkItem
      workerCount    int
      shutdownChan   chan struct{}
      wg            sync.WaitGroup
      metrics       *WorkerMetrics
  }

  type Worker struct {
      id           int
      pool         *WorkerPool
      workQueue    chan WorkItem
      stopChan     chan struct{}
      currentWork  *WorkItem
      totalProcessed int64
  }

  type WorkItem struct {
      Type        WorkType
      Connection  *Connection
      Handler     HandlerFunc
      Context     context.Context
      Priority    Priority
      QueuedAt    time.Time
  }

  type WorkType int
  const (
      WorkTypeNewConnection WorkType = iota
      WorkTypeMenuAction
      WorkTypeGameIO
      WorkTypeAuthentication
      WorkTypeCleanup
  )

  Key Features:
  - Fixed Goroutine Pool: Prevent goroutine explosion
  - Work Queue: Buffered queue for task distribution
  - Priority Handling: Critical tasks get priority
  - Metrics Collection: Worker utilization and performance

  3. PTY Resource Pool

  pools/pty_pool.go
  type PTYPool struct {
      maxPTYs       int
      activePTYs    map[string]*PTYResource
      availablePTYs chan *PTYResource
      fdLimit       int
      currentFDs    int64
      metrics       *PTYMetrics
      mu           sync.RWMutex
  }

  type PTYResource struct {
      ID          string
      FD          int
      Master      *os.File
      Slave       *os.File
      CreatedAt   time.Time
      LastUsed    time.Time
      InUse       bool
      SessionID   string
  }

  Key Features:
  - PTY Recycling: Reuse PTY resources when possible
  - FD Management: Track and limit file descriptors
  - Resource Limits: Prevent PTY exhaustion
  - Cleanup: Automatic PTY cleanup and recycling

  4. Backpressure Management

  pools/backpressure.go
  type BackpressureManager struct {
      connectionQueue   *Queue
      workQueue         *Queue
      circuitBreaker    *CircuitBreaker
      loadShedder      *LoadShedder
      metrics          *BackpressureMetrics
  }

  type Queue struct {
      items        []QueueItem
      maxSize      int
      currentSize  int
      dropPolicy   DropPolicy
      mu          sync.RWMutex
  }

  type CircuitBreaker struct {
      state          CircuitState
      failureCount   int64
      successCount   int64
      threshold      int64
      timeout        time.Duration
      lastFailure    time.Time
  }

  Key Features:
  - Queue Management: Bounded queues with drop policies
  - Circuit Breaker: Fail fast when overwhelmed
  - Load Shedding: Drop low-priority requests under load
  - Adaptive Limits: Dynamic adjustment based on performance

  5. Refactored Handler Architecture

  handlers/session_handler.go (Main Coordinator)
  type SessionHandler struct {
      connectionPool  *pools.ConnectionPool
      authHandler     *AuthHandler
      gameHandler     *GameHandler
      menuHandler     *MenuHandler  // Enhanced existing
      streamHandler   *StreamHandler
      middleware      []Middleware
      logger         *slog.Logger
  }

  func (sh *SessionHandler) HandleNewConnection(ctx context.Context, channel ssh.Channel, conn *ssh.ServerConn) {
      // 1. Create connection request
      req := &pools.ConnectionRequest{
          SSHChannel:   channel,
          SSHConn:      conn,
          ResponseChan: make(chan *pools.ConnectionResponse, 1),
          Priority:     pools.PriorityNormal,
      }

      // 2. Submit to connection pool
      select {
      case sh.connectionPool.RequestQueue() <- req:
          // Request queued successfully
      case <-time.After(sh.connectionPool.QueueTimeout()):
          // Queue full, reject connection
          sh.rejectConnection(channel, "Server overloaded")
          return
      }

      // 3. Wait for worker assignment
      select {
      case resp := <-req.ResponseChan:
          if resp.Error != nil {
              sh.rejectConnection(channel, resp.Error.Error())
              return
          }
          // Connection accepted, proceed with session
          sh.handleSessionLifecycle(ctx, resp.Connection)
      case <-ctx.Done():
          return
      }
  }

  6. Enhanced Resource Management

  resources/limiter.go
  type ResourceLimiter struct {
      limits     map[ResourceType]*Limit
      usage      map[string]*ResourceUsage  // keyed by user/connection ID
      quotas     map[string]*ResourceQuota
      metrics    *ResourceMetrics
      mu        sync.RWMutex
  }

  type ResourceType int
  const (
      ResourceTypeConnections ResourceType = iota
      ResourceTypePTYs
      ResourceTypeFileDescriptors
      ResourceTypeMemory
      ResourceTypeBandwidth
  )

  type ResourceQuota struct {
      UserID           string
      MaxConnections   int
      MaxPTYs         int
      MaxBandwidth    int64  // bytes/sec
      MaxMemory       int64  // bytes
      ExpiresAt       time.Time
  }

  7. Integration with Existing Menu System

  Since the menu system was recently updated, we'll enhance it to work with the pool architecture:

  Enhanced menu/action_handler.go
  type PoolAwareActionHandler struct {
      *MenuHandler  // Embed existing MenuHandler
      workerPool    *pools.WorkerPool
      limiter       *resources.ResourceLimiter
  }

  func (pah *PoolAwareActionHandler) ExecuteAction(ctx context.Context, conn *pools.Connection, choice *MenuChoice) error {
      // Check resource limits
      if !pah.limiter.CanExecute(conn.UserID, choice.Action) {
          return ErrResourceLimitExceeded
      }

      // Create work item
      work := &pools.WorkItem{
          Type:       pah.getWorkType(choice.Action),
          Connection: conn,
          Handler:    pah.getHandler(choice.Action),
          Context:    ctx,
          Priority:   pah.getPriority(choice.Action),
          QueuedAt:   time.Now(),
      }

      // Submit to worker pool
      return pah.workerPool.Submit(work)
  }

  8. Configuration Structure

  config/pool_config.go
  type PoolConfig struct {
      Connection struct {
          MaxConnections      int           `yaml:"max_connections" default:"1000"`
          QueueSize          int           `yaml:"queue_size" default:"100"`
          QueueTimeout       time.Duration `yaml:"queue_timeout" default:"5s"`
          IdleTimeout        time.Duration `yaml:"idle_timeout" default:"30m"`
          DrainTimeout       time.Duration `yaml:"drain_timeout" default:"30s"`
      } `yaml:"connection"`

      Worker struct {
          PoolSize           int           `yaml:"pool_size" default:"50"`
          QueueSize          int           `yaml:"queue_size" default:"1000"`
          WorkerTimeout      time.Duration `yaml:"worker_timeout" default:"30s"`
          ShutdownTimeout    time.Duration `yaml:"shutdown_timeout" default:"10s"`
      } `yaml:"worker"`

      PTY struct {
          MaxPTYs           int `yaml:"max_ptys" default:"500"`
          ReuseTimeout      time.Duration `yaml:"reuse_timeout" default:"5m"`
          CleanupInterval   time.Duration `yaml:"cleanup_interval" default:"1m"`
          FDLimit          int `yaml:"fd_limit" default:"1024"`
      } `yaml:"pty"`

      Backpressure struct {
          Enabled           bool          `yaml:"enabled" default:"true"`
          CircuitBreaker    bool          `yaml:"circuit_breaker" default:"true"`
          LoadShedding      bool          `yaml:"load_shedding" default:"true"`
          FailureThreshold  int           `yaml:"failure_threshold" default:"10"`
          RecoveryTimeout   time.Duration `yaml:"recovery_timeout" default:"60s"`
      } `yaml:"backpressure"`
  }

  9. Migration Strategy

  Phase 1: Infrastructure (Week 1)
  1. Create pool infrastructure (pools/ package)
  2. Implement resource management (resources/ package)
  3. Add configuration support

  Phase 2: Handler Refactoring (Week 2)
  1. Extract SessionHandler from monolithic handler
  2. Create specialized handlers (auth_handler.go, game_handler.go, stream_handler.go)
  3. Integrate with existing MenuHandler

  Phase 3: Pool Integration (Week 3)
  1. Replace direct goroutine spawning with worker pools
  2. Implement connection pooling
  3. Add backpressure management

  Phase 4: Testing & Optimization (Week 4)
  1. Load testing with the new architecture
  2. Performance tuning
  3. Resource limit calibration

  10. Benefits of This Approach

  Immediate Benefits:
  - Resource Control: Hard limits prevent resource exhaustion
  - Better Performance: Worker pools eliminate goroutine overhead
  - Scalability: Designed for horizontal scaling from day one
  - Maintainability: Clear separation of concerns

  Future Benefits:
  - Easy Load Balancing: Stateless design works with any LB
  - Monitoring: Built-in metrics for all resource pools
  - Graceful Degradation: Handles overload scenarios elegantly
  - Zero-Downtime Updates: Connection draining support

  This refactor transforms the current monolithic handler into a modern, scalable architecture that's ready for the planned connection optimization features while
  maintaining compatibility with the existing menu system and functionality.