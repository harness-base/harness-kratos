#!/usr/bin/env bash
# PRD 账本自检：docs/prds/<dir>/ 与 docs/prds/index.yaml 登记一致 + prd.md 含必备章节。
# 防"加了 PRD 忘登记 / 缺关键章节"。空账本（prds: []）时平凡通过。
# 质量（验收可观测/原型可点通等）由 eval 考题 013 / rule-0010 判，不在本机器校验内。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
PRDS_DIR="docs/prds"
IDX="$PRDS_DIR/index.yaml"
fail=0
[ -f "$IDX" ] || { echo "  ✗ 缺 $IDX"; exit 1; }

# 已登记的目录名（index.yaml 里各条目的 dir: 字段）
registered="$(grep -E '^[[:space:]]*dir:[[:space:]]*' "$IDX" | sed -E 's/^[[:space:]]*dir:[[:space:]]*//; s/[[:space:]]*$//' | tr -d '"' || true)"

# 正向：实际存在的 PRD 目录必须登记 + 含 prd.md + 必备章节
for d in "$PRDS_DIR"/*/; do
  [ -d "$d" ] || continue
  name="$(basename "$d")"
  # ADR-0007 第2步合法中间态：只有 user-stories.md、还没产 prd.md（第3步才登记 index）→ 跳过登记/章节校验
  if [ ! -f "$d/prd.md" ]; then
    [ -f "$d/user-stories.md" ] && continue
    echo "  ✗ $name 既无 prd.md 也无 user-stories.md"; fail=1; continue
  fi
  printf '%s\n' "$registered" | grep -qxF "$name" || { echo "  ✗ PRD 目录 $name 未登记进 $IDX"; fail=1; }
  for sec in "## 范围" "## 功能点清单" "## 页面与流程" "## 状态"; do
    grep -qF "$sec" "$d/prd.md" || { echo "  ✗ $name/prd.md 缺章节：$sec"; fail=1; }
  done
  # 用户故事是 PRD 的上游事实视角（ADR-0007）：有 prd.md 就该有 user-stories.md
  [ -f "$d/user-stories.md" ] || { echo "  ✗ $name 缺 user-stories.md（PRD 须以已确认用户故事为上游，ADR-0007）"; fail=1; }
done

# 反向：登记了但目录不存在
while IFS= read -r name; do
  [ -z "$name" ] && continue
  [ -d "$PRDS_DIR/$name" ] || { echo "  ✗ 登记的 PRD 目录 $name 不存在"; fail=1; }
done <<< "$registered"

[ "$fail" -eq 0 ] && echo "  ✓ PRD 账本一致"
exit "$fail"
