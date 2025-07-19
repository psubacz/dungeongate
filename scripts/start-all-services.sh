#!/bin/sh

# Startup script for running all DungeonGate services in a single container
# This is primarily for development/testing - production should use separate containers

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
AUTH_CONFIG="/app/configs/auth-service.yaml"
GAME_CONFIG="/app/configs/game-service.yaml"  
SESSION_CONFIG="/app/configs/session-service.yaml"

# PID file directory
PID_DIR="/tmp"

echo -e "${BLUE}=================================${NC}"
echo -e "${BLUE}DungeonGate All-in-One Startup${NC}"
echo -e "${BLUE}=================================${NC}"

# Function to start a service
start_service() {
    local service_name="$1"
    local binary_path="$2"
    local config_path="$3"
    local pid_file="$PID_DIR/${service_name}.pid"
    
    echo -e "${YELLOW}Starting ${service_name}...${NC}"
    
    if [ ! -f "$binary_path" ]; then
        echo -e "${RED}Error: Binary not found: $binary_path${NC}"
        return 1
    fi
    
    if [ ! -f "$config_path" ]; then
        echo -e "${RED}Error: Config not found: $config_path${NC}"
        return 1
    fi
    
    # Start service in background
    nohup "$binary_path" -config="$config_path" > "/tmp/${service_name}.log" 2>&1 &
    local pid=$!
    echo $pid > "$pid_file"
    
    # Brief check if service started
    sleep 2
    if kill -0 $pid 2>/dev/null; then
        echo -e "${GREEN}✓ ${service_name} started (PID: $pid)${NC}"
        return 0
    else
        echo -e "${RED}✗ ${service_name} failed to start${NC}"
        return 1
    fi
}

# Function to check service health
check_service() {
    local service_name="$1"
    local pid_file="$PID_DIR/${service_name}.pid"
    
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if kill -0 $pid 2>/dev/null; then
            return 0
        fi
    fi
    return 1
}

# Function to stop all services
stop_services() {
    echo -e "${YELLOW}Stopping services...${NC}"
    
    for service in auth game session; do
        local pid_file="$PID_DIR/${service}-service.pid"
        if [ -f "$pid_file" ]; then
            local pid=$(cat "$pid_file")
            if kill -0 $pid 2>/dev/null; then
                echo -e "${YELLOW}Stopping ${service}-service (PID: $pid)${NC}"
                kill $pid
                rm -f "$pid_file"
            fi
        fi
    done
    
    echo -e "${GREEN}All services stopped${NC}"
}

# Trap signals to gracefully shutdown
trap stop_services TERM INT QUIT

# Start services in dependency order
echo -e "${BLUE}Starting services in dependency order...${NC}"

# 1. Start Auth Service first (other services depend on it)
start_service "auth-service" "/app/bin/dungeongate-auth-service" "$AUTH_CONFIG" || {
    echo -e "${RED}Failed to start auth service. Exiting.${NC}"
    exit 1
}

# Wait a moment for auth service to be ready
sleep 3

# 2. Start Game Service
start_service "game-service" "/app/bin/dungeongate-game-service" "$GAME_CONFIG" || {
    echo -e "${RED}Failed to start game service. Continuing anyway.${NC}"
}

# Wait a moment for game service to be ready  
sleep 3

# 3. Start Session Service (depends on both auth and game services)
start_service "session-service" "/app/bin/dungeongate-session-service" "$SESSION_CONFIG" || {
    echo -e "${RED}Failed to start session service. Continuing anyway.${NC}"
}

echo -e "${GREEN}=================================${NC}"
echo -e "${GREEN}All services started!${NC}"
echo -e "${GREEN}=================================${NC}"

# Show service status
echo -e "${BLUE}Service Status:${NC}"
for service in auth game session; do
    if check_service "${service}-service"; then
        echo -e "${GREEN}✓ ${service}-service: Running${NC}"
    else
        echo -e "${RED}✗ ${service}-service: Not running${NC}"
    fi
done

echo -e "${BLUE}=================================${NC}"
echo -e "${YELLOW}Service Endpoints:${NC}"
echo -e "${YELLOW}Auth Service:    HTTP :8081, gRPC :8082${NC}"
echo -e "${YELLOW}Game Service:    HTTP :8085, gRPC :50051${NC}"
echo -e "${YELLOW}Session Service: HTTP :8083, gRPC :9093, SSH :2222${NC}"
echo -e "${BLUE}=================================${NC}"

# Monitor services and restart if needed
echo -e "${YELLOW}Monitoring services... (Ctrl+C to stop)${NC}"

while true; do
    sleep 30
    
    # Check each service and restart if needed
    for service in auth game session; do
        if ! check_service "${service}-service"; then
            echo -e "${RED}${service}-service died, attempting restart...${NC}"
            case $service in
                auth)
                    start_service "auth-service" "/app/bin/dungeongate-auth-service" "$AUTH_CONFIG"
                    ;;
                game)
                    start_service "game-service" "/app/bin/dungeongate-game-service" "$GAME_CONFIG"
                    ;;
                session)
                    start_service "session-service" "/app/bin/dungeongate-session-service" "$SESSION_CONFIG"
                    ;;
            esac
        fi
    done
done