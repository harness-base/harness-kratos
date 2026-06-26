#!/usr/bin/env bash
# 文档自检：校验各 .md frontmatter 里 source_files / related_docs 指向的目标是否存在。
# 路径相对该文档所在目录解析。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
fail=0
checked=0

while IFS= read -r f; do
  head -1 "$f" | grep -q '^---$' || continue   # 只查带 frontmatter 的
  dir="$(dirname "$f")"
  entries="$(awk '
    NR==1 && $0=="---"{infm=1; next}
    infm && $0=="---"{exit}
    infm && /^(source_files|related_docs):/{key=1; next}
    infm && key && /^[A-Za-z_]+:/{key=0}
    infm && key && /^[[:space:]]*-[[:space:]]*/{ s=$0; sub(/^[[:space:]]*-[[:space:]]*/,"",s); print s }
  ' "$f")"
  while IFS= read -r ref; do
    [ -z "$ref" ] && continue
    ref="$(printf '%s' "$ref" | sed "s/^[\"']//;s/[\"']$//;s/[[:space:]]*$//")"
    [ -z "$ref" ] && continue
    if [ ! -e "$dir/$ref" ]; then
      echo "  ✗ $f → 引用不存在：$ref"
      fail=1
    fi
  done <<< "$entries"
  checked=$((checked+1))
done < <(find docs -name '*.md' -type f | sort)

if [ "$fail" -eq 0 ]; then
  echo "✓ docs-audit 通过（检查了 $checked 篇带 frontmatter 的文档）"
else
  echo "✗ docs-audit 失败"; exit 1
fi
