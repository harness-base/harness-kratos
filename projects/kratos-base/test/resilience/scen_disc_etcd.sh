#!/usr/bin/env bash
# AC-D1: 注册非致命 — etcd 宕时 demo 仍起、/v1/ping=200、日志有注册重试 WARN。
# AC-D2: 发现闭环 — etcd 恢复后注册成功；discoveryprobe 通过 etcd discovery 调 Ping 返回 pong。
#
# 流程：
#   1. sandbox 完全停（确保 etcd 宕）
#   2. 起 demo (mode=local + registry kind=etcd) → 进程活、/v1/ping=200、日志有注册重试 WARN
#   3. 起 sandbox (etcd 上线) → 轮询日志出现注册成功 INFO
#   4. build + run discoveryprobe → 断言输出含 "pong"
#   5. cleanup
#
# Note: bootstrap uses configs/bootstrap.disc-sandbox.yaml:
#   mode=local  (config from file, etcd outage does NOT block startup)
#   registry.kind=etcd (only registry is tested for non-fatal AC-D1)
#
# Usage: bash test/resilience/scen_disc_etcd.sh
# Must be run from the project root (projects/kratos-base/).
set -euo pipefail

# CWD-independent: always run from project root
cd "$(dirname "$0")/../.."

DEMO_PID=""
COMPOSE_CMD="docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml"
DEMO_LOG="/tmp/demo_disc_etcd_$$.log"
PROBE_BIN="/tmp/discoveryprobe_$$"

cleanup() {
    echo ""
    echo "=== [AC-D] cleanup ==="
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

echo "=== AC-D1/D2: scen_disc_etcd (service discovery etcd) ==="

# ── Step 1: ensure sandbox is fully down (etcd is NOT running) ────────────────
echo ""
echo "=== Step 1: ensure sandbox is down (etcd absent) ==="
make sandbox-down 2>/dev/null || true
echo ">> sandbox is down"

# ── Step 2: build demo ────────────────────────────────────────────────────────
echo ""
echo "=== Step 2: build demo ==="
go build -o bin/demo ./app/demo/cmd
echo ">> built bin/demo"

# ── Step 3: start demo with etcd absent (AC-D1) ───────────────────────────────
echo ""
echo "=== Step 3: start demo (etcd is down — non-fatal registration test) ==="
# disc-sandbox bootstrap: mode=local (config from file), registry.kind=etcd
./bin/demo -conf configs/bootstrap.disc-sandbox.yaml >"$DEMO_LOG" 2>&1 &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID"

# Poll /v1/ping until 200 — process must start despite etcd being down
echo ""
echo "=== Step 4: poll /v1/ping until 200 (process lives without etcd) ==="
STARTED=0
for i in $(seq 1 30); do
    RC=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/v1/ping 2>/dev/null || echo "000")
    echo "  attempt $i: /v1/ping → $RC"
    if [ "$RC" = "200" ]; then
        STARTED=1
        echo ">> PASS /v1/ping=200 (demo started without etcd)"
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
    echo "FAIL AC-D1: demo never responded to /v1/ping after 30 attempts"
    cat "$DEMO_LOG" || true
    exit 1
fi

# ── Step 5: wait for registration retry WARN in log (AC-D1) ─────────────────
echo ""
echo "=== Step 5: wait for registration retry WARN in log (AC-D1) ==="
WARNED=0
for i in $(seq 1 20); do
    # rule-0009: anchor on the exact producer string (registryx.go:225), same as
    # scen_disc_nacos.sh. The old loose `registry.*warn|registration.*warn` regex
    # could false-pass on unrelated lines that merely mention registry+warn.
    if grep -qF "registryx: registration failed, will retry" "$DEMO_LOG" 2>/dev/null; then
        WARNED=1
        echo ">> PASS: registration retry WARN found in log (attempt $i)"
        grep -F "registryx: registration failed, will retry" "$DEMO_LOG" | tail -3 || true
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-D1: demo died while waiting for retry WARN"
        exit 1
    fi
    sleep 1
done
if [ "$WARNED" -ne 1 ]; then
    echo ">> Log tail (last 20 lines):"
    tail -20 "$DEMO_LOG" || true
    echo "FAIL AC-D1: no registration retry WARN found in log"
    exit 1
fi

# ── Step 6: assert process still alive ───────────────────────────────────────
echo ""
echo "=== Step 6: assert process alive (AC-D1) ==="
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-D1: demo process died"
    exit 1
fi
echo ">> PASS demo process alive (non-fatal registration confirmed)"

# ── Step 7: bring etcd up (AC-D2) ────────────────────────────────────────────
echo ""
echo "=== Step 7: bring sandbox up (etcd comes online) ==="
make sandbox-up
echo ">> sandbox (including etcd) healthy"

# ── Step 8: poll log for registration success INFO ───────────────────────────
echo ""
echo "=== Step 8: poll log for registration success INFO (AC-D2) ==="
REGISTERED=0
for i in $(seq 1 60); do
    if grep -q "service registered" "$DEMO_LOG" 2>/dev/null; then
        REGISTERED=1
        echo ">> PASS: 'service registered' found in log (attempt $i)"
        grep "service registered" "$DEMO_LOG" | tail -3 || true
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-D2: demo died while waiting for registration success"
        exit 1
    fi
    sleep 1
done
if [ "$REGISTERED" -ne 1 ]; then
    echo ">> Log tail (last 30 lines):"
    tail -30 "$DEMO_LOG" || true
    echo "FAIL AC-D2: no 'service registered' found in log after etcd came online"
    exit 1
fi

# ── Step 9: build discoveryprobe (AC-D2) ─────────────────────────────────────
echo ""
echo "=== Step 9: build discoveryprobe ==="
go build -o "$PROBE_BIN" ./test/discoveryprobe
echo ">> built discoveryprobe at $PROBE_BIN"

# ── Step 10: run discoveryprobe — assert "pong" in output ───────────────────
echo ""
echo "=== Step 10: run discoveryprobe → assert 'pong' output (AC-D2) ==="
PROBE_OUT=$("$PROBE_BIN" 2>&1 || true)
echo ">> probe output: $PROBE_OUT"
if echo "$PROBE_OUT" | grep -q "pong"; then
    echo ">> PASS: discoveryprobe output contains 'pong'"
else
    echo "FAIL AC-D2: discoveryprobe did not output 'pong'"
    echo ">> full output: $PROBE_OUT"
    exit 1
fi

echo ""
echo "=== scen_disc_etcd.sh PASSED (AC-D1 + AC-D2) ==="
echo "  AC-D1: process started without etcd, /v1/ping=200, retry WARN logged"
echo "  AC-D2: etcd came online → service registered INFO → discoveryprobe returned pong"
