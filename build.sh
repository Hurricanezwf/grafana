#!/bin/bash
set -e

# Grafana 编译脚本

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "==> 生成 wire 依赖注入代码..."
make gen-go

echo "==> 编译后端..."
make build-backend

echo "==> 编译完成！"
ls -lah bin/linux-amd64/

cp bin/linux-amd64/*  /usr/share/grafana/bin/
