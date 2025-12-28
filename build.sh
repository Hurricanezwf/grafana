#!/bin/bash
set -e

# Grafana 编译脚本 (CentOS 7.9 兼容)
# 使用 Docker 多阶段构建，生成的二进制可在 CentOS 7.9+ 上运行

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

IMAGE_NAME="grafana-centos7"
OUTPUT_DIR="./bin"

echo "==> 构建 CentOS 7 兼容的 Docker 镜像..."
docker build -f Dockerfile.centos7 -t "$IMAGE_NAME" .

echo "==> 创建输出目录..."
mkdir -p "$OUTPUT_DIR"

echo "==> 从容器中提取二进制文件..."
docker run --rm -v "$(pwd)/$OUTPUT_DIR:/host-bin" "$IMAGE_NAME"

echo "==> 编译完成！二进制文件位于 $OUTPUT_DIR/"
ls -la "$OUTPUT_DIR"
