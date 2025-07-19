# DungeonGate Container Builds

This document explains the containerized build system for DungeonGate services.

## Quick Start

### Extract Binaries to ./build
```bash
# From project root directory
# Using make
make container-binaries

# Using script directly  
./scripts/build-container.sh binaries
```

### Build Development Image
```bash
# Full development environment with tools
make container-development

# Run development container
docker-compose --profile dev up
```

### Build Production Images
```bash
# Build all production images
make container-production

# Build individual services
make container-session
make container-auth
make container-game

# Build everything
make container-all
```

## Multi-Stage Containerfile

The `Containerfile` provides several build targets:

### Builder Stage
- **Target**: `builder`
- **Purpose**: Compiles all three services
- **Output**: Binaries in `/app/build/`

### Development Image
- **Target**: `development`
- **Base**: `golang:1.24-alpine`
- **Features**: Full Go toolchain, development tools (air, golangci-lint), debugging tools
- **Use Case**: Local development, debugging, testing
- **Size**: ~500MB (includes full toolchain)

### Production Images
- **Base**: `alpine:latest` (minimal)
- **Security**: Non-root user, minimal attack surface
- **Size**: ~20-30MB per service

#### Individual Service Images
- **session-service**: SSH server and terminal session management
- **auth-service**: Authentication and authorization
- **game-service**: Game management and orchestration

#### All-in-One Production
- **Target**: `production`
- **Purpose**: Single container with all services
- **Use Case**: Simple deployments, testing
- **Startup**: Automatic service management with health monitoring

### Export Stage
- **Target**: `export`
- **Purpose**: Extract compiled binaries to host system
- **Output**: Binaries in `./build/` directory

## Usage Examples

### 1. Extract Binaries Only
```bash
./scripts/build-container.sh binaries
ls -la build/
```

### 2. Development Environment
```bash
# Build dev image
./scripts/build-container.sh development

# Run with live reload
docker run -it --rm \
  -v $(pwd):/app \
  -p 2222:2222 \
  -p 8081-8085:8081-8085 \
  localhost/dungeongate:dev-latest
```

### 3. Production Deployment
```bash
# Build production images
./scripts/build-container.sh production

# Deploy with compose
docker-compose up -d

# Deploy individual services
docker run -d --name auth \
  -p 8081:8081 -p 8082:8082 \
  localhost/dungeongate-auth:latest

docker run -d --name game \
  -p 8085:8085 -p 50051:50051 \
  localhost/dungeongate-game:latest

docker run -d --name session \
  -p 2222:2222 -p 8083:8083 \
  --link auth --link game \
  localhost/dungeongate-session:latest
```

### 4. All-in-One Simple Deployment
```bash
# Build and run single container
./scripts/build-container.sh production
docker-compose --profile simple up
```

## Docker Compose Profiles

### Default Profile (Microservices)
```bash
# Individual containers for each service
docker-compose up
```
- Separate containers for auth, game, session services
- Full service isolation
- Recommended for production

### Development Profile
```bash
docker-compose --profile dev up
```
- Development image with full toolchain
- Live code reloading with air
- Debug tools available

### Simple Profile (All-in-One)
```bash
docker-compose --profile simple up
```
- Single container with all services
- Good for testing and simple deployments
- Shared resources

### Monitoring Profile
```bash
docker-compose --profile monitoring up
```
- Includes Prometheus for metrics
- Future: Grafana dashboards

### Database Profile
```bash
docker-compose --profile postgres up
```
- PostgreSQL database container
- For production database setup

## Build Script Options

```bash
./scripts/build-container.sh [TARGET]
```

**Targets:**
- `binaries` - Extract compiled binaries (default)
- `development` - Build dev image with toolchain
- `production` - Build all production images
- `session` - Build session service only
- `auth` - Build auth service only
- `game` - Build game service only
- `all` - Build everything

**Environment Variables:**
- `VERSION` - Image version tag (default: git describe)
- `REGISTRY` - Container registry (default: localhost)

**Examples:**
```bash
# Custom version
VERSION=v1.2.3 ./scripts/build-container.sh production

# Push to registry
REGISTRY=myregistry.com ./scripts/build-container.sh all
```

## Image Naming Convention

- **Development**: `localhost/dungeongate:dev-VERSION`
- **Production Services**: `localhost/dungeongate-{service}:VERSION`
- **All-in-One**: `localhost/dungeongate:VERSION`

## Security Features

- Non-root user execution
- Minimal base images (Alpine Linux)
- No unnecessary packages
- Health checks included
- Read-only configuration mounts

## File Structure

```
├── build/
│   ├── container/             # Container build artifacts
│   │   ├── Containerfile      # Multi-stage build (x86_64)
│   │   ├── Containerfile.arm  # Multi-stage build (ARM64)
│   │   ├── docker-compose.yml # Service orchestration
│   │   ├── skaffold.yaml      # Kubernetes development
│   │   ├── k8s/               # Kubernetes manifests
│   │   │   ├── *.yaml         # Service deployments
│   │   │   ├── dev/           # Development manifests
│   │   │   └── prod/          # Production manifests
│   │   ├── CONTAINERS.md      # This documentation
│   │   └── SKAFFOLD.md        # Kubernetes development guide
│   ├── dungeongate-session-service  # Extracted binaries
│   ├── dungeongate-auth-service
│   └── dungeongate-game-service
├── scripts/
│   ├── build-container.sh     # Build script (auto-detects arch)
│   └── start-all-services.sh  # All-in-one startup
└── configs/                   # Service configurations
```

## Make Targets

```bash
# Container builds (auto-detects ARM64 vs x86_64)
make container-binaries      # Extract to ./build
make container-development   # Dev image
make container-production    # All production images
make container-session       # Session service only
make container-auth         # Auth service only
make container-game         # Game service only
make container-all          # Everything

# Container run (uses podman or docker)
make container-run-dev      # Run development container
make container-run-session  # Run session service
make container-run-auth     # Run auth service
make container-run-game     # Run game service
make container-run-all      # Run all-in-one container
make container-stop         # Stop all containers

# Docker Compose (from build/container directory)
make compose-up             # Start microservices
make compose-up-dev         # Start development
make compose-up-simple      # Start all-in-one
make compose-down           # Stop and remove
make compose-logs           # View logs

# Skaffold (Kubernetes development)
make skaffold-dev           # Development with hot-reload
make skaffold-dev-local     # Local development with docker-compose
make skaffold-run           # Deploy to Kubernetes
make skaffold-run-prod      # Deploy to production
make skaffold-delete        # Clean up deployments
make skaffold-debug         # Debug mode

# Legacy (deprecated)
make docker-build-all       # Use container-production instead
```

## Troubleshooting

### Build Issues
```bash
# Check container runtime
podman --version
# or
docker --version

# Clean build
docker system prune -f
./scripts/build-container.sh binaries
```

### Runtime Issues
```bash
# Check logs
docker-compose logs auth-service
docker-compose logs game-service
docker-compose logs session-service

# Debug development container
docker run -it --rm localhost/dungeongate:dev-latest sh
```

### Permission Issues
```bash
# Fix build directory permissions
sudo chown -R $USER:$USER build/
chmod +x build/dungeongate-*
```