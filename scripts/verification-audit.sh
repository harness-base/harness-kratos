#!/usr/bin/env bash
# 被管工程接入点占位自检（ADR-0017）：workspace/verification.yaml 里每个接入点
#   (verify/unit/api/e2e/sandbox) 的值必须是**显式三态**之一——
#     · 真命令（已接实）                        → pass
#     · "PENDING: <为啥空/补的条件>"（待接实）   → ⚠ warn 提醒（不阻断），记得补
#     · "N/A: <理由>"（这项目不需要这个接入点）  → pass
#   红（fail-closed：占位/含糊一律红，不是"不在黑名单就放行"）——静默空 / 纯空格 / 值只是注释(# …) /
#     裸占位词(TODO·TBD·FIXME·XXX·待补·待定·以后补… 大小写不敏感) / 裸 PENDING·N/A(无理由) /
#     纯点或省略号(... · …) / <占位尖括号>。PENDING·N/A 前缀大小写不敏感。
# 目的：新项目接入留的占位"看得见 + 绕不过去"，防"开发过了却没补"。只查**已声明**的字段
#   （字段整条缺省 = 项目没声明，归 hc-onboard-reviewer 判，机检不误伤）。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
YAML="${VERIFY_YAML:-workspace/verification.yaml}"   # 可被自测以环境变量覆盖，隔离真实文件
[ -f "$YAML" ] || { echo "  ✗ 缺 $YAML"; exit 1; }

# 逐行：track 当前工程(name:)；对 verify/unit/api/e2e/sandbox 行取值、剥尾注释与引号、判态。
out="$(awk '
  # 剥值：trim → 整值即注释判空 → 剥行尾注释 → trim → 剥双引号再剥单引号(\047) → 再 trim（堵双/单引号包的纯空格）
  function clean(s){
    gsub(/^[[:space:]]*|[[:space:]]*$/, "", s)
    if (s ~ /^#/) return ""
    sub(/[[:space:]]+#.*$/, "", s)
    gsub(/^[[:space:]]*|[[:space:]]*$/, "", s)
    gsub(/^"|"$/, "", s)
    gsub(/^\047|\047$/, "", s)
    gsub(/^[[:space:]]*|[[:space:]]*$/, "", s)
    return s
  }
  /^[[:space:]]*-[[:space:]]*name:/ { p=$0; sub(/^.*name:[[:space:]]*/, "", p); p=clean(p); next }
  /^[[:space:]]*(verify|unit|api|e2e|sandbox):/ {
    f=$0; sub(/:.*/, "", f); gsub(/[[:space:]]/, "", f)
    v=$0; sub(/^[^:]*:[[:space:]]*/, "", v); v=clean(v)
    U=toupper(v)                                                                            # 前缀/占位词大小写不敏感
    if (U ~ /^PENDING:[[:space:]]*[^[:space:]]/) { print "W\t" p "\t" f "\t" v; next }      # 待接实（有理由）
    if (U ~ /^N\/A:[[:space:]]*[^[:space:]]/)     { next }                                  # 显式不需要
    # 红（fail-closed，占位/含糊一律红；下面命中即红，其余才当真命令）
    if (v == "" \
        || U ~ /^(TODO|TBD|FIXME|XXX|N\/A|PENDING|待补|待接实|待定|待填|以后补|随便|占位)$/ \
        || U ~ /^PENDING:[[:space:]]*$/ || U ~ /^N\/A:[[:space:]]*$/ \
        || v ~ /^\.+$/ || v ~ /^…+$/ || v ~ /[<>]/) { print "R\t" p "\t" f "\t" v; next }
    # 否则=真命令 → pass
  }
' "$YAML")"

fail=0
# 红：静默 / 无理由占位
if printf '%s\n' "$out" | grep -q '^R'; then
  fail=1
  printf '%s\n' "$out" | sed -n 's/^R\t//p' | while IFS="$(printf '\t')" read -r proj field val; do
    echo "  ✗ 工程 $proj 的接入点 $field 是静默/无理由占位（'${val:-空}'）——必须显式：真命令 / 'PENDING: <理由>' / 'N/A: <理由>'（ADR-0017）"
  done
fi
# 黄：PENDING 待接实（提醒、不阻断）
if printf '%s\n' "$out" | grep -q '^W'; then
  printf '%s\n' "$out" | sed -n 's/^W\t//p' | while IFS="$(printf '\t')" read -r proj field val; do
    echo "  ⚠ 工程 $proj 的接入点 $field 待接实（$val）——记得补；补 sandbox 用 create-sandbox skill"
  done
fi

[ "$fail" -eq 0 ] && echo "  ✓ 接入点占位显式（真命令 / PENDING 有理由 / N/A 有理由，无静默空）"
exit "$fail"
