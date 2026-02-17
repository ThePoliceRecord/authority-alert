#!/bin/bash

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

BUILD_DIR="build"

echo -e "${BLUE}Building camera-recorder with CMake${NC}"

# Create build directory
mkdir -p ${BUILD_DIR}
cd ${BUILD_DIR}

# Configure with CMake
# Note: toolchain is automatically included from cmake/toolchain-riscv64-linux-musl-x86_64.cmake
echo -e "${GREEN}Configuring project...${NC}"
cmake .. -DCMAKE_BUILD_TYPE=Release

# Build
echo -e "${GREEN}Building project...${NC}"
cmake --build . -j$(nproc)

echo -e "${GREEN}Build complete: ${BUILD_DIR}/camera-recorder${NC}"
