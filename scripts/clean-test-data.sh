#!/bin/bash

# clean-test-data.sh - Clean up test data and reset development environment
# Usage: ./scripts/clean-test-data.sh [options]

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

# Default options
CLEAN_TEST_DATA=true
CLEAN_DEV_DATA=false
CLEAN_LOGS=true
CLEAN_SSH_KEYS=false
FORCE=false
VERBOSE=false

show_usage() {
    cat << EOF
DungeonGate Test Data Cleanup Script

Usage: $0 [options]

Options:
    --all               Clean everything (test-data, data, logs, SSH keys)
    --test-only         Clean only test-data directory (default)
    --dev-data          Also clean development data directory
    --logs              Clean log files (default)
    --ssh-keys          Also clean generated SSH keys
    --no-logs           Don't clean log files
    --force             Don't ask for confirmation
    --verbose           Show detailed output
    -h, --help          Show this help message

Examples:
    $0                  # Clean test data and logs (safe default)
    $0 --all --force    # Clean everything without confirmation
    $0 --test-only      # Only clean test-data directory
    $0 --dev-data       # Clean test-data and development data

What gets cleaned:
    test-data/          SQLite databases, recordings, temp files
    data/ (optional)    Development databases and logs
    *.log (optional)    Log files in project root
    SSH keys (optional) Generated test SSH keys
EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --all)
            CLEAN_TEST_DATA=true
            CLEAN_DEV_DATA=true
            CLEAN_LOGS=true
            CLEAN_SSH_KEYS=true
            shift
            ;;
        --test-only)
            CLEAN_TEST_DATA=true
            CLEAN_DEV_DATA=false
            CLEAN_LOGS=false
            CLEAN_SSH_KEYS=false
            shift
            ;;
        --dev-data)
            CLEAN_DEV_DATA=true
            shift
            ;;
        --logs)
            CLEAN_LOGS=true
            shift
            ;;
        --no-logs)
            CLEAN_LOGS=false
            shift
            ;;
        --ssh-keys)
            CLEAN_SSH_KEYS=true
            shift
            ;;
        --force)
            FORCE=true
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
        *)
            print_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Check if we're in the right directory
if [[ ! -f "go.mod" ]] || ! grep -q "github.com/dungeongate" go.mod; then
    print_error "This script must be run from the DungeonGate project root directory"
    exit 1
fi

print_info "DungeonGate Test Data Cleanup"
echo ""

# Show what will be cleaned
print_info "The following will be cleaned:"
if $CLEAN_TEST_DATA; then
    echo "  • test-data/ directory (SQLite DBs, recordings, temp files)"
fi
if $CLEAN_DEV_DATA; then
    echo "  • data/ directory (development databases and files)"
fi
if $CLEAN_LOGS; then
    echo "  • *.log files in project root"
fi
if $CLEAN_SSH_KEYS; then
    echo "  • Generated SSH keys in test-data/ssh_keys/"
fi
echo ""

# Show current disk usage
if command -v du &> /dev/null; then
    print_info "Current disk usage:"
    if [[ -d "test-data" ]]; then
        echo "  test-data: $(du -sh test-data 2>/dev/null | cut -f1 || echo "0B")"
    fi
    if [[ -d "data" ]]; then
        echo "  data: $(du -sh data 2>/dev/null | cut -f1 || echo "0B")"
    fi
    LOG_SIZE=$(find . -maxdepth 1 -name "*.log" -exec du -ch {} + 2>/dev/null | tail -1 | cut -f1 || echo "0B")
    if [[ "$LOG_SIZE" != "0B" ]]; then
        echo "  log files: $LOG_SIZE"
    fi
    echo ""
fi

# Confirmation unless forced
if ! $FORCE; then
    read -p "Are you sure you want to proceed? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Cleanup cancelled"
        exit 0
    fi
    echo ""
fi

CLEANED_ITEMS=0

# Clean test-data directory
if $CLEAN_TEST_DATA && [[ -d "test-data" ]]; then
    print_info "Cleaning test-data directory..."
    
    if $CLEAN_SSH_KEYS; then
        if $VERBOSE; then
            rm -rvf test-data/
        else
            rm -rf test-data/
        fi
    else
        # Preserve SSH keys
        if [[ -d "test-data/ssh_keys" ]]; then
            cp -r test-data/ssh_keys /tmp/dungeongate_ssh_keys_backup 2>/dev/null || true
        fi
        
        if $VERBOSE; then
            rm -rvf test-data/
        else
            rm -rf test-data/
        fi
        
        # Restore SSH keys
        if [[ -d "/tmp/dungeongate_ssh_keys_backup" ]]; then
            mkdir -p test-data/ssh_keys
            cp -r /tmp/dungeongate_ssh_keys_backup/* test-data/ssh_keys/ 2>/dev/null || true
            rm -rf /tmp/dungeongate_ssh_keys_backup
            print_info "SSH keys preserved"
        fi
    fi
    
    print_status "test-data directory cleaned"
    ((CLEANED_ITEMS++))
fi

# Clean development data directory
if $CLEAN_DEV_DATA && [[ -d "data" ]]; then
    print_info "Cleaning data directory..."
    if $VERBOSE; then
        rm -rvf data/
    else
        rm -rf data/
    fi
    print_status "data directory cleaned"
    ((CLEANED_ITEMS++))
fi

# Clean log files
if $CLEAN_LOGS; then
    LOG_FILES=$(find . -maxdepth 1 -name "*.log" 2>/dev/null)
    if [[ -n "$LOG_FILES" ]]; then
        print_info "Cleaning log files..."
        if $VERBOSE; then
            echo "$LOG_FILES" | xargs rm -vf
        else
            echo "$LOG_FILES" | xargs rm -f
        fi
        print_status "Log files cleaned"
        ((CLEANED_ITEMS++))
    fi
fi

# Recreate basic directory structure
print_info "Recreating basic directory structure..."
mkdir -p {
    test-data/{sqlite/{ttyrec,tmp},ssh_keys},
    data/{sqlite,logs,ttyrec,tmp}
}

if [[ $CLEANED_ITEMS -eq 0 ]]; then
    print_warning "Nothing to clean (directories/files don't exist)"
else
    print_status "Cleanup completed! Cleaned $CLEANED_ITEMS item(s)"
fi

echo ""
print_info "Next steps:"
echo "  • Run ./scripts/dev-setup.sh to regenerate SSH keys and config"
echo "  • Run ./scripts/test-config.sh to verify your setup"
echo "  • Use go run test-build.go to test your configuration"

# Show final disk usage
if command -v du &> /dev/null && $VERBOSE; then
    echo ""
    print_info "Final disk usage:"
    if [[ -d "test-data" ]]; then
        echo "  test-data: $(du -sh test-data 2>/dev/null | cut -f1 || echo "0B")"
    fi
    if [[ -d "data" ]]; then
        echo "  data: $(du -sh data 2>/dev/null | cut -f1 || echo "0B")"
    fi
fi
