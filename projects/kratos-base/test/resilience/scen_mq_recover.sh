#!/usr/bin/env bash
# AC-M2: MQ 不重启自愈 — 接 AC-M1 态（rabbit 宕、demo 运行中）→
#   start rabbitmq（demo 不重启）→ 轮询 /readyz→200 →
#   POST /v1/events（唯一 payload recover-$$）→ 200 返回 id →
#   轮询 demo 日志出现消费记录且含该 payload →（消费者自动连上并消费）→
#   同 pid 断言。
# Usage: bash test/resilience/scen_mq_recover.sh
# CWD-independent: cd to project root from this script's location.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.." || { echo "cannot cd to project root"; exit 1; }

DEMO_PID=""
COMPOSE_CMD="docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml"

cleanup() {
    echo ""
    echo "=== [AC-M2] cleanup ==="
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

echo "=== AC-M2: scen_mq_recover ==="

# ── Step 1: ensure clean sandbox ──────────────────────────────────────────────
echo ""
echo "=== Step 1: ensure sandbox is down ==="
make sandbox-down 2>/dev/null || true
echo ">> sandbox is down"

# ── Step 2: sandbox-up (all four services healthy) ───────────────────────────
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

./bin/demo -conf configs/bootstrap.sandbox.yaml >/tmp/demo_mq_recover.log 2>&1 &
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
        cat /tmp/demo_mq_recover.log || true
        exit 1
    fi
    sleep 1
done

if [ "$STARTED" -ne 1 ]; then
    echo "ERROR: demo never started after 20 attempts"
    cat /tmp/demo_mq_recover.log || true
    exit 1
fi

# ── Step 5: confirm /readyz = 503 initially (rabbit absent) ─────────────────
echo ""
echo "=== Step 5: confirm /readyz = 503 (rabbit not yet up) ==="
READYZ_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz)
echo ">> /readyz → $READYZ_CODE"
if [ "$READYZ_CODE" != "503" ]; then
    echo "FAIL AC-M2: expected /readyz=503 before rabbit start, got $READYZ_CODE"
    exit 1
fi
echo ">> PASS: /readyz=503 confirmed (rabbit absent)"

# ── Step 6: start rabbitmq (demo is NOT restarted) ───────────────────────────
echo ""
echo "=== Step 6: start rabbitmq (demo stays running, same pid: $DEMO_PID) ==="
$COMPOSE_CMD start rabbitmq
echo ">> rabbitmq container started"

# Wait for rabbitmq container to become healthy
echo ">> waiting for rabbitmq container to be healthy..."
for i in $(seq 1 90); do
    hc=$(docker inspect --format='{{.State.Health.Status}}' kratosbase-sandbox-rabbitmq-1 2>/dev/null || echo "unknown")
    echo "  rabbitmq health attempt $i: $hc"
    if [ "$hc" = "healthy" ]; then
        echo ">> rabbitmq container is healthy"
        break
    fi
    if [ "$i" -eq 90 ]; then
        echo "FAIL AC-M2: rabbitmq container did not become healthy after 90s"
        exit 1
    fi
    sleep 1
done

# Verify demo was NOT restarted
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-M2: demo process $DEMO_PID died during rabbitmq start"
    exit 1
fi
echo ">> demo pid $DEMO_PID still alive (not restarted)"

# ── Step 7: poll /readyz until 200 (self-heal, max 60s) ──────────────────────
echo ""
echo "=== Step 7: poll /readyz until 200 (self-heal without restart) ==="
HEALED=0
for i in $(seq 1 60); do
    READYZ=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "000")
    echo "  attempt $i: /readyz → $READYZ"
    if [ "$READYZ" = "200" ]; then
        HEALED=1
        echo ">> PASS: /readyz self-healed to 200 on attempt $i (demo not restarted)"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-M2: demo process died during heal poll"
        exit 1
    fi
    sleep 1
done

if [ "$HEALED" -ne 1 ]; then
    echo "FAIL AC-M2: /readyz never healed to 200 after 60 attempts"
    exit 1
fi

# ── Step 8: POST /v1/events with unique payload ───────────────────────────────
echo ""
echo "=== Step 8: POST /v1/events (unique payload) ==="
UNIQUE_PAYLOAD="recover-$$"
echo ">> payload: $UNIQUE_PAYLOAD"

EVENTS_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    -X POST http://localhost:8000/v1/events \
    -H 'Content-Type: application/json' \
    -d "{\"payload\":\"$UNIQUE_PAYLOAD\"}" 2>/dev/null || echo "HTTP_STATUS:000")
EVENTS_CODE=$(echo "$EVENTS_RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
EVENTS_BODY=$(echo "$EVENTS_RESP" | grep -v "HTTP_STATUS:")
echo ">> POST /v1/events → $EVENTS_CODE  body: $EVENTS_BODY"

if [ "$EVENTS_CODE" != "200" ]; then
    echo "FAIL AC-M2: POST /v1/events expected 200 after rabbit recovered, got $EVENTS_CODE"
    exit 1
fi

EVENT_ID=$(echo "$EVENTS_BODY" | grep -oE '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | grep -oE '"[^"]*"$' | tr -d '"' || echo "")
echo ">> PASS POST /v1/events = 200, id=$EVENT_ID"

# ── Step 9: poll consumer receipt — anchored on the consumer log line + event id ──
echo ""
echo "=== Step 9: poll consumer receipt of event id '$EVENT_ID' ==="
# consumer_runner.go handler logs a structured line: "consumer":"received" with
# "key":"<event id>" (the publisher carries the id as MessageId). Anchor BOTH —
# a bare payload grep would also match the HTTP access log of the publish
# request (it echoes args), which proves publication, not consumption.
if [ -z "$EVENT_ID" ]; then
    echo "FAIL AC-M2: could not extract event id from publish response: $EVENTS_BODY"
    exit 1
fi
CONSUMED=0
for i in $(seq 1 60); do
    if grep '"consumer":"received"' /tmp/demo_mq_recover.log 2>/dev/null | grep -q "$EVENT_ID"; then
        CONSUMED=1
        CONSUME_LINE=$(grep '"consumer":"received"' /tmp/demo_mq_recover.log | grep "$EVENT_ID" | tail -1)
        echo ">> PASS: consumer logged receipt of event $EVENT_ID on attempt $i"
        echo "   log line: $CONSUME_LINE"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-M2: demo process died while waiting for consumer"
        exit 1
    fi
    sleep 1
done

if [ "$CONSUMED" -ne 1 ]; then
    echo "FAIL AC-M2: consumer did not log receipt of event id '$EVENT_ID' within 60s"
    echo "=== demo log tail ==="
    tail -30 /tmp/demo_mq_recover.log || true
    exit 1
fi

# ── Step 10: confirm demo was NOT restarted ───────────────────────────────────
echo ""
echo "=== Step 10: confirm demo process identity (no restart) ==="
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-M2: demo pid $DEMO_PID is gone — it was restarted"
    exit 1
fi
echo ">> PASS demo pid $DEMO_PID is still the same process (no restart)"

echo ""
echo "=== scen_mq_recover.sh PASSED (AC-M2) ==="
echo "  rabbit start (no demo restart) → /readyz healed from 503 → 200"
echo "  POST /v1/events = 200, payload=$UNIQUE_PAYLOAD id=$EVENT_ID"
echo "  consumer received the message (supervisor loop auto-reconnected)"
echo "  demo process never restarted (same pid: $DEMO_PID)"
