#!/bin/bash

# Build script: Generate executables for all platforms
PROJECT_NAME="ratel-server"
TARGET_DIR="target"

# Create target directory if it doesn't exist
if [ ! -d "$TARGET_DIR" ]; then
    mkdir -p "$TARGET_DIR"
    echo "Created directory: $TARGET_DIR"
fi

# 定义目标平台 (OS/Arch)
PLATFORMS=(
    "windows/amd64"
    "windows/386"
    "windows/arm64"
    "linux/amd64"
    "linux/386"
    "linux/arm64"
    "linux/arm"
    "darwin/amd64"
    "darwin/arm64"
)

echo -e "\033[36mStarting project build: $PROJECT_NAME\033[0m\n"

for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS=${PLATFORM%/*}
    GOARCH=${PLATFORM#*/}
    
    EXTENSION=""
    if [ "$GOOS" == "windows" ]; then
        EXTENSION=".exe"
    fi
    
    OUTPUT_NAME="${PROJECT_NAME}-${GOOS}-${GOARCH}${EXTENSION}"
    OUTPUT_PATH="${TARGET_DIR}/${OUTPUT_NAME}"
    
    echo -n "Building: ${PLATFORM} ..."
    
    # Set environment variables and build
    export GOOS=$GOOS
    export GOARCH=$goarch
    
    go build -o "$OUTPUT_PATH" main.go 2>/dev/null
    
    if [ $? -eq 0 ]; then
        echo -e " \033[32m[Done]\033[0m -> $OUTPUT_NAME"
    else
        echo -e " \033[31m[Failed]\033[0m"
    fi
done

# 重置环境变量
unset GOOS
unset GOARCH

echo -e "\n\033[33mAll build tasks completed. Output directory: $TARGET_DIR\033[0m"
