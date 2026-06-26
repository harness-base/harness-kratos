#!/usr/bin/env bash
# AC-R1: redis 启动期宕机 — sandbox-up（pg+redis 都起）→ stop redis（仅 redis 宕）→ 起 demo →
#   断言 /readyz=503 且 details 含 redis、/v1/hits=5xx 结构化错误、
#   /v1/ping=200、/v1/greet/1=200（pg 不受影响）、进程存活。
# Usage: bash test/resilience/scen_redis_boot_down.sh
# CWD-independent: cd to project root from this script's location.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.." || { echo "cannot cd to project root"; exit 1; }

DEMO_PID=""
COMPOSE_CMD="docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml"

cleanup() {
    echo ""
    echo "=== [AC-R1] cleanup ==="
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
    # ensure redis is running before sandbox-down (idempotent)
    $COMPOSE_CMD start redis 2>/dev/null || true
    echo ">> make sandbox-down"
    make sandbox-down 2>/dev/null || true
}
trap cleanup EXIT

echo "=== AC-R1: scen_redis_boot_down ==="

# ── Step 1: ensure clean sandbox ──────────────────────────────────────────────
echo ""
echo "=== Step 1: ensure sandbox is down ==="
make sandbox-down 2>/dev/null || true
echo ">> sandbox is down"

# ── Step 2: sandbox-up (both pg + redis) ──────────────────────────────────────
echo ""
echo "=== Step 2: sandbox-up (pg + redis both healthy) ==="
make sandbox-up
echo ">> pg + redis are healthy"

# ── Step 3: stop redis only ───────────────────────────────────────────────────
echo ""
echo "=== Step 3: stop redis container (keep pg running) ==="
$COMPOSE_CMD stop redis
echo ">> redis stopped; pg still running"

# ── Step 4: build demo ────────────────────────────────────────────────────────
echo ""
echo "=== Step 4: build demo ==="
go build -o bin/demo ./app/demo/cmd
echo ">> built bin/demo"

# ── Step 5: start demo with redis absent ──────────────────────────────────────
echo ""
echo "=== Step 5: start demo (redis is down, pg is up) ==="
./bin/demo -conf configs/bootstrap.sandbox.yaml >/tmp/demo_redis_boot_down.log 2>&1 &
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
        cat /tmp/demo_redis_boot_down.log || true
        exit 1
    fi
    sleep 1
done

if [ "$STARTED" -ne 1 ]; then
    echo "ERROR: demo never responded to /healthz after 20 attempts"
    cat /tmp/demo_redis_boot_down.log || true
    exit 1
fi

# ── Step 6: assert /readyz = 503 (redis absent) ───────────────────────────────
echo ""
echo "=== Step 6: assert /readyz = 503 (redis down) ==="
READYZ_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "HTTP_STATUS:000")
READYZ_CODE=$(echo "$READYZ_RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
READYZ_BODY=$(echo "$READYZ_RESP" | grep -v "HTTP_STATUS:")
echo ">> /readyz HTTP $READYZ_CODE"
echo ">> /readyz body: $READYZ_BODY"
if [ "$READYZ_CODE" != "503" ]; then
    echo "FAIL AC-R1: /readyz expected 503 (redis down), got $READYZ_CODE"
    exit 1
fi
echo ">> PASS /readyz = 503"

# Assert details contain "redis"
if ! echo "$READYZ_BODY" | grep -qi "redis"; then
    echo "FAIL AC-R1: /readyz body does not mention 'redis': $READYZ_BODY"
    exit 1
fi
echo ">> PASS /readyz details contain 'redis'"

# ── Step 7: assert /v1/hits = 5xx structured error ────────────────────────────
echo ""
echo "=== Step 7: assert /v1/hits = 5xx structured error ==="
HITS_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" http://localhost:8000/v1/hits 2>/dev/null || echo "HTTP_STATUS:000")
HITS_CODE=$(echo "$HITS_RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
HITS_BODY=$(echo "$HITS_RESP" | grep -v "HTTP_STATUS:")
echo ">> /v1/hits HTTP $HITS_CODE"
echo ">> /v1/hits body: $HITS_BODY"

# Must be 5xx
if [[ "$HITS_CODE" != 5* ]]; then
    echo "FAIL AC-R1: /v1/hits expected 5xx, got $HITS_CODE"
    exit 1
fi
# Must be structured JSON error
if ! echo "$HITS_BODY" | grep -qE '"code"|"reason"|"message"'; then
    echo "FAIL AC-R1: /v1/hits body is not a structured error: $HITS_BODY"
    exit 1
fi
echo ">> PASS /v1/hits = $HITS_CODE with structured error"

# ── Step 8: assert /v1/ping = 200 (no DB dep) ────────────────────────────────
echo ""
echo "=== Step 8: assert /v1/ping = 200 ==="
PING_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/v1/ping)
echo ">> /v1/ping → $PING_CODE"
if [ "$PING_CODE" != "200" ]; then
    echo "FAIL AC-R1: /v1/ping expected 200, got $PING_CODE"
    exit 1
fi
echo ">> PASS /v1/ping = 200"

# ── Step 9: assert /v1/greet/1 = 200 (pg is still up) ────────────────────────
echo ""
echo "=== Step 9: assert /v1/greet/1 = 200 (pg unaffected) ==="
GREET_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" http://localhost:8000/v1/greet/1 2>/dev/null || echo "HTTP_STATUS:000")
GREET_CODE=$(echo "$GREET_RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
GREET_BODY=$(echo "$GREET_RESP" | grep -v "HTTP_STATUS:")
echo ">> HTTP status: $GREET_CODE"
echo ">> body: $GREET_BODY"
if [ "$GREET_CODE" != "200" ]; then
    echo "FAIL AC-R1: /v1/greet/1 expected 200 (pg still up), got $GREET_CODE"
    exit 1
fi
if ! echo "$GREET_BODY" | grep -q "hello from sandbox"; then
    echo "FAIL AC-R1: /v1/greet/1 body does not contain 'hello from sandbox'"
    exit 1
fi
echo ">> PASS /v1/greet/1 = 200 with 'hello from sandbox' (pg unaffected)"

# ── Step 10: assert process is still alive (no panic) ─────────────────────────
echo ""
echo "=== Step 10: assert demo process is alive ==="
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-R1: demo process $DEMO_PID is no longer alive"
    exit 1
fi
echo ">> PASS demo process $DEMO_PID is alive"

echo ""
echo "=== scen_redis_boot_down.sh PASSED (AC-R1) ==="
echo "  /readyz      → 503 (redis absent, details contain 'redis')"
echo "  /v1/hits     → $HITS_CODE (5xx structured error)"
echo "  /v1/ping     → 200 (no DB dep)"
echo "  /v1/greet/1  → 200 (pg unaffected)"
echo "  process      → alive"
