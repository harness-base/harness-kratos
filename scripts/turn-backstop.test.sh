#!/usr/bin/env bash
# turn-backstop 安全性自测（不调 Haiku）：递归 guard / 不触发静默 / 永不阻断 / 诊断留痕。
# 用 BACKSTOP_CNT/BASE/LOG/DLOG 指向临时文件，hermetic、不碰真实运行态（含诊断日志）。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
pass=0; fail=0
ok(){ pass=$((pass+1)); }
no(){ echo "  ✗ $1"; fail=$((fail+1)); }

tmp="$(mktemp -d)"
export BACKSTOP_CNT="$tmp/cnt" BACKSTOP_BASE="$tmp/base" BACKSTOP_LOG="$tmp/log" BACKSTOP_DLOG="$tmp/dlog"
: > "$BACKSTOP_LOG"

# 1) 递归 guard：HARNESS_TRIAGE=1 秒退、不写状态
HARNESS_TRIAGE=1 bash scripts/turn-backstop.sh /dev/null >/dev/null 2>&1; rc=$?
{ [ "$rc" -eq 0 ] && [ ! -f "$BACKSTOP_CNT" ]; } && ok || no "递归 guard 未挡住（rc=$rc 或写了状态）"

# 2) 不触发（K/阈值拉大 + baseline 对齐）→ 静默、log 不动、exit 0
printf '%s\n%s\n%s\n' 0 "$(git rev-parse HEAD 2>/dev/null)" 99999 > "$BACKSTOP_BASE"
BACKSTOP_TURNS=999999 BACKSTOP_CHANGED=999999 bash scripts/turn-backstop.sh /dev/null >/dev/null 2>&1; rc=$?
{ [ "$rc" -eq 0 ] && [ ! -s "$BACKSTOP_LOG" ]; } && ok || no "不触发场景异常（rc=$rc 或 log 被写）"

# 3) 永不阻断：transcript 缺失也 exit 0（绝不卡收尾）
rm -f "$BACKSTOP_CNT" "$BACKSTOP_BASE"
bash scripts/turn-backstop.sh /nonexistent/path >/dev/null 2>&1
[ "$?" -eq 0 ] && ok || no "transcript 缺失时未 exit 0（会阻断收尾）"

# 4) 单一来源接通：turn-backstop 引用文档同步对照表，且能取到「🔴手」行（≥5）
#    断了（文件改名 / 标记换 / 行被删空）= Haiku 判据变空、静默退化，必须红。
cm="$(grep -E '^\|.*🔴' "$ROOT/docs/harness/doc-sync-checklist.md" 2>/dev/null | wc -l | tr -d ' ')"
{ grep -q 'docs/harness/doc-sync-checklist.md' "$ROOT/scripts/turn-backstop.sh" && [ "${cm:-0}" -ge 5 ]; } \
  && ok || no "文档同步对照表未接通（turn-backstop 没引用 或 🔴行<5：取到 ${cm:-0} 行）"

# 5) 诊断留痕：未触发也写 DLOG（看得见"为何没产出"）——dlog 失效则 DLOG 空、翻红（变异自证）
printf '%s\n%s\n%s\n' 0 "$(git rev-parse HEAD 2>/dev/null)" 99999 > "$BACKSTOP_BASE"
BACKSTOP_DLOG="$tmp/dlog5" BACKSTOP_TURNS=999999 BACKSTOP_CHANGED=999999 bash scripts/turn-backstop.sh /dev/null >/dev/null 2>&1
{ [ -s "$tmp/dlog5" ] && grep -q 'skip 未触发' "$tmp/dlog5"; } && ok || no "诊断日志未记录未触发原因（dlog 失效 = 静默失效仍无迹可循）"

# 6) 诊断日志写的是 DLOG、不是 optimization-log（不能污染真账本）——未触发时 BACKSTOP_LOG 仍空
[ ! -s "$tmp/dlog5" ] && no "用例5 前置坏了（dlog5 该非空）"
{ [ ! -s "$BACKSTOP_LOG" ]; } && ok || no "诊断日志误写进 optimization-log（应只写 DLOG）"

rm -rf "$tmp"
echo "turn-backstop.test: pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
