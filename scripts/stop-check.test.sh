#!/usr/bin/env bash
# stop-check eval 闸自测（hermetic：隔离 todo/reviews/backstop 状态，不调 Haiku）。
# 不变量：闸只在 level≥L2 且 todo 有 ## Review 段（=收尾，rule-0013）且无对应 eval 产出时拦（exit 2）；
#         任务进行中（无 Review）不拦（exit 0）——本次修复的核心（lessons 2026-06-27）。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
pass=0; fail=0
ok(){ pass=$((pass+1)); }
no(){ echo "  ✗ $1"; fail=$((fail+1)); }

tmp="$(mktemp -d)"; rev="$tmp/reviews"; mkdir -p "$rev"
# 跑一次 stop-check 返回 exit code；隔离真状态，并压死 turn-backstop 触发（阈值拉爆，不调 Haiku）
run(){ printf '{}' | STOP_TODO="$1" STOP_REVIEWS_DIR="$rev" \
  BACKSTOP_CNT="$tmp/c" BACKSTOP_BASE="$tmp/b" BACKSTOP_LOG="$tmp/l" \
  BACKSTOP_TURNS=999999 BACKSTOP_CHANGED=999999 \
  bash scripts/stop-check.sh >/dev/null 2>&1; echo $?; }

# 1) L3 + 无 Review（进行中）→ 不拦（exit 0）。核心：mid-task 不误拦。
printf '> 元：level: L3 ｜ task: foo\n## 当前\n- [ ] x\n' > "$tmp/t1.md"
[ "$(run "$tmp/t1.md")" = "0" ] && ok || no "L3 无 Review 被误拦（mid-task 不该拦）"

# 2) L3 + 有 Review + 无 eval → 拦（exit 2）。收尾该拦。（与 1 仅差 Review，证明 Review 条件 load-bearing）
printf '> 元：level: L3 ｜ task: foo\n## Review\n- 小结\n' > "$tmp/t2.md"
[ "$(run "$tmp/t2.md")" = "2" ] && ok || no "L3+Review 无 eval 未拦（收尾该拦）"

# 3) L3 + 有 Review + 有对应 eval 产出 → 放行（exit 0）。
mkdir -p "$rev/20260101T0000Z-foo"
[ "$(run "$tmp/t2.md")" = "0" ] && ok || no "L3+Review 有 eval 仍拦（不该）"

# 4) L1 + 有 Review → 放行（低档不要 eval）。
printf '> 元：level: L1 ｜ task: bar\n## Review\n' > "$tmp/t4.md"
[ "$(run "$tmp/t4.md")" = "0" ] && ok || no "L1 被拦（低档不该要 eval）"

# 5) L2 边界 + 有 Review + 无 eval → 拦（exit 2）。守住 rule-0005 "L2 以上"下边界（杀 -ge 2→-ge 3 变异）。
printf '> 元：level: L2 ｜ task: baz\n## Review\n- 小结\n' > "$tmp/t5.md"
[ "$(run "$tmp/t5.md")" = "2" ] && ok || no "L2+Review 无 eval 未拦（L2 是闸门下边界，该拦）"

# 6) "Review" 仅在正文（非 ## 标题）→ 不拦（exit 0）。钉死 ^## 标题锚定（杀正则弱化成裸 Review）。task=foo2 无 eval，确保是 finishing_now=false 放行而非 eval 存在放行。
printf '> 元：level: L3 ｜ task: foo2\n## 当前\n- [ ] 待补 Review 段\n' > "$tmp/t6.md"
[ "$(run "$tmp/t6.md")" = "0" ] && ok || no "正文含 Review（非##标题）被误拦（应只认 ## 标题）"

# 7) 多块：当前节无 Review + 暂挂块有旧 Review → 不拦（exit 0）。钉死"只看当前节"（杀全文件 grep 回归）。task=foo2 无 eval。
printf '> 元：level: L3 ｜ task: foo2\n## 当前\n- [ ] 实现中\n\n## 暂挂\n旧任务\n## Review\n- 旧小结\n' > "$tmp/t7.md"
[ "$(run "$tmp/t7.md")" = "0" ] && ok || no "暂挂块旧 Review 让闸 mid-task 误拦（只该看当前节）"

# 8) 中文收尾段标题（## 评审）+ 无 eval → 拦（exit 2）。善意写的收尾段不该被静默放行。
printf '> 元：level: L3 ｜ task: baz\n## 评审\n- 小结\n' > "$tmp/t8.md"
[ "$(run "$tmp/t8.md")" = "2" ] && ok || no "## 评审 收尾段未拦（收尾段标题认 Review/评审/复盘）"

rm -rf "$tmp"
echo "stop-check.test: pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
