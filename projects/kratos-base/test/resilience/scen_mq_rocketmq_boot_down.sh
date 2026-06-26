#!/usr/bin/env bash
# AC-MR2: RocketMQ 启动期宕 + 自愈 e2e
# 流程：rocketmq 不可达时 demo 照起 → /v1/ping=200 → /readyz 反映 mq 不健康(503)
#   （readyz 用 15s 探测 ctx，宕时等探测超时才 503，轮询给足）→
#   起 rocketmq → 自愈：/readyz=200 → 发布→消费成功（id 锚定），进程未重启。
# Usage: bash test/resilience/scen_mq_rocketmq_boot_down.sh
# CWD-independent: cd to project root from this script's location.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.." || { echo "cannot cd to project root"; exit 1; }

DEMO_PID=""
COMPOSE_CMD="docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml"

cleanup() {
    echo ""
    echo "=== [AC-MR2] cleanup ==="
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
    # ensure rocketmq is running before sandbox-down (idempotent)
    $COMPOSE_CMD start rmqnamesrv rmqbroker 2>/dev/null || true
    echo ">> make sandbox-down"
    make sandbox-down 2>/dev/null || true
}
trap cleanup EXIT

echo "=== AC-MR2: scen_mq_rocketmq_boot_down (startup with rocketmq absent → self-heal) ==="

# ── Step 1: ensure clean sandbox ──────────────────────────────────────────────
echo ""
echo "=== Step 1: ensure sandbox is down ==="
make sandbox-down 2>/dev/null || true
echo ">> sandbox is down"

# ── Step 2: sandbox-up (all services) ────────────────────────────────────────
echo ""
echo "=== Step 2: sandbox-up (pg + redis + etcd + rabbitmq + nacos + rocketmq all healthy) ==="
make sandbox-up
echo ">> all services healthy"

# ── Step 3: stop rocketmq broker + namesrv ───────────────────────────────────
echo ""
echo "=== Step 3: stop rmqbroker + rmqnamesrv (keep pg + redis + etcd running) ==="
$COMPOSE_CMD stop rmqbroker
$COMPOSE_CMD stop rmqnamesrv
echo ">> rmqbroker and rmqnamesrv stopped"

# ── Step 4: build and start demo with rocketmq config (rocketmq is down) ─────
echo ""
echo "=== Step 4: build and start demo (rocketmq is down) ==="
go build -o bin/demo ./app/demo/cmd
echo ">> built bin/demo"

./bin/demo -conf configs/bootstrap.rocketmq-sandbox.yaml >/tmp/demo_rmq_boot_down.log 2>&1 &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID"

# Poll until demo HTTP server is listening (healthz, not readyz — mq is down)
STARTED=0
for i in $(seq 1 30); do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/healthz 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "200" ]; then
        STARTED=1
        echo ">> demo started (healthz=200, attempt $i)"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "ERROR: demo process died unexpectedly"
        cat /tmp/demo_rmq_boot_down.log || true
        exit 1
    fi
    sleep 1
done

if [ "$STARTED" -ne 1 ]; then
    echo "ERROR: demo never started (healthz) after 30 attempts"
    cat /tmp/demo_rmq_boot_down.log || true
    exit 1
fi

# ── Step 5: assert /v1/ping = 200 (no MQ dep) ────────────────────────────────
echo ""
echo "=== Step 5: assert /v1/ping = 200 (no MQ dep) ==="
PING_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/v1/ping)
echo ">> /v1/ping → $PING_CODE"
if [ "$PING_CODE" != "200" ]; then
    echo "FAIL AC-MR2: /v1/ping expected 200, got $PING_CODE"
    exit 1
fi
echo ">> PASS /v1/ping = 200"

# ── Step 6: poll /readyz until 503 (mq unhealthy — wait up to 45s for probe) ─
# readyz uses a 15s probe ctx; with rocketmq down the producer Start() times
# out after requestTimeout(10s) and the health check reports mq down.
# We poll up to 45s to give the first probe cycle time to run.
echo ""
echo "=== Step 6: poll /readyz until 503 (mq unhealthy, up to 45s for probe timeout) ==="
READYZ_503=0
for i in $(seq 1 45); do
    READYZ_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "HTTP_STATUS:000")
    READYZ_CODE=$(echo "$READYZ_RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
    READYZ_BODY=$(echo "$READYZ_RESP" | grep -v "HTTP_STATUS:")
    echo "  attempt $i: /readyz → $READYZ_CODE  body: $READYZ_BODY"
    if [ "$READYZ_CODE" = "503" ]; then
        READYZ_503=1
        echo ">> /readyz = 503 (mq unhealthy detected on attempt $i)"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-MR2: demo process died during readyz poll"
        exit 1
    fi
    sleep 1
done

if [ "$READYZ_503" -ne 1 ]; then
    echo "FAIL AC-MR2: /readyz never became 503 (mq unhealthy) after 45s"
    cat /tmp/demo_rmq_boot_down.log || true
    exit 1
fi

# Check body contains "mq"
if ! echo "$READYZ_BODY" | grep -q "mq"; then
    echo "FAIL AC-MR2: /readyz body does not contain 'mq': $READYZ_BODY"
    exit 1
fi
echo ">> PASS /readyz = 503 and body contains 'mq'"

# ── Step 7: assert process is alive ──────────────────────────────────────────
echo ""
echo "=== Step 7: assert demo process is alive ==="
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-MR2: demo process $DEMO_PID is not alive"
    exit 1
fi
echo ">> PASS demo process $DEMO_PID is alive"

# ── Step 8: start rocketmq (demo is NOT restarted) ───────────────────────────
echo ""
echo "=== Step 8: start rmqnamesrv + rmqbroker (demo stays running, same pid: $DEMO_PID) ==="
$COMPOSE_CMD start rmqnamesrv
echo ">> rmqnamesrv container started"

# Wait for rmqnamesrv to be healthy
echo ">> waiting for rmqnamesrv to be healthy..."
for i in $(seq 1 60); do
    hc=$(docker inspect --format='{{.State.Health.Status}}' kratosbase-sandbox-rmqnamesrv-1 2>/dev/null || echo "unknown")
    echo "  rmqnamesrv health attempt $i: $hc"
    if [ "$hc" = "healthy" ]; then
        echo ">> rmqnamesrv container is healthy"
        break
    fi
    if [ "$i" -eq 60 ]; then
        echo "FAIL AC-MR2: rmqnamesrv container did not become healthy after 60s"
        exit 1
    fi
    sleep 1
done

$COMPOSE_CMD start rmqbroker
echo ">> rmqbroker container started"

# Wait for rmqbroker (proxy) to be healthy
echo ">> waiting for rmqbroker to be healthy..."
for i in $(seq 1 120); do
    hc=$(docker inspect --format='{{.State.Health.Status}}' kratosbase-sandbox-rmqbroker-1 2>/dev/null || echo "unknown")
    echo "  rmqbroker health attempt $i: $hc"
    if [ "$hc" = "healthy" ]; then
        echo ">> rmqbroker container is healthy"
        break
    fi
    if [ "$i" -eq 120 ]; then
        echo "FAIL AC-MR2: rmqbroker container did not become healthy after 120s"
        exit 1
    fi
    sleep 1
done

# Ensure topic exists (mqadmin updateTopic is idempotent)
echo ">> ensuring topic demo-events exists..."
docker exec kratosbase-sandbox-rmqbroker-1 sh mqadmin updateTopic -n rmqnamesrv:9876 -t demo-events -c DefaultCluster 2>&1 | grep -v "^$" || true
echo ">> topic demo-events ensured"

# Verify demo was NOT restarted
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-MR2: demo process $DEMO_PID died during rocketmq start"
    exit 1
fi
echo ">> demo pid $DEMO_PID still alive (not restarted)"

# ── Step 9: poll /readyz until 200 (self-heal, max 120s) ─────────────────────
# The resource.Provider will retry the producer Build (via Get) in the next
# health probe cycle; once the producer starts successfully the breaker clears
# and readyz returns 200.
echo ""
echo "=== Step 9: poll /readyz until 200 (self-heal without restart, max 120s) ==="
HEALED=0
for i in $(seq 1 120); do
    READYZ=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "000")
    echo "  attempt $i: /readyz → $READYZ"
    if [ "$READYZ" = "200" ]; then
        HEALED=1
        echo ">> PASS: /readyz self-healed to 200 on attempt $i (demo not restarted)"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-MR2: demo process died during heal poll"
        exit 1
    fi
    sleep 1
done

if [ "$HEALED" -ne 1 ]; then
    echo "FAIL AC-MR2: /readyz never healed to 200 after 120 attempts"
    cat /tmp/demo_rmq_boot_down.log || true
    exit 1
fi

# ── Step 10: POST /v1/events with unique payload (self-heal e2e) ─────────────
echo ""
echo "=== Step 10: POST /v1/events (unique payload after self-heal) ==="
UNIQUE_PAYLOAD="rmq-boot-down-recover-$$"
echo ">> payload: $UNIQUE_PAYLOAD"

EVENTS_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    -X POST http://localhost:8000/v1/events \
    -H 'Content-Type: application/json' \
    -d "{\"payload\":\"$UNIQUE_PAYLOAD\"}" 2>/dev/null || echo "HTTP_STATUS:000")
EVENTS_CODE=$(echo "$EVENTS_RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
EVENTS_BODY=$(echo "$EVENTS_RESP" | grep -v "HTTP_STATUS:")
echo ">> POST /v1/events → $EVENTS_CODE  body: $EVENTS_BODY"

if [ "$EVENTS_CODE" != "200" ]; then
    echo "FAIL AC-MR2: POST /v1/events expected 200 after rocketmq healed, got $EVENTS_CODE"
    exit 1
fi

EVENT_ID=$(echo "$EVENTS_BODY" | grep -oE '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | grep -oE '"[^"]*"$' | tr -d '"' || echo "")
echo ">> PASS POST /v1/events = 200, id=$EVENT_ID"

# ── Step 11: poll consumer receipt — anchored on consumer log line + event id ─
echo ""
echo "=== Step 11: poll consumer receipt of event id '$EVENT_ID' ==="
if [ -z "$EVENT_ID" ]; then
    echo "FAIL AC-MR2: could not extract event id from publish response: $EVENTS_BODY"
    exit 1
fi
CONSUMED=0
for i in $(seq 1 60); do
    if grep '"consumer":"received"' /tmp/demo_rmq_boot_down.log 2>/dev/null | grep -q "$EVENT_ID"; then
        CONSUMED=1
        CONSUME_LINE=$(grep '"consumer":"received"' /tmp/demo_rmq_boot_down.log | grep "$EVENT_ID" | tail -1)
        echo ">> PASS: consumer logged receipt of event $EVENT_ID on attempt $i"
        echo "   log line: $CONSUME_LINE"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-MR2: demo process died while waiting for consumer"
        exit 1
    fi
    sleep 1
done

if [ "$CONSUMED" -ne 1 ]; then
    echo "FAIL AC-MR2: consumer did not log receipt of event id '$EVENT_ID' within 60s"
    echo "=== demo log tail ==="
    tail -30 /tmp/demo_rmq_boot_down.log || true
    exit 1
fi

# ── Step 12: confirm demo was NOT restarted ───────────────────────────────────
echo ""
echo "=== Step 12: confirm process identity (no restart) ==="
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-MR2: demo pid $DEMO_PID is gone — it was restarted"
    exit 1
fi
echo ">> PASS demo pid $DEMO_PID is still the same process (no restart)"

echo ""
echo "=== scen_mq_rocketmq_boot_down.sh PASSED (AC-MR2) ==="
echo "  rocketmq down → demo started (healthz=200)"
echo "  /v1/ping = 200 (no MQ dep)"
echo "  /readyz = 503 (mq check present)"
echo "  rocketmq start (no demo restart) → /readyz healed from 503 → 200"
echo "  POST /v1/events = 200, id=$EVENT_ID"
echo "  consumer received the message (id-anchored: '${CONSUME_LINE}')"
echo "  demo process never restarted (same pid: $DEMO_PID)"
