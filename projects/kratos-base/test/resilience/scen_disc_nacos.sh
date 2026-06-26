#!/usr/bin/env bash
# AC-N2: nacos 注册中心闭环（注册非致命 + 发现闭环）。
#
# 验收断言（rule-0009 锚定产出方证据）：
#   - AC-N2-a: nacos 未起时 demo 仍启动、/v1/ping=200、日志有注册重试 WARN
#   - AC-N2-b: nacos 起后自动注册 INFO → discoveryprobe（nacos discovery）调 Ping 返回 "pong"
#
# 流程：
#   1. sandbox-down 确保 nacos 不存在
#   2. 起 demo (mode=local + registry kind=nacos) → 进程活 + /v1/ping=200 + 日志 WARN
#   3. sandbox-up → nacos 上线 → 轮询日志出现注册成功 INFO
#   4. build + run discoveryprobe (DISCOVERY_BACKEND=nacos) → 断言输出含 "pong"
#   5. cleanup
#
# CWD 无关（脚本内 cd 到 project root）。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.."

DEMO_PID=""
COMPOSE_CMD="docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml"
DEMO_LOG="/tmp/demo_disc_nacos_$$.log"
PROBE_BIN="/tmp/discoveryprobe_nacos_$$"

cleanup() {
    echo ""
    echo "=== [AC-N2] cleanup ==="
    if [ -n "$DEMO_PID" ] && kill -0 "$DEMO_PID" 2>/dev/null; then
        echo ">> stopping demo (SIGTERM)"
        kill -TERM "$DEMO_PID" || true
        for i in $(seq 1 5); do
            if ! kill -0 "$DEMO_PID" 2>/dev/null; then break; fi
            sleep 1
        done
        kill -KILL "$DEMO_PID" 2>/dev/null || true
        wait "$DEMO_PID" 2>/dev/null || true
    fi
    echo ">> sandbox-down"
    make sandbox-down 2>/dev/null || true
    rm -f "$DEMO_LOG" "$PROBE_BIN"
}
trap cleanup EXIT

echo "=== AC-N2: scen_disc_nacos (nacos service discovery e2e) ==="

# ── Step 1: ensure sandbox is fully down (nacos is NOT running) ───────────────
echo ""
echo "=== Step 1: ensure sandbox is down (nacos absent) ==="
make sandbox-down 2>/dev/null || true
echo ">> sandbox is down"

# ── Step 2: build demo ────────────────────────────────────────────────────────
echo ""
echo "=== Step 2: build demo ==="
go build -o bin/demo ./app/demo/cmd
echo ">> built bin/demo"

# ── Step 3: start demo with nacos absent (AC-N2-a) ───────────────────────────
echo ""
echo "=== Step 3: start demo (nacos absent — non-fatal registration test) ==="
# nacos-disc bootstrap: mode=local (config from file), registry.kind=nacos
./bin/demo -conf configs/bootstrap.nacos-disc.yaml >"$DEMO_LOG" 2>&1 &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID"

# ── Step 4: poll /v1/ping until 200 ──────────────────────────────────────────
echo ""
echo "=== Step 4: poll /v1/ping until 200 (process lives without nacos) ==="
STARTED=0
for i in $(seq 1 30); do
    RC=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/v1/ping 2>/dev/null || echo "000")
    echo "  attempt $i: /v1/ping → $RC"
    if [ "$RC" = "200" ]; then
        STARTED=1
        echo ">> PASS /v1/ping=200 (demo started without nacos)"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "ERROR: demo died during startup"
        cat "$DEMO_LOG" || true
        exit 1
    fi
    sleep 1
done
if [ "$STARTED" -ne 1 ]; then
    echo "FAIL AC-N2-a: demo never responded to /v1/ping after 30 attempts"
    cat "$DEMO_LOG" || true
    exit 1
fi

# ── Step 5: wait for registration retry WARN in log (AC-N2-a) ───────────────
echo ""
echo "=== Step 5: wait for registration retry WARN in log (AC-N2-a) ==="
WARNED=0
for i in $(seq 1 20); do
    if grep -qF "registryx: registration failed, will retry" "$DEMO_LOG" 2>/dev/null; then
        WARNED=1
        echo ">> PASS: registration retry WARN found in log (attempt $i)"
        grep -F "registryx: registration failed, will retry" "$DEMO_LOG" | tail -3 || true
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-N2-a: demo died while waiting for retry WARN"
        exit 1
    fi
    sleep 1
done
if [ "$WARNED" -ne 1 ]; then
    echo ">> Log tail (last 20 lines):"
    tail -20 "$DEMO_LOG" || true
    echo "FAIL AC-N2-a: no registration retry WARN found in log"
    exit 1
fi

# ── Step 6: assert process still alive (AC-N2-a) ─────────────────────────────
echo ""
echo "=== Step 6: assert process alive (AC-N2-a) ==="
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-N2-a: demo process died"
    exit 1
fi
echo ">> PASS demo process alive (non-fatal nacos registration confirmed)"

# ── Step 7: bring sandbox up (nacos comes online) ────────────────────────────
echo ""
echo "=== Step 7: sandbox-up (nacos comes online) ==="
make sandbox-up
echo ">> sandbox (including nacos) healthy"

# ── Step 8: poll log for registration success INFO (AC-N2-b) ─────────────────
echo ""
echo "=== Step 8: poll log for registration success INFO (AC-N2-b) ==="
REGISTERED=0
for i in $(seq 1 60); do
    if grep -q "service registered" "$DEMO_LOG" 2>/dev/null; then
        REGISTERED=1
        echo ">> PASS: 'service registered' found in log (attempt $i)"
        grep "service registered" "$DEMO_LOG" | tail -3 || true
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-N2-b: demo died while waiting for registration success"
        exit 1
    fi
    sleep 1
done
if [ "$REGISTERED" -ne 1 ]; then
    echo ">> Log tail (last 30 lines):"
    tail -30 "$DEMO_LOG" || true
    echo "FAIL AC-N2-b: no 'service registered' found in log after nacos came online"
    exit 1
fi

# ── Step 9: build discoveryprobe ──────────────────────────────────────────────
echo ""
echo "=== Step 9: build discoveryprobe ==="
go build -o "$PROBE_BIN" ./test/discoveryprobe
echo ">> built discoveryprobe at $PROBE_BIN"

# ── Step 10: run discoveryprobe with nacos backend → assert "pong" ───────────
echo ""
echo "=== Step 10: run discoveryprobe (DISCOVERY_BACKEND=nacos) → assert 'pong' (AC-N2-b) ==="
PROBE_OUT=$(DISCOVERY_BACKEND=nacos NACOS_ADDR="${NACOS_ADDR:-127.0.0.1:8848}" "$PROBE_BIN" 2>&1 || true)
echo ">> probe output: $PROBE_OUT"
if echo "$PROBE_OUT" | grep -q "pong"; then
    echo ">> PASS: discoveryprobe output contains 'pong'"
else
    echo "FAIL AC-N2-b: discoveryprobe did not output 'pong'"
    echo ">> full output: $PROBE_OUT"
    exit 1
fi

echo ""
echo "=== scen_disc_nacos.sh PASSED (AC-N2) ==="
echo "  AC-N2-a: demo started without nacos, /v1/ping=200, registration retry WARN logged"
echo "  AC-N2-b: nacos came online → service registered INFO → discoveryprobe returned pong"
