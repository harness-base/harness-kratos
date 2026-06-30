#!/usr/bin/env bash
# test-cases-audit 自测（hermetic：TC_DIR 指向 tmp 伪造账本，绝不碰真实 docs/test-cases）。
# 契约（严格 + fail-closed）：声明一行一个 id `- AC-n：…`（id 紧跟 dash、紧接冒号，可加粗）；
#   多 id 拆行、标注移到冒号后；covers 行收该行所有 id（任意分隔）；段标题前缀锚定；剥围栏（同字符裸行才闭合）。
# 不变量：每条 AC/FP 必被 ≥1 用例 covers 覆盖、covers 无悬空、目录↔账本登记双向一致；空账本平凡通过；
#   模糊/畸形一律判红（宁可误红不静默假绿）。每个坏样本与好样本仅差「那一处违规」→ 变异自证（rule-0009）。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
pass=0; fail=0
ok(){ pass=$((pass+1)); }
no(){ echo "  ✗ $1"; fail=$((fail+1)); }
run(){ TC_DIR="$1" bash scripts/test-cases-audit.sh >/dev/null 2>&1; echo $?; }

IDX_DEMO='version: 1
test-cases:
  - id: demo
    dir: demo
    title: t
    status: draft
'
one(){ rm -rf "$1"; mkdir -p "$1/demo"; printf '%s' "$IDX_DEMO" > "$1/index.yaml"; printf '%s' "$2" > "$1/demo/test-cases.md"; }

GOOD='## 验收点 AC
- AC-1：登录成功后跳转首页
## 功能点 FP
- FP-1：登录表单校验
## 用例
### TC-1：正常登录
- 类型：正常
- covers: AC-1, FP-1
'

tmp="$(mktemp -d)"

# 0) 空账本 → 绿。
e="$tmp/empty"; mkdir -p "$e"; printf 'version: 1\ntest-cases: []\n' > "$e/index.yaml"
[ "$(run "$e")" = "0" ] && ok || no "空账本被判失败（应平凡通过）"

# 1) 全合规 → 绿。正向锚（否则下面断红全假过，rule-0009）。
g="$tmp/good"; one "$g" "$GOOD"
[ "$(run "$g")" = "0" ] && ok || no "合规好样本被判失败"

# 2) 声明 AC-2 却无人 covers → 红（覆盖检查①）。
one "$tmp/b2" '## 验收点 AC
- AC-1：a
- AC-2：错误密码提示
## 功能点 FP
- FP-1：c
## 用例
### TC-1：正常
- covers: AC-1, FP-1
'
[ "$(run "$tmp/b2")" != "0" ] && ok || no "未覆盖的 AC-2 未判失败（覆盖检查①失守）"

# 3) covers 引用未声明 AC-9 → 红（悬空检查②）。
one "$tmp/b3" '## 验收点 AC
- AC-1：a
## 功能点 FP
- FP-1：c
## 用例
### TC-1：正常
- covers: AC-1, FP-1, AC-9
'
[ "$(run "$tmp/b3")" != "0" ] && ok || no "悬空 covers AC-9 未判失败（悬空检查②失守）"

# 4) FP-2 未覆盖 → 红（覆盖检查作用于 FP）。
one "$tmp/b4" '## 验收点 AC
- AC-1：a
## 功能点 FP
- FP-1：c
- FP-2：记住登录态
## 用例
### TC-1：正常
- covers: AC-1, FP-1
'
[ "$(run "$tmp/b4")" != "0" ] && ok || no "未覆盖的 FP-2 未判失败（覆盖检查未作用于 FP）"

# 5) 合规用例但未登记 → 红（正向登记）。
b5="$tmp/b5"; rm -rf "$b5"; mkdir -p "$b5/demo"; printf 'version: 1\ntest-cases: []\n' > "$b5/index.yaml"; printf '%s' "$GOOD" > "$b5/demo/test-cases.md"
[ "$(run "$b5")" != "0" ] && ok || no "未登记的用例目录未判失败（正向登记检查失守）"

# 6) 登记了不存在的 ghost 目录 → 红（反向登记）。
one "$tmp/b6" "$GOOD"; printf '  - id: ghost\n    dir: ghost\n    title: g\n    status: draft\n' >> "$tmp/b6/index.yaml"
[ "$(run "$tmp/b6")" != "0" ] && ok || no "ghost 目录不存在却未判失败（反向登记检查失守）"

# 7) 围栏内示例 covers AC-2 不算真覆盖 → AC-2 未覆盖红（剥围栏·covered 侧）。
one "$tmp/b7" '## 验收点 AC
- AC-1：a
- AC-2：b
## 功能点 FP
- FP-1：c
## 用例
### TC-1：正常
- covers: AC-1, FP-1

covers 写法示例：
```
- covers: AC-2
```
'
[ "$(run "$tmp/b7")" != "0" ] && ok || no "围栏内示例 covers AC-2 被当真覆盖（剥围栏失守=假阴）"

# 8) 声明段内围栏里 `- AC-7：` 不算声明 → 全覆盖绿（剥围栏·declared 侧）。
one "$tmp/b8" '## 验收点 AC
- AC-1：a

附录（围栏内不算声明）：
```
- AC-7：别处文档的验收点
```
## 功能点 FP
- FP-1：c
## 用例
### TC-1：正常
- covers: AC-1, FP-1
'
[ "$(run "$tmp/b8")" = "0" ] && ok || no "声明段内围栏里 `- AC-7：` 被当声明致误红（声明侧剥围栏失守）"

# 9) 缩进单 id 声明 `  - AC-2：` 未覆盖 → 红（缩进容忍，对称）。
one "$tmp/b9" '## 验收点 AC
- AC-1：a
  - AC-2：缩进子项声明
## 功能点 FP
- FP-1：c
## 用例
### TC-1：正常
- covers: AC-1, FP-1
'
[ "$(run "$tmp/b9")" != "0" ] && ok || no "缩进声明 AC-2 未覆盖却判绿（声明扫描不容缩进=假绿）"

# 10) 加粗 + 全角冒号 covers 被识别 → 全覆盖绿。
one "$tmp/b10" '## 验收点 AC
- AC-1：a
## 功能点 FP
- FP-1：c
## 用例
### TC-1：正常
- **covers**：AC-1, FP-1
'
[ "$(run "$tmp/b10")" = "0" ] && ok || no "加粗/全角冒号 covers 未被识别致误红"

# 11) 已登记目录缺 test-cases.md → 红。
b11="$tmp/b11"; rm -rf "$b11"; mkdir -p "$b11/demo"; printf '%s' "$IDX_DEMO" > "$b11/index.yaml"
[ "$(run "$b11")" != "0" ] && ok || no "已登记目录缺 test-cases.md 却未判失败（缺-md 检查无守护）"

# 12) 逗号单行多 id `- AC-1, AC-2：` 非单 id 形 → 护栏 b 红（一行一 id 契约）。
one "$tmp/b12" '## 验收点 AC
- AC-1, AC-2：逗号多 id 一行
## 功能点 FP
- FP-1：c
## 用例
### TC-1：正常
- covers: AC-1, AC-2, FP-1
'
[ "$(run "$tmp/b12")" != "0" ] && ok || no "逗号单行多 id 声明未判红（一行一 id 契约失守）"

# 13) AC 用非标准标题 `## 接受条件` 声明（落段外）→ 护栏 a 红。
one "$tmp/b13" '## 接受条件
- AC-1：用非标准标题声明，应被护栏 a 抓
## 用例
### TC-1：正常
- 步骤：无
'
[ "$(run "$tmp/b13")" != "0" ] && ok || no "非标准标题致声明落段外、护栏 a 未判红（vacuous 假绿）"

# 14) 未闭合围栏（吞掉后文）→ 护栏 c 红。
one "$tmp/b14" '## 验收点 AC
- AC-1：a
## 功能点 FP
- FP-1：c
## 用例
### TC-1：正常
- covers: AC-1, FP-1

未闭合围栏：
```
- AC-2：被未闭合围栏吞掉、无人覆盖
'
[ "$(run "$tmp/b14")" != "0" ] && ok || no "未闭合围栏未被护栏 c 判红（吞掉的未覆盖 AC 假绿）"

# 15) 放宽标题：`## 测试用例` 前缀被识别 → 全覆盖绿（DX）。
one "$tmp/b15" '## 验收点 AC
- AC-1：a
## 功能点 FP
- FP-1：c
## 测试用例
### TC-1：正常
- covers: AC-1, FP-1
'
[ "$(run "$tmp/b15")" = "0" ] && ok || no "`## 测试用例` 段未被识别致误红（标题放宽失效）"

# 16) 括注 label `- AC-1（见 AC-9）：` 非单 id 形 → 护栏 b 红（标注须移到冒号后）。
one "$tmp/b16" '## 验收点 AC
- AC-1（见旧版 AC-9）：本验收点取代 AC-9
## 功能点 FP
- FP-1：c
## 用例
### TC-1：正常
- covers: AC-1, FP-1
'
[ "$(run "$tmp/b16")" != "0" ] && ok || no "括注 label 声明未判红（标注须移到冒号后）"

# 17) 嵌套异种围栏（~~~ 套 ```）里的 covers 不算覆盖 → FP-1 未覆盖红（同字符配对）。
one "$tmp/b17" '## 验收点 AC
- AC-1：a
## 功能点 FP
- FP-1：c
## 用例
### TC-1：正常
- covers: AC-1

~~~
```
- covers: FP-1
```
~~~
'
[ "$(run "$tmp/b17")" != "0" ] && ok || no "嵌套围栏内 covers FP-1 被当真覆盖（围栏须同字符配对）"

# 18) DECL 段 list 行 id 不紧跟 dash（`- 边界场景: AC-2`）→ 护栏 b 红。
one "$tmp/b18" '## 验收点 AC
- AC-1：a
- 边界场景: AC-2 真实第二验收点但格式不规范
## 功能点 FP
- FP-1：c
## 用例
### TC-1：正常
- covers: AC-1, FP-1
'
[ "$(run "$tmp/b18")" != "0" ] && ok || no "畸形声明 `- 边界场景: AC-2` 未判红（护栏 b 失守=假阴）"

# 19) 标注写在冒号后 `- AC-1：（另见 AC-9）` → 只声明 AC-1、全覆盖绿（冒号后 id 不算声明，不假阳）。
one "$tmp/b19" '## 验收点 AC
- AC-1：（另见旧版 AC-9）登录
## 功能点 FP
- FP-1：c
## 用例
### TC-1：正常
- covers: AC-1, FP-1
'
[ "$(run "$tmp/b19")" = "0" ] && ok || no "冒号后标注里的 AC-9 被误当声明致误红（应只认冒号前单 id）"

# 20) 顿号单行多 id `- AC-1、AC-2：` 非单 id 形 → 护栏 b 红（顿号也不放过，fail-closed）。
one "$tmp/b20" '## 验收点 AC
- AC-1、AC-2：顿号多 id 一行
## 功能点 FP
- FP-1：c
## 用例
### TC-1：正常
- covers: AC-1, AC-2, FP-1
'
[ "$(run "$tmp/b20")" != "0" ] && ok || no "顿号单行多 id 声明未判红（任意分隔符都该 fail-closed）"

# 21) covers 行顿号分隔 `covers: AC-1、AC-2、FP-1` → 全 id 被收、全覆盖绿（covers 任意分隔都稳）。
one "$tmp/b21" '## 验收点 AC
- AC-1：a
- AC-2：b
## 功能点 FP
- FP-1：c
## 用例
### TC-1：正常
- covers: AC-1、AC-2、FP-1
'
[ "$(run "$tmp/b21")" = "0" ] && ok || no "covers 顿号分隔致 id 漏收、全覆盖误红（covers 应收所有 id）"

# 22) covers 行顿号分隔含悬空 AC-9 → 红（covers 收全后悬空检查仍生效）。
one "$tmp/b22" '## 验收点 AC
- AC-1：a
## 功能点 FP
- FP-1：c
## 用例
### TC-1：正常
- covers: AC-1、AC-9、FP-1
'
[ "$(run "$tmp/b22")" != "0" ] && ok || no "covers 顿号分隔的悬空 AC-9 未判红（covers 收全=悬空仍可查）"

# 23) 标题子串非前缀 `## 补充用例说明`（含「用例」但非前缀）→ 归 OTHER，其下 covers 不采集 → 覆盖缺口红。
one "$tmp/b23" '## 验收点 AC
- AC-1：a
## 功能点 FP
- FP-1：c
## 补充用例说明
### TC-1：正常
- covers: AC-1, FP-1
'
[ "$(run "$tmp/b23")" != "0" ] && ok || no "`## 补充用例说明` 被子串误判为用例段（前缀锚定失守致泄漏 covers）"

# 24) 伪闭合围栏（闭合栏带后缀文字，不算闭合）→ 其后 covers 不泄漏 + 护栏 c 红。
one "$tmp/b24" '## 验收点 AC
- AC-1：a
- AC-2：b
## 功能点 FP
- FP-1：c
## 用例
### TC-1：正常
- covers: AC-1, FP-1

```text
示例内容
```这是带后缀文字的伪闭合
- covers: AC-2
'
[ "$(run "$tmp/b24")" != "0" ] && ok || no "伪闭合围栏后的 covers AC-2 泄漏成真覆盖（闭合栏须裸行）"

# ===== 覆盖矩阵护栏 ④（e2e 用例；与 25/26 仅差那一格 → 变异自证 rule-0009）=====
MX_HEAD='## 验收点 AC
- AC-1：a
## 功能点 FP
- FP-1：c
## 交互点 × 类型 覆盖矩阵
| 交互点（UX） | 成功 | 失败 | 边界 |
|---|---|---|---|'
MX_TAIL='## 用例
### TC-1：正常
- covers: AC-1, FP-1
'

# 25) 矩阵「失败」格空着、无「无·理由:」→ 红（矩阵硬闸）。
one "$tmp/b25" "$MX_HEAD
| 登录提交 | TC-1 |  | 无·理由:无边界态 |
$MX_TAIL"
[ "$(run "$tmp/b25")" != "0" ] && ok || no "覆盖矩阵失败格空着未判红（矩阵硬闸失守=假绿）"

# 26) 矩阵三格全填（TC / 无·理由）→ 绿（正向锚，与 25 仅差失败那一格）。
one "$tmp/b26" "$MX_HEAD
| 登录提交 | TC-1 | TC-1 | 无·理由:无边界态 |
$MX_TAIL"
[ "$(run "$tmp/b26")" = "0" ] && ok || no "覆盖矩阵全填却误红（矩阵硬闸误伤）"

# 27) 无矩阵段（api/旧用例）→ 绿（矩阵闸只查含矩阵段的文件，文件级不误伤）。
one "$tmp/b27" "$GOOD"
[ "$(run "$tmp/b27")" = "0" ] && ok || no "无矩阵段的用例被矩阵闸误伤（应只查含矩阵段的文件）"

# 28) 矩阵格留占位 `<TC-NN 或 无·理由:…>`（含 < > …）→ 红（占位当未填，fail-closed）。
one "$tmp/b28" "$MX_HEAD
| 登录提交 | TC-1 | <TC-NN 或 无·理由:…> | 无·理由:无边界态 |
$MX_TAIL"
[ "$(run "$tmp/b28")" != "0" ] && ok || no "矩阵占位符 <…无·理由:…> 被当已填（占位未当未填=假绿）"

rm -rf "$tmp"
echo "test-cases-audit.test: pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
