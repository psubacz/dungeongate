#!/bin/bash

# dev-setup.sh - Development environment setup for DungeonGate
# Sets up directories, generates keys, and prepares development environment

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${GREEN}✅${NC} $1"
}

print_error() {
    echo -e "${RED}❌${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠️${NC} $1"
}

print_info() {
    echo -e "${BLUE}ℹ️${NC} $1"
}

print_header() {
    echo ""
    echo -e "${BLUE}================================${NC}"
    echo -e "${BLUE} $1${NC}"
    echo -e "${BLUE}================================${NC}"
}

# Check if we're in the right directory
if [[ ! -f "go.mod" ]] || ! grep -q "github.com/dungeongate" go.mod; then
    print_error "This script must be run from the DungeonGate project root directory"
    exit 1
fi

print_header "DungeonGate Development Setup"

# 1. Create directory structure
print_info "Creating directory structure..."
mkdir -p test-data/sqlite/ttyrec
mkdir -p test-data/sqlite/tmp
mkdir -p test-data/ssh_keys
mkdir -p data/sqlite
mkdir -p data/logs
mkdir -p data/ttyrec
mkdir -p data/tmp
mkdir -p migrations
mkdir -p banners
mkdir -p configs/development
mkdir -p configs/testing
mkdir -p configs/production
print_status "Directory structure created"

# 2. Generate SSH host key for testing
print_info "Generating SSH host key for testing..."
if [[ ! -f "test-data/ssh_keys/test_host_key" ]]; then
    ssh-keygen -t rsa -b 2048 -f test-data/ssh_keys/test_host_key -N "" -C "dungeongate-test"
    chmod 600 test-data/ssh_keys/test_host_key
    chmod 644 test-data/ssh_keys/test_host_key.pub
    print_status "SSH test key generated: test-data/ssh_keys/test_host_key"
else
    print_status "SSH test key already exists"
fi

# 3. Create development configuration if it doesn't exist
print_info "Creating development configuration..."
if [[ ! -f "configs/development/local.yaml" ]]; then
    cat > configs/development/local.yaml << 'EOF'
# Development configuration for local testing
database:
  mode: "embedded"
  embedded:
    type: "sqlite"
    path: "./data/sqlite/dungeongate-dev.db"
    migration_path: "./migrations"
    backup_enabled: false
    wal_mode: true
    cache:
      enabled: true
      size: 32
      ttl: "30m"
      type: "memory"
  settings:
    log_queries: true
    timeout: "30s"
    retry_attempts: 2
    health_check: true
    metrics_enabled: true

session_service:
  server:
    port: 8083
    grpc_port: 9093
    host: "localhost"
    timeout: "30s"
    max_connections: 100

  ssh:
    enabled: true
    port: 2222  # Non-privileged port for development
    host: "localhost"
    host_key_path: "./test-data/ssh_keys/test_host_key"
    banner: "Welcome to DungeonGate Development Server!\r\n"
    max_sessions: 10
    session_timeout: "1h"
    idle_timeout: "15m"
    auth:
      password_auth: true
      public_key_auth: false
      allow_anonymous: true
    terminal:
      default_size: "80x24"
      max_size: "120x40"
      supported_terminals:
        - "xterm"
        - "xterm-256color"
        - "screen"

  session_management:
    terminal:
      default_size: "80x24"
      max_size: "120x40"
      encoding: "utf-8"
    timeouts:
      idle_timeout: "15m"
      max_session_duration: "1h"
      cleanup_interval: "1m"
    ttyrec:
      enabled: true
      compression: "gzip"
      directory: "./data/ttyrec"
      max_file_size: "10MB"
      retention_days: 7
    spectating:
      enabled: true
      max_spectators_per_session: 3
      spectator_timeout: "30m"

  services:
    auth_service: "localhost:9090"
    user_service: "localhost:9091"
    game_service: "localhost:9092"

  storage:
    ttyrec_path: "./data/ttyrec"
    temp_path: "./data/tmp"

  logging:
    level: "debug"
    format: "text"
    output: "stdout"

  security:
    rate_limiting:
      enabled: false  # Disabled for development
    brute_force_protection:
      enabled: false  # Disabled for development
    session_security:
      require_encryption: false
      session_token_length: 16
EOF
    print_status "Development configuration created: configs/development/local.yaml"
else
    print_status "Development configuration already exists"
fi

# 4. Create basic banner files
print_info "Creating banner files..."
if [[ ! -f "banners/main_anon.txt" ]]; then
    cat > banners/main_anon.txt << 'EOF'
    ____                                   ____       _
   |  _ \ _   _ _ __   __ _  ___  ___  _ __ / ___| __ _| |_ ___
   | | | | | | | '_ \ / _` |/ _ \/ _ \| '_ \\___ \/ _` | __/ _ \
   | |_| | |_| | | | | (_| |  __/ (_) | | | |___) | (_| | ||  __/
   |____/ \__,_|_| |_|\__, |\___|\___|_| |_|____/ \__,_|\__\___|
                      |___/

   Welcome, Anonymous Player!
   
   Choose your adventure:
EOF
    print_status "Anonymous banner created"
else
    print_status "Anonymous banner already exists"
fi

if [[ ! -f "banners/main_user.txt" ]]; then
    cat > banners/main_user.txt << 'EOF'
    ____                                   ____       _
   |  _ \ _   _ _ __   __ _  ___  ___  _ __ / ___| __ _| |_ ___
   | | | | | | | '_ \ / _` |/ _ \/ _ \| '_ \\___ \/ _` | __/ _ \
   | |_| | |_| | | | | (_| |  __/ (_) | | | |___) | (_| | ||  __/
   |____/ \__,_|_| |_|\__, |\___|\___|_| |_|____/ \__,_|\__\___|
                      |___/

   Welcome back, Player!
   
   Your adventures await:
EOF
    print_status "User banner created"
else
    print_status "User banner already exists"
fi

# 5. Download Go dependencies
print_info "Downloading Go dependencies..."
if go mod tidy; then
    print_status "Go dependencies updated"
else
    print_warning "Failed to update Go dependencies (continuing anyway)"
fi

# 6. Test the configuration
print_info "Testing development configuration..."
if go run test-build.go -config configs/development/local.yaml -validate-only; then
    print_status "Development configuration is valid"
else
    print_error "Development configuration validation failed"
    print_info "You may need to fix the configuration manually"
fi

# 7. Create .env template
print_info "Creating environment template..."
if [[ ! -f ".env.example" ]]; then
    cat > .env.example << 'EOF'
# DungeonGate Environment Variables
# Copy this file to .env and customize for your environment

# Database settings (for external databases)
DB_HOST=localhost
DB_PORT=5432
DB_NAME=dungeongate
DB_USER=dungeongate
DB_PASSWORD=your_password_here

# SSH settings
SSH_HOST_KEY_PATH=./test-data/ssh_keys/test_host_key

# Logging
LOG_LEVEL=debug

# Service endpoints
AUTH_SERVICE_ENDPOINT=localhost:9090
USER_SERVICE_ENDPOINT=localhost:9091
GAME_SERVICE_ENDPOINT=localhost:9092

# Session settings
SESSION_TIMEOUT=1h
IDLE_TIMEOUT=15m
MAX_SESSIONS=10
EOF
    print_status "Environment template created: .env.example"
else
    print_status "Environment template already exists"
fi

# 8. Create gitignore entries for development files
print_info "Updating .gitignore..."
if ! grep -q "# Development files" .gitignore 2>/dev/null; then
    cat >> .gitignore << 'EOF'

# Development files
test-data/
data/
.env
*.log

# SSH keys
ssh_host_*
*.pem
*.key
!*.key.example

# Database files
*.db
*.db-*
EOF
    print_status "Updated .gitignore"
else
    print_status ".gitignore already contains development entries"
fi

print_header "Setup Complete!"

print_info "Development environment is ready. Here's what was created:"
echo ""
echo "📁 Directory Structure:"
echo "   ├── configs/development/local.yaml  (development config)"
echo "   ├── test-data/ssh_keys/            (SSH keys for testing)"
echo "   ├── data/sqlite/                   (development databases)"
echo "   ├── banners/                       (welcome banners)"
echo "   └── .env.example                   (environment template)"
echo ""
echo "🔧 Quick Start Commands:"
echo "   # Test your setup"
echo "   ./scripts/test-config.sh configs/development/local.yaml"
echo ""
echo "   # Test database connectivity"
echo "   go run test-build.go -config configs/development/local.yaml -test-db"
echo ""
echo "   # Run performance benchmark"
echo "   go run test-build.go -config configs/development/local.yaml -benchmark"
echo ""
echo "📚 Documentation:"
echo "   • docs/CONFIG.md    - Complete configuration guide"
echo "   • docs/TESTING.md   - Testing and troubleshooting"
echo ""
print_status "Happy coding! 🚀"
