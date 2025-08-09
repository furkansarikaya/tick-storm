#!/usr/bin/env bash

# Tick-Storm Build Script
# Builds static binaries with optimizations

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BINARY_NAME="tick-storm"
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GO_VERSION=$(go version | cut -d' ' -f3)

# Build flags for static binary
LDFLAGS="-s -w \
    -X 'main.Version=${VERSION}' \
    -X 'main.BuildTime=${BUILD_TIME}' \
    -X 'main.GitCommit=${GIT_COMMIT}' \
    -X 'main.GoVersion=${GO_VERSION}'"

echo -e "${GREEN}Building ${BINARY_NAME} v${VERSION}...${NC}"
echo "  Git Commit: ${GIT_COMMIT}"
echo "  Build Time: ${BUILD_TIME}"
echo "  Go Version: ${GO_VERSION}"
echo ""

# Ensure output directory exists
mkdir -p bin

# Build static binary
echo -e "${YELLOW}Compiling with CGO_ENABLED=0...${NC}"
CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o "bin/${BINARY_NAME}" ./cmd/server

# Check if build was successful
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Build successful!${NC}"
    echo -e "${GREEN}Binary location: bin/${BINARY_NAME}${NC}"
    
    # Display binary info
    echo ""
    echo "Binary details:"
    ls -lh "bin/${BINARY_NAME}"
    
    # Check if binary is static
    echo ""
    echo "Checking binary type:"
    file "bin/${BINARY_NAME}"
    
    # Display binary size
    SIZE=$(du -h "bin/${BINARY_NAME}" | cut -f1)
    echo ""
    echo -e "${GREEN}Binary size: ${SIZE}${NC}"
else
    echo -e "${RED}✗ Build failed!${NC}"
    exit 1
fi
