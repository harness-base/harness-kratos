#!/usr/bin/env bash
# 检查 eval 资产结构：核心文件在、index 登记的考题都存在、prompts 都登记了。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EVAL="$ROOT/docs/eval"
fail=0

for x in README.md index.yaml rubric.md evaluator.md; do
  [ -f "$EVAL/$x" ] || { echo "  ✗ 缺 docs/eval/$x"; fail=1; }
done

# index 登记的考题文件都存在
while read -r pf; do
  [ -z "$pf" ] && continue
  [ -f "$EVAL/$pf" ] || { echo "  ✗ index 登记的考题不存在：$pf"; fail=1; }
done < <(grep -E '^[[:space:]]*file:[[:space:]]*prompts/' "$EVAL/index.yaml" | sed -E 's/.*file:[[:space:]]*//')

# prompts 里的考题都登记进 index
for f in "$EVAL"/prompts/*.md; do
  [ -e "$f" ] || continue
  base="prompts/$(basename "$f")"
  grep -q "$base" "$EVAL/index.yaml" || { echo "  ✗ 考题未登记进 index：$base"; fail=1; }
done

[ "$fail" -eq 0 ] && echo "✓ eval 资产结构 OK" || { echo "✗ eval 资产有问题"; exit 1; }
