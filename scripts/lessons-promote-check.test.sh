#!/usr/bin/env bash
# lessons-promote-check 自测：只数无 <!-- opt: --> 标记的 lesson；标记过的（seen/skip/rule）不算；缺文件→0。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
pass=0; fail=0
ok(){ pass=$((pass+1)); }
no(){ echo "  ✗ $1"; fail=$((fail+1)); }

tmp="$(mktemp -d)"; f="$tmp/lessons.md"

# 1) 混合：5 条标题，3 条已标记 → pending 应为 2（load-bearing：若不看标记会得 5）
cat > "$f" <<'EOF'
# 错题本
## 2026-01-01：未整理一
## 2026-01-02：未整理二
## 2026-01-03：已提醒 <!-- opt: seen -->
## 2026-01-04：跳过 <!-- opt: skip -->
## 2026-01-05：已升 <!-- opt: rule-0099 -->
EOF
n="$(LESSONS_FILE="$f" bash scripts/lessons-promote-check.sh 2>/dev/null)"
[ "$n" = "2" ] && ok || no "pending 计数错：期望 2，得 '$n'"

# 2) 全标记 → 0
cat > "$f" <<'EOF'
## 2026-01-01：a <!-- opt: skip -->
## 2026-01-02：b <!-- opt: rule-0001 -->
EOF
n="$(LESSONS_FILE="$f" bash scripts/lessons-promote-check.sh 2>/dev/null)"
[ "$n" = "0" ] && ok || no "全标记应 0，得 '$n'"

# 3) 缺文件 → 0、exit 0（不报错）
n="$(LESSONS_FILE="$tmp/nope.md" bash scripts/lessons-promote-check.sh 2>/dev/null)"; rc=$?
{ [ "$rc" -eq 0 ] && [ "$n" = "0" ]; } && ok || no "缺文件应 0/exit0，得 '$n'/rc=$rc"

rm -rf "$tmp"
echo "lessons-promote-check.test: pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
