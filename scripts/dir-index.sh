#!/usr/bin/env bash
# 通用目录索引生成器：扫 <dir> 下的 *.md，取每个文件的标题（frontmatter title/name，
# 退化到首个 # 标题）生成 <dir>/README.md 索引。防手维护漂移；自检/自进化时当"地图"。
# 用法：scripts/dir-index.sh <dir>          # 重生成（禁手改 README）
#       scripts/dir-index.sh <dir> --check   # 只比对，漂移则非零退出（进 make verify）
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
dir="${1:?用法: dir-index.sh <dir> [--check]}"
dir="${dir%/}"
mode="${2:-}"
out="$dir/README.md"

gen() {
  echo "# $(basename "$dir") 索引"
  echo ""
  echo "> 由 \`scripts/dir-index.sh\` 自动生成、禁手改。默认不加载；自检 / 自进化时当地图查。"
  echo ""
  for f in "$dir"/*.md; do
    [ -e "$f" ] || continue
    base="$(basename "$f")"
    [ "$base" = "README.md" ] && continue
    title="$(sed -nE 's/^(title|name):[[:space:]]*//p' "$f" 2>/dev/null | head -1)"
    [ -z "$title" ] && title="$(sed -nE 's/^#[[:space:]]+//p' "$f" 2>/dev/null | head -1)"
    echo "- \`$base\` — ${title:-$base}"
  done
}

if [ "$mode" = "--check" ]; then
  tmp="$(mktemp)"; gen > "$tmp"
  if diff -q "$tmp" "$out" >/dev/null 2>&1; then echo "  ✓ $dir 索引无漂移"; rm -f "$tmp"
  else echo "  ✗ $dir 索引漂移 → 跑 scripts/dir-index.sh $dir 重生成"; diff "$out" "$tmp" 2>/dev/null | head -10; rm -f "$tmp"; exit 1; fi
else
  gen > "$out"; echo "✓ 生成 $out"
fi
