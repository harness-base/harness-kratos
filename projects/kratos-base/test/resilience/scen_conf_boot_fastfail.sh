#!/usr/bin/env bash
# AC-CF: etcd 配置源启动 fail-fast。
#
# 设计前提（已与用户对齐）：启动期真没配置、又没本地缓存快照 → 直接 fail-fast
#   退出，不带空配置半残上路（用户认可此行为，明确不要启动快照）。
#   对比 nacos：SDK 有本地快照、能用缓存先起来，故不是 fail-fast，本场景只针对 etcd。
#
# 验收断言（rule-0009 锚定产出方证据）：
#   - etcd 缺席时 mode=etcd 启动 → demo 进程【快速非零退出】（不是挂起、不是 0）
#   - 退出原因是【配置加载失败】的产出方证据：日志含 "new config source"（main.go）
#     与 "etcd"（backends/etcd probe 失败链路），非泛化报错
#   - demo 未对外服务：/readyz 无 HTTP 监听（curl 拿不到状态码，code=000）
#
# 注意：本场景【不需要】sandbox-up——只需 etcd 不在（sandbox-down 即可），
#       config 源加载在任何数据面之前，连不上 etcd 立即 Fatalf 退出。
# CWD 无关（脚本内 cd 到 project root）。
set -uo pipefail   # 故意不加 -e：demo 预期非零退出，需自行判定

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.."

DEMO_LOG="/tmp/demo_conf_boot_fastfail_$$.log"

cleanup() {
    echo ""
    echo "=== [AC-CF] cleanup ==="
    make sandbox-down 2>/dev/null || true
    rm -f "$DEMO_LOG"
}
trap cleanup EXIT

echo "=== AC-CF: scen_conf_boot_fastfail (etcd 配置源启动 fail-fast) ==="

# ── Step 1: 确保 etcd 不在（整个 sandbox down）─────────────────────────────────
echo ""
echo "=== Step 1: ensure etcd is absent (sandbox down) ==="
make sandbox-down 2>/dev/null || true
echo ">> sandbox down — nothing on etcd :2379"

# ── Step 2: build demo ────────────────────────────────────────────────────────
echo ""
echo "=== Step 2: build demo ==="
go build -o bin/demo ./app/demo/cmd
echo ">> built bin/demo"

# ── Step 3: 起 demo（mode=etcd）→ 预期快速非零退出 ────────────────────────────
echo ""
echo "=== Step 3: start demo (mode=etcd) with no etcd → expect fast non-zero exit ==="
./bin/demo -conf configs/bootstrap.etcd-sandbox.yaml >"$DEMO_LOG" 2>&1 &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID"

EXITED=0
for i in $(seq 1 30); do
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        EXITED=1
        echo ">> demo exited at attempt $i"
        break
    fi
    sleep 1
done

if [ "$EXITED" -ne 1 ]; then
    echo "FAIL AC-CF: demo did NOT fail-fast — still running after 30s (expected immediate exit)"
    kill -KILL "$DEMO_PID" 2>/dev/null || true
    tail -30 "$DEMO_LOG" || true
    exit 1
fi

# 收割退出码
wait "$DEMO_PID"
RC=$?
echo ">> demo exit code: $RC"

# ── 断言 1：非零退出 ──────────────────────────────────────────────────────────
if [ "$RC" -eq 0 ]; then
    echo "FAIL AC-CF: demo exited 0 (expected non-zero fail-fast)"
    tail -30 "$DEMO_LOG" || true
    exit 1
fi
echo ">> PASS AC-CF-a: demo fail-fast with non-zero exit ($RC)"

# ── 断言 2：退出原因是配置加载失败（产出方证据，非泛化）────────────────────────
if grep -q "new config source" "$DEMO_LOG" && grep -qi "etcd" "$DEMO_LOG"; then
    echo ">> PASS AC-CF-b: exit caused by config-source load failure (etcd):"
    grep -iE "new config source|etcd" "$DEMO_LOG" | tail -3
else
    echo "FAIL AC-CF-b: no config-source(etcd) load-failure evidence in log"
    tail -30 "$DEMO_LOG" || true
    exit 1
fi

# ── 断言 3：未对外服务（无 HTTP 监听）──────────────────────────────────────────
# curl 连接失败时 -w 已输出 "000" 且退出非零；用 || true 收口，避免叠加多个 "000"
RC_HTTP=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || true)
if [ "$RC_HTTP" = "000" ]; then
    echo ">> PASS AC-CF-c: not serving (no HTTP listener on :8000)"
else
    echo "FAIL AC-CF-c: demo still serving (/readyz=$RC_HTTP) despite config-load failure"
    exit 1
fi

echo ""
echo "=== scen_conf_boot_fastfail.sh PASSED (AC-CF) ==="
echo "  AC-CF: etcd 缺席 → mode=etcd 启动 fail-fast（非零退出 + 配置加载失败证据 + 不对外服务）"
