#!/usr/bin/env bash
# 研发方案账本自检：docs/designs/<dir>/ 与 docs/designs/index.yaml 登记双向一致 +
#   每个 design 目录有 design.md（必需）+ 定稿零 TBD 机检（design.md / api-contract.md）。
# 防"加了设计忘登记 / 缺主文档 / 留 TBD 占位就当定稿"。空账本（designs: []）时平凡通过。
# 零 TBD 机检 = hc-design 产出门槛（定稿必须可执行、零 TBD/待确认，rule-0008 / ADR-0015）的机检兜底；
#   只扫 design 目录内文件，不碰 templates/（模板里的占位 <…> 与 README 的说明文字不在校验范围）。
# 质量（方案是否真合理 / 接口字段是否对齐）由 hc-design-reviewer 对抗评审 + eval 判，不在本机器校验内。
# 格式无关：不依赖具体接口形态（REST / gRPC / 消息皆可）；只校验登记一致 + 文件在 + 无 TBD 占位词。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
DESIGNS_DIR="${DESIGNS_DIR:-docs/designs}"   # 可被自测以环境变量覆盖，隔离真实账本
IDX="$DESIGNS_DIR/index.yaml"
fail=0
[ -f "$IDX" ] || { echo "  ✗ 缺 $IDX"; exit 1; }

# 已登记的目录名（index.yaml 里各条目的 dir: 字段）
registered="$(grep -E '^[[:space:]]*dir:[[:space:]]*' "$IDX" | sed -E 's/^[[:space:]]*dir:[[:space:]]*//; s/[[:space:]]*$//' | tr -d '"' || true)"

# 定稿零 TBD：扫 design.md + api-contract.md（若有）里的占位词 → 命中即红。
# 词表锚定常见占位：TBD / 待确认 / 待定 / FIXME（大小写不敏感）。
tbd_check(){ # $1=file $2=name(目录名，仅报错用)
  local f="$1" name="$2"
  [ -f "$f" ] || return 0
  if grep -qiE 'TBD|待确认|待定|待补|FIXME|TODO|留待实现' "$f"; then
    echo "  ✗ $name/$(basename "$f") 含 TBD/待确认/待定/FIXME 占位（定稿须零 TBD、可执行，ADR-0015）"; fail=1
  fi
}

# 正向：实际存在的 design 目录必须登记 + 含 design.md + 零 TBD
for d in "$DESIGNS_DIR"/*/; do
  [ -d "$d" ] || continue
  name="$(basename "$d")"
  printf '%s\n' "$registered" | grep -qxF "$name" || { echo "  ✗ design 目录 $name 未登记进 $IDX"; fail=1; }
  if [ ! -f "$d/design.md" ]; then
    echo "  ✗ $name 缺 design.md（研发方案主文档必需）"; fail=1; continue
  fi
  tbd_check "$d/design.md" "$name"
  tbd_check "$d/api-contract.md" "$name"   # 接口契约可选（无则跳过），有则一并查
done

# 反向：登记了但目录不存在
while IFS= read -r name; do
  [ -z "$name" ] && continue
  [ -d "$DESIGNS_DIR/$name" ] || { echo "  ✗ 登记的 design 目录 $name 不存在"; fail=1; }
done <<< "$registered"

[ "$fail" -eq 0 ] && echo "  ✓ 研发方案账本一致（登记双向一致 + design.md 在 + 零 TBD）"
exit "$fail"
