# DungeonGate Development Configurations

This directory contains separate configuration files for each service in the DungeonGate microservices architecture.

## Configuration Files

### Service-Specific Configurations

| Service | Config File | Default Ports | Description |
|---------|-------------|---------------|-------------|
| **Session Service** | `session-service.yaml` | HTTP: 8083, gRPC: 9093, SSH: 2222 | Main SSH gateway service |
| **Auth Service** | `auth-service.yaml` | HTTP: 8081, gRPC: 8082 | Authentication and authorization |
| **Game Service** | `game-service.yaml` | HTTP: 8085, gRPC: 50051 | Game management and execution |
| **User Service** | `user-service.yaml` | HTTP: 8084, gRPC: 9084 | User registration and management |

### Configuration Features

All configuration files are extensively annotated with:
- **Detailed explanations** of every configuration option
- **Production vs development** guidance
- **Security considerations** and best practices
- **Port assignments** and service communication
- **Database configuration** options (SQLite vs PostgreSQL)
- **Performance tuning** parameters

## Running Services

### Individual Services
```bash
# Run individual services
make run-session    # Session service only
make run-auth       # Auth service only  
make run-game       # Game service only
make run-user       # User service only
```

### All Services
```bash
# Run all services together
make run-all        # Starts all services with proper dependencies
```

## Port Assignments

### Development Ports
- **Session Service**: 
  - HTTP API: 8083
  - gRPC API: 9093
  - SSH Server: 2222
  - Metrics: 8085
- **Auth Service**:
  - HTTP API: 8081
  - gRPC API: 8082
- **Game Service**:
  - HTTP API: 8085
  - gRPC API: 50051
  - Metrics: 8086
- **User Service**:
  - HTTP API: 8084
  - gRPC API: 9084

### Service Communication
- Auth Service gRPC: `localhost:8082`
- Game Service gRPC: `localhost:50051`
- User Service HTTP: `localhost:8084`

## Configuration Features

### Shared Features
- **Database**: All services share the same SQLite database in development
- **Logging**: Debug level logging to stdout
- **Health Checks**: All services provide `/health` endpoints
- **Configurable Ports**: All ports can be overridden in configuration

### Service-Specific Features

#### Session Service
- SSH server configuration
- Terminal and TTY recording settings
- Game launcher configuration
- Menu and banner customization
- Spectating system

#### Auth Service
- JWT token configuration
- Password policies
- Rate limiting and brute force protection
- Session management

#### Game Service
- Game binary paths and execution
- Container/process isolation
- Resource limits
- Game-specific settings

#### User Service
- User registration workflows
- Validation rules
- Email configuration
- Two-factor authentication

## Migration from local.yaml

The original `local.yaml` has been broken out into service-specific configurations:

1. **Session Service**: Contains SSH, terminal, games, and session management config
2. **Auth Service**: Contains authentication, JWT, and security config  
3. **Game Service**: Contains game execution, paths, and resource config
4. **User Service**: Contains user management, registration, and validation config

Each service now uses its specific configuration file by default, but can be overridden with the `-config` flag.

## Development Workflow

1. **Start all services**: `make run-all`
2. **Connect via SSH**: `ssh -p 2222 localhost`
3. **Check health**: Visit `http://localhost:8081/health`, etc.
4. **View metrics**: Visit `http://localhost:8085/metrics` (session), `http://localhost:8086/metrics` (game)

## Production Considerations

For production deployments:
- Use separate configuration files per environment
- Override ports via environment variables or config files
- Use external PostgreSQL instead of SQLite
- Enable security features (rate limiting, brute force protection)
- Configure proper logging and monitoring