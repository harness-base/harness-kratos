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

echo "== eval 资产 =="
bash scripts/verify-eval-materials.sh || fail=1

echo "== skills 目录无漂移 =="
bash scripts/skills-index.sh --check || fail=1

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
