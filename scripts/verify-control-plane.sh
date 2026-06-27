#!/usr/bin/env bash
# 控制面自检：结构 + 文档自检 + hook policy 自测 + eval 资产 + skills 目录无漂移。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
fail=0

echo "== 结构检查 =="
for p in AGENTS.md CLAUDE.md README.md Makefile \
         docs/README.md docs/rules/index.yaml docs/decisions/index.yaml docs/eval/index.yaml \
         docs/prds/index.yaml workspace/verification.yaml .agents/skills; do
  [ -e "$p" ] || { echo "  ✗ 缺 $p"; fail=1; }
done
[ "$fail" -eq 0 ] && echo "  ✓ 结构齐全"

echo "== 文档自检 =="
bash scripts/docs-audit.sh || fail=1

echo "== hook policy 自测 =="
bash scripts/hook-policy.test.sh || fail=1

echo "== 自进化兜底自测 =="
bash scripts/turn-backstop.test.sh || fail=1

echo "== 纠错提醒钩子自测 =="
bash scripts/correction-nudge.test.sh || fail=1

echo "== lessons 整理计数自测 =="
bash scripts/lessons-promote-check.test.sh || fail=1

echo "== eval 资产 =="
bash scripts/verify-eval-materials.sh || fail=1

echo "== skills 目录无漂移 =="
bash scripts/skills-index.sh --check || fail=1

echo "== 状态文档不硬编码可自生成枚举（rule-0012）=="
# CURRENT_STATUS 的 `.agents/skills/` 行应指向自动生成的 .agents/skills/README.md（skills-index），
# 不复刻 skill 清单；该行列举 >=4 个真实 skill 名 = 在硬编码枚举（举 1-2 例豁免）→ 失败。
status_doc="docs/context/CURRENT_STATUS.md"
skills_row="$(grep -E '^\| `\.agents/skills/`' "$status_doc" || true)"
enum_n=0
for s in $(find .agents/skills -maxdepth 2 -name SKILL.md -exec dirname {} \; | xargs -n1 basename | sort -u); do
  printf '%s' "$skills_row" | grep -q -- "$s" && enum_n=$((enum_n+1))
done
if [ "$enum_n" -ge 4 ]; then
  echo "  ✗ $status_doc 的 .agents/skills/ 行枚举了 $enum_n 个 skill 名（rule-0012）——改为指向自动生成的 .agents/skills/README.md，别硬编码清单"
  fail=1
else
  echo "  ✓ 状态文档未硬编码 skill 枚举（rule-0012：指向自动生成索引）"
fi

echo "== rules 索引无漂移 =="
bash scripts/rules-index.sh --check || fail=1

echo "== 目录索引无漂移 =="
for d in docs/context docs/harness templates .claude/agents; do
  bash scripts/dir-index.sh "$d" --check || fail=1
done

echo "== 索引一致性（decisions/features）=="
for d in docs/decisions docs/features; do bash scripts/index-audit.sh "$d" || fail=1; done

echo "== 验证路由工程路径可达 =="
rfail=0
while IFS= read -r p; do
  [ -z "$p" ] && continue
  [ -d "$p" ] || { echo "  ✗ verification.yaml 路由的工程路径不存在: $p"; rfail=1; }
done < <(grep -E '^[[:space:]]*path:[[:space:]]*' workspace/verification.yaml | sed -E 's/^[[:space:]]*path:[[:space:]]*//; s/[[:space:]]*$//')
[ "$rfail" -eq 0 ] && echo "  ✓ 路由工程路径可达（命令真能跑由各工程 e2e 负责）" || fail=1

echo "== AGENTS.md ↔ CLAUDE.md shim =="
shimfail=0
while IFS= read -r am; do
  d="$(dirname "$am")"
  if [ ! -f "$d/CLAUDE.md" ]; then echo "  ✗ $d 有 AGENTS.md 但缺 CLAUDE.md shim"; shimfail=1
  elif ! grep -q '@AGENTS.md' "$d/CLAUDE.md"; then echo "  ✗ $d/CLAUDE.md 未 @import AGENTS.md"; shimfail=1; fi
done < <(find . -name AGENTS.md -not -path './.git/*')
[ "$shimfail" -eq 0 ] && echo "  ✓ 每个 AGENTS.md 都有 CLAUDE.md shim" || fail=1

echo "== PRD 账本自检 =="
bash scripts/prds-audit.sh || fail=1

echo "== references 路径残留 =="
hits=$(grep -rln 'harness-empty\|-Users-zhouhaiyin-project-harness-empty' .agents/skills 2>/dev/null)
if [ -n "$hits" ]; then
  echo "  ✗ references 内含 harness-empty 路径残留（应改用 \$(git rev-parse --show-toplevel) 或本仓名）："
  echo "$hits" | sed 's|^|    |'
  fail=1
else
  echo "  ✓ references 路径无 harness-empty 残留"
fi

echo
[ "$fail" -eq 0 ] && echo "✓ 控制面自检通过" || { echo "✗ 控制面自检失败"; exit 1; }
