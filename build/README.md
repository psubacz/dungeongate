# DungeonGate Build Directory

This directory contains all build artifacts for the DungeonGate project.

## Structure

```
build/
├── README.md                          # This file
├── container/                         # Container build artifacts
│   ├── Containerfile                  # Multi-stage build for x86_64
│   ├── Containerfile.arm              # Multi-stage build for ARM64
│   ├── docker-compose.yml             # Service orchestration
│   └── CONTAINERS.md                  # Container documentation
├── dungeongate-session-service        # Compiled session service binary
├── dungeongate-auth-service           # Compiled auth service binary
└── dungeongate-game-service           # Compiled game service binary
```

## Usage

### Native Binaries
The compiled binaries in this directory can be run directly:
```bash
# From project root
./build/dungeongate-auth-service -config=configs/auth-service.yaml
./build/dungeongate-game-service -config=configs/game-service.yaml  
./build/dungeongate-session-service -config=configs/session-service.yaml
```

### Container Builds
All container-related files are in the `container/` subdirectory:

```bash
# Extract binaries using containers (auto-detects ARM64/x86_64)
make container-binaries

# Build development image
make container-development

# Build production images
make container-production

# Run with podman/docker
make container-run-dev
make container-run-all

# Use docker-compose
make compose-up
```

## Architecture Detection

The build system automatically detects your architecture:
- **ARM64** (Apple Silicon): Uses `Containerfile.arm` with `TARGETARCH=arm64`
- **x86_64** (Intel): Uses `Containerfile` with `TARGETARCH=amd64`

## Build Methods

1. **Native Go builds**: `make build-all` → `bin/` directory
2. **Container extraction**: `make container-binaries` → `build/` directory  
3. **Container images**: `make container-production` → Local container registry

See `container/CONTAINERS.md` for detailed container documentation.