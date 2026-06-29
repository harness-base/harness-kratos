#!/usr/bin/env bash
# 测试用例覆盖自检：docs/test-cases/<dir>/test-cases.md 的覆盖闭合 + 账本一致。
# 校验：① 每条 AC、每个 FP 都被 ≥1 条用例 covers 覆盖（无遗漏）
#       ② 用例 covers 引用的 AC/FP 都已声明（无悬空）
#       ③ 目录 ↔ index.yaml 登记一致（正反双向）
# covers 是覆盖关系的唯一真相源（不另存映射表）。空账本（test-cases: []）时平凡通过。
# 解析（严格 + fail-closed，宁可误红不可静默假绿）：
#   - sed 's/：/:/g' 归一全角冒号；剥围栏（CommonMark：记开栏字符 ```/~~~，仅"同字符且裸行"才闭合，治嵌套/伪闭合）。
#   - 段标题前缀锚定切段：DECL=以「验收点」/「功能点」起始、CASES=以「用例」/「测试用例」起始、余 OTHER。
#   - 声明：**一行一个 id**——DECL 段 list 行必须是 `- AC-n：…`（id 紧跟 dash、紧接冒号，可加 **粗体**）。
#     标注写到冒号后（`- AC-1：取代旧版 AC-9` 里 AC-9 是描述，不算声明）。单行多 id / 括注 / id 前有文字
#     等任何不合形的 list 行只要含 AC/FP id 一律判红（M），杜绝静默漏算。
#   - covers：CASES 段 covers 行取该行**所有** AC/FP id（纯 id 列表，任意分隔符都稳）。
#   护栏：a) 有声明样式行(L)却 declared 为空（段标题写歪/声明落段外）→ 红；b) 畸形声明行(M)→ 红；c) 围栏未闭合(F)→ 红。
# 质量（用例是否真覆盖语义 / 边界异常齐不齐 / covers 须单行写）由 eval 考题 015 / rule-0014 判。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
TC_DIR="${TC_DIR:-docs/test-cases}"   # 可被自测以环境变量覆盖，隔离真实账本
IDX="$TC_DIR/index.yaml"
fail=0
[ -f "$IDX" ] || { echo "  ✗ 缺 $IDX"; exit 1; }

registered="$(grep -E '^[[:space:]]*dir:[[:space:]]*' "$IDX" | sed -E 's/^[[:space:]]*dir:[[:space:]]*//; s/[[:space:]]*$//' | tr -d '"' || true)"

# 解析一份 test-cases.md，输出带前缀 token：D <id>=声明 / C <id>=covers / L=声明样式行 / M=畸形声明行 / F=围栏未闭合
emit(){ # $1=file
  sed 's/：/:/g' "$1" | awk '
    {
      isbt = ($0 ~ /^[[:space:]]*```/); isti = ($0 ~ /^[[:space:]]*~~~/)
      if (isbt || isti) {                                   # 围栏标记
        fchar = isbt ? "BT" : "TI"
        bare  = ($0 ~ /^[[:space:]]*(```+|~~~+)[[:space:]]*$/)   # 闭合栏须裸行（开栏可带 info string）
        if (fence=="") fence=fchar
        else if (fence==fchar && bare) fence=""
        next
      }
      if (fence != "") next                                 # 围栏内一律跳过
      if ($0 ~ /^##[[:space:]]/) {                          # level-2 标题切段（前缀锚定，对称）
        if ($0 ~ /^##[[:space:]]+验收点/ || $0 ~ /^##[[:space:]]+功能点/) sect="DECL"
        else if ($0 ~ /^##[[:space:]]+(测试)?用例/) sect="CASES"
        else sect="OTHER"
        next
      }
      if ($0 ~ /^[[:space:]]*-[[:space:]]*\**(AC|FP)-[0-9]+/) print "L"   # 声明样式行（任意段，护栏 a）
      if (sect=="DECL" && $0 ~ /^[[:space:]]*-[[:space:]]/) {
        if (match($0, /^[[:space:]]*-[[:space:]]*\**(AC|FP)-[0-9]+\**[[:space:]]*:/)) {   # 严格：- AC-n：单 id
          s=$0; sub(/^[[:space:]]*-[[:space:]]*\**/, "", s); match(s, /^(AC|FP)-[0-9]+/); print "D " substr(s,RSTART,RLENGTH)
        } else if ($0 ~ /(AC|FP)-[0-9]+/) print "M"         # list 行含 id 却非「- AC-n：」单 id 形 → fail-closed
      } else if (sect=="CASES" && $0 ~ /^[[:space:]]*-?[[:space:]]*\**covers\**:/) {
        rest=$0; sub(/^.*covers[*_]*:[[:space:]]*/, "", rest)
        while (match(rest, /(AC|FP)-[0-9]+/)) { print "C " substr(rest,RSTART,RLENGTH); rest=substr(rest,RSTART+RLENGTH) }
      }
    }
    END { if (fence != "") print "F" }
  '
}

# 正向：实际存在的用例目录必须登记 + 含 test-cases.md + 覆盖闭合
for d in "$TC_DIR"/*/; do
  [ -d "$d" ] || continue
  name="$(basename "$d")"
  f="$d/test-cases.md"
  [ -f "$f" ] || { echo "  ✗ $name 缺 test-cases.md"; fail=1; continue; }
  printf '%s\n' "$registered" | grep -qxF "$name" || { echo "  ✗ 用例目录 $name 未登记进 $IDX"; fail=1; }

  out="$(emit "$f")"
  declared="$(printf '%s\n' "$out" | sed -n 's/^D //p' | sort -u)"
  covered="$(printf '%s\n' "$out" | sed -n 's/^C //p' | sort -u)"

  # 护栏 a：有声明样式行却没解析出任何声明 → 段标题写歪/声明落段外
  if [ -z "$declared" ] && printf '%s\n' "$out" | grep -q '^L'; then
    echo "  ✗ $name：找到 AC/FP 声明行但不在标准声明段（## 验收点 AC / ## 功能点 FP）内——疑似段标题写错或声明落段外"; fail=1
  fi
  # 护栏 b：DECL 段 list 行含 id 却非「- AC-n：」单 id 形
  printf '%s\n' "$out" | grep -q '^M' && { echo "  ✗ $name：声明段有 list 行含 AC/FP id 却非 \`- AC-n：…\` 单 id 形（多 id 拆行、标注移到冒号后；拒绝静默漏算）"; fail=1; }
  # 护栏 c：围栏未闭合
  printf '%s\n' "$out" | grep -q '^F' && { echo "  ✗ $name：检测到未闭合代码围栏（\`\`\` / ~~~），解析不可靠"; fail=1; }

  # ① 每个声明的 AC/FP 都要被覆盖
  while IFS= read -r id; do
    [ -z "$id" ] && continue
    printf '%s\n' "$covered" | grep -qxF "$id" || { echo "  ✗ $name：$id 未被任何用例 covers 覆盖"; fail=1; }
  done <<< "$declared"

  # ② covers 引用的 id 都要已声明（无悬空）
  while IFS= read -r id; do
    [ -z "$id" ] && continue
    printf '%s\n' "$declared" | grep -qxF "$id" || { echo "  ✗ $name：用例 covers 引用了未声明的需求点 $id（悬空）"; fail=1; }
  done <<< "$covered"
done

# 反向：登记了但目录不存在
while IFS= read -r name; do
  [ -z "$name" ] && continue
  [ -d "$TC_DIR/$name" ] || { echo "  ✗ 登记的用例目录 $name 不存在"; fail=1; }
done <<< "$registered"

[ "$fail" -eq 0 ] && echo "  ✓ 测试用例覆盖闭合 + 账本一致"
exit "$fail"
