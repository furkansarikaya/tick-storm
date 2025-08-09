#!/usr/bin/env bash

# Tick-Storm Cross-Compilation Script
# Builds binaries for multiple platforms

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BINARY_NAME="tick-storm"
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GO_VERSION=$(go version | cut -d' ' -f3)

# Build flags
LDFLAGS="-s -w \
    -X 'main.Version=${VERSION}' \
    -X 'main.BuildTime=${BUILD_TIME}' \
    -X 'main.GitCommit=${GIT_COMMIT}' \
    -X 'main.GoVersion=${GO_VERSION}'"

# Target platforms
PLATFORMS=(
    "darwin/amd64"
    "darwin/arm64"
    "linux/amd64"
    "linux/arm64"
    "linux/386"
    "windows/amd64"
    "windows/386"
)

echo -e "${BLUE}═══════════════════════════════════════════${NC}"
echo -e "${BLUE}  Tick-Storm Cross-Platform Build${NC}"
echo -e "${BLUE}═══════════════════════════════════════════${NC}"
echo "  Version:    ${VERSION}"
echo "  Git Commit: ${GIT_COMMIT}"
echo "  Build Time: ${BUILD_TIME}"
echo ""

# Create output directory
mkdir -p bin/releases

# Build for each platform
for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS=${PLATFORM%/*}
    GOARCH=${PLATFORM#*/}
    OUTPUT_NAME="${BINARY_NAME}-${VERSION}-${GOOS}-${GOARCH}"
    
    if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi
    
    echo -e "${YELLOW}Building for ${GOOS}/${GOARCH}...${NC}"
    
    # Build
    GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 \
        go build -ldflags "${LDFLAGS}" \
        -o "bin/releases/${OUTPUT_NAME}" \
        ./cmd/server
    
    if [ $? -eq 0 ]; then
        SIZE=$(du -h "bin/releases/${OUTPUT_NAME}" | cut -f1)
        echo -e "${GREEN}✓ ${OUTPUT_NAME} (${SIZE})${NC}"
    else
        echo -e "${RED}✗ Failed to build for ${GOOS}/${GOARCH}${NC}"
    fi
    echo ""
done

# Create checksums
echo -e "${YELLOW}Generating checksums...${NC}"
cd bin/releases
shasum -a 256 tick-storm-* > checksums.txt
cd ../..
echo -e "${GREEN}✓ Checksums generated: bin/releases/checksums.txt${NC}"

# Summary
echo ""
echo -e "${BLUE}═══════════════════════════════════════════${NC}"
echo -e "${GREEN}Build complete!${NC}"
echo "Binaries available in: bin/releases/"
echo ""
ls -lh bin/releases/
echo -e "${BLUE}═══════════════════════════════════════════${NC}"
