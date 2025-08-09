#!/usr/bin/env bash

# Tick-Storm Protobuf Generation Script
# Generates Go code from .proto files

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
PROTO_DIR="api/proto"
OUTPUT_DIR="internal/protocol/pb"

echo -e "${GREEN}Tick-Storm Protobuf Generation${NC}"
echo "================================"

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo -e "${RED}Error: protoc is not installed${NC}"
    echo "Install protoc from: https://github.com/protocolbuffers/protobuf/releases"
    exit 1
fi

# Check if protoc-gen-go is installed
if ! command -v protoc-gen-go &> /dev/null; then
    echo -e "${YELLOW}Installing protoc-gen-go...${NC}"
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

# Check if protoc-gen-go-grpc is installed (optional, for future gRPC support)
if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo -e "${YELLOW}Installing protoc-gen-go-grpc...${NC}"
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

# Generate Go code from proto files
echo -e "${YELLOW}Generating Go code from proto files...${NC}"

protoc \
    --go_out="$OUTPUT_DIR" \
    --go_opt=paths=source_relative \
    --go-grpc_out="$OUTPUT_DIR" \
    --go-grpc_opt=paths=source_relative \
    -I "$PROTO_DIR" \
    "$PROTO_DIR"/*.proto

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Protobuf generation successful!${NC}"
    echo ""
    echo "Generated files:"
    find "$OUTPUT_DIR" -name "*.pb.go" -type f | while read -r file; do
        echo "  - $file"
    done
else
    echo -e "${RED}✗ Protobuf generation failed!${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}Done!${NC}"
