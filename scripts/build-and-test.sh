#!/bin/bash

# build-and-test.sh - Unified build and test script for DungeonGate
# This script consolidates all build, test, and validation operations

set -e

# Script metadata
SCRIPT_VERSION="0.0.5"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
declare -r RED='\033[0;31m'
declare -r GREEN='\033[0;32m'
declare -r YELLOW='\033[1;33m'
declare -r BLUE='\033[0;34m'
declare -r PURPLE='\033[0;35m'
declare -r CYAN='\033[0;36m'
declare -r NC='\033[0m' # No Color

# Emojis for better UX
declare -r CHECK_MARK="✅"
declare -r CROSS_MARK="❌"
declare -r WARNING="⚠️"
declare -r INFO="ℹ️"
declare -r ROCKET="🚀"
declare -r GEAR="⚙️"
declare -r MICROSCOPE="🔬"

# Configuration
BUILD_DIR="$PROJECT_ROOT/build"
TEST_DATA_DIR="$PROJECT_ROOT/test-data"
CONFIG_FILE="$PROJECT_ROOT/configs/development/local.yaml"
BINARY_NAME="dungeongate-session-service"
COVERAGE_DIR="$PROJECT_ROOT/coverage"

# Functions for colored output
print_header() {
    echo ""
    echo -e "${BLUE}================================${NC}"
    echo -e "${BLUE} $1${NC}"
    echo -e "${BLUE}================================${NC}"
    echo ""
}

print_success() {
    echo -e "${GREEN}${CHECK_MARK} $1${NC}"
}1,

print_error() {
    echo -e "${RED}${CROSS_MARK} $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}${WARNING} $1${NC}"
}

print_info() {
    echo -e "${CYAN}${INFO} $1${NC}"
}

print_step() {
    echo -e "${PURPLE}${GEAR} $1${NC}"
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to check if we're in the right directory
check_project_root() {
    if [[ ! -f "$PROJECT_ROOT/go.mod" ]] || ! grep -q "github.com/dungeongate" "$PROJECT_ROOT/go.mod" 2>/dev/null; then
        print_error "This script must be run from the DungeonGate project root or scripts directory"
        echo "Current directory: $(pwd)"
        echo "Expected project root: $PROJECT_ROOT"
        exit 1
    fi
}

# Function to check dependencies
check_dependencies() {
    print_step "Checking dependencies..."
    
    local missing_deps=()
    
    if ! command_exists go; then
        missing_deps+=("go")
    fi
    
    if ! command_exists git; then
        missing_deps+=("git")
    fi
    
    if ! command_exists ssh-keygen; then
        missing_deps+=("ssh-keygen (OpenSSH)")
    fi
    
    if [ ${#missing_deps[@]} -ne 0 ]; then
        print_error "Missing required dependencies: ${missing_deps[*]}"
        echo ""
        echo "Please install the missing dependencies:"
        for dep in "${missing_deps[@]}"; do
            echo "  - $dep"
        done
        exit 1
    fi
    
    # Check Go version
    local go_version=$(go version | awk '{print $3}' | sed 's/go//')
    print_info "Go version: $go_version"
    
    # Check if we have development tools (optional but recommended)
    local optional_tools=()
    command_exists golangci-lint || optional_tools+=("golangci-lint")
    command_exists govulncheck || optional_tools+=("govulncheck")
    command_exists air || optional_tools+=("air")
    
    if [ ${#optional_tools[@]} -ne 0 ]; then
        print_warning "Optional development tools not found: ${optional_tools[*]}"
        print_info "Install with: make deps-tools"
    fi
    
    print_success "Dependencies check completed"
}

# Function to setup test environment
setup_test_environment() {
    print_step "Setting up test environment..."
    
    # Create required directories
    mkdir -p "$TEST_DATA_DIR"/{ssh_keys,sqlite,logs,ttyrec}
    mkdir -p "$BUILD_DIR"
    mkdir -p "$COVERAGE_DIR"
    
    # Generate SSH host key if needed
    if [[ ! -f "$TEST_DATA_DIR/ssh_keys/test_host_key" ]]; then
        print_step "Generating SSH host key..."
        ssh-keygen -t rsa -b 2048 -f "$TEST_DATA_DIR/ssh_keys/test_host_key" -N "" -C "dungeongate-test" -q
        chmod 600 "$TEST_DATA_DIR/ssh_keys/test_host_key"
        chmod 644 "$TEST_DATA_DIR/ssh_keys/test_host_key.pub"
        print_success "SSH host key generated"
    fi
    
    print_success "Test environment ready"
}

# Function to clean environment
clean_environment() {
    print_step "Cleaning build artifacts and test data..."
    
    rm -rf "$BUILD_DIR"
    rm -rf "$TEST_DATA_DIR"
    rm -rf "$COVERAGE_DIR"
    
    # Clean Go cache
    go clean -cache -testcache -modcache 2>/dev/null || true
    
    print_success "Environment cleaned"
}

# Function to install/update dependencies
install_dependencies() {
    print_step "Installing Go dependencies..."
    
    cd "$PROJECT_ROOT"
    
    go mod download
    go mod tidy
    go mod verify
    
    print_success "Dependencies installed and verified"
}

# Function to format code
format_code() {
    print_step "Formatting Go code..."
    
    cd "$PROJECT_ROOT"
    
    # Check if code needs formatting
    if [ -n "$(gofmt -s -l .)" ]; then
        print_info "Formatting code..."
        gofmt -s -w .
        print_success "Code formatted"
    else
        print_success "Code already properly formatted"
    fi
}

# Function to run linting
run_linting() {
    print_step "Running code linting..."
    
    cd "$PROJECT_ROOT"
    
    # Run go vet
    if go vet ./...; then
        print_success "go vet passed"
    else
        print_error "go vet failed"
        return 1
    fi
    
    # Run golangci-lint if available
    if command_exists golangci-lint; then
        if golangci-lint run; then
            print_success "golangci-lint passed"
        else
            print_error "golangci-lint failed"
            return 1
        fi
    else
        print_warning "golangci-lint not available (install with: make deps-tools)"
    fi
}

# Function to run security checks
run_security_checks() {
    print_step "Running security vulnerability checks..."
    
    cd "$PROJECT_ROOT"
    
    if command_exists govulncheck; then
        if govulncheck ./...; then
            print_success "Security vulnerability check passed"
        else
            print_error "Security vulnerabilities found"
            return 1
        fi
    else
        print_warning "govulncheck not available (install with: make deps-tools)"
    fi
}

# Function to build the project
build_project() {
    local build_type="${1:-release}"
    
    print_step "Building DungeonGate Session Service ($build_type)..."
    
    cd "$PROJECT_ROOT"
    
    local binary_suffix=""
    local build_flags=""
    local ldflags="-s -w"
    
    # Add version info to ldflags
    local version=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
    local build_time=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    local git_commit=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    
    ldflags="$ldflags -X main.version=$version -X main.buildTime=$build_time -X main.gitCommit=$git_commit"
    
    case $build_type in
        "debug")
            binary_suffix="-debug"
            build_flags="-gcflags=all=-N -l"
            ldflags="" # No stripping for debug builds
            ;;
        "race")
            binary_suffix="-race"
            build_flags="-race"
            ;;
        "release")
            # Default settings already set
            ;;
        *)
            print_error "Unknown build type: $build_type"
            return 1
            ;;
    esac
    
    local binary_path="$BUILD_DIR/$BINARY_NAME$binary_suffix"
    
    if go build $build_flags -ldflags="$ldflags" -o "$binary_path" ./cmd/session-service; then
        print_success "Build completed: $binary_path"
        ls -lh "$binary_path"
    else
        print_error "Build failed"
        return 1
    fi
}

# Function to run tests
run_tests() {
    local test_type="${1:-all}"
    local verbose="${2:-false}"
    
    print_step "Running tests ($test_type)..."
    
    cd "$PROJECT_ROOT"
    
    local test_flags="-timeout=60s"
    local test_pattern=""
    
    if [[ "$verbose" == "true" ]]; then
        test_flags="$test_flags -v"
    fi
    
    case $test_type in
        "all")
            test_pattern="./..."
            ;;
        "short")
            test_flags="$test_flags -short"
            test_pattern="./..."
            ;;
        "ssh")
            test_flags="$test_flags -v"
            test_pattern="./internal/session/..."
            test_flags="$test_flags -run SSH"
            ;;
        "spectating")
            test_flags="$test_flags -v"
            test_pattern="./internal/session/..."
            test_flags="$test_flags -run 'Spectating|Spectator|Registry|Stream|Immutable'"
            ;;
        "race")
            test_flags="$test_flags -race"
            test_pattern="./..."
            ;;
        "coverage")
            test_flags="$test_flags -coverprofile=$COVERAGE_DIR/coverage.out"
            test_pattern="./..."
            ;;
        *)
            print_error "Unknown test type: $test_type"
            return 1
            ;;
    esac
    
    if go test $test_flags $test_pattern; then
        print_success "Tests passed ($test_type)"
        
        # Generate coverage report if coverage test was run
        if [[ "$test_type" == "coverage" ]] && [[ -f "$COVERAGE_DIR/coverage.out" ]]; then
            go tool cover -html="$COVERAGE_DIR/coverage.out" -o "$COVERAGE_DIR/coverage.html"
            print_success "Coverage report generated: $COVERAGE_DIR/coverage.html"
        fi
    else
        print_error "Tests failed ($test_type)"
        return 1
    fi
}

# Function to run benchmarks
run_benchmarks() {
    local bench_type="${1:-all}"
    
    print_step "Running benchmarks ($bench_type)..."
    
    cd "$PROJECT_ROOT"
    
    local bench_flags="-bench=. -benchmem -timeout=5m"
    local test_pattern=""
    
    case $bench_type in
        "all")
            test_pattern="./..."
            ;;
        "ssh")
            bench_flags="-bench=SSH -benchmem -timeout=5m"
            test_pattern="./internal/session/..."
            ;;
        "spectating")
            bench_flags="-bench='Spectating|Registry|Stream' -benchmem -timeout=5m"
            test_pattern="./internal/session/..."
            ;;
        *)
            print_error "Unknown benchmark type: $bench_type"
            return 1
            ;;
    esac
    
    if go test $bench_flags $test_pattern; then
        print_success "Benchmarks completed ($bench_type)"
    else
        print_error "Benchmarks failed ($bench_type)"
        return 1
    fi
}

# Function to validate configuration
validate_config() {
    local config_file="${1:-$CONFIG_FILE}"
    
    print_step "Validating configuration: $config_file"
    
    if [[ ! -f "$config_file" ]]; then
        print_error "Configuration file not found: $config_file"
        return 1
    fi
    
    # Basic YAML syntax check
    if command_exists python3; then
        if python3 -c "import yaml; yaml.safe_load(open('$config_file'))" 2>/dev/null; then
            print_success "YAML syntax is valid"
        else
            print_error "YAML syntax error in $config_file"
            return 1
        fi
    else
        print_warning "Python3 not available for YAML validation"
    fi
    
    # Check if required sections exist
    local required_sections=("ssh" "database" "session_management")
    for section in "${required_sections[@]}"; do
        if grep -q "^${section}:" "$config_file"; then
            print_success "Found required section: $section"
        else
            print_warning "Missing section in config: $section"
        fi
    done
}

# Function to start test server
start_test_server() {
    print_step "Starting test server..."
    
    # Ensure binary is built
    if [[ ! -f "$BUILD_DIR/$BINARY_NAME" ]]; then
        build_project
    fi
    
    # Setup test environment
    setup_test_environment
    
    # Copy config to temporary location
    local temp_config="/tmp/dungeongate-session-service.yaml"
    cp "$CONFIG_FILE" "$temp_config"
    
    print_info "Starting server on port 2222..."
    print_info "Connect with: ssh -p 2222 localhost"
    print_info "Press Ctrl+C to stop the server"
    
    exec "$BUILD_DIR/$BINARY_NAME" -config="$temp_config"
}

# Function to check server status
check_server_status() {
    print_step "Checking SSH server status..."
    
    if lsof -Pi :2222 -sTCP:LISTEN -t >/dev/null 2>&1; then
        print_success "SSH server is running on port 2222"
        lsof -Pi :2222 -sTCP:LISTEN
    else
        print_warning "SSH server is not running on port 2222"
        echo "Start with: make test-run or ./scripts/build-and-test.sh start"
    fi
}

# Function to run integration tests
run_integration_tests() {
    print_step "Running integration tests..."
    
    # Build if needed
    if [[ ! -f "$BUILD_DIR/$BINARY_NAME" ]]; then
        build_project
    fi
    
    setup_test_environment
    
    # Start server in background
    local temp_config="/tmp/dungeongate-session-service.yaml"
    cp "$CONFIG_FILE" "$temp_config"
    
    print_info "Starting test server..."
    "$BUILD_DIR/$BINARY_NAME" -config="$temp_config" &
    local server_pid=$!
    
    # Wait for server to start
    sleep 3
    
    # Check if server is running
    if ! kill -0 $server_pid 2>/dev/null; then
        print_error "Test server failed to start"
        return 1
    fi
    
    # Run basic connectivity test
    if nc -z localhost 2222 2>/dev/null; then
        print_success "Server is accepting connections on port 2222"
    else
        print_error "Server is not accepting connections"
        kill $server_pid 2>/dev/null
        return 1
    fi
    
    # TODO: Add more sophisticated integration tests here
    # For now, just test basic connectivity
    
    # Cleanup
    kill $server_pid 2>/dev/null
    wait $server_pid 2>/dev/null || true
    
    print_success "Integration tests completed"
}

# Function to show usage
show_usage() {
    cat << EOF
DungeonGate Build and Test Script v$SCRIPT_VERSION

Usage: $0 <command> [options]

Commands:
  Environment:
    setup                   Setup development environment
    clean                   Clean build artifacts and test data
    deps                    Install/update Go dependencies
    check                   Check dependencies and project status

  Code Quality:
    format                  Format Go code
    lint                    Run code linting
    security                Run security vulnerability checks
    validate [config]       Validate configuration file

  Build:
    build [type]           Build project (types: release, debug, race)
    build-all              Build all variants (release, debug, race)

  Testing:
    test [type] [verbose]  Run tests (types: all, short, ssh, spectating, race, coverage)
    benchmark [type]       Run benchmarks (types: all, ssh, spectating)
    integration            Run integration tests

  Server:
    start                  Start test server on port 2222
    status                 Check server status

  Workflows:
    verify                 Run all verification checks (format, lint, security, test)
    ci                     Run CI pipeline (verify + coverage + integration)
    release-check          Run all release readiness checks

Examples:
  $0 setup                     # Setup development environment
  $0 build                     # Build release binary
  $0 test ssh true            # Run SSH tests with verbose output
  $0 benchmark spectating     # Run spectating benchmarks
  $0 verify                   # Run all verification checks
  $0 start                    # Start test server

Environment Variables:
  VERBOSE                     Enable verbose output (true/false)
  CONFIG_FILE                Configuration file path
  BUILD_TYPE                 Default build type (release/debug/race)
EOF
}

# Function to run all verification checks
run_verification() {
    print_header "Running All Verification Checks"
    
    local failed_checks=()
    
    format_code || failed_checks+=("format")
    run_linting || failed_checks+=("lint")
    run_security_checks || failed_checks+=("security")
    run_tests "all" || failed_checks+=("test")
    
    if [ ${#failed_checks[@]} -eq 0 ]; then
        print_success "All verification checks passed!"
        return 0
    else
        print_error "Failed checks: ${failed_checks[*]}"
        return 1
    fi
}

# Function to run CI pipeline
run_ci_pipeline() {
    print_header "Running CI Pipeline"
    
    local failed_steps=()
    
    install_dependencies || failed_steps+=("dependencies")
    run_verification || failed_steps+=("verification")
    run_tests "coverage" || failed_steps+=("coverage")
    run_integration_tests || failed_steps+=("integration")
    
    if [ ${#failed_steps[@]} -eq 0 ]; then
        print_success "CI pipeline completed successfully!"
        return 0
    else
        print_error "Failed CI steps: ${failed_steps[*]}"
        return 1
    fi
}

# Function to run release readiness checks
run_release_checks() {
    print_header "Running Release Readiness Checks"
    
    local failed_checks=()
    
    run_verification || failed_checks+=("verification")
    run_tests "coverage" || failed_checks+=("coverage")
    run_benchmarks "all" || failed_checks+=("benchmarks")
    build_project "release" || failed_checks+=("build")
    
    if [ ${#failed_checks[@]} -eq 0 ]; then
        print_success "All release checks passed! Ready for release."
        return 0
    else
        print_error "Failed release checks: ${failed_checks[*]}"
        return 1
    fi
}

# Main script execution
main() {
    print_header "DungeonGate Build and Test Script v$SCRIPT_VERSION"
    
    check_project_root
    cd "$PROJECT_ROOT"
    
    local command="${1:-help}"
    shift || true
    
    case "$command" in
        # Environment commands
        setup)
            check_dependencies
            setup_test_environment
            install_dependencies
            print_success "Development environment is ready!"
            ;;
        clean)
            clean_environment
            ;;
        deps)
            check_dependencies
            install_dependencies
            ;;
        check)
            check_dependencies
            print_success "Project status check completed"
            ;;
            
        # Code quality commands
        format)
            format_code
            ;;
        lint)
            run_linting
            ;;
        security)
            run_security_checks
            ;;
        validate)
            validate_config "${1:-$CONFIG_FILE}"
            ;;
            
        # Build commands
        build)
            check_dependencies
            install_dependencies
            build_project "${1:-release}"
            ;;
        build-all)
            check_dependencies
            install_dependencies
            for build_type in release debug race; do
                build_project "$build_type"
            done
            ;;
            
        # Testing commands
        test)
            check_dependencies
            setup_test_environment
            run_tests "${1:-all}" "${2:-false}"
            ;;
        benchmark)
            check_dependencies
            run_benchmarks "${1:-all}"
            ;;
        integration)
            check_dependencies
            run_integration_tests
            ;;
            
        # Server commands
        start)
            check_dependencies
            start_test_server
            ;;
        status)
            check_server_status
            ;;
            
        # Workflow commands
        verify)
            check_dependencies
            run_verification
            ;;
        ci)
            check_dependencies
            run_ci_pipeline
            ;;
        release-check)
            check_dependencies
            run_release_checks
            ;;
            
        # Help
        help|--help|-h)
            show_usage
            ;;
            
        *)
            print_error "Unknown command: $command"
            echo ""
            show_usage
            exit 1
            ;;
    esac
}

# Execute main function with all arguments
main "$@"