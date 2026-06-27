#!/usr/bin/env bash
# correction-nudge 自测（不依赖 Claude Code 运行态）：注入的提醒非空、指向 lessons.md + rule-0011、
# 永远 exit 0（绝不阻断 prompt）、不被空/垃圾 stdin 噎住。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
pass=0; fail=0
ok(){ pass=$((pass+1)); }
no(){ echo "  ✗ $1"; fail=$((fail+1)); }

# 1) 正常 payload：exit 0 + 输出非空 + 指向 lessons.md + 引 rule-0011（load-bearing：少任一就漏提醒）
out="$(printf '{"prompt":"你理解错了"}' | bash scripts/correction-nudge.sh 2>/dev/null)"; rc=$?
{ [ "$rc" -eq 0 ] \
  && printf '%s' "$out" | grep -q 'lessons.md' \
  && printf '%s' "$out" | grep -q 'rule-0011'; } \
  && ok || no "正常输入未注入有效提醒（rc=$rc）"

# 2) 空 stdin：仍 exit 0（绝不阻断收发）
printf '' | bash scripts/correction-nudge.sh >/dev/null 2>&1; [ "$?" -eq 0 ] && ok || no "空 stdin 未 exit 0"

# 3) 垃圾 stdin：仍 exit 0、不报错
printf 'not json at all' | bash scripts/correction-nudge.sh >/dev/null 2>&1; [ "$?" -eq 0 ] && ok || no "垃圾 stdin 未 exit 0"

# 4) 未整理 lesson 超阈值 → 注入"整理"提醒（step 4；load-bearing：漏第二条就抓不到）
tmpd="$(mktemp -d)"; printf '## 2026-01-01：a\n## 2026-01-02：b\n## 2026-01-03：c\n' > "$tmpd/L.md"
out="$(printf '{}' | LESSONS_FILE="$tmpd/L.md" LESSONS_PROMOTE_THRESHOLD=2 bash scripts/correction-nudge.sh 2>/dev/null)"
printf '%s' "$out" | grep -q '整理' && ok || no "pending 超阈值未注入整理提醒"

# 5) 未超阈值 → 不注入整理提醒（证明它真受计数控制，不是恒发）
out="$(printf '{}' | LESSONS_FILE="$tmpd/L.md" LESSONS_PROMOTE_THRESHOLD=99 bash scripts/correction-nudge.sh 2>/dev/null)"
printf '%s' "$out" | grep -q '整理' && no "未超阈值却注入了整理提醒" || ok
rm -rf "$tmpd"

echo "correction-nudge.test: pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
