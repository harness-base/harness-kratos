#!/usr/bin/env bash
# 索引一致性检查：<dir>/index.yaml 登记的每个 file: 都存在，且 <dir> 下每个 NNNN-*.md 都被登记。
# 防手维护的 index.yaml 漂移（决策/需求等账本）。用法：scripts/index-audit.sh <dir>
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
dir="${1:?用法: index-audit.sh <dir>}"
dir="${dir%/}"
idx="$dir/index.yaml"
fail=0
[ -f "$idx" ] || { echo "  ✗ 缺 $idx"; exit 1; }

# 正向：index 登记的 file: 都存在
while IFS= read -r f; do
  [ -z "$f" ] && continue
  [ -f "$dir/$f" ] || { echo "  ✗ $idx 登记的 $f 不存在"; fail=1; }
done < <(grep -E '^[[:space:]]*file:[[:space:]]*' "$idx" | sed -E 's/^[[:space:]]*file:[[:space:]]*//; s/[[:space:]]*$//' | tr -d '"')

# 反向：dir 下每个 NNNN-*.md 都被登记
for f in "$dir"/[0-9]*.md; do
  [ -e "$f" ] || continue
  base="$(basename "$f")"
  grep -qF "$base" "$idx" || { echo "  ✗ $base 未登记进 $idx"; fail=1; }
done

[ "$fail" -eq 0 ] && echo "  ✓ $dir 索引一致"
exit "$fail"
