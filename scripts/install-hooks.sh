#!/usr/bin/env bash
# 安装 git hooks：把 core.hooksPath 指向 .githooks。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
if [ ! -d .git ]; then
  echo "当前不是 git 仓，请先 git init 再运行 make hooks"; exit 1
fi
git config core.hooksPath .githooks
chmod +x .githooks/* 2>/dev/null || true
echo "✓ 已设 core.hooksPath -> .githooks"
