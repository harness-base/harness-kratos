#!/usr/bin/env bash
# 每轮落文档提醒（capture，①；≠ 自进化审查②=hc-self-evolution skill）：机械触发（K 轮 /
# commit 边界 / 变更文件数增量）→ headless Haiku 复查最近对话，把"做了决策 / 学了偏好 /
# 有知识却没写进文档"捞进 tasks/optimization-log.md，提醒落文档。
#
# 设计：脚本管 WHEN（确定性触发，全在下面常量+逻辑），Haiku 管 WHAT（只在触发后判该记什么）。
# 全程 best-effort：任何失败一律 exit 0，绝不阻断收尾。
# 用法：scripts/turn-backstop.sh <transcript_path>
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT" || exit 0

# 递归保险：headless triage 自己触发的钩子带 HARNESS_TRIAGE=1，直接放行
[ -n "${HARNESS_TRIAGE:-}" ] && exit 0

# ---- 可调常量（触发逻辑全在这里）----
K="${BACKSTOP_TURNS:-8}"                       # 每 K 轮兜底一次
CHANGED_THRESHOLD="${BACKSTOP_CHANGED:-10}"    # 自上次兜底以来"变更文件数增量" ≥ 此值就兜底（涨多少，非绝对值）
MODEL="${BACKSTOP_MODEL:-claude-haiku-4-5-20251001}"
BUDGET="${BACKSTOP_BUDGET:-0.03}"
HL_TIMEOUT="${BACKSTOP_TIMEOUT:-60}"           # headless 超时（秒，perl alarm，本机无 timeout）
TAIL_BYTES="${BACKSTOP_TAIL_BYTES:-12000}"     # 喂给 Haiku 的 transcript 末尾字节数（按字节截，避免 JSONL 行过大→prompt 过长）

TRANSCRIPT="${1:-}"
CNT_FILE="${BACKSTOP_CNT:-$ROOT/tasks/.turn-count}"
BASE_FILE="${BACKSTOP_BASE:-$ROOT/tasks/.last-backstop}"   # 第1行=上次兜底轮号, 第2行=上次 HEAD, 第3行=上次变更文件数
LOG="${BACKSTOP_LOG:-$ROOT/tasks/optimization-log.md}"

# ---- 轮数 +1 ----
turns=0; [ -f "$CNT_FILE" ] && turns="$(cat "$CNT_FILE" 2>/dev/null || echo 0)"
case "$turns" in ''|*[!0-9]*) turns=0 ;; esac
turns=$((turns + 1)); echo "$turns" > "$CNT_FILE" 2>/dev/null || true

# ---- 上次兜底基线 ----
last_turn=0; last_sha=""; last_changed=0
if [ -f "$BASE_FILE" ]; then
  last_turn="$(sed -n '1p' "$BASE_FILE" 2>/dev/null || echo 0)"
  last_sha="$(sed -n '2p' "$BASE_FILE" 2>/dev/null || echo '')"
  last_changed="$(sed -n '3p' "$BASE_FILE" 2>/dev/null || echo 0)"
fi
case "$last_turn" in ''|*[!0-9]*) last_turn=0 ;; esac
case "$last_changed" in ''|*[!0-9]*) last_changed=0 ;; esac
head_sha="$(git rev-parse HEAD 2>/dev/null || echo '')"
changed="$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')"
case "$changed" in ''|*[!0-9]*) changed=0 ;; esac

# ---- 触发判定（机械、确定性）----
fire=""; reason=""
[ $((turns - last_turn)) -ge "$K" ] && { fire=1; reason="K=$K轮到点"; }
if [ -n "$head_sha" ] && [ -n "$last_sha" ] && [ "$head_sha" != "$last_sha" ]; then fire=1; reason="$reason commit边界"; fi
[ $((changed - last_changed)) -ge "$CHANGED_THRESHOLD" ] && { fire=1; reason="$reason 变更涨=$((changed-last_changed))≥${CHANGED_THRESHOLD}"; }
[ -z "$fire" ] && exit 0   # 没到触发条件，静默

# ---- 触发：headless Haiku 兜底（best-effort）----
[ -f "$TRANSCRIPT" ] || exit 0
slice="$(tail -c "$TAIL_BYTES" "$TRANSCRIPT" 2>/dev/null || true)"
[ -z "$slice" ] && exit 0

# 单一来源：从文档同步对照表取「🔴手」行（无机器兜底、只能人改的同步点）当判据，
# 不在本脚本里自抄一份子集（否则会各自漂）。取不到就退化为通用判据，仍 best-effort。
checkmap="$(grep -E '^\|.*🔴' "$ROOT/docs/harness/doc-sync-checklist.md" 2>/dev/null || true)"

instr="你是一个 AI 编码 agent 最近对话的【独立兜底复查员】。下面是最近若干轮对话(JSONL transcript 末尾片段)。
找出本该写入持久文件、但可能没写的东西：(a)做出的决策 (b)用户表达的偏好/工作方式 (c)取舍理由/被否决的方案 (d)踩坑/教训 (e)该进文档/AGENTS.md/规则的知识 (f)动了文件但没同步对应文档——**逐行对照下面的【漂移对照表】判断**：每行是「改了左边→须查右边是否跟改」，且都是无机器兜底、只能人手同步的点（机器能兜的[skills-index/rules-index/dir-index/prds/shim]已被 make verify 拦，不在表里、不用你管）。
规则：若 transcript 显示这些已写进文件(有 Write/Edit/git 操作)，就别再报。低噪声、只报真遗漏。
输出：每条一行，格式 [类别] 一句话；若无遗漏，只输出 NONE。
--- 漂移对照表（改左边→查右边；来源 docs/harness/doc-sync-checklist.md）---
$checkmap
--- transcript 片段 ---
$slice"

result="$(printf '%s' "$instr" | ( cd /tmp && HARNESS_TRIAGE=1 perl -e 'alarm shift @ARGV; exec @ARGV' "$HL_TIMEOUT" \
  claude -p --model "$MODEL" --max-budget-usd "$BUDGET" ) 2>/dev/null || true)"

# ---- 重置基线（无论结果，避免重复触发）----
printf '%s\n%s\n%s\n' "$turns" "$head_sha" "$changed" > "$BASE_FILE" 2>/dev/null || true

# ---- 记录有遗漏的 ----
# 只认形如 "[类别] ..." 的发现行；NONE / 报错(如 Prompt is too long) / 空 一律不记
findings="$(printf '%s\n' "$result" | grep '^\[' 2>/dev/null || true)"
if [ -n "$findings" ]; then
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  # 每条带 `- [ ]` 复选框 = 状态(待处理)；处理完改 `- [x]`、暂缓改 `- [~]`。
  # UserPromptSubmit 钩子(correction-nudge.sh)下一轮会把"有 N 条待处理"注入给主 agent(反馈通道)。
  { echo ""; echo "## $ts \`落文档\`（触发：$reason）"; printf '%s\n' "$findings" | sed 's/^/- [ ] /'; } >> "$LOG" 2>/dev/null || true
  echo "ℹ 落文档提醒：捞到可能没写进文档的决策/知识/文档漂移，已记 tasks/optimization-log.md(标 - [ ] 待处理)——下一轮会反馈给你；落到对应文档后把该行改 - [x]。" >&2
fi
exit 0
