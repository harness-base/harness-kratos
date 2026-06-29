#!/usr/bin/env bash
# Stop hook：agent 收尾时——若 todo 声明 L2+，必须有对应 eval 评审产出（rule-0005）；并提醒记 lessons。
# Claude Code 在 Stop 事件调用本脚本；exit 2 = 拦住收尾并把 stderr 反馈给 agent，exit 0 = 放行。
set -uo pipefail
# 递归保险：headless 兜底(turn-backstop)自触发的钩子带 HARNESS_TRIAGE=1，直接放行
[ -n "${HARNESS_TRIAGE:-}" ] && exit 0
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TODO="${STOP_TODO:-$ROOT/tasks/todo.md}"
REVIEWS_DIR="${STOP_REVIEWS_DIR:-$ROOT/docs/eval/task-reviews}"

# 防循环：已在 stop-hook 续跑里就直接放行
payload="$(cat 2>/dev/null || true)"
printf '%s' "$payload" | grep -q '"stop_hook_active"[[:space:]]*:[[:space:]]*true' && exit 0
transcript="$(printf '%s' "$payload" | sed -nE 's/.*"transcript_path"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' | head -1)"

[ -f "$TODO" ] || exit 0

# 从 todo 读声明的档位与 task
level="$(grep -oE 'level:[[:space:]]*L[0-9]' "$TODO" | grep -oE 'L[0-9]' | head -1)"
task="$(grep -oE 'task:[[:space:]]*[A-Za-z0-9._-]+' "$TODO" | sed -E 's/task:[[:space:]]*//' | head -1)"

# 仅在声明 L2+ 且【当前任务节】要收尾时才强制 eval。
# 关键：只在当前任务节（到第一个 ## 暂挂/归档 标题之前）找收尾段——否则暂挂/归档块里的旧 Review
#       会让闸 mid-task 误拦（lessons 2026-06-27 两条）。收尾段标题认 Review/评审/复盘（大小写不限）。
# 档位 / 收尾段声明靠 agent 诚实——见 docs/harness/HOOKS.md 的局限说明。
finishing_now() {
  awk '/^##[[:space:]]*(暂挂|归档|[Aa]rchive)/{exit} {print}' "$TODO" \
    | grep -qiE '^##[[:space:]].*(review|评审|复盘)'
}
if [ -n "$level" ] && [ "${level#L}" -ge 2 ] 2>/dev/null && finishing_now; then
  found=""
  [ -n "$task" ] && found="$(ls -d "$REVIEWS_DIR/"*"-$task" 2>/dev/null | head -1)"
  if [ -z "$found" ]; then
    echo "⛔ 收尾拦截（rule-0005）：todo 声明 $level 且已补 Review（=收尾），但没找到 task「${task:-未声明}」的 eval 评审产出。" >&2
    echo "   请先跑 eval 再收尾（交互：用 eval 子 agent，免 key；CI：make eval）；确属轻量就把 todo 的 level 降到 L1。" >&2
    exit 2
  fi
fi

# 自进化兜底：机械触发(K轮/commit/变更数)→ headless Haiku 复查最近对话、捞遗漏的决策/知识。best-effort，从不阻断。
bash "$ROOT/scripts/turn-backstop.sh" "${transcript:-}" || true

# 「纠错 → 记 lesson」提醒已移到 UserPromptSubmit 钩子（scripts/correction-nudge.sh）：它注入 agent 当轮上下文、
# 真到得了；这里原本那行 exit-0 stderr 不注入、等于没提醒，已删（见 docs/harness/HOOKS.md）。
exit 0
