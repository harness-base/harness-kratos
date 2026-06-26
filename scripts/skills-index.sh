#!/usr/bin/env bash
# 扫 .agents/skills/*/SKILL.md 的 frontmatter，生成技能目录 .agents/skills/README.md。
# --check：只比对、不写；漂移则非零退出（防"加了技能忘登记"）。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
TARGET=".agents/skills/README.md"

fm_field(){ # $1=file $2=field
  awk -v k="$2" '
    NR==1 && $0=="---"{infm=1; next}
    infm && $0=="---"{exit}
    infm && $0 ~ ("^" k ":"){ sub("^" k ":[[:space:]]*",""); print; exit }
  ' "$1"
}

gen(){
  echo "# 技能目录"
  echo
  echo "> 由 \`bash scripts/skills-index.sh\` 从各 SKILL.md frontmatter 自动生成，请勿手改。"
  echo
  echo "| name | description |"
  echo "| --- | --- |"
  for d in .agents/skills/*/; do
    f="${d}SKILL.md"
    [ -f "$f" ] || continue
    name="$(fm_field "$f" name)"
    [ -z "$name" ] && continue
    desc="$(fm_field "$f" description)"
    echo "| $name | $desc |"
  done
}

if [ "${1:-}" = "--check" ]; then
  if ! diff <(gen) "$TARGET" >/dev/null 2>&1; then
    echo "  ✗ $TARGET 与 SKILL.md 漂移，请运行 bash scripts/skills-index.sh"
    exit 1
  fi
  echo "✓ skills 目录无漂移"
else
  gen > "$TARGET"
  echo "✓ 已生成 $TARGET"
fi
