#!/bin/bash

# Docker build script for TickStorm
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
IMAGE_NAME="tick-storm"
TAG="latest"
REGISTRY=""
PUSH=false
BUILD_ARGS=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -t|--tag)
            TAG="$2"
            shift 2
            ;;
        -r|--registry)
            REGISTRY="$2"
            shift 2
            ;;
        -p|--push)
            PUSH=true
            shift
            ;;
        --build-arg)
            BUILD_ARGS="$BUILD_ARGS --build-arg $2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  -t, --tag TAG        Image tag (default: latest)"
            echo "  -r, --registry REG   Registry prefix"
            echo "  -p, --push          Push image to registry"
            echo "  --build-arg ARG     Pass build argument"
            echo "  -h, --help          Show this help"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Construct full image name
if [ -n "$REGISTRY" ]; then
    FULL_IMAGE_NAME="$REGISTRY/$IMAGE_NAME:$TAG"
else
    FULL_IMAGE_NAME="$IMAGE_NAME:$TAG"
fi

echo -e "${YELLOW}Building Docker image: $FULL_IMAGE_NAME${NC}"

# Build the image
echo -e "${YELLOW}Running docker build...${NC}"
if docker build $BUILD_ARGS -t "$FULL_IMAGE_NAME" .; then
    echo -e "${GREEN}✓ Build successful${NC}"
else
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
fi

# Get image size
IMAGE_SIZE=$(docker images "$FULL_IMAGE_NAME" --format "table {{.Size}}" | tail -n 1)
echo -e "${GREEN}Image size: $IMAGE_SIZE${NC}"

# Check if image size is under 20MB target
SIZE_BYTES=$(docker inspect "$FULL_IMAGE_NAME" --format='{{.Size}}')
SIZE_MB=$((SIZE_BYTES / 1024 / 1024))

if [ $SIZE_MB -le 20 ]; then
    echo -e "${GREEN}✓ Image size ($SIZE_MB MB) meets target (<20MB)${NC}"
else
    echo -e "${YELLOW}⚠ Image size ($SIZE_MB MB) exceeds target (20MB)${NC}"
fi

# Test the image
echo -e "${YELLOW}Testing image...${NC}"
if docker run --rm "$FULL_IMAGE_NAME" -health-check; then
    echo -e "${GREEN}✓ Health check passed${NC}"
else
    echo -e "${RED}✗ Health check failed${NC}"
    exit 1
fi

# Push if requested
if [ "$PUSH" = true ]; then
    if [ -z "$REGISTRY" ]; then
        echo -e "${RED}✗ Registry not specified for push${NC}"
        exit 1
    fi
    
    echo -e "${YELLOW}Pushing image to registry...${NC}"
    if docker push "$FULL_IMAGE_NAME"; then
        echo -e "${GREEN}✓ Push successful${NC}"
    else
        echo -e "${RED}✗ Push failed${NC}"
        exit 1
    fi
fi

echo -e "${GREEN}✓ Docker build completed successfully${NC}"
echo -e "${GREEN}Image: $FULL_IMAGE_NAME${NC}"
echo -e "${GREEN}Size: $IMAGE_SIZE${NC}"
