#!/usr/bin/env bash
# 数 tasks/lessons.md 里"还没整理过"的 lesson：标题行 ^## 20YY… 且行尾无 <!-- opt: … --> 整理标记。
# 输出一个数字（pending 条数），供 correction-nudge 钩子判断要不要提醒用户整理（step 4）。
# 标记约定见 tasks/lessons.md 头部：无标记=未整理 / opt: seen=提醒过待决定 / opt: skip=不升 / opt: rule-00NN=已升。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LESSONS="${LESSONS_FILE:-$ROOT/tasks/lessons.md}"
[ -f "$LESSONS" ] || { echo 0; exit 0; }
pending="$(grep '^## 20[0-9][0-9]' "$LESSONS" 2>/dev/null | grep -vc '<!-- opt:' || true)"
case "$pending" in ''|*[!0-9]*) pending=0 ;; esac
echo "$pending"
exit 0
