#!/bin/bash

# test-config.sh - Quick configuration testing script for DungeonGate
# NOTICE: This script now uses the unified build-and-test.sh system
# Usage: ./scripts/test-config.sh [config-file] [options]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
CONFIG_FILE="configs/testing/sqlite-embedded.yaml"
MODE="test-db"
VERBOSE=false

# Function to print colored output
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

# Function to show usage
show_usage() {
    cat << EOF
DungeonGate Configuration Test Script

Usage: $0 [config-file] [options]

Arguments:
    config-file         Path to configuration file (default: configs/testing/sqlite-embedded.yaml)

Options:
    -v, --validate      Only validate configuration (don't test connections)
    -b, --benchmark     Run performance benchmarks
    -t, --test-db       Test database connection (default)
    --verbose           Enable verbose output
    -h, --help          Show this help message

Examples:
    $0                                          # Test default SQLite config
    $0 configs/production.yaml                  # Test production config
    $0 configs/development.yaml --validate      # Validate only
    $0 configs/testing/sqlite-embedded.yaml -b  # Run benchmarks

Environment Variables:
    DB_PASSWORD         Database password (for external databases)
    DB_HOST            Database host
    LOG_LEVEL          Logging level (debug, info, warn, error)
EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--validate)
            MODE="validate-only"
            shift
            ;;
        -b|--benchmark)
            MODE="benchmark"
            shift
            ;;
        -t|--test-db)
            MODE="test-db"
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        -*)
            print_error "Unknown option $1"
            show_usage
            exit 1
            ;;
        *)
            CONFIG_FILE="$1"
            shift
            ;;
    esac
done

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed or not in PATH"
    exit 1
fi

# Check if config file exists
if [[ ! -f "$CONFIG_FILE" ]]; then
    print_error "Configuration file not found: $CONFIG_FILE"
    print_info "Available configurations:"
    find configs -name "*.yaml" -o -name "*.yml" 2>/dev/null | head -10
    exit 1
fi

print_info "Testing DungeonGate configuration..."
print_info "Config file: $CONFIG_FILE"
print_info "Mode: $MODE"

# Ensure required directories exist
print_info "Creating required directories..."
mkdir -p test-data/sqlite/{ttyrec,tmp}
mkdir -p test-data/ssh_keys

# Build test flags
TEST_FLAGS="-config $CONFIG_FILE"
case $MODE in
    "validate-only")
        TEST_FLAGS="$TEST_FLAGS -validate-only"
        ;;
    "test-db")
        TEST_FLAGS="$TEST_FLAGS -test-db"
        ;;
    "benchmark")
        TEST_FLAGS="$TEST_FLAGS -benchmark"
        ;;
esac

# Run the test
print_info "Running test command: go run test-build.go $TEST_FLAGS"
echo "----------------------------------------"

if $VERBOSE; then
    go run test-build.go $TEST_FLAGS
else
    go run test-build.go $TEST_FLAGS 2>&1
fi

TEST_EXIT_CODE=$?

echo "----------------------------------------"

if [[ $TEST_EXIT_CODE -eq 0 ]]; then
    print_status "Configuration test completed successfully!"
    
    # Additional success information based on mode
    case $MODE in
        "validate-only")
            print_info "Configuration is valid and properly structured"
            ;;
        "test-db")
            print_info "Database connection and table creation successful"
            print_info "Configuration is ready for use"
            ;;
        "benchmark")
            print_info "Performance benchmarks completed"
            print_info "Check output above for performance metrics"
            ;;
    esac
    
    # Show next steps
    echo ""
    print_info "Next steps:"
    echo "  • Review the configuration in $CONFIG_FILE"
    echo "  • Check the test-data directory for created files"
    if [[ "$MODE" != "benchmark" ]]; then
        echo "  • Run with --benchmark to test performance"
    fi
    if [[ "$MODE" != "validate-only" ]]; then
        echo "  • Run with --validate to check config syntax only"
    fi
    echo "  • See docs/TESTING.md for more testing options"
    
else
    print_error "Configuration test failed with exit code $TEST_EXIT_CODE"
    echo ""
    print_info "Troubleshooting tips:"
    echo "  • Check that all required dependencies are installed (go mod tidy)"
    echo "  • Verify configuration file syntax with --validate"
    echo "  • Check file permissions for test-data directory"
    echo "  • Review docs/TESTING.md for common issues"
    echo "  • Enable verbose output with --verbose for more details"
    
    # Check for common issues
    if [[ "$CONFIG_FILE" == *"external"* ]] || grep -q "mode.*external" "$CONFIG_FILE" 2>/dev/null; then
        echo ""
        print_warning "Using external database configuration:"
        echo "  • Ensure database server is running and accessible"
        echo "  • Check DB_PASSWORD and other environment variables"
        echo "  • Verify network connectivity to database"
    fi
    
    exit $TEST_EXIT_CODE
fi
