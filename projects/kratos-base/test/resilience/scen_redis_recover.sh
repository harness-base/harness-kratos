#!/usr/bin/env bash
# AC-R2: redis 不重启自愈 — 接 AC-R1 态（redis 宕、demo 运行中）→
#   start redis（demo 不重启）→ 轮询 /readyz→200 → /v1/hits 连续两次 count 递增。
# Usage: bash test/resilience/scen_redis_recover.sh
# CWD-independent: cd to project root from this script's location.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.." || { echo "cannot cd to project root"; exit 1; }

DEMO_PID=""
COMPOSE_CMD="docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml"

cleanup() {
    echo ""
    echo "=== [AC-R2] cleanup ==="
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

echo "=== AC-R2: scen_redis_recover ==="

# ── Step 1: ensure clean sandbox ──────────────────────────────────────────────
echo ""
echo "=== Step 1: ensure sandbox is down ==="
make sandbox-down 2>/dev/null || true
echo ">> sandbox is down"

# ── Step 2: sandbox-up (both pg + redis healthy) ──────────────────────────────
echo ""
echo "=== Step 2: sandbox-up (pg + redis both healthy) ==="
make sandbox-up
echo ">> pg + redis are healthy"

# ── Step 3: stop redis ────────────────────────────────────────────────────────
echo ""
echo "=== Step 3: stop redis (keep pg running) ==="
$COMPOSE_CMD stop redis
echo ">> redis stopped"

# ── Step 4: build and start demo with redis absent ────────────────────────────
echo ""
echo "=== Step 4: build and start demo (redis is down) ==="
go build -o bin/demo ./app/demo/cmd
echo ">> built bin/demo"

./bin/demo -conf configs/bootstrap.sandbox.yaml >/tmp/demo_redis_recover.log 2>&1 &
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
        cat /tmp/demo_redis_recover.log || true
        exit 1
    fi
    sleep 1
done

if [ "$STARTED" -ne 1 ]; then
    echo "ERROR: demo never started after 20 attempts"
    exit 1
fi

# ── Step 5: confirm /readyz = 503 initially (redis absent) ───────────────────
echo ""
echo "=== Step 5: confirm /readyz = 503 (redis not yet up) ==="
READYZ_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz)
echo ">> /readyz → $READYZ_CODE"
if [ "$READYZ_CODE" != "503" ]; then
    echo "FAIL AC-R2: expected /readyz=503 before redis start, got $READYZ_CODE"
    exit 1
fi
echo ">> PASS: /readyz=503 confirmed (redis absent)"

# ── Step 6: start redis (demo is NOT restarted) ───────────────────────────────
echo ""
echo "=== Step 6: start redis (demo stays running, same pid: $DEMO_PID) ==="
$COMPOSE_CMD start redis
echo ">> redis container started"

# Wait for redis container to become healthy
echo ">> waiting for redis container to be healthy..."
for i in $(seq 1 30); do
    hc=$(docker inspect --format='{{.State.Health.Status}}' kratosbase-sandbox-redis-1 2>/dev/null || echo "unknown")
    echo "  redis health attempt $i: $hc"
    if [ "$hc" = "healthy" ]; then
        echo ">> redis container is healthy"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "FAIL AC-R2: redis container did not become healthy after 30s"
        exit 1
    fi
    sleep 1
done

# Verify demo was NOT restarted
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-R2: demo process $DEMO_PID died during redis start"
    exit 1
fi
echo ">> demo pid $DEMO_PID still alive (not restarted)"

# ── Step 7: poll /readyz until 200 (self-heal, max 30s) ──────────────────────
echo ""
echo "=== Step 7: poll /readyz until 200 (self-heal without restart) ==="
HEALED=0
for i in $(seq 1 30); do
    READYZ=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "000")
    echo "  attempt $i: /readyz → $READYZ"
    if [ "$READYZ" = "200" ]; then
        HEALED=1
        echo ">> PASS: /readyz self-healed to 200 on attempt $i (demo not restarted)"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-R2: demo process died during heal poll"
        exit 1
    fi
    sleep 1
done

if [ "$HEALED" -ne 1 ]; then
    echo "FAIL AC-R2: /readyz never healed to 200 after 30 attempts"
    exit 1
fi

# ── Step 8: /v1/hits two consecutive calls → count increments ────────────────
echo ""
echo "=== Step 8: /v1/hits two consecutive calls → count must increment ==="
HITS_RESP1=$(curl -s -w "\nHTTP_STATUS:%{http_code}" http://localhost:8000/v1/hits 2>/dev/null || echo "HTTP_STATUS:000")
HITS_CODE1=$(echo "$HITS_RESP1" | grep "HTTP_STATUS:" | cut -d: -f2)
HITS_BODY1=$(echo "$HITS_RESP1" | grep -v "HTTP_STATUS:")
echo ">> call 1: HTTP $HITS_CODE1  body: $HITS_BODY1"
if [ "$HITS_CODE1" != "200" ]; then
    echo "FAIL AC-R2: /v1/hits call 1 expected 200, got $HITS_CODE1"
    exit 1
fi
COUNT1=$(echo "$HITS_BODY1" | grep -oE '"count"[[:space:]]*:[[:space:]]*"?[0-9]+"?' | grep -oE '[0-9]+' | tail -1 || echo "0")

HITS_RESP2=$(curl -s -w "\nHTTP_STATUS:%{http_code}" http://localhost:8000/v1/hits 2>/dev/null || echo "HTTP_STATUS:000")
HITS_CODE2=$(echo "$HITS_RESP2" | grep "HTTP_STATUS:" | cut -d: -f2)
HITS_BODY2=$(echo "$HITS_RESP2" | grep -v "HTTP_STATUS:")
echo ">> call 2: HTTP $HITS_CODE2  body: $HITS_BODY2"
if [ "$HITS_CODE2" != "200" ]; then
    echo "FAIL AC-R2: /v1/hits call 2 expected 200, got $HITS_CODE2"
    exit 1
fi
COUNT2=$(echo "$HITS_BODY2" | grep -oE '"count"[[:space:]]*:[[:space:]]*"?[0-9]+"?' | grep -oE '[0-9]+' | tail -1 || echo "0")

echo ">> count after call 1: $COUNT1"
echo ">> count after call 2: $COUNT2"

if ! awk "BEGIN { exit ($COUNT2 > $COUNT1) ? 0 : 1 }"; then
    echo "FAIL AC-R2: count did not increment: call1=$COUNT1 call2=$COUNT2"
    exit 1
fi
echo ">> PASS /v1/hits count increments: $COUNT1 → $COUNT2"

# ── Step 9: confirm demo was NOT restarted ───────────────────────────────────
echo ""
echo "=== Step 9: confirm demo process identity (no restart) ==="
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-R2: demo pid $DEMO_PID is gone — it was restarted"
    exit 1
fi
echo ">> PASS demo pid $DEMO_PID is still the same process (no restart)"

echo ""
echo "=== scen_redis_recover.sh PASSED (AC-R2) ==="
echo "  redis start (no demo restart) → /readyz healed from 503 → 200"
echo "  /v1/hits count increments: $COUNT1 → $COUNT2"
echo "  demo process never restarted (same pid: $DEMO_PID)"
