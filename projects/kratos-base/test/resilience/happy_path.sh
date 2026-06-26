#!/usr/bin/env bash
# T9a: happy-path end-to-end verification — real postgres, real demo server.
# Usage: bash test/resilience/happy_path.sh
# Must be run from the project root (projects/kratos-base/).
set -euo pipefail

DEMO_PID=""

cleanup() {
    echo ""
    echo "=== cleanup ==="
    if [ -n "$DEMO_PID" ] && kill -0 "$DEMO_PID" 2>/dev/null; then
        echo ">> killing demo (pid $DEMO_PID)"
        kill "$DEMO_PID" || true
        wait "$DEMO_PID" 2>/dev/null || true
    fi
    echo ">> make sandbox-down"
    make sandbox-down || true
}
trap cleanup EXIT

# ── Step 1: bring up sandbox postgres ────────────────────────────────────────
echo "=== Step 1: sandbox-up ==="
make sandbox-up
echo ">> postgres sandbox healthy"

# ── Step 2: build and start demo server ──────────────────────────────────────
echo ""
echo "=== Step 2: build demo ==="
go build -o bin/demo ./app/demo/cmd
echo ">> built bin/demo"

echo ">> starting demo server (conf: configs/bootstrap.sandbox.yaml)"
./bin/demo -conf configs/bootstrap.sandbox.yaml &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID"

# ── Step 3: poll /readyz until 200 (max 30 attempts) ─────────────────────────
echo ""
echo "=== Step 3: poll /readyz ==="
READY=0
for i in $(seq 1 30); do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz || echo "000")
    echo "  attempt $i: /readyz → $HTTP_CODE"
    if [ "$HTTP_CODE" = "200" ]; then
        READY=1
        echo ">> /readyz returned 200 on attempt $i"
        break
    fi
    # poll with 1s interval — condition-driven, not fixed-sleep guessing
    if [ $i -lt 30 ]; then
        sleep 1
    fi
done

if [ "$READY" -ne 1 ]; then
    echo "ERROR: /readyz never returned 200 after 30 attempts"
    exit 1
fi

# ── Step 4: GET /v1/greet/1 — assert 200 + seed body ────────────────────────
echo ""
echo "=== Step 4: GET /v1/greet/1 ==="
GREET_BODY=$(curl -s -w "\nHTTP_STATUS:%{http_code}" http://localhost:8000/v1/greet/1)
GREET_HTTP=$(echo "$GREET_BODY" | grep "HTTP_STATUS:" | cut -d: -f2)
GREET_JSON=$(echo "$GREET_BODY" | grep -v "HTTP_STATUS:")

echo ">> HTTP status: $GREET_HTTP"
echo ">> body: $GREET_JSON"

if [ "$GREET_HTTP" != "200" ]; then
    echo "ERROR: /v1/greet/1 returned HTTP $GREET_HTTP (expected 200)"
    exit 1
fi

if ! echo "$GREET_JSON" | grep -q "hello from sandbox"; then
    echo "ERROR: /v1/greet/1 body does not contain 'hello from sandbox'"
    echo "  actual body: $GREET_JSON"
    exit 1
fi
echo ">> /v1/greet/1 OK — body contains 'hello from sandbox'"

# ── Step 5: GET /v1/ping ─────────────────────────────────────────────────────
echo ""
echo "=== Step 5: GET /v1/ping ==="
PING_BODY=$(curl -s -w "\nHTTP_STATUS:%{http_code}" http://localhost:8000/v1/ping)
PING_HTTP=$(echo "$PING_BODY" | grep "HTTP_STATUS:" | cut -d: -f2)
PING_JSON=$(echo "$PING_BODY" | grep -v "HTTP_STATUS:")

echo ">> HTTP status: $PING_HTTP"
echo ">> body: $PING_JSON"

if [ "$PING_HTTP" != "200" ]; then
    echo "ERROR: /v1/ping returned HTTP $PING_HTTP (expected 200)"
    exit 1
fi
echo ">> /v1/ping OK"

# ── All steps passed ──────────────────────────────────────────────────────────
echo ""
echo "=== happy_path.sh PASSED ==="
echo "  /readyz    → 200"
echo "  /v1/greet/1 → 200, body contains 'hello from sandbox'"
echo "  /v1/ping   → 200"
