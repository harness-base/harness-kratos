#!/usr/bin/env bash
# 规则索引（catalog）生成器：扫描所有 AGENTS.md 里的
#   <!-- rule: rule-00NN | sev: <severity> | eval: <ids> -->
# 标记，生成 docs/rules/index.yaml（编号 + 简述 + 位置 + severity + eval）。
# 规则全文在对应 AGENTS.md；编号 rule-00NN 是 eval/ADR/feature 的稳定引用键。
# 用法：
#   bash scripts/rules-index.sh          # 重生成（禁手改 index.yaml）
#   bash scripts/rules-index.sh --check   # 只比对，不写；漂移则非零退出（进 make verify）
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
OUT="docs/rules/index.yaml"

gen() {
  echo "# 规则总览（catalog）。由 scripts/rules-index.sh 从各 AGENTS.md 的标记自动生成。禁手改。"
  echo "# 规则全文在对应 AGENTS.md；编号 rule-00NN 是 eval/ADR/feature 的稳定引用键。"
  echo "version: 1"
  echo "rules:"
  # 只认真正的标记：rule: 后面必须是 rule-<数字>（排除文档里讲解格式的占位行）
  find . -name AGENTS.md -not -path './.git/*' | sort | while IFS= read -r f; do
    loc="${f#./}"
    # 认两类 id：harness 的 rule-00NN 与项目的 命名空间/slug（如 kratos/mq-quorum-dlq）；
    # 占位行 <!-- rule: --> 因 rule: 后是 '-->' 非字母而被排除
    grep -n '<!--[[:space:]]*rule:[[:space:]]*[A-Za-z]' "$f" | while IFS= read -r line; do
      content="${line#*:}"
      id="$(printf '%s' "$content"   | sed -nE 's/.*<!--[[:space:]]*rule:[[:space:]]*([A-Za-z][A-Za-z0-9_/-]+).*/\1/p')"
      sev="$(printf '%s' "$content"  | sed -nE 's/.*[|][[:space:]]*sev:[[:space:]]*([A-Za-z0-9_-]+).*/\1/p')"
      ev="$(printf '%s' "$content"   | sed -nE 's/.*[|][[:space:]]*eval:[[:space:]]*([0-9, ]+).*-->.*/\1/p' | tr -d ' ')"
      brief="$(printf '%s' "$content"| sed -nE 's/^[^*]*\*\*([^*]+)\*\*.*/\1/p')"
      brief="${brief//\"/}"
      [ -z "$id" ] && continue
      # 用 \037（US，非空白）分隔，避免 TAB 作 IFS 时空字段被折叠
      printf '%s\037%s\037%s\037%s\037%s\n' "$id" "$sev" "$ev" "$loc" "$brief"
    done
  done | sort | while IFS="$(printf '\037')" read -r id sev ev loc brief; do
    echo "  - id: $id"
    echo "    brief: \"$brief\""
    echo "    location: $loc"
    echo "    severity: ${sev:-info}"
    if [ -n "$ev" ]; then
      echo "    eval: [\"$(printf '%s' "$ev" | sed -E 's/,/", "/g')\"]"
    else
      echo "    eval: []"
    fi
  done
}

# 校验：每个 eval 标记引用的考题号都有对应 prompt 文件（防"凭空指针"，本类 bug 的固化拦截）
check_eval_pointers() {
  local bad=0 id
  for id in $(grep -oE '"[0-9]+"' "$OUT" | tr -d '"' | sort -u); do
    ls docs/eval/prompts/"${id}"-*.md >/dev/null 2>&1 || { echo "  ✗ 规则标记引用了不存在的考题：prompt $id（docs/eval/prompts/${id}-*.md 不存在）"; bad=1; }
  done
  return $bad
}

if [ "${1:-}" = "--check" ]; then
  rc=0
  tmp="$(mktemp)"; gen > "$tmp"
  if ! diff -q "$tmp" "$OUT" >/dev/null 2>&1; then
    echo "  ✗ rules 索引漂移（AGENTS.md 标记与 $OUT 不一致）→ 跑 bash scripts/rules-index.sh 重生成"
    diff "$OUT" "$tmp" | head -20; rc=1
  fi
  rm -f "$tmp"
  check_eval_pointers || rc=1
  [ "$rc" -eq 0 ] && echo "  ✓ rules 索引无漂移 + eval 指针有效"
  exit "$rc"
else
  gen > "$OUT"
  echo "✓ 已生成 $OUT"
  check_eval_pointers || { echo "✗ catalog 含无效 eval 指针，请修 AGENTS.md 标记"; exit 1; }
fi
