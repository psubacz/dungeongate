#!/bin/bash

# migrate.sh - Database migration script for DungeonGate
# Handles database schema migrations and data management

set -e

# Colors and formatting
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

CHECK_MARK="✅"
CROSS_MARK="❌"
WARNING="⚠️"
INFO="ℹ️"

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MIGRATIONS_DIR="$PROJECT_ROOT/migrations"
CONFIG_FILE="$PROJECT_ROOT/configs/development/local.yaml"

print_success() {
    echo -e "${GREEN}${CHECK_MARK} $1${NC}"
}

print_error() {
    echo -e "${RED}${CROSS_MARK} $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}${WARNING} $1${NC}"
}

print_info() {
    echo -e "${BLUE}${INFO} $1${NC}"
}

print_step() {
    echo -e "${BLUE}🔧 $1${NC}"
}

# Function to show usage
show_usage() {
    cat << EOF
DungeonGate Database Migration Script

Usage: $0 <command> [options]

Commands:
  up                      Apply all pending migrations
  down [count]           Rollback migrations (default: 1)
  reset                  Reset database (DESTRUCTIVE)
  status                 Show migration status
  create <name>          Create a new migration file
  validate               Validate migration files

Options:
  --config <file>        Use specific configuration file
  --dry-run             Show what would be done without executing
  --force               Force operations (skip confirmations)

Examples:
  $0 up                          # Apply all pending migrations
  $0 down 2                      # Rollback last 2 migrations
  $0 create add_user_table       # Create new migration
  $0 status                      # Show current migration status

EOF
}

# Function to extract database config
get_database_config() {
    local config_file="${1:-$CONFIG_FILE}"
    
    if [[ ! -f "$config_file" ]]; then
        print_error "Configuration file not found: $config_file"
        return 1
    fi
    
    # For now, we'll focus on SQLite embedded mode
    # In the future, this would parse the YAML and support PostgreSQL
    local db_path
    db_path=$(grep -A 10 "embedded:" "$config_file" | grep "path:" | awk '{print $2}' | tr -d '"' | head -1)
    
    if [[ -n "$db_path" ]]; then
        # Convert relative path to absolute
        if [[ ! "$db_path" =~ ^/ ]]; then
            db_path="$PROJECT_ROOT/$db_path"
        fi
        echo "$db_path"
    else
        print_error "Could not extract database path from configuration"
        return 1
    fi
}

# Function to create migrations directory
create_migrations_dir() {
    if [[ ! -d "$MIGRATIONS_DIR" ]]; then
        mkdir -p "$MIGRATIONS_DIR"
        print_success "Created migrations directory: $MIGRATIONS_DIR"
    fi
}

# Function to create a new migration
create_migration() {
    local name="$1"
    
    if [[ -z "$name" ]]; then
        print_error "Migration name is required"
        echo "Usage: $0 create <migration_name>"
        return 1
    fi
    
    create_migrations_dir
    
    # Generate timestamp
    local timestamp=$(date -u +"%Y%m%d%H%M%S")
    local filename="${timestamp}_${name}.sql"
    local filepath="$MIGRATIONS_DIR/$filename"
    
    # Create migration file template
    cat > "$filepath" << EOF
-- Migration: $name
-- Created: $(date -u +"%Y-%m-%d %H:%M:%S UTC")
-- Description: Add description here

-- +migrate Up
-- SQL for applying this migration


-- +migrate Down
-- SQL for rolling back this migration

EOF
    
    print_success "Created migration: $filepath"
    print_info "Edit the file to add your SQL statements"
}

# Function to validate migration files
validate_migrations() {
    print_step "Validating migration files..."
    
    if [[ ! -d "$MIGRATIONS_DIR" ]]; then
        print_warning "No migrations directory found"
        return 0
    fi
    
    local migration_files
    migration_files=$(find "$MIGRATIONS_DIR" -name "*.sql" | sort)
    
    if [[ -z "$migration_files" ]]; then
        print_info "No migration files found"
        return 0
    fi
    
    local valid_count=0
    local invalid_count=0
    
    while IFS= read -r file; do
        if [[ -f "$file" ]]; then
            # Check for required migration markers
            if grep -q "-- +migrate Up" "$file" && grep -q "-- +migrate Down" "$file"; then
                print_success "Valid migration: $(basename "$file")"
                valid_count=$((valid_count + 1))
            else
                print_error "Invalid migration (missing markers): $(basename "$file")"
                invalid_count=$((invalid_count + 1))
            fi
        fi
    done <<< "$migration_files"
    
    print_info "Migration validation: $valid_count valid, $invalid_count invalid"
    
    if [[ $invalid_count -gt 0 ]]; then
        return 1
    fi
}

# Function to show migration status
show_status() {
    print_step "Checking migration status..."
    
    local db_path
    db_path=$(get_database_config)
    
    if [[ ! -f "$db_path" ]]; then
        print_warning "Database file does not exist: $db_path"
        print_info "Run migrations to create and initialize the database"
        return 0
    fi
    
    print_success "Database file exists: $db_path"
    
    # Show database size
    local db_size
    db_size=$(du -h "$db_path" 2>/dev/null | cut -f1)
    print_info "Database size: $db_size"
    
    # TODO: In a real implementation, we would:
    # 1. Check if migrations table exists
    # 2. List applied migrations
    # 3. List pending migrations
    # 4. Show migration history
    
    print_info "Migration tracking not yet implemented"
    print_info "Future enhancement: Track applied migrations in database"
}

# Function to apply migrations
migrate_up() {
    print_step "Applying migrations..."
    
    local db_path
    db_path=$(get_database_config)
    
    # Create database directory if it doesn't exist
    mkdir -p "$(dirname "$db_path")"
    
    validate_migrations || {
        print_error "Migration validation failed"
        return 1
    }
    
    # TODO: In a real implementation, we would:
    # 1. Connect to database
    # 2. Create migrations table if not exists
    # 3. Find unapplied migrations
    # 4. Apply them in order
    # 5. Record applied migrations
    
    print_warning "Migration application not yet implemented"
    print_info "This is a placeholder for future database migration functionality"
    print_success "Database path configured: $db_path"
}

# Function to rollback migrations
migrate_down() {
    local count="${1:-1}"
    
    print_step "Rolling back $count migration(s)..."
    
    print_warning "Migration rollback not yet implemented"
    print_info "This is a placeholder for future database rollback functionality"
}

# Function to reset database
reset_database() {
    local force="$1"
    
    print_warning "This will delete all data in the database!"
    
    if [[ "$force" != "true" ]]; then
        echo -n "Are you sure you want to reset the database? [y/N] "
        read -r response
        if [[ ! "$response" =~ ^[Yy]$ ]]; then
            print_info "Database reset cancelled"
            return 0
        fi
    fi
    
    local db_path
    db_path=$(get_database_config)
    
    if [[ -f "$db_path" ]]; then
        rm -f "$db_path"*  # Remove database and any associated files (WAL, SHM)
        print_success "Database reset: $db_path"
    else
        print_info "Database file does not exist: $db_path"
    fi
    
    # Re-apply migrations
    migrate_up
}

# Main execution
main() {
    echo ""
    echo -e "${BLUE}================================${NC}"
    echo -e "${BLUE} DungeonGate Database Migrations${NC}"
    echo -e "${BLUE}================================${NC}"
    echo ""
    
    cd "$PROJECT_ROOT"
    
    local command="${1:-help}"
    shift || true
    
    # Parse global options
    local config_override=""
    local dry_run=false
    local force=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --config)
                config_override="$2"
                shift 2
                ;;
            --dry-run)
                dry_run=true
                shift
                ;;
            --force)
                force=true
                shift
                ;;
            *)
                break
                ;;
        esac
    done
    
    # Use config override if provided
    if [[ -n "$config_override" ]]; then
        CONFIG_FILE="$config_override"
    fi
    
    case "$command" in
        up)
            migrate_up
            ;;
        down)
            migrate_down "$1"
            ;;
        reset)
            reset_database "$force"
            ;;
        status)
            show_status
            ;;
        create)
            create_migration "$1"
            ;;
        validate)
            validate_migrations
            ;;
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

# Execute main function
main "$@"