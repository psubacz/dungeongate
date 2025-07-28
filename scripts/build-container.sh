#!/bin/bash

# Build script for DungeonGate services using containers
# Supports building development, production, and binary extraction

set -euo pipefail

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m' 
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
TARGET=${1:-"binaries"}  # binaries, development, production, individual services
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "container-build")}
GIT_COMMIT=${GIT_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}
REGISTRY=${REGISTRY:-"localhost"}

# Paths
BUILD_DIR="build"

# Detect architecture and set appropriate Containerfile
ARCH=$(uname -m)
if [[ "$ARCH" == "arm64" ]] || [[ "$ARCH" == "aarch64" ]]; then
    CONTAINERFILE="build/container/Containerfile.arm"
    echo -e "${BLUE}Detected ARM64 architecture${NC}"
else
    CONTAINERFILE="build/container/Containerfile"
    echo -e "${BLUE}Detected x86_64 architecture${NC}"
fi

# Show usage
show_usage() {
    echo "Usage: $0 [TARGET]"
    echo ""
    echo "Targets:"
    echo "  binaries     - Extract compiled binaries to ./build (default)"
    echo "  development  - Build development image with full toolchain"
    echo "  production   - Build all production images"
    echo "  session      - Build only session service production image"
    echo "  auth         - Build only auth service production image"
    echo "  game         - Build only game service production image"
    echo "  all          - Build everything (binaries + all images)"
    echo ""
    echo "Environment variables:"
    echo "  VERSION      - Image version tag (default: git describe)"
    echo "  REGISTRY     - Container registry (default: localhost)"
    echo ""
    echo "Examples:"
    echo "  $0 binaries                    # Extract binaries only"
    echo "  $0 development                 # Build dev image"
    echo "  $0 production                  # Build all production images"
    echo "  VERSION=v1.0.0 $0 all         # Build everything with version tag"
}

if [[ "$TARGET" == "help" || "$TARGET" == "--help" || "$TARGET" == "-h" ]]; then
    show_usage
    exit 0
fi

echo -e "${BLUE}=================================${NC}"
echo -e "${BLUE}DungeonGate Container Build${NC}"
echo -e "${BLUE}=================================${NC}"
echo -e "${YELLOW}Target:     ${NC}${TARGET}"
echo -e "${YELLOW}Version:    ${NC}${VERSION}"
echo -e "${YELLOW}Build Time: ${NC}${BUILD_TIME}"
echo -e "${YELLOW}Git Commit: ${NC}${GIT_COMMIT}"
echo -e "${YELLOW}Registry:   ${NC}${REGISTRY}"
echo -e "${BLUE}=================================${NC}"

# Check if podman or docker is available
CONTAINER_CMD=""
if command -v podman &> /dev/null; then
    CONTAINER_CMD="podman"
    echo -e "${GREEN}Using Podman${NC}"
elif command -v docker &> /dev/null; then
    CONTAINER_CMD="docker"
    echo -e "${GREEN}Using Docker${NC}"
else
    echo -e "${RED}Error: Neither podman nor docker found${NC}"
    exit 1
fi

# Common build arguments
BUILD_ARGS=(
    --build-arg "VERSION=$VERSION"
    --build-arg "BUILD_TIME=$BUILD_TIME" 
    --build-arg "GIT_COMMIT=$GIT_COMMIT"
)

# Function to build and tag image
build_image() {
    local target="$1"
    local tag="$2"
    local extra_tags=("${@:3}")
    
    echo -e "${YELLOW}Building $target image...${NC}"
    
    local cmd=(
        "$CONTAINER_CMD" build
        --target "$target"
        "${BUILD_ARGS[@]}"
        -t "$tag"
    )
    
    # Add extra tags
    for extra_tag in "${extra_tags[@]}"; do
        cmd+=(-t "$extra_tag")
    done
    
    cmd+=(-f "$CONTAINERFILE" .)
    
    "${cmd[@]}"
    echo -e "${GREEN}✓ Built: $tag${NC}"
}

# Function to extract binaries
extract_binaries() {
    echo -e "${YELLOW}Creating build directory...${NC}"
    mkdir -p "$BUILD_DIR"

    echo -e "${YELLOW}Building export image...${NC}"
    $CONTAINER_CMD build \
        --target export \
        "${BUILD_ARGS[@]}" \
        -t dungeongate-export \
        -f "$CONTAINERFILE" .

    echo -e "${YELLOW}Extracting binaries to ./$BUILD_DIR...${NC}"
    TEMP_CONTAINER=$($CONTAINER_CMD create dungeongate-export)
    $CONTAINER_CMD cp "$TEMP_CONTAINER:/" ./"$BUILD_DIR"/
    $CONTAINER_CMD rm "$TEMP_CONTAINER"

    # Verify binaries
    echo -e "${YELLOW}Verifying builds...${NC}"
    BINARIES=("dungeongate-session-service" "dungeongate-auth-service" "dungeongate-game-service")
    ALL_BUILT=true

    for binary in "${BINARIES[@]}"; do
        if [[ -f "$BUILD_DIR/$binary" ]]; then
            SIZE=$(ls -lh "$BUILD_DIR/$binary" | awk '{print $5}')
            echo -e "${GREEN}✓ $binary ($SIZE)${NC}"
        else
            echo -e "${RED}✗ $binary - NOT FOUND${NC}"
            ALL_BUILT=false
        fi
    done

    if $ALL_BUILT; then
        echo -e "${GREEN}✓ All binaries extracted successfully${NC}"
        return 0
    else
        echo -e "${RED}✗ Binary extraction failed${NC}"
        return 1
    fi
}

# Build based on target
case "$TARGET" in
    "binaries")
        extract_binaries
        ;;
        
    "development")
        build_image "development" \
            "$REGISTRY/dungeongate:dev-$VERSION" \
            "$REGISTRY/dungeongate:dev-latest"
        ;;
        
    "session")
        build_image "session-service" \
            "$REGISTRY/dungeongate-session:$VERSION" \
            "$REGISTRY/dungeongate-session:latest"
        ;;
        
    "auth")
        build_image "auth-service" \
            "$REGISTRY/dungeongate-auth:$VERSION" \
            "$REGISTRY/dungeongate-auth:latest"
        ;;
        
    "game")
        build_image "game-service" \
            "$REGISTRY/dungeongate-game:$VERSION" \
            "$REGISTRY/dungeongate-game:latest"
        ;;
        
    "production")
        echo -e "${BLUE}Building all production images...${NC}"
        
        # Individual service images
        build_image "session-service" \
            "$REGISTRY/dungeongate-session:$VERSION" \
            "$REGISTRY/dungeongate-session:latest"
            
        build_image "auth-service" \
            "$REGISTRY/dungeongate-auth:$VERSION" \
            "$REGISTRY/dungeongate-auth:latest"
            
        build_image "game-service" \
            "$REGISTRY/dungeongate-game:$VERSION" \
            "$REGISTRY/dungeongate-game:latest"
            
        # All-in-one production image
        build_image "production" \
            "$REGISTRY/dungeongate:$VERSION" \
            "$REGISTRY/dungeongate:latest"
        ;;
        
    "all")
        echo -e "${BLUE}Building everything...${NC}"
        
        # Extract binaries
        extract_binaries
        
        # Development image
        build_image "development" \
            "$REGISTRY/dungeongate:dev-$VERSION" \
            "$REGISTRY/dungeongate:dev-latest"
        
        # Production images
        build_image "session-service" \
            "$REGISTRY/dungeongate-session:$VERSION" \
            "$REGISTRY/dungeongate-session:latest"
            
        build_image "auth-service" \
            "$REGISTRY/dungeongate-auth:$VERSION" \
            "$REGISTRY/dungeongate-auth:latest"
            
        build_image "game-service" \
            "$REGISTRY/dungeongate-game:$VERSION" \
            "$REGISTRY/dungeongate-game:latest"
            
        build_image "production" \
            "$REGISTRY/dungeongate:$VERSION" \
            "$REGISTRY/dungeongate:latest"
        ;;
        
    *)
        echo -e "${RED}Unknown target: $TARGET${NC}"
        show_usage
        exit 1
        ;;
esac

echo -e "${GREEN}=================================${NC}"
echo -e "${GREEN}Build completed successfully!${NC}"
echo -e "${GREEN}=================================${NC}"

# Show what was built
case "$TARGET" in
    "binaries")
        echo -e "${BLUE}Binaries extracted to: ${NC}./build/"
        ;;
    "development")
        echo -e "${BLUE}Development image: ${NC}$REGISTRY/dungeongate:dev-$VERSION"
        ;;
    "production")
        echo -e "${BLUE}Production images built:${NC}"
        echo -e "${YELLOW}  - $REGISTRY/dungeongate-session:$VERSION${NC}"
        echo -e "${YELLOW}  - $REGISTRY/dungeongate-auth:$VERSION${NC}"
        echo -e "${YELLOW}  - $REGISTRY/dungeongate-game:$VERSION${NC}"
        echo -e "${YELLOW}  - $REGISTRY/dungeongate:$VERSION (all-in-one)${NC}"
        ;;
    "all")
        echo -e "${BLUE}Everything built:${NC}"
        echo -e "${YELLOW}  - Binaries in ./build/${NC}"
        echo -e "${YELLOW}  - Development: $REGISTRY/dungeongate:dev-$VERSION${NC}"
        echo -e "${YELLOW}  - Production: $REGISTRY/dungeongate-*:$VERSION${NC}"
        ;;
    *)
        echo -e "${BLUE}Image built: ${NC}$REGISTRY/dungeongate-${TARGET}:$VERSION"
        ;;
esac