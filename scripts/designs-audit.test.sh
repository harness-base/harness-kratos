#!/usr/bin/env bash
# designs-audit 自测（hermetic：DESIGNS_DIR 指向 tmp 伪造账本，绝不碰真实 docs/designs）。
# 不变量：目录↔账本登记双向一致、每 design 目录含 design.md、定稿零 TBD（design.md + api-contract.md）；
#   空账本（designs: []）平凡通过。每个坏样本与好样本仅差「那一处违规」→ 变异自证（rule-0009）。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
pass=0; fail=0
ok(){ pass=$((pass+1)); }
no(){ echo "  ✗ $1"; fail=$((fail+1)); }
run(){ DESIGNS_DIR="$1" bash scripts/designs-audit.sh >/dev/null 2>&1; echo $?; }

IDX_DEMO='version: 1
designs:
  - id: demo
    dir: demo
    title: t
    status: reviewed
'
# one <root> <design.md内容> [api-contract.md内容]：建带 demo 目录的伪账本
one(){ rm -rf "$1"; mkdir -p "$1/demo"; printf '%s' "$IDX_DEMO" > "$1/index.yaml"; printf '%s' "$2" > "$1/demo/design.md"; [ "$#" -ge 3 ] && printf '%s' "$3" > "$1/demo/api-contract.md"; return 0; }

GOOD_DESIGN='# 研发方案：demo 示例设计
## ① 背景 & 范围
- 核心目标：为订单服务增加退款能力
## ④ 接口设计
- 契约位置：api-contract.md
'
GOOD_API='# 接口契约：demo
## 端点索引
| # | Method | Path | 用途 | 鉴权 |
|---|---|---|---|---|
| 1 | POST | /v1/refunds | 创建退款 | 需要 |
'

tmp="$(mktemp -d)"

# 0) 空账本 → 绿。
e="$tmp/empty"; mkdir -p "$e"; printf 'version: 1\ndesigns: []\n' > "$e/index.yaml"
[ "$(run "$e")" = "0" ] && ok || no "空账本被判失败（应平凡通过）"

# 1) 全合规（含 api-contract）→ 绿。正向锚（否则下面断红全假过，rule-0009）。
g="$tmp/good"; one "$g" "$GOOD_DESIGN" "$GOOD_API"
[ "$(run "$g")" = "0" ] && ok || no "合规好样本被判失败"

# 1b) 全合规、无 api-contract.md（契约可选）→ 绿。
g2="$tmp/good2"; one "$g2" "$GOOD_DESIGN"
[ "$(run "$g2")" = "0" ] && ok || no "无 api-contract 的合规样本被判失败（契约应可选）"

# 2) 合规设计但未登记 → 红（正向登记）。
b2="$tmp/b2"; rm -rf "$b2"; mkdir -p "$b2/demo"; printf 'version: 1\ndesigns: []\n' > "$b2/index.yaml"; printf '%s' "$GOOD_DESIGN" > "$b2/demo/design.md"
[ "$(run "$b2")" != "0" ] && ok || no "未登记的 design 目录未判失败（正向登记检查失守）"

# 3) 登记了不存在的 ghost 目录 → 红（反向登记）。
one "$tmp/b3" "$GOOD_DESIGN"; printf '  - id: ghost\n    dir: ghost\n    title: g\n    status: draft\n' >> "$tmp/b3/index.yaml"
[ "$(run "$tmp/b3")" != "0" ] && ok || no "ghost 目录不存在却未判失败（反向登记检查失守）"

# 4) 登记目录缺 design.md → 红（主文档必需）。
b4="$tmp/b4"; rm -rf "$b4"; mkdir -p "$b4/demo"; printf '%s' "$IDX_DEMO" > "$b4/index.yaml"
[ "$(run "$b4")" != "0" ] && ok || no "已登记目录缺 design.md 却未判失败（缺-md 检查无守护）"

# 5) design.md 含 TBD → 红（零 TBD 机检·design 侧）。与好样本仅差一处 TBD。
one "$tmp/b5" "$GOOD_DESIGN
## ③ 数据模型
- 表结构：TBD
" "$GOOD_API"
[ "$(run "$tmp/b5")" != "0" ] && ok || no "design.md 含 TBD 未判失败（零 TBD 机检·design 侧失守）"

# 6) design.md 含「待确认」→ 红（中文占位词同样命中）。
one "$tmp/b6" "$GOOD_DESIGN
- 存储选型：待确认
" "$GOOD_API"
[ "$(run "$tmp/b6")" != "0" ] && ok || no "design.md 含「待确认」未判失败（中文占位词失守）"

# 7) api-contract.md 含 FIXME → 红（零 TBD 机检·api 侧；契约有就一并查）。
one "$tmp/b7" "$GOOD_DESIGN" "$GOOD_API
| 2 | GET | /v1/refunds | FIXME 待补 | 需要 |
"
[ "$(run "$tmp/b7")" != "0" ] && ok || no "api-contract.md 含 FIXME 未判失败（零 TBD 机检·api 侧失守）"

# 8) design.md 含「待定」（小写不敏感同理）→ 红。
one "$tmp/b8" "$GOOD_DESIGN
- 范围边界：待定
" "$GOOD_API"
[ "$(run "$tmp/b8")" != "0" ] && ok || no "design.md 含「待定」未判失败（占位词表漏项）"

# 9) 干净 api-contract（无占位词）+ 干净 design → 绿（api 侧正向锚，防 7/8 假红）。
one "$tmp/b9" "$GOOD_DESIGN" "$GOOD_API"
[ "$(run "$tmp/b9")" = "0" ] && ok || no "干净 design + api 被误红（占位词机检误伤合规样本）"

rm -rf "$tmp"
echo "designs-audit.test: pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
