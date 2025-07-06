#!/bin/bash
# scripts/test-database-configs.sh
# Test script for different database configurations

set -e

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

# Create test data directories
create_test_directories() {
    print_status "Creating test data directories..."
    
    mkdir -p test-data/{sqlite,postgresql,postgresql-replica,aurora,mysql}/{ttyrec,tmp,logs}
    mkdir -p test-data/ssh_keys
    mkdir -p migrations
    
    print_success "Test directories created"
}

# Generate SSH test key if not exists
generate_ssh_key() {
    if [ ! -f "test-data/ssh_keys/test_host_key" ]; then
        print_status "Generating SSH test key..."
        ssh-keygen -t rsa -b 2048 -f test-data/ssh_keys/test_host_key -N "" -C "dungeongate-test"
        print_success "SSH test key generated"
    else
        print_status "SSH test key already exists"
    fi
}

# Test SQLite configuration
test_sqlite() {
    print_status "Testing SQLite embedded configuration..."
    
    # Create SQLite database directory
    mkdir -p test-data/sqlite
    
    # Test configuration loading
    if go run test-build.go -config configs/testing/sqlite-embedded.yaml -test-db; then
        print_success "SQLite configuration test passed"
    else
        print_error "SQLite configuration test failed"
        return 1
    fi
}

# Test PostgreSQL setup
test_postgresql() {
    print_status "Testing PostgreSQL configuration..."
    
    # Check if PostgreSQL is running
    if ! command -v psql &> /dev/null; then
        print_warning "PostgreSQL client not found, skipping PostgreSQL tests"
        return 0
    fi
    
    if ! pg_isready -h localhost -p 5432 &> /dev/null; then
        print_warning "PostgreSQL server not running on localhost:5432, skipping tests"
        return 0
    fi
    
    # Create test database
    print_status "Creating PostgreSQL test database..."
    createdb dungeongate_test 2>/dev/null || true
    
    # Test single instance configuration
    if go run test-build.go -config configs/testing/postgresql-single.yaml -test-db; then
        print_success "PostgreSQL single instance test passed"
    else
        print_error "PostgreSQL single instance test failed"
        return 1
    fi
}

# Test MySQL setup
test_mysql() {
    print_status "Testing MySQL configuration..."
    
    # Check if MySQL is running
    if ! command -v mysql &> /dev/null; then
        print_warning "MySQL client not found, skipping MySQL tests"
        return 0
    fi
    
    if ! mysqladmin ping -h localhost -P 3306 &> /dev/null; then
        print_warning "MySQL server not running on localhost:3306, skipping tests"
        return 0
    fi
    
    # Create test database
    print_status "Creating MySQL test database..."
    mysql -h localhost -e "CREATE DATABASE IF NOT EXISTS dungeongate_mysql_test;" 2>/dev/null || true
    
    # Test MySQL configuration
    if go run test-build.go -config configs/testing/mysql-test.yaml -test-db; then
        print_success "MySQL test passed"
    else
        print_error "MySQL test failed"
        return 1
    fi
}

# Test configuration validation
test_config_validation() {
    print_status "Testing configuration validation..."
    
    for config in configs/testing/*.yaml; do
        print_status "Validating $(basename "$config")..."
        if go run test-build.go -config "$config" -validate-only; then
            print_success "$(basename "$config") validation passed"
        else
            print_error "$(basename "$config") validation failed"
            return 1
        fi
    done
}

# Cleanup function
cleanup() {
    print_status "Cleaning up test artifacts..."
    
    # Stop any test processes
    pkill -f "dungeongate.*test" 2>/dev/null || true
    
    # Clean up test databases (optional)
    # dropdb dungeongate_test 2>/dev/null || true
    # mysql -h localhost -e "DROP DATABASE IF EXISTS dungeongate_mysql_test;" 2>/dev/null || true
    
    print_success "Cleanup completed"
}

# Main test function
run_tests() {
    print_status "Starting DungeonGate database configuration tests..."
    
    # Setup
    create_test_directories
    generate_ssh_key
    
    # Configuration validation tests
    test_config_validation
    
    # Database-specific tests
    test_sqlite
    test_postgresql
    test_mysql
    
    print_success "All database configuration tests completed!"
}

# Command line options
case "${1:-}" in
    "sqlite")
        create_test_directories
        generate_ssh_key
        test_sqlite
        ;;
    "postgresql")
        create_test_directories
        test_postgresql
        ;;
    "mysql")
        create_test_directories
        test_mysql
        ;;
    "validate")
        test_config_validation
        ;;
    "cleanup")
        cleanup
        ;;
    "")
        run_tests
        ;;
    *)
        echo "Usage: $0 [sqlite|postgresql|mysql|validate|cleanup]"
        echo ""
        echo "  sqlite      - Test SQLite embedded configuration only"
        echo "  postgresql  - Test PostgreSQL configuration only"
        echo "  mysql       - Test MySQL configuration only"
        echo "  validate    - Validate all configurations only"
        echo "  cleanup     - Clean up test artifacts"
        echo "  (no args)   - Run all tests"
        exit 1
        ;;
esac
