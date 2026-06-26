#!/usr/bin/env bash
# AC1: 启动期依赖宕 — PG 未起时 demo 进程正常起，healthz=200 readyz=503 ping=200
# greet/1 返回非2xx结构化错误（非 panic/进程退出）。
# Usage: bash test/resilience/scen_boot_dep_down.sh
# Must be run from the project root (projects/kratos-base/).
set -euo pipefail

DEMO_PID=""

cleanup() {
    echo ""
    echo "=== [AC1] cleanup ==="
    if [ -n "$DEMO_PID" ] && kill -0 "$DEMO_PID" 2>/dev/null; then
        echo ">> stopping demo gracefully (SIGTERM)"
        kill -TERM "$DEMO_PID" || true
        for i in $(seq 1 5); do
            if ! kill -0 "$DEMO_PID" 2>/dev/null; then break; fi
            sleep 1
        done
        kill -KILL "$DEMO_PID" 2>/dev/null || true
        wait "$DEMO_PID" 2>/dev/null || true
    fi
    echo ">> make sandbox-down (ensure pg is stopped)"
    make sandbox-down 2>/dev/null || true
}
trap cleanup EXIT

echo "=== AC1: scen_boot_dep_down ==="

# ── Step 1: ensure PG is NOT running ─────────────────────────────────────────
echo ""
echo "=== Step 1: ensure sandbox is down ==="
make sandbox-down 2>/dev/null || true
echo ">> sandbox is down"

# ── Step 2: build demo ───────────────────────────────────────────────────────
echo ""
echo "=== Step 2: build demo ==="
go build -o bin/demo ./app/demo/cmd
echo ">> built bin/demo"

# ── Step 3: start demo with PG absent ────────────────────────────────────────
echo ""
echo "=== Step 3: start demo (PG is down) ==="
./bin/demo -conf configs/bootstrap.sandbox.yaml >/tmp/demo_boot_dep_down.log 2>&1 &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID"

# Poll until HTTP server is listening (max 20 attempts)
STARTED=0
for i in $(seq 1 20); do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/healthz 2>/dev/null || echo "000")
    echo "  attempt $i: /healthz → $HTTP_CODE"
    if [ "$HTTP_CODE" = "200" ]; then
        STARTED=1
        echo ">> demo server started (attempt $i)"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "ERROR: demo process died unexpectedly"
        cat /tmp/demo_boot_dep_down.log || true
        exit 1
    fi
    sleep 1
done

if [ "$STARTED" -ne 1 ]; then
    echo "ERROR: demo never responded to /healthz after 20 attempts"
    cat /tmp/demo_boot_dep_down.log || true
    exit 1
fi

# ── Step 4: assert /healthz = 200 ────────────────────────────────────────────
echo ""
echo "=== Step 4: assert /healthz = 200 ==="
HEALTHZ_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/healthz)
echo ">> /healthz → $HEALTHZ_CODE"
if [ "$HEALTHZ_CODE" != "200" ]; then
    echo "FAIL AC1: /healthz expected 200, got $HEALTHZ_CODE"
    exit 1
fi
echo ">> PASS /healthz = 200"

# ── Step 5: assert /readyz = 503 ─────────────────────────────────────────────
echo ""
echo "=== Step 5: assert /readyz = 503 (PG down) ==="
READYZ_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz)
echo ">> /readyz → $READYZ_CODE"
if [ "$READYZ_CODE" != "503" ]; then
    echo "FAIL AC1: /readyz expected 503, got $READYZ_CODE"
    exit 1
fi
echo ">> PASS /readyz = 503"

# ── Step 6: assert /v1/ping = 200 ────────────────────────────────────────────
echo ""
echo "=== Step 6: assert /v1/ping = 200 ==="
PING_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/v1/ping)
echo ">> /v1/ping → $PING_CODE"
if [ "$PING_CODE" != "200" ]; then
    echo "FAIL AC1: /v1/ping expected 200, got $PING_CODE"
    exit 1
fi
echo ">> PASS /v1/ping = 200"

# ── Step 7: assert /v1/greet/1 is non-2xx with structured error ──────────────
echo ""
echo "=== Step 7: assert /v1/greet/1 returns structured error (non-2xx) ==="
GREET_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" http://localhost:8000/v1/greet/1 2>/dev/null || echo "HTTP_STATUS:000")
GREET_CODE=$(echo "$GREET_RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
GREET_BODY=$(echo "$GREET_RESP" | grep -v "HTTP_STATUS:")
echo ">> HTTP status: $GREET_CODE"
echo ">> body: $GREET_BODY"

if [ "$GREET_CODE" = "200" ]; then
    echo "FAIL AC1: /v1/greet/1 should NOT return 200 when PG is down"
    exit 1
fi

# Verify it's a structured error (JSON with "code" or "reason" field)
if ! echo "$GREET_BODY" | grep -qE '"code"|"reason"|"message"'; then
    echo "FAIL AC1: /v1/greet/1 body is not a structured error: $GREET_BODY"
    exit 1
fi
echo ">> PASS /v1/greet/1 returned structured error ($GREET_CODE)"

# ── Step 8: assert process is still alive (no panic) ─────────────────────────
echo ""
echo "=== Step 8: assert demo process is alive (kill -0) ==="
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC1: demo process $DEMO_PID is no longer alive"
    exit 1
fi
echo ">> PASS demo process $DEMO_PID is alive"

echo ""
echo "=== scen_boot_dep_down.sh PASSED (AC1) ==="
echo "  /healthz    → 200 (liveness OK)"
echo "  /readyz     → 503 (PG absent)"
echo "  /v1/ping    → 200 (no DB dep)"
echo "  /v1/greet/1 → $GREET_CODE (structured error, not panic)"
echo "  process     → alive"
