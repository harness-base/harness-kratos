#!/usr/bin/env bash
# UserPromptSubmit hook：每轮把「自检是否被用户纠正 → 记 lesson」的提醒注入 agent 当轮上下文（rule-0011）。
# 设计：判"是不是纠正"交给 agent 自己（它上下文最全，比关键词/小模型都准）；本钩子只负责把提醒
#       可靠地塞到眼前——替掉 stop-check 里那行 exit-0 stderr（不注入、等于没提醒）的死提醒。
# 机制：UserPromptSubmit 的 stdout 在 exit 0 时注入当轮上下文（Claude Code hooks）。
# 全程 best-effort：消费掉 stdin、绝不阻断（永远 exit 0，从不 exit 2）。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cat >/dev/null 2>&1 || true   # 消费 payload（本钩子不依赖字段；读掉避免上游 EPIPE）

# 提醒 1：自检是否被用户纠正 → 当轮记 lesson
cat <<'EOF'
[自检·rule-0011] 用户上一句是否在纠正 / 否定 / 反驳你（含没有"错 / 不对"等关键词的**语义**纠正，如"这其实是 X 不是 Y""你搞混了"）？
是 → 本轮就在 tasks/lessons.md 记一条三段式（错在哪 / 怎么防 / 怎么更早发现），别拖到下一轮。
否 → 忽略本行。
EOF

# 提醒 2（step 4）：未整理的 lesson 攒够 → 提示整理（升规则 / 跳过 / 待决定）
THRESHOLD="${LESSONS_PROMOTE_THRESHOLD:-10}"
pending="$(bash "$ROOT/scripts/lessons-promote-check.sh" 2>/dev/null || echo 0)"
case "$pending" in ''|*[!0-9]*) pending=0 ;; esac
if [ "$pending" -gt "$THRESHOLD" ]; then
  printf '\n[整理·rule-0011] tasks/lessons.md 攒了 %s 条还没整理的 lesson（超 %s）。\n转达用户：可走 hc-self-evolution 挑哪些该升成规则（走 hc-add-rule）、不值得的标 skip。\n转达后把这批未标记的标题行尾加 <!-- opt: seen -->（提醒过·待决定），免得下轮重复打扰。\n' "$pending" "$THRESHOLD"
fi

# 提醒 3：optimization-log 里有「待处理」(未打勾)的 backstop 发现 → 反馈给 agent 去处理。
# turn-backstop 把发现写成 `- [ ]`；这里是它的送达通道(替代不被看见的 exit-0 stderr)。
OPTLOG="${BACKSTOP_LOG:-$ROOT/tasks/optimization-log.md}"
undone="$(grep -cE '^- \[ \]' "$OPTLOG" 2>/dev/null || echo 0)"
case "$undone" in ''|*[!0-9]*) undone=0 ;; esac
if [ "$undone" -gt 0 ]; then
  printf '\n[待处理·rule-0011] tasks/optimization-log.md 有 %s 条待处理发现（含文档漂移/没落文档的知识）。逐条处理：改掉对应文档 / 落到 ADR·lessons·规则，处理完把该行 `- [ ]` 改 `- [x]`（暂缓改 `- [~]` 并写一句理由）。别让它烂在 log 里。\n' "$undone"
fi
exit 0
