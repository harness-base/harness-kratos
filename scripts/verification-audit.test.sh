#!/usr/bin/env bash
# verification-audit 自测（hermetic：VERIFY_YAML 指向 tmp 伪造文件，绝不碰真实 workspace/verification.yaml）。
# 契约：接入点值三态——真命令 / "PENDING: 理由"(warn·绿) / "N/A: 理由"(绿)；静默空 / 无理由 / TODO / <占位> = 红。
# 每个坏样本与好样本仅差「那一处」→ 变异自证（rule-0009）。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
pass=0; fail=0
ok(){ pass=$((pass+1)); }
no(){ echo "  ✗ $1"; fail=$((fail+1)); }
run(){ VERIFY_YAML="$1" bash scripts/verification-audit.sh >/dev/null 2>&1; echo $?; }
one(){ printf '%s' "$2" > "$1"; }

HEAD='version: 1
projects:
  - name: demo
    path: projects/demo'

tmp="$(mktemp -d)"

# 1) 全真命令 → 绿（正向锚，否则下面断红全假过）。
one "$tmp/f1" "$HEAD
    verify: \"make -C projects/demo verify\"
    e2e: \"bash projects/demo/e2e.sh\""
[ "$(run "$tmp/f1")" = "0" ] && ok || no "全真命令被判失败"

# 2) PENDING 有理由 → 绿（warn 不阻断）。
one "$tmp/f2" "$HEAD
    verify: \"make -C projects/demo verify\"
    sandbox: \"PENDING: 还没写 e2e，要跑时用 create-sandbox\""
[ "$(run "$tmp/f2")" = "0" ] && ok || no "PENDING 有理由被判失败（应 warn 不阻断）"

# 3) N/A 有理由 → 绿。
one "$tmp/f3" "$HEAD
    verify: \"make -C projects/demo verify\"
    e2e: \"N/A: 纯库、无 e2e\""
[ "$(run "$tmp/f3")" = "0" ] && ok || no "N/A 有理由被判失败"

# 4) 空值 → 红（静默空 fail-closed）。
one "$tmp/f4" "$HEAD
    verify: \"\"
    e2e: \"bash x\""
[ "$(run "$tmp/f4")" != "0" ] && ok || no "空接入点值未判红（静默空失守）"

# 5) 裸 PENDING 无理由 → 红。
one "$tmp/f5" "$HEAD
    verify: \"make x\"
    sandbox: \"PENDING:\""
[ "$(run "$tmp/f5")" != "0" ] && ok || no "裸 PENDING 无理由未判红"

# 6) 裸 TODO → 红。
one "$tmp/f6" "$HEAD
    verify: \"TODO\""
[ "$(run "$tmp/f6")" != "0" ] && ok || no "裸 TODO 未判红"

# 7) <占位尖括号> → 红。
one "$tmp/f7" "$HEAD
    verify: \"<待填>\""
[ "$(run "$tmp/f7")" != "0" ] && ok || no "<占位> 未判红"

# 8) 真命令 + 一个 PENDING 混合 → 绿（PENDING 只 warn 不拉红）。
one "$tmp/f8" "$HEAD
    verify: \"make -C projects/demo verify\"
    unit: \"go test ./...\"
    sandbox: \"PENDING: 待接 e2e\""
[ "$(run "$tmp/f8")" = "0" ] && ok || no "真命令+PENDING 混合被判失败"

# ===== fail-closed 假绿守护（9-15，对抗评审 M1/M2 挖出的漏洞，每条与好样本仅差那一处）=====
# 9) 纯空格 → 红（clean 剥引号后须再 trim）。
one "$tmp/f9" "$HEAD
    verify: \" \"
    e2e: \"bash x\""
[ "$(run "$tmp/f9")" != "0" ] && ok || no "纯空格接入点未判红（剥引号后未再 trim=假绿）"

# 10) 三个点 "..." → 红。
one "$tmp/f10" "$HEAD
    verify: \"...\""
[ "$(run "$tmp/f10")" != "0" ] && ok || no "'...' 占位未判红（黑名单漏网=假绿）"

# 11) 小写 todo → 红（大小写不敏感）。
one "$tmp/f11" "$HEAD
    verify: \"todo\""
[ "$(run "$tmp/f11")" != "0" ] && ok || no "小写 todo 未判红（占位词大小写敏感=假绿）"

# 12) 值只是注释（verify: # 待补）→ 红（当空）。
one "$tmp/f12" "$HEAD
    verify: # 待补
    e2e: \"bash x\""
[ "$(run "$tmp/f12")" != "0" ] && ok || no "值只是注释未判红（clean 未当空=假绿）"

# 13) 裸 PENDING（无冒号无理由）→ 红。
one "$tmp/f13" "$HEAD
    verify: \"PENDING\""
[ "$(run "$tmp/f13")" != "0" ] && ok || no "裸 PENDING 未判红"

# 14) 中文含糊词 待定 → 红。
one "$tmp/f14" "$HEAD
    verify: \"待定\""
[ "$(run "$tmp/f14")" != "0" ] && ok || no "待定 未判红"

# 15) 小写 pending: 有理由 → 绿+warn（m3：前缀大小写不敏感、保留提醒语义）。
one "$tmp/f15" "$HEAD
    verify: \"make x\"
    sandbox: \"pending: 待接 e2e\""
[ "$(run "$tmp/f15")" = "0" ] && ok || no "小写 pending: 有理由被判红（m3：前缀应大小写不敏感）"

# ===== 多工程（projects/ 有多个工程，逐工程核、别串、别误伤，用户 2026-07-01 提）=====
# 16) 两工程：demo-a 全真命令、demo-b 静默空 → 红（多工程逐工程核，一个坏就红）。
one "$tmp/f16" 'version: 1
projects:
  - name: demo-a
    path: projects/demo-a
    verify: "make -C projects/demo-a verify"
  - name: demo-b
    path: projects/demo-b
    verify: ""'
[ "$(run "$tmp/f16")" != "0" ] && ok || no "多工程里 demo-b 静默空未判红（多工程逐工程核失守）"

# 17) 两工程都合规（真命令 + PENDING 有理由）→ 绿（别误伤他人工程）。
one "$tmp/f17" 'version: 1
projects:
  - name: demo-a
    path: projects/demo-a
    verify: "make -C projects/demo-a verify"
  - name: demo-b
    path: projects/demo-b
    verify: "make -C projects/demo-b verify"
    sandbox: "PENDING: 待接 e2e"'
[ "$(run "$tmp/f17")" = "0" ] && ok || no "多工程都合规却误红（多工程误伤他人工程）"

# 18) 单引号纯空格 '  ' → 红（clean 也剥单引号，eval F-1 补；对比 case 9 的双引号空格）。
one "$tmp/f18" "$HEAD
    verify: '  '
    e2e: \"bash x\""
[ "$(run "$tmp/f18")" != "0" ] && ok || no "单引号纯空格未判红（clean 未剥单引号=假绿，F-1）"

rm -rf "$tmp"
echo "verification-audit.test: pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
