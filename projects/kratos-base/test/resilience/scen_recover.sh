#!/usr/bin/env bash
# AC2: 按需连 + 自愈 — PG 起来后，不重启 demo，/readyz 自愈为 200。
# Usage: bash test/resilience/scen_recover.sh
# Must be run from the project root (projects/kratos-base/).
set -euo pipefail

DEMO_PID=""

cleanup() {
    echo ""
    echo "=== [AC2] cleanup ==="
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
    echo ">> make sandbox-down"
    make sandbox-down 2>/dev/null || true
}
trap cleanup EXIT

echo "=== AC2: scen_recover ==="

# ── Step 1: ensure PG is NOT running ─────────────────────────────────────────
echo ""
echo "=== Step 1: ensure sandbox is down ==="
make sandbox-down 2>/dev/null || true
echo ">> sandbox is down"

# ── Step 2: build & start demo with PG absent ────────────────────────────────
echo ""
echo "=== Step 2: build and start demo (PG is down) ==="
go build -o bin/demo ./app/demo/cmd
echo ">> built bin/demo"

./bin/demo -conf configs/bootstrap.sandbox.yaml >/tmp/demo_recover.log 2>&1 &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID"

# Poll until demo is up (healthz responds)
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
        cat /tmp/demo_recover.log || true
        exit 1
    fi
    sleep 1
done

if [ "$STARTED" -ne 1 ]; then
    echo "ERROR: demo never started"
    exit 1
fi

# ── Step 3: confirm /readyz = 503 initially ───────────────────────────────────
echo ""
echo "=== Step 3: confirm /readyz = 503 (PG not yet up) ==="
READYZ_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz)
echo ">> /readyz → $READYZ_CODE"
if [ "$READYZ_CODE" != "503" ]; then
    echo "FAIL AC2: expected /readyz=503 before PG start, got $READYZ_CODE"
    exit 1
fi
echo ">> PASS: /readyz=503 confirmed before PG"

# ── Step 4: start PG sandbox (demo is NOT restarted) ─────────────────────────
echo ""
echo "=== Step 4: sandbox-up (start PG, demo stays running) ==="
echo ">> demo pid before sandbox-up: $DEMO_PID"
make sandbox-up
echo ">> sandbox PG is up"
echo ">> demo pid after sandbox-up: $DEMO_PID"

# Verify demo was NOT restarted
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC2: demo process $DEMO_PID died during sandbox-up"
    exit 1
fi

# ── Step 5: poll /readyz until 200 (self-heal, max 30s) ──────────────────────
echo ""
echo "=== Step 5: poll /readyz until 200 (self-heal without restart) ==="
HEALED=0
for i in $(seq 1 30); do
    READYZ=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "000")
    echo "  attempt $i: /readyz → $READYZ"
    if [ "$READYZ" = "200" ]; then
        HEALED=1
        echo ">> PASS: /readyz self-healed to 200 on attempt $i"
        break
    fi
    # Verify demo still the same process
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC2: demo process died during heal poll"
        exit 1
    fi
    sleep 1
done

if [ "$HEALED" -ne 1 ]; then
    echo "FAIL AC2: /readyz never healed to 200 after 30 attempts"
    exit 1
fi

# ── Step 6: assert /v1/greet/1 = 200 with seed data ──────────────────────────
echo ""
echo "=== Step 6: assert /v1/greet/1 = 200 with 'hello from sandbox' ==="
GREET_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" http://localhost:8000/v1/greet/1)
GREET_CODE=$(echo "$GREET_RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
GREET_BODY=$(echo "$GREET_RESP" | grep -v "HTTP_STATUS:")
echo ">> HTTP status: $GREET_CODE"
echo ">> body: $GREET_BODY"

if [ "$GREET_CODE" != "200" ]; then
    echo "FAIL AC2: /v1/greet/1 expected 200, got $GREET_CODE"
    exit 1
fi

if ! echo "$GREET_BODY" | grep -q "hello from sandbox"; then
    echo "FAIL AC2: response does not contain 'hello from sandbox'"
    exit 1
fi
echo ">> PASS /v1/greet/1 = 200 with 'hello from sandbox'"

# ── Step 7: confirm demo was NOT restarted ───────────────────────────────────
echo ""
echo "=== Step 7: confirm demo process identity (no restart) ==="
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC2: demo pid $DEMO_PID is gone — it was restarted"
    exit 1
fi
echo ">> PASS demo pid $DEMO_PID is still the same process (no restart)"

echo ""
echo "=== scen_recover.sh PASSED (AC2) ==="
echo "  /readyz healed from 503 → 200 after PG start"
echo "  /v1/greet/1 → 200, 'hello from sandbox'"
echo "  demo process never restarted (same pid: $DEMO_PID)"
