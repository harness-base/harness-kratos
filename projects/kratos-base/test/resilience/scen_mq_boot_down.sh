#!/usr/bin/env bash
# AC-M1: MQ 启动期宕 — sandbox-up (rabbit stopped) → 起 demo
#   断言：/healthz=200、/readyz=503 且 checks 含 "mq"、POST /v1/events=503、
#         /v1/ping=200、/v1/greet/1=200（pg 无关）、/v1/hits=200（redis 无关）、进程活。
# Usage: bash test/resilience/scen_mq_boot_down.sh
# CWD-independent: cd to project root from this script's location.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.." || { echo "cannot cd to project root"; exit 1; }

DEMO_PID=""
COMPOSE_CMD="docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml"

cleanup() {
    echo ""
    echo "=== [AC-M1] cleanup ==="
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
    # ensure rabbitmq is running before sandbox-down (idempotent)
    $COMPOSE_CMD start rabbitmq 2>/dev/null || true
    echo ">> make sandbox-down"
    make sandbox-down 2>/dev/null || true
}
trap cleanup EXIT

echo "=== AC-M1: scen_mq_boot_down ==="

# ── Step 1: ensure clean sandbox ──────────────────────────────────────────────
echo ""
echo "=== Step 1: ensure sandbox is down ==="
make sandbox-down 2>/dev/null || true
echo ">> sandbox is down"

# ── Step 2: sandbox-up (all four services, including rabbit) ──────────────────
echo ""
echo "=== Step 2: sandbox-up (pg + redis + etcd + rabbit all healthy) ==="
make sandbox-up
echo ">> all four services healthy"

# ── Step 3: stop rabbitmq ─────────────────────────────────────────────────────
echo ""
echo "=== Step 3: stop rabbitmq (keep pg + redis + etcd running) ==="
$COMPOSE_CMD stop rabbitmq
echo ">> rabbitmq stopped"

# ── Step 4: build and start demo with rabbit absent ───────────────────────────
echo ""
echo "=== Step 4: build and start demo (rabbitmq is down) ==="
go build -o bin/demo ./app/demo/cmd
echo ">> built bin/demo"

./bin/demo -conf configs/bootstrap.sandbox.yaml >/tmp/demo_mq_boot_down.log 2>&1 &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID"

# Poll until demo HTTP server is listening
STARTED=0
for i in $(seq 1 20); do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/healthz 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "200" ]; then
        STARTED=1
        echo ">> demo started (attempt $i)"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "ERROR: demo process died unexpectedly"
        cat /tmp/demo_mq_boot_down.log || true
        exit 1
    fi
    sleep 1
done

if [ "$STARTED" -ne 1 ]; then
    echo "ERROR: demo never started after 20 attempts"
    cat /tmp/demo_mq_boot_down.log || true
    exit 1
fi

# ── Step 5: assert /healthz = 200 (liveness always up) ───────────────────────
echo ""
echo "=== Step 5: assert /healthz = 200 ==="
HEALTHZ_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/healthz)
echo ">> /healthz → $HEALTHZ_CODE"
if [ "$HEALTHZ_CODE" != "200" ]; then
    echo "FAIL AC-M1: /healthz expected 200, got $HEALTHZ_CODE"
    exit 1
fi
echo ">> PASS /healthz = 200"

# ── Step 6: assert /readyz = 503 and body contains "mq" ─────────────────────
echo ""
echo "=== Step 6: assert /readyz = 503 and checks contain 'mq' ==="
READYZ_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "HTTP_STATUS:000")
READYZ_CODE=$(echo "$READYZ_RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
READYZ_BODY=$(echo "$READYZ_RESP" | grep -v "HTTP_STATUS:")
echo ">> /readyz → $READYZ_CODE  body: $READYZ_BODY"

if [ "$READYZ_CODE" != "503" ]; then
    echo "FAIL AC-M1: /readyz expected 503, got $READYZ_CODE"
    exit 1
fi
echo ">> PASS /readyz = 503"

if ! echo "$READYZ_BODY" | grep -q "mq"; then
    echo "FAIL AC-M1: /readyz body does not contain 'mq': $READYZ_BODY"
    exit 1
fi
echo ">> PASS /readyz body contains 'mq'"

# ── Step 7: assert POST /v1/events = 503 (MQ unavailable) ────────────────────
echo ""
echo "=== Step 7: assert POST /v1/events = 503 (MQ unavailable) ==="
EVENTS_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    -X POST http://localhost:8000/v1/events \
    -H 'Content-Type: application/json' \
    -d '{"payload":"mq-boot-down-test"}' 2>/dev/null || echo "HTTP_STATUS:000")
EVENTS_CODE=$(echo "$EVENTS_RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
EVENTS_BODY=$(echo "$EVENTS_RESP" | grep -v "HTTP_STATUS:")
echo ">> POST /v1/events → $EVENTS_CODE  body: $EVENTS_BODY"

if [ "$EVENTS_CODE" != "503" ]; then
    echo "FAIL AC-M1: POST /v1/events expected 503, got $EVENTS_CODE"
    exit 1
fi
echo ">> PASS POST /v1/events = 503 (structured error)"

# ── Step 8: assert /v1/ping = 200 (no MQ dep) ────────────────────────────────
echo ""
echo "=== Step 8: assert /v1/ping = 200 (no MQ dep) ==="
PING_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/v1/ping)
echo ">> /v1/ping → $PING_CODE"
if [ "$PING_CODE" != "200" ]; then
    echo "FAIL AC-M1: /v1/ping expected 200, got $PING_CODE"
    exit 1
fi
echo ">> PASS /v1/ping = 200"

# ── Step 9: assert /v1/greet/1 = 200 (PG-only, no MQ dep) ───────────────────
echo ""
echo "=== Step 9: assert /v1/greet/1 = 200 (PG path, not MQ) ==="
GREET_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" http://localhost:8000/v1/greet/1 2>/dev/null || echo "HTTP_STATUS:000")
GREET_CODE=$(echo "$GREET_RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
GREET_BODY=$(echo "$GREET_RESP" | grep -v "HTTP_STATUS:")
echo ">> /v1/greet/1 → $GREET_CODE  body: $GREET_BODY"
if [ "$GREET_CODE" != "200" ]; then
    echo "FAIL AC-M1: /v1/greet/1 expected 200 (pg up, rabbit irrelevant), got $GREET_CODE"
    exit 1
fi
echo ">> PASS /v1/greet/1 = 200"

# ── Step 10: assert /v1/hits = 200 (Redis-only, no MQ dep) ───────────────────
echo ""
echo "=== Step 10: assert /v1/hits = 200 (Redis path, not MQ) ==="
HITS_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/v1/hits)
echo ">> /v1/hits → $HITS_CODE"
if [ "$HITS_CODE" != "200" ]; then
    echo "FAIL AC-M1: /v1/hits expected 200 (redis up, rabbit irrelevant), got $HITS_CODE"
    exit 1
fi
echo ">> PASS /v1/hits = 200"

# ── Step 11: assert process is alive ─────────────────────────────────────────
echo ""
echo "=== Step 11: assert demo process is alive ==="
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-M1: demo process $DEMO_PID is not alive"
    exit 1
fi
echo ">> PASS demo process $DEMO_PID is alive"

echo ""
echo "=== scen_mq_boot_down.sh PASSED (AC-M1) ==="
echo "  /healthz   → 200 (liveness OK)"
echo "  /readyz    → 503 (mq check present)"
echo "  POST /v1/events → 503 (structured error)"
echo "  /v1/ping   → 200 (no MQ dep)"
echo "  /v1/greet/1 → 200 (pg up, MQ irrelevant)"
echo "  /v1/hits   → 200 (redis up, MQ irrelevant)"
echo "  demo process alive (pid $DEMO_PID)"
