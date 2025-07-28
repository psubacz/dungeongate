#!/bin/bash

# DungeonGate SSH Build and Test Script
# DEPRECATED: Use ./scripts/build-and-test.sh instead
# This script is kept for backward compatibility

echo "⚠️  DEPRECATED: This script is deprecated."
echo "ℹ️  Please use: ./scripts/build-and-test.sh"
echo "ℹ️  For SSH-specific tests: ./scripts/build-and-test.sh test ssh"
echo ""
echo "Redirecting to new unified script..."
echo ""

set -e

PROJECT_ROOT="/Users/caboose/Desktop/dungeongate"
BUILD_DIR="$PROJECT_ROOT/build"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to check dependencies
check_dependencies() {
    print_status "Checking dependencies..."
    
    if ! command_exists go; then
        print_error "Go is not installed. Please install Go 1.21 or higher."
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    print_status "Go version: $GO_VERSION"
    
    print_success "Dependencies check completed"
}

# Function to run tests
run_tests() {
    print_status "Running tests..."
    
    cd "$PROJECT_ROOT"
    
    # Run tests for session package (including SSH)
    go test -v ./internal/session/...
    
    if [ $? -eq 0 ]; then
        print_success "All tests passed"
    else
        print_error "Some tests failed"
        exit 1
    fi
}

# Function to build the project
build_project() {
    print_status "Building DungeonGate Session Service..."
    
    cd "$PROJECT_ROOT"
    
    # Clean previous builds
    mkdir -p "$BUILD_DIR"
    rm -f "$BUILD_DIR/dungeongate-session-service"
    
    # Get dependencies
    print_status "Downloading dependencies..."
    go mod tidy
    
    # Build with proper tags and flags
    print_status "Compiling session service..."
    go build -ldflags="-s -w" -o "$BUILD_DIR/dungeongate-session-service" ./cmd/session-service
    
    if [ $? -eq 0 ]; then
        print_success "Build completed successfully"
        ls -la "$BUILD_DIR/dungeongate-session-service"
    else
        print_error "Build failed"
        exit 1
    fi
}

# Function to run SSH-specific tests
run_ssh_tests() {
    print_status "Running SSH-specific tests..."
    
    cd "$PROJECT_ROOT"
    
    # Run SSH tests only
    go test -v ./internal/session/ -run "SSH"
    
    if [ $? -eq 0 ]; then
        print_success "SSH tests passed"
    else
        print_error "SSH tests failed"
        exit 1
    fi
}

# Function to benchmark SSH performance
benchmark_ssh() {
    print_status "Running SSH benchmarks..."
    
    cd "$PROJECT_ROOT"
    
    # Run SSH benchmarks
    go test -bench="SSH" -benchmem ./internal/session/
    
    if [ $? -eq 0 ]; then
        print_success "SSH benchmarks completed"
    else
        print_error "SSH benchmarks failed"
        exit 1
    fi
}

# Function to check SSH functionality
check_ssh_functionality() {
    print_status "Checking SSH functionality..."
    
    cd "$PROJECT_ROOT"
    
    # Check if SSH key exists
    if [ ! -f "$PROJECT_ROOT/ssh_keys/ssh_host_rsa_key" ]; then
        print_warning "SSH host key not found. Creating one..."
        mkdir -p "$PROJECT_ROOT/ssh_keys"
        ssh-keygen -t rsa -b 2048 -f "$PROJECT_ROOT/ssh_keys/ssh_host_rsa_key" -N "" -C "dungeongate-dev"
        chmod 600 "$PROJECT_ROOT/ssh_keys/ssh_host_rsa_key"
        chmod 644 "$PROJECT_ROOT/ssh_keys/ssh_host_rsa_key.pub"
    fi
    
    # Test SSH server creation
    go run -ldflags="-s -w" ./cmd/session-service --version
    
    if [ $? -eq 0 ]; then
        print_success "SSH functionality check passed"
    else
        print_error "SSH functionality check failed"
        exit 1
    fi
}

# Function to show usage information
show_usage() {
    cat << EOF
DungeonGate SSH Build and Test Script

Usage: $0 [COMMAND]

Commands:
    build       Build the session service binary
    test        Run all tests including SSH tests
    ssh-test    Run SSH-specific tests only
    benchmark   Run SSH performance benchmarks
    check       Check SSH functionality
    help        Show this help message

Examples:
    $0 build        # Build the service
    $0 test         # Run all tests
    $0 ssh-test     # Run only SSH tests
    $0 benchmark    # Run SSH benchmarks
    $0 check        # Check SSH functionality

EOF
}

# Main script logic - redirect to new unified script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NEW_SCRIPT="$SCRIPT_DIR/build-and-test.sh"

case "${1:-help}" in
    build)
        exec "$NEW_SCRIPT" build
        ;;
    test)
        exec "$NEW_SCRIPT" test all true
        ;;
    ssh-test)
        exec "$NEW_SCRIPT" test ssh true
        ;;
    benchmark)
        exec "$NEW_SCRIPT" benchmark ssh
        ;;
    check)
        exec "$NEW_SCRIPT" check
        ;;
    help)
        echo "Legacy SSH build script - redirecting to new unified script"
        echo ""
        exec "$NEW_SCRIPT" help
        ;;
    *)
        echo "Unknown command: $1"
        echo "Redirecting to new unified script..."
        exec "$NEW_SCRIPT" help
        ;;
esac
