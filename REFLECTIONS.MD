I started this project to test cluade code functionality. Here are a few thoughts:

- Requirements; Spend time working out what you want and what requirements need to be generated.
- Design; Ask a ton of questions when create a roadmap and ask for possiblities. having a understanding comp. engineering, alogrithms and data structures is useful if not required.
- Roadmap; Generate a roadmap per service is critical for multi-session working as it gives past, present, and future state of clauades work.
- Claude.md; At milestone intervlals regenerate the cluade.md to reflect the actual state of the repo and set expectations. Use it to set standards and conventions.
- Interrogation; use cluade to inspect the work done from a fresh prompt (no history). 
- .claude/settings.local.json; setting this up depends on your goals. this project was entirely vibecoded so more permissions were needed. I expect reads are better here.
- Testing; utilize the output of testing, linting, code coverage, audits to refine the roadmap.
- workflows; you can pip the output of something into claude such as `make tests | claude`
- troubleshooting; this is probably the issue with that gives teh biggest headache becuase you can find yourself in a loop that llm cant break out of. prompt engineering and direction are required to break out
- incorrect assumption; the LLM, claude in this case, will make assumptions about what to do. here is an example of a prompt:
    - scan the code for all instances of hardcode values and generate a todo list to fix. then 
- running on auto; its very tempting but you will eventually run into the `troubleshooting` issue above. this thing nneds to be watched like a sheep dog herds a flock.
- prompt generation. if you some into an issue is complex, ask cluade to generate a prompt to fix the issue, clear the history, and inject that prompt as the next steps:
# DungeonGate Logging Standardization: Migration to Go slog

## Objective
Systematically replace all logging implementations in the DungeonGate codebase with Go's native structured logging (slog) to achieve consistency, better performance, and improved observability.

## Current Logging State Analysis

### Existing Logging Systems Found:
1. **github.com/op/go-logging** - Legacy structured logging (primary system)
   - Used in: `pkg/log/log.go`, main service files, nethack adapter
   - Pattern: `serviceLogger.Info()`, `serviceLogger.Fatalf()`
   
2. **log/slog** - Go native structured logging (partially adopted)
   - Used in: Session service components, auth service components, game service components
   - Pattern: `logger.Info()`, `logger.Error()` with context
   
3. **Standard log package** - Basic logging
   - Used in: Games infrastructure, config components
   - Pattern: `log.Printf()`, `log.Println()`

### Files Requiring Migration:

#### High Priority (Service Entry Points):
- `cmd/auth-service/main.go` - Mixed: serviceLogger (go-logging) + logger (slog)
- `cmd/session-service/main.go` - Uses custom log package wrapper
- `cmd/game-service/main.go` - Mixed: serviceLogger (go-logging) + logger (slog)

#### Medium Priority (Infrastructure):
- `pkg/log/log.go` - Core logging package using go-logging
- `internal/games/adapters/nethack_adapter.go` - Uses go-logging
- `internal/games/application/session_manager.go` - Uses standard log
- `internal/games/application/cleanup_service.go` - Uses standard log
- `internal/games/config/*.go` - Uses standard log package

#### Low Priority (Already using slog):
- All `internal/session/*` components
- All `internal/auth/*` components  
- Most `internal/games/infrastructure/grpc/*` components

## Migration Strategy

### Phase 1: Replace pkg/log Package
1. **Create new slog-based logging package** at `pkg/logging/slog.go`
2. **Implement structured configuration** compatible with existing YAML configs
3. **Add context-aware logging helpers** for common patterns
4. **Maintain backward compatibility** during transition

### Phase 2: Update Service Entry Points
1. **Replace go-logging imports** with new slog package
2. **Update service initialization** to use structured loggers
3. **Standardize log field naming** across services
4. **Add request/session context propagation**

### Phase 3: Infrastructure Components
1. **Update games infrastructure** to use slog
2. **Replace standard log calls** with structured equivalents
3. **Add contextual information** to all log entries
4. **Implement consistent error logging patterns**

### Phase 4: Configuration Integration
1. **Update config structs** to support slog configuration
2. **Remove go-logging dependencies** from config package
3. **Standardize log level handling** across all components

## Implementation Requirements

### New Logging Package Structure (`pkg/logging/slog.go`):
```go
package logging

import (
    "context"
    "log/slog"
    "os"
    "path/filepath"
)

// Config represents slog-compatible logging configuration
type Config struct {
    Level    string      `yaml:"level"`    // debug, info, warn, error
    Format   string      `yaml:"format"`   // json, text
    Output   string      `yaml:"output"`   // stdout, stderr, file
    File     *FileConfig `yaml:"file,omitempty"`
}

// NewLogger creates a configured slog.Logger
func NewLogger(serviceName string, config Config) *slog.Logger

// NewLoggerWithContext creates logger with default context fields
func NewLoggerWithContext(serviceName string, config Config, fields map[string]any) *slog.Logger

// ContextLogger returns logger with context values
func ContextLogger(ctx context.Context, logger *slog.Logger) *slog.Logger
```

### Standard Log Fields:
- `service`: Service name (auth-service, session-service, game-service)
- `component`: Component name (ssh-server, game-client, menu-handler)
- `session_id`: User session identifier (when available)
- `user_id`: User identifier (when available)
- `request_id`: Request identifier for tracing
- `game_id`: Game identifier (when in game context)

### Error Logging Pattern:
```go
// Before (go-logging)
serviceLogger.Errorf("Failed to start game: %v", err)

// After (slog)
logger.Error("Failed to start game",
    "error", err,
    "game_id", gameID,
    "user_id", userID)
```

### Context Propagation Pattern:
```go
// Service handlers should propagate context with logger
func (s *Service) HandleRequest(ctx context.Context, req *Request) error {
    logger := logging.ContextLogger(ctx, s.logger).With(
        "request_id", req.ID,
        "user_id", req.UserID,
    )
    
    logger.Info("Processing request", "type", req.Type)
    // ... rest of handler
}
```

## Migration Checklist

### Core Package Migration:
- [ ] Create `pkg/logging/slog.go` with slog-based implementation
- [ ] Implement file rotation using `lumberjack` or `slog` handlers
- [ ] Add JSON and text formatting options
- [ ] Create context helpers for request tracing
- [ ] Add service-specific logger factories

### Service Entry Point Migration:
- [ ] Update `cmd/auth-service/main.go`:
  - [ ] Replace `serviceLogger *logging.Logger` with `logger *slog.Logger`
  - [ ] Remove `github.com/op/go-logging` import
  - [ ] Use new logging package for initialization
  - [ ] Update all `serviceLogger.*` calls to `logger.*`
  
- [ ] Update `cmd/session-service/main.go`:
  - [ ] Replace custom log config with slog package
  - [ ] Standardize logger creation
  - [ ] Update service instantiation with slog logger
  
- [ ] Update `cmd/game-service/main.go`:
  - [ ] Replace mixed logging with unified slog approach
  - [ ] Update gRPC service initialization
  - [ ] Standardize error and info logging

### Infrastructure Migration:
- [ ] Update `internal/games/adapters/nethack_adapter.go`:
  - [ ] Replace `github.com/op/go-logging` with slog
  - [ ] Add game context to log entries
  - [ ] Use structured error logging
  
- [ ] Update `internal/games/application/*.go`:
  - [ ] Replace `log.*` calls with `logger.*`
  - [ ] Add context propagation
  - [ ] Include session/game IDs in logs
  
- [ ] Update `internal/games/config/*.go`:
  - [ ] Replace standard log with slog
  - [ ] Add validation context to errors
  - [ ] Structure configuration error messages

### Configuration Migration:
- [ ] Update configuration structs:
  - [ ] Replace `*log.Config` references with new logging config
  - [ ] Update YAML parsing for slog compatibility
  - [ ] Maintain backward compatibility for existing configs
  
- [ ] Update common configuration:
  - [ ] Standardize logging config across all services
  - [ ] Add service-specific log field defaults
  - [ ] Configure appropriate log levels per environment

### Testing and Validation:
- [ ] Update all test files using logging:
  - [ ] Replace test logger initialization
  - [ ] Add structured assertion helpers
  - [ ] Test context propagation in handlers
  
- [ ] Verify log output:
  - [ ] Check JSON format validity
  - [ ] Verify all required fields present
  - [ ] Test file rotation functionality
  - [ ] Validate log level filtering

### Cleanup:
- [ ] Remove unused imports:
  - [ ] `github.com/op/go-logging`
  - [ ] Standard `log` package where replaced
  
- [ ] Update documentation:
  - [ ] README logging section
  - [ ] Configuration examples
  - [ ] Developer guidelines
  
- [ ] Remove legacy code:
  - [ ] Old `pkg/log/log.go` (after full migration)
  - [ ] Unused configuration structs
  - [ ] Legacy logger factory functions

## Success Criteria

1. **Consistency**: All components use slog with standardized field names
2. **Observability**: Rich context in all log entries (service, component, user, session)
3. **Performance**: Structured logging with minimal allocation overhead
4. **Maintainability**: Single logging package with clear patterns
5. **Configuration**: Unified logging configuration across all services
6. **Backward Compatibility**: Existing YAML configs continue to work

## Example Before/After

### Before (Mixed):
```go
// auth-service/main.go
serviceLogger.Info("Starting DungeonGate Auth Service")
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

// games/application/session_manager.go  
log.Printf("Starting session for user %s", userID)

// games/adapters/nethack_adapter.go
logger.Infof("NetHack process started with PID %d", cmd.Process.Pid)
```

### After (Unified):
```go
// All services
logger := logging.NewLogger("auth-service", config.Logging)
logger.Info("Starting DungeonGate Auth Service")

// All components  
logger.Info("Starting session",
    "user_id", userID,
    "session_id", sessionID,
    "component", "session_manager")

// All adapters
logger.Info("NetHack process started",
    "pid", cmd.Process.Pid,
    "game_id", "nethack",
    "user_id", userID,
    "component", "nethack_adapter")
```

This migration will establish a robust, consistent, and observable logging foundation for the entire DungeonGate platform.


- debugging. Depending on what your trying to accomplish, it might be faster to regenerate a file than to fix it. i
- longer term. there is an intereting issue here. As claude is coding at the level of a junior to mid level developer. For me this mean that the barrier to entry for younger individuals will be high to clear as you have to out complete llms for debugging and code generation. I think we might be seeing the commidiification of code here where writing software for projects will trend to the cost of maintancing a reference code base and infra to utilize the llms. what the issue is, for me, is that subject matter experts no longer have a pipeline to learn their trade craft.

- [parrellization. use git worktrees ](https://docs.anthropic.com/en/docs/claude-code/common-workflows#run-parallel-claude-code-sessions-with-git-worktrees). 