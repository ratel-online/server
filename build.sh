#!/bin/bash

# 编译脚本：生成全系统版本的可执行程序
PROJECT_NAME="ratel-server"
TARGET_DIR="target"

# 如果 target 目录不存在则创建
if [ ! -d "$TARGET_DIR" ]; then
    mkdir -p "$TARGET_DIR"
    echo "已创建目录: $TARGET_DIR"
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

echo -e "\033[36m开始全系统编译项目: $PROJECT_NAME\033[0m\n"

for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS=${PLATFORM%/*}
    GOARCH=${PLATFORM#*/}
    
    EXTENSION=""
    if [ "$GOOS" == "windows" ]; then
        EXTENSION=".exe"
    fi
    
    OUTPUT_NAME="${PROJECT_NAME}-${GOOS}-${GOARCH}${EXTENSION}"
    OUTPUT_PATH="${TARGET_DIR}/${OUTPUT_NAME}"
    
    echo -n "正在编译: ${PLATFORM} ..."
    
    # 设置环境变量并运行编译
    export GOOS=$GOOS
    export GOARCH=$goarch
    
    go build -o "$OUTPUT_PATH" main.go 2>/dev/null
    
    if [ $? -eq 0 ]; then
        echo -e " \033[32m[完成]\033[0m -> $OUTPUT_NAME"
    else
        echo -e " \033[31m[失败]\033[0m"
    fi
done

# 重置环境变量
unset GOOS
unset GOARCH

echo -e "\n\033[33m所有编译任务已完成，输出目录: $TARGET_DIR\033[0m"
