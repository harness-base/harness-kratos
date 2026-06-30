#!/usr/bin/env bash
# eval 的【可选】CI/headless 路径：拼 evaluator+rubric+考题+候选 → 调外部 LLM API（OpenAI 兼容）→ 写 task-review。
# 交互时默认用 hc-eval 子 agent（.claude/agents/hc-eval.md），免 key；本脚本用于无人值守自动跑分。
# 用法：bash scripts/run-eval.sh --context-level L3 --candidate-file <file> [--prompts 010,011]
# 需要：curl、jq；环境变量 EVAL_API_BASE / EVAL_API_KEY，可选 EVAL_MODEL。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EVAL="$ROOT/docs/eval"

level=""; candidate=""; prompts="010"
while [ $# -gt 0 ]; do
  case "$1" in
    --context-level) level="$2"; shift 2;;
    --candidate-file) candidate="$2"; shift 2;;
    --prompts) prompts="$2"; shift 2;;
    *) echo "未知参数：$1"; exit 2;;
  esac
done
[ -f "$candidate" ] || { echo "缺 --candidate-file <存在的文件>"; exit 2; }
command -v jq >/dev/null   || { echo "需要 jq"; exit 2; }
command -v curl >/dev/null || { echo "需要 curl"; exit 2; }
: "${EVAL_API_BASE:?run-eval 是可选的 CI/headless 路径，需 EVAL_API_BASE；交互时改用 hc-eval 子 agent（.claude/agents/hc-eval.md），免 key}"
: "${EVAL_API_KEY:?run-eval 需 EVAL_API_KEY；交互时改用 hc-eval 子 agent，免 key}"
model="${EVAL_MODEL:-claude-sonnet-4-6}"

sys="$(cat "$EVAL/evaluator.md" "$EVAL/rubric.md")"
ptext=""
oldifs="$IFS"; IFS=','
for p in $prompts; do
  f="$(ls "$EVAL"/prompts/"${p}"-*.md 2>/dev/null | head -1)"
  [ -f "$f" ] && ptext="$ptext"$'\n\n'"=== 考题 $p ==="$'\n'"$(cat "$f")"
done
IFS="$oldifs"
cand="$(cat "$candidate")"
user="任务档位：$level"$'\n\n'"要套用的考题：$ptext"$'\n\n'"=== 候选产出 ==="$'\n'"$cand"

body="$(jq -n --arg m "$model" --arg s "$sys" --arg u "$user" \
  '{model:$m, temperature:0, messages:[{role:"system",content:$s},{role:"user",content:$u}]}')"
resp="$(curl -sS "$EVAL_API_BASE/chat/completions" \
  -H "Authorization: Bearer $EVAL_API_KEY" -H "Content-Type: application/json" -d "$body")"
out="$(printf '%s' "$resp" | jq -r '.choices[0].message.content // .error.message // "（无返回）"')"

ts="$(date -u +%Y%m%dT%H%M%SZ)"
slug="$(basename "$candidate" | sed 's/\.[^.]*$//')"
dir="$EVAL/task-reviews/${ts}-${slug}"
mkdir -p "$dir"
cp "$candidate" "$dir/candidate.md"
printf '%s\n' "$out" > "$dir/decision.md"
printf 'level: %s\nprompts: %s\ngenerated: %s\n' "$level" "$prompts" "$ts" > "$dir/summary.md"
echo "✓ eval 完成 → $dir/decision.md"
echo "----"
printf '%s\n' "$out"
