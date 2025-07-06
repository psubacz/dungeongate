# DungeonGate Configuration Guide

This document covers the complete configuration system for DungeonGate, including database setup, session service configuration, and deployment options.

## Configuration Overview

DungeonGate uses a YAML-based configuration system with support for:
- Multiple database modes (embedded SQLite, external PostgreSQL/MySQL)
- Environment variable expansion
- Comprehensive validation
- Service-specific configurations

## Configuration File Structure

```yaml
# Database configuration
database:
  mode: "embedded|external"
  embedded: { ... }    # SQLite configuration
  external: { ... }    # PostgreSQL/MySQL configuration
  settings: { ... }    # Common database settings

# Session service configuration  
session_service:
  server: { ... }      # HTTP/gRPC server settings
  ssh: { ... }         # SSH server configuration
  storage: { ... }     # File storage paths
  logging: { ... }     # Logging configuration
  services: { ... }    # External service endpoints
```

## Database Configuration

### Embedded Mode (SQLite)

Perfect for development, testing, and single-user deployments:

```yaml
database:
  mode: "embedded"
  embedded:
    type: "sqlite"
    path: "./data/dungeongate.db"
    migration_path: "./migrations"
    backup_enabled: true
    backup_interval: "24h"
    backup_retention: 7
    wal_mode: true
    cache:
      enabled: true
      size: 64              # MB
      ttl: "1h"
      type: "memory"
  settings:
    log_queries: false
    timeout: "30s"
    retry_attempts: 3
    retry_delay: "1s"
    health_check: true
    health_interval: "30s"
    metrics_enabled: true
```

**Key Features:**
- **WAL Mode:** Write-Ahead Logging for better concurrency
- **Automatic Backups:** Configurable backup schedule
- **Memory Caching:** Improved read performance
- **Health Monitoring:** Connection status tracking

### External Mode (PostgreSQL/MySQL)

For production deployments with high availability:

```yaml
database:
  mode: "external"
  external:
    type: "postgresql"           # or "mysql"
    
    # Writer database (primary)
    writer_endpoint: "db-primary.internal:5432"
    
    # Reader database (replica)
    reader_use_writer: false
    reader_endpoint: "db-replica.internal:5432"
    
    # Connection details
    database: "dungeongate"
    username: "dungeongate"
    password: "${DB_PASSWORD}"
    ssl_mode: "require"
    
    # Connection pooling
    max_connections: 100
    max_idle_conns: 10
    conn_max_lifetime: "1h"
    
    # Reader-specific pooling
    reader_max_connections: 50
    reader_max_idle_conns: 5
    
    # Schema and migrations
    migration_path: "./migrations"
    schema: "public"
    
    # Additional connection options
    options:
      application_name: "dungeongate"
      connect_timeout: "10"
    
    # High availability
    failover:
      enabled: true
      health_check_interval: "30s"
      failover_timeout: "10s"
      retry_interval: "5s"
      max_retries: 3
      reader_to_writer_fallback: true
```

**Production Features:**
- **Read/Write Separation:** Automatic query routing
- **Connection Pooling:** Optimized resource usage
- **Automatic Failover:** High availability support
- **Health Monitoring:** Continuous connection checks
- **SSL/TLS Support:** Secure connections

## Session Service Configuration

### Server Settings

```yaml
session_service:
  server:
    port: 8083              # HTTP API port
    grpc_port: 9093         # gRPC service port
    host: "0.0.0.0"        # Bind address
    timeout: "60s"          # Request timeout
    max_connections: 1000   # Concurrent connections
```

### SSH Server Configuration

```yaml
session_service:
  ssh:
    enabled: true
    port: 22                # SSH port (or 2222 for non-privileged)
    host: "0.0.0.0"        # Bind address
    host_key_path: "/etc/ssh/ssh_host_rsa_key"
    banner: "Welcome to DungeonGate!\r\n"
    max_sessions: 100
    session_timeout: "4h"
    idle_timeout: "30m"
    
    # Authentication methods
    auth:
      password_auth: true
      public_key_auth: false
      allow_anonymous: true
    
    # Terminal settings
    terminal:
      default_size: "80x24"
      max_size: "200x50"
      supported_terminals:
        - "xterm"
        - "xterm-256color"
        - "screen"
        - "tmux"
        - "vt100"
```

### Session Management

```yaml
session_service:
  session_management:
    terminal:
      default_size: "80x24"
      max_size: "200x50"
      encoding: "utf-8"
    
    timeouts:
      idle_timeout: "30m"
      max_session_duration: "4h"
      cleanup_interval: "5m"
    
    # Session recording
    ttyrec:
      enabled: true
      compression: "gzip"
      directory: "/var/lib/dungeongate/ttyrec"
      max_file_size: "100MB"
      retention_days: 30
    
    # Session monitoring
    monitoring:
      enabled: true
      port: 8085
    
    # Session spectating
    spectating:
      enabled: true
      max_spectators_per_session: 10
      spectator_timeout: "2h"
```

### Storage Configuration

```yaml
session_service:
  storage:
    ttyrec_path: "/var/lib/dungeongate/ttyrec"
    temp_path: "/tmp/sessions"
```

### Service Discovery

```yaml
session_service:
  services:
    auth_service: "auth-service:9090"
    user_service: "user-service:9091"
    game_service: "game-service:9092"
```

### Logging Configuration

```yaml
session_service:
  logging:
    level: "info"           # debug, info, warn, error
    format: "json"          # json, text
    output: "stdout"        # stdout, stderr, file path
```

### Security Configuration

```yaml
session_service:
  security:
    rate_limiting:
      enabled: true
      max_connections_per_ip: 10
      connection_window: "1m"
    
    brute_force_protection:
      enabled: true
      max_failed_attempts: 5
      lockout_duration: "15m"
    
    session_security:
      require_encryption: true
      session_token_length: 32
      secure_random: true
```

## Environment Variables

Configuration files support environment variable expansion:

```yaml
database:
  external:
    password: "${DB_PASSWORD}"
    host: "${DB_HOST:-localhost}"
    port: "${DB_PORT:-5432}"
```

### Common Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_PASSWORD` | Database password | - |
| `DB_HOST` | Database host | localhost |
| `DB_PORT` | Database port | 5432/3306 |
| `SSH_HOST_KEY_PATH` | SSH host key path | /etc/ssh/ssh_host_rsa_key |
| `LOG_LEVEL` | Logging level | info |
| `SESSION_TIMEOUT` | Session timeout | 4h |

## Configuration Examples

### Development Configuration

```yaml
# configs/development.yaml
database:
  mode: "embedded"
  embedded:
    type: "sqlite"
    path: "./dev-data/dungeongate.db"
    wal_mode: true

session_service:
  server:
    port: 8083
    host: "localhost"
  ssh:
    enabled: true
    port: 2222  # Non-privileged port
    host: "localhost"
    allow_anonymous: true
  logging:
    level: "debug"
    format: "text"
```

### Production Configuration

```yaml
# configs/production.yaml
database:
  mode: "external"
  external:
    type: "postgresql"
    writer_endpoint: "${DB_PRIMARY_HOST}:5432"
    reader_endpoint: "${DB_REPLICA_HOST}:5432"
    database: "${DB_NAME}"
    username: "${DB_USER}"
    password: "${DB_PASSWORD}"
    ssl_mode: "require"
    max_connections: 200
    failover:
      enabled: true

session_service:
  server:
    port: 8083
    host: "0.0.0.0"
  ssh:
    enabled: true
    port: 22
    host: "0.0.0.0"
    host_key_path: "/etc/ssh/ssh_host_rsa_key"
  logging:
    level: "info"
    format: "json"
  security:
    rate_limiting:
      enabled: true
      max_connections_per_ip: 10
```

### Docker Configuration

```yaml
# configs/docker.yaml
database:
  mode: "external"
  external:
    type: "postgresql"
    writer_endpoint: "postgres:5432"
    reader_use_writer: true
    database: "dungeongate"
    username: "dungeongate"
    password: "${POSTGRES_PASSWORD}"
    ssl_mode: "disable"

session_service:
  server:
    host: "0.0.0.0"
  ssh:
    host: "0.0.0.0"
    host_key_path: "/etc/ssh/ssh_host_rsa_key"
  storage:
    ttyrec_path: "/data/ttyrec"
    temp_path: "/tmp"
```

## Configuration Validation

The system performs comprehensive validation:

1. **Syntax Validation:** YAML parsing and structure
2. **Type Validation:** Field types and constraints
3. **Logic Validation:** Cross-field dependencies
4. **Runtime Validation:** Connection tests and path verification

### Validation Examples

```bash
# Validate configuration only
go run test-build.go -config myconfig.yaml -validate-only

# Test with connection verification
go run test-build.go -config myconfig.yaml -test-db
```

## Migration Path

### From dgamelaunch

If migrating from traditional dgamelaunch:

1. **Database:** Export existing user data to SQLite or PostgreSQL
2. **Configuration:** Convert dgamelaunch.conf to YAML format
3. **Paths:** Update file paths for recordings and temporary files
4. **Testing:** Use embedded mode for initial testing

### Configuration Migration Script

```bash
# scripts/migrate-config.sh will help convert existing configurations
./scripts/migrate-config.sh /path/to/dgamelaunch.conf > configs/migrated.yaml
```

## Best Practices

### Security
- Use environment variables for sensitive data
- Enable SSL/TLS for external databases
- Configure appropriate rate limiting
- Regularly rotate SSH host keys

### Performance
- Use read replicas for high-read workloads
- Configure appropriate connection pools
- Enable query logging only during debugging
- Monitor connection metrics

### Reliability
- Enable health checks and monitoring
- Configure automatic failover
- Set appropriate timeouts
- Plan for graceful degradation

## Troubleshooting

See `TESTING.md` for common issues and solutions.

## Next Steps

1. Choose your deployment mode (embedded vs external)
2. Create your configuration file
3. Test with `test-build.go`
4. Deploy and monitor

For testing instructions, see `TESTING.md`.
