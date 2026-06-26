#!/usr/bin/env bash
# AC-MR1: RocketMQ publish→consume 闭环 e2e
# 流程：sandbox-up → demo(mode=rocketmq, -conf configs/bootstrap.rocketmq-sandbox.yaml)
#   /readyz=200 → HTTP 触发发布一个带唯一事件 id 的事件 →
#   轮询 demo 日志直到消费方结构化回执命中该事件 id
#   （"consumer":"received" + "key":"<event-id>"，照 rabbitmq 同字段）。
# 严禁匹配发布请求的访问日志回显。
# Usage: bash test/resilience/scen_mq_rocketmq.sh
# CWD-independent: cd to project root from this script's location.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.." || { echo "cannot cd to project root"; exit 1; }

DEMO_PID=""
COMPOSE_CMD="docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml"

cleanup() {
    echo ""
    echo "=== [AC-MR1] cleanup ==="
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

echo "=== AC-MR1: scen_mq_rocketmq (publish→consume e2e) ==="

# ── Step 1: ensure clean sandbox ──────────────────────────────────────────────
echo ""
echo "=== Step 1: ensure sandbox is down ==="
make sandbox-down 2>/dev/null || true
echo ">> sandbox is down"

# ── Step 2: sandbox-up (all services including rocketmq) ─────────────────────
echo ""
echo "=== Step 2: sandbox-up (pg + redis + etcd + rabbitmq + nacos + rmqnamesrv + rmqbroker) ==="
make sandbox-up
echo ">> all services healthy"

# ── Step 3: build and start demo with rocketmq config ────────────────────────
echo ""
echo "=== Step 3: build and start demo (rocketmq mode) ==="
go build -o bin/demo ./app/demo/cmd
echo ">> built bin/demo"

./bin/demo -conf configs/bootstrap.rocketmq-sandbox.yaml >/tmp/demo_mq_rocketmq.log 2>&1 &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID"

# ── Step 4: poll /readyz until 200 ───────────────────────────────────────────
echo ""
echo "=== Step 4: poll /readyz until 200 ==="
READY=0
for i in $(seq 1 90); do
    RC=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "000")
    echo "  attempt $i: /readyz → $RC"
    if [ "$RC" = "200" ]; then
        READY=1
        echo ">> /readyz = 200 (attempt $i)"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "ERROR: demo process died during startup"
        cat /tmp/demo_mq_rocketmq.log || true
        exit 1
    fi
    sleep 1
done

if [ "$READY" -ne 1 ]; then
    echo "FAIL AC-MR1: /readyz never became 200 after 90 attempts"
    cat /tmp/demo_mq_rocketmq.log || true
    exit 1
fi

# ── Step 5: POST /v1/events with unique event ─────────────────────────────────
echo ""
echo "=== Step 5: POST /v1/events (unique payload + event id) ==="
UNIQUE_PAYLOAD="rmq-e2e-$$"
echo ">> payload: $UNIQUE_PAYLOAD"

EVENTS_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    -X POST http://localhost:8000/v1/events \
    -H 'Content-Type: application/json' \
    -d "{\"payload\":\"$UNIQUE_PAYLOAD\"}" 2>/dev/null || echo "HTTP_STATUS:000")
EVENTS_CODE=$(echo "$EVENTS_RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
EVENTS_BODY=$(echo "$EVENTS_RESP" | grep -v "HTTP_STATUS:")
echo ">> POST /v1/events → $EVENTS_CODE  body: $EVENTS_BODY"

if [ "$EVENTS_CODE" != "200" ]; then
    echo "FAIL AC-MR1: POST /v1/events expected 200, got $EVENTS_CODE"
    cat /tmp/demo_mq_rocketmq.log || true
    exit 1
fi

EVENT_ID=$(echo "$EVENTS_BODY" | grep -oE '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | grep -oE '"[^"]*"$' | tr -d '"' || echo "")
if [ -z "$EVENT_ID" ]; then
    echo "FAIL AC-MR1: could not extract event id from publish response: $EVENTS_BODY"
    exit 1
fi
echo ">> PASS POST /v1/events = 200, id=$EVENT_ID"

# ── Step 6: poll consumer receipt — anchored on consumer log line + event id ──
echo ""
echo "=== Step 6: poll consumer receipt of event id '$EVENT_ID' ==="
# consumer_runner.go handler logs: "consumer":"received" with "key":"<event id>"
# Anchoring on BOTH fields ensures we match actual consumption, not the publish
# request access log (which would be a false positive).
CONSUMED=0
for i in $(seq 1 60); do
    if grep '"consumer":"received"' /tmp/demo_mq_rocketmq.log 2>/dev/null | grep -q "$EVENT_ID"; then
        CONSUMED=1
        CONSUME_LINE=$(grep '"consumer":"received"' /tmp/demo_mq_rocketmq.log | grep "$EVENT_ID" | tail -1)
        echo ">> PASS consumer logged receipt of event $EVENT_ID on attempt $i"
        echo "   log line: $CONSUME_LINE"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-MR1: demo process died while waiting for consumer"
        exit 1
    fi
    sleep 1
done

if [ "$CONSUMED" -ne 1 ]; then
    echo "FAIL AC-MR1: consumer did not log receipt of event id '$EVENT_ID' within 60s"
    echo "=== demo log tail ==="
    tail -30 /tmp/demo_mq_rocketmq.log || true
    exit 1
fi

echo ""
echo "=== scen_mq_rocketmq.sh PASSED (AC-MR1) ==="
echo "  /readyz   → 200 (rocketmq healthy)"
echo "  POST /v1/events = 200, id=$EVENT_ID"
echo "  consumer received the message (id-anchored receipt: '${CONSUME_LINE}')"
