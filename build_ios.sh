#!/bin/bash
# ==============================================================================
# 裁剪版 Xray 核心 iOS 静态库编译脚本
# 仅保留 VLESS/VMess 协议，最小化内存占用
# 依赖：macOS + Xcode 15+ + Go 1.22+
# ==============================================================================

set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 项目目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SCRIPT_DIR"
OUTPUT_DIR="$PROJECT_DIR/build"
XCFRAMEWORK_DIR="$OUTPUT_DIR/Xray.xcframework"

# Go 相关设置
export GOOS=ios
export GOARCH=arm64
export CGO_ENABLED=1
export CC="$(xcrun --sdk iphoneos --find clang)"
export CXX="$(xcrun --sdk iphoneos --find clang++)"
export SDKROOT="$(xcrun --sdk iphoneos --show-sdk-path)"

# iOS 最低版本
export IPHONEOS_DEPLOYMENT_TARGET="16.0"

# CFLAGS：指定架构和 SDK
export CGO_CFLAGS="-arch arm64 -isysroot $SDKROOT -miphoneos-version-min=16.0 -fembed-bitcode"
export CGO_LDFLAGS="-arch arm64 -isysroot $SDKROOT -miphoneos-version-min=16.0"

echo -e "${GREEN}=== 开始编译裁剪版 Xray iOS 静态库 ===${NC}"
echo "Go版本: $(go version)"
echo "SDK: $SDKROOT"
echo "输出目录: $OUTPUT_DIR"

# 清理旧产物
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# 编译真机静态库
echo -e "${YELLOW}编译真机 arm64 静态库...${NC}"
cd "$PROJECT_DIR"
go build -buildmode=c-archive -o "$OUTPUT_DIR/libxray-ios-arm64.a" -trimpath -ldflags "-s -w" .

echo -e "${GREEN}真机静态库编译完成: $(du -sh $OUTPUT_DIR/libxray-ios-arm64.a | cut -f1)${NC}"

# 编译模拟器静态库（arm64 + x86_64）
echo -e "${YELLOW}编译模拟器 arm64 静态库...${NC}"
export GOOS=ios
export GOARCH=arm64
export SDKROOT="$(xcrun --sdk iphonesimulator --show-sdk-path)"
export CGO_CFLAGS="-arch arm64 -isysroot $SDKROOT -miphonesimulator-version-min=16.0 -fembed-bitcode"
export CGO_LDFLAGS="-arch arm64 -isysroot $SDKROOT -miphonesimulator-version-min=16.0"
go build -buildmode=c-archive -o "$OUTPUT_DIR/libxray-ios-sim-arm64.a" -trimpath -ldflags "-s -w" .

echo -e "${YELLOW}编译模拟器 x86_64 静态库...${NC}"
export GOARCH=amd64
export CGO_CFLAGS="-arch x86_64 -isysroot $SDKROOT -miphonesimulator-version-min=16.0 -fembed-bitcode"
export CGO_LDFLAGS="-arch x86_64 -isysroot $SDKROOT -miphonesimulator-version-min=16.0"
go build -buildmode=c-archive -o "$OUTPUT_DIR/libxray-ios-sim-x86_64.a" -trimpath -ldflags "-s -w" .

# 合并模拟器静态库
echo -e "${YELLOW}合并模拟器静态库...${NC}"
lipo -create "$OUTPUT_DIR/libxray-ios-sim-arm64.a" "$OUTPUT_DIR/libxray-ios-sim-x86_64.a" -output "$OUTPUT_DIR/libxray-ios-simulator.a"

echo -e "${GREEN}模拟器静态库编译完成: $(du -sh $OUTPUT_DIR/libxray-ios-simulator.a | cut -f1)${NC}"

# 创建 XCFramework
echo -e "${YELLOW}创建 XCFramework...${NC}"

# 创建单独的头文件目录（避免 -headers 复制整个目录导致静态库冲突）
HEADERS_DIR="$OUTPUT_DIR/headers"
mkdir -p "$HEADERS_DIR"
cp "$OUTPUT_DIR/libxray-ios-arm64.h" "$HEADERS_DIR/libxray.h"

# 先删除可能存在的旧 XCFramework
rm -rf "$XCFRAMEWORK_DIR"

xcodebuild -create-xcframework \
    -library "$OUTPUT_DIR/libxray-ios-arm64.a" \
    -headers "$HEADERS_DIR" \
    -library "$OUTPUT_DIR/libxray-ios-simulator.a" \
    -headers "$HEADERS_DIR" \
    -output "$XCFRAMEWORK_DIR"

echo -e "${GREEN}=== 编译完成 ===${NC}"
echo "XCFramework: $XCFRAMEWORK_DIR"

# 自动检测真机和模拟器目录名（xcodebuild 可能使用不同命名）
真机目录=$(find "$XCFRAMEWORK_DIR" -maxdepth 1 -type d -name "ios-arm64" -o -name "ios-arm64_*" | grep -v simulator | head -1)
模拟器目录=$(find "$XCFRAMEWORK_DIR" -maxdepth 1 -type d -name "*simulator*" | head -1)

if [ -n "$真机目录" ] && [ -f "$真机目录/libxray.a" ]; then
    echo "真机版本: $(du -sh $真机目录/libxray.a | cut -f1)"
else
    echo "⚠️  未找到真机静态库，XCFramework 目录结构："
    ls -la "$XCFRAMEWORK_DIR/"
fi

if [ -n "$模拟器目录" ] && [ -f "$模拟器目录/libxray.a" ]; then
    echo "模拟器版本: $(du -sh $模拟器目录/libxray.a | cut -f1)"
else
    echo "⚠️  未找到模拟器静态库"
fi

echo ""
echo "使用方法：将 Xray.xcframework 拖入 Xcode 项目，链接到 VPN 扩展 target"
