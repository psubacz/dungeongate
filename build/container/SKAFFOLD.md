# DungeonGate Skaffold Configuration

This document explains how to use Skaffold for Kubernetes development with DungeonGate.

## Prerequisites

### Required Tools
```bash
# Install Skaffold
curl -Lo skaffold https://storage.googleapis.com/skaffold/releases/latest/skaffold-linux-amd64
sudo install skaffold /usr/local/bin/

# Verify installation
skaffold version
```

### Kubernetes Cluster
You need a running Kubernetes cluster. Options:
- **minikube**: `minikube start`
- **kind**: `kind create cluster`
- **Docker Desktop**: Enable Kubernetes
- **k3s**: Lightweight production option
- **Cloud providers**: GKE, EKS, AKS

### Container Runtime
- **Docker** or **Podman** installed and running
- Access to container registry (local or remote)

## Quick Start

### 1. Development Mode (Recommended)
```bash
# Start development with hot-reload
make skaffold-dev

# Or manually
cd build/container
skaffold dev --port-forward
```

This will:
- ✅ Build images for all services
- ✅ Deploy to Kubernetes
- ✅ Set up port forwarding
- ✅ Watch for file changes and auto-rebuild
- ✅ Stream logs from all pods

### 2. Local Docker Compose Development
```bash
# Use docker-compose with Skaffold
make skaffold-dev-local

# Or manually
cd build/container
skaffold dev --profile local --port-forward
```

### 3. Production Deployment
```bash
# Deploy to production namespace
make skaffold-run-prod

# Or manually
cd build/container
skaffold run --profile prod
```

## Skaffold Profiles

### Default Profile (Microservices)
- **Target**: Individual production containers
- **Services**: session-service, auth-service, game-service
- **Namespace**: `default`
- **Storage**: Persistent volumes

### Development Profile (`dev`)
- **Target**: Development image with full toolchain
- **Services**: All-in-one development container
- **Features**: Live reload, debug tools
- **Storage**: Host path mounts for live development

### Local Profile (`local`)
- **Target**: Uses docker-compose
- **Services**: Run locally with docker-compose
- **Benefits**: Faster feedback, local debugging

### Production Profile (`prod`)
- **Target**: Production-hardened deployment
- **Namespace**: `dungeongate-prod`
- **Features**: Resource limits, PostgreSQL, monitoring
- **Security**: Network policies, resource quotas

## File Structure

```
build/container/
├── skaffold.yaml                 # Main Skaffold configuration
├── k8s/                         # Kubernetes manifests
│   ├── auth-service.yaml        # Auth service deployment
│   ├── game-service.yaml        # Game service deployment
│   ├── session-service.yaml     # Session service deployment
│   ├── configmap.yaml           # Configuration
│   ├── secrets.yaml             # Secrets (development only)
│   ├── ingress.yaml             # Ingress controller
│   ├── dev/
│   │   └── all-in-one.yaml     # Development deployment
│   └── prod/
│       ├── namespace.yaml       # Production namespace
│       └── database.yaml        # PostgreSQL StatefulSet
└── SKAFFOLD.md                 # This documentation
```

## Port Forwarding

When using `skaffold dev --port-forward`, these ports are automatically forwarded:

### Default Profile
- `2222` → SSH (session service)
- `8081` → Auth HTTP API
- `8082` → Auth gRPC API
- `8083` → Session HTTP API
- `8085` → Game HTTP API
- `9090` → Game metrics
- `9091` → Auth metrics
- `9093` → Session gRPC API
- `50051` → Game gRPC API

### Development Profile
- `2222` → SSH
- `8081` → All HTTP APIs

## Usage Examples

### Development Workflow
```bash
# 1. Start development mode
make skaffold-dev

# 2. Edit Go code in your editor
# Files are automatically synced to containers

# 3. Changes trigger automatic rebuilds
# Watch the terminal for rebuild progress

# 4. Test your changes
ssh -p 2222 localhost
```

### Building Only
```bash
# Build all images without deploying
make skaffold-build

# Build specific profile
cd build/container
skaffold build --profile prod
```

### Debugging
```bash
# Start in debug mode
make skaffold-debug

# This enables debugging capabilities
# Connect your IDE debugger to the forwarded ports
```

### Cleanup
```bash
# Delete all Skaffold deployments
make skaffold-delete

# Or manually
cd build/container
skaffold delete
```

## Configuration

### Environment Variables
```bash
# Set image registry
export REGISTRY=your-registry.com

# Set image tag
export VERSION=v1.0.0

# Use with Skaffold
skaffold dev --default-repo=$REGISTRY
```

### File Sync
Skaffold automatically syncs these file changes:
- `**/*.go` → Triggers rebuild
- `cmd/session-service/**/*` → Session service sync
- `cmd/auth-service/**/*` → Auth service sync
- `cmd/game-service/**/*` → Game service sync
- `configs/**/*` → Config sync

### Resource Limits
Default resource limits per service:
```yaml
requests:
  memory: "64Mi"   # Auth service
  memory: "128Mi"  # Game/Session services
  cpu: "100m"

limits:
  memory: "128Mi"  # Auth service
  memory: "256Mi"  # Game/Session services
  cpu: "200m"      # Auth service
  cpu: "300m"      # Game/Session services
```

## Troubleshooting

### Build Issues
```bash
# Check Skaffold status
skaffold diagnose

# Verbose output
skaffold dev -v info

# Very verbose
skaffold dev -v debug
```

### Kubernetes Issues
```bash
# Check pod status
kubectl get pods

# Check logs
kubectl logs -l app=dungeongate

# Describe problematic pods
kubectl describe pod <pod-name>

# Check events
kubectl get events --sort-by=.metadata.creationTimestamp
```

### Port Forwarding Issues
```bash
# Manual port forwarding
kubectl port-forward svc/dungeongate-session 2222:2222

# Check service endpoints
kubectl get endpoints

# Check service configuration
kubectl describe svc dungeongate-session
```

### File Sync Issues
```bash
# Check if files are syncing
skaffold dev -v debug | grep sync

# Force rebuild
# Make a dummy change to trigger rebuild
```

## Advanced Usage

### Custom Kubernetes Context
```bash
# Use specific context
skaffold dev --kube-context=minikube

# Use specific namespace
skaffold dev --namespace=my-namespace
```

### Registry Configuration
```bash
# Push to registry
skaffold dev --default-repo=gcr.io/my-project

# Use insecure registry
skaffold dev --insecure-registry=localhost:5000
```

### Multiple Environments
```bash
# Development
skaffold dev --profile dev

# Staging
skaffold run --profile staging

# Production
skaffold run --profile prod
```

### GitOps Integration
```bash
# Render manifests for GitOps
skaffold render --profile prod > k8s-manifests.yaml

# Apply via GitOps tool
kubectl apply -f k8s-manifests.yaml
```

## Monitoring

### Logs
```bash
# Stream all logs
skaffold dev

# Follow specific service
kubectl logs -f -l service=auth

# Previous container logs
kubectl logs -p <pod-name>
```

### Metrics
Access metrics endpoints through port forwarding:
- Auth metrics: `http://localhost:9091/metrics`
- Game metrics: `http://localhost:9090/metrics`
- Session metrics: `http://localhost:8085/metrics`

### Health Checks
```bash
# Check health endpoints
curl http://localhost:8081/health
curl http://localhost:8083/health
curl http://localhost:8085/health
```

## Integration with CI/CD

### GitHub Actions Example
```yaml
- name: Deploy with Skaffold
  run: |
    skaffold run --default-repo=${{ secrets.REGISTRY }}
  env:
    KUBECONFIG: ${{ secrets.KUBECONFIG }}
```

### Automated Testing
```bash
# Run tests before deployment
skaffold test

# Custom test command
skaffold dev --pre-test="make test"
```

This Skaffold configuration provides a complete Kubernetes development workflow for DungeonGate with hot reloading, debugging, and production deployment capabilities.