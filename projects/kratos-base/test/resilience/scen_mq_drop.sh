#!/usr/bin/env bash
# AC-M3: MQ 运行中断连 → 快速失败（熔断）→ 恢复续上。全程不重启 demo。
# 流程：sandbox-up + 起 demo → readyz=200、发一条+消费日志确认基线 →
#   stop rabbitmq → POST /v1/events 快速失败 503 (curl -w %{time_total} 有界)、
#   /readyz=503、进程活 →
#   start rabbitmq → 轮询 readyz=200 → 再发一条唯一 payload → 轮询消费日志出现它
#   （证明消费者断线后自动重订阅续上）→ 全程同 pid。
# Usage: bash test/resilience/scen_mq_drop.sh
# CWD-independent: cd to project root from this script's location.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.." || { echo "cannot cd to project root"; exit 1; }

DEMO_PID=""
COMPOSE_CMD="docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml"
FAST_FAIL_TIME="9.999"

cleanup() {
    echo ""
    echo "=== [AC-M3] cleanup ==="
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

echo "=== AC-M3: scen_mq_drop ==="

# ── Step 1: sandbox-up + start demo ──────────────────────────────────────────
echo ""
echo "=== Step 1: sandbox-up (pg + redis + etcd + rabbitmq all healthy) ==="
make sandbox-down 2>/dev/null || true
make sandbox-up
echo ">> all four services healthy"

echo ""
echo "=== Step 2: build and start demo ==="
go build -o bin/demo ./app/demo/cmd
./bin/demo -conf configs/bootstrap.sandbox.yaml >/tmp/demo_mq_drop.log 2>&1 &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID"

# Poll /readyz until 200
echo ""
echo "=== Step 3: poll /readyz until 200 ==="
READY=0
for i in $(seq 1 60); do
    RC=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "000")
    echo "  attempt $i: /readyz → $RC"
    if [ "$RC" = "200" ]; then
        READY=1
        echo ">> /readyz = 200 (attempt $i)"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "ERROR: demo died during startup"
        cat /tmp/demo_mq_drop.log || true
        exit 1
    fi
    sleep 1
done
if [ "$READY" -ne 1 ]; then
    echo "FAIL AC-M3: /readyz never became 200 in step 3"
    cat /tmp/demo_mq_drop.log || true
    exit 1
fi

# ── Step 4: baseline — send one event, confirm consumer receives it ───────────
echo ""
echo "=== Step 4: baseline — POST /v1/events + confirm consumer receipt ==="
BASELINE_PAYLOAD="baseline-mq-drop-$$"
EVENTS_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    -X POST http://localhost:8000/v1/events \
    -H 'Content-Type: application/json' \
    -d "{\"payload\":\"$BASELINE_PAYLOAD\"}" 2>/dev/null || echo "HTTP_STATUS:000")
EVENTS_CODE=$(echo "$EVENTS_RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
EVENTS_BODY=$(echo "$EVENTS_RESP" | grep -v "HTTP_STATUS:")
echo ">> POST /v1/events → $EVENTS_CODE  body: $EVENTS_BODY"

if [ "$EVENTS_CODE" != "200" ]; then
    echo "FAIL AC-M3: baseline POST /v1/events expected 200, got $EVENTS_CODE"
    exit 1
fi

BASELINE_EVENT_ID=$(echo "$EVENTS_BODY" | grep -oE '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | grep -oE '"[^"]*"$' | tr -d '"' || echo "")
if [ -z "$BASELINE_EVENT_ID" ]; then
    echo "FAIL AC-M3: could not extract baseline event id from: $EVENTS_BODY"
    exit 1
fi

# Poll for consumer receipt — anchored on the consumer log line + event id.
# (A bare payload grep would match the publish request's HTTP access log,
# which proves publication, not consumption.)
BASELINE_CONSUMED=0
for i in $(seq 1 30); do
    if grep '"consumer":"received"' /tmp/demo_mq_drop.log 2>/dev/null | grep -q "$BASELINE_EVENT_ID"; then
        BASELINE_CONSUMED=1
        BL_LINE=$(grep '"consumer":"received"' /tmp/demo_mq_drop.log | grep "$BASELINE_EVENT_ID" | tail -1)
        echo ">> PASS baseline consumer logged receipt of $BASELINE_EVENT_ID on attempt $i"
        echo "   log line: $BL_LINE"
        break
    fi
    sleep 1
done

if [ "$BASELINE_CONSUMED" -ne 1 ]; then
    echo "FAIL AC-M3: consumer did not log receipt of baseline event '$BASELINE_EVENT_ID' after 30s"
    tail -20 /tmp/demo_mq_drop.log || true
    exit 1
fi
echo ">> PASS baseline: /readyz=200, POST /v1/events=200, consumer received (id-anchored)"

# ── Step 5: stop rabbitmq (simulate runtime drop) ────────────────────────────
echo ""
echo "=== Step 5: stop rabbitmq (simulate runtime drop) ==="
$COMPOSE_CMD stop rabbitmq
echo ">> rabbitmq stopped"

# ── Step 6: warm up the breaker — send several requests to trip it ────────────
echo ""
echo "=== Step 6: send requests to trip the circuit breaker ==="
for i in $(seq 1 15); do
    CODE=$(curl -s -o /dev/null -w "%{http_code}" \
        -X POST http://localhost:8000/v1/events \
        -H 'Content-Type: application/json' \
        -d '{"payload":"breaker-warmup"}' 2>/dev/null || echo "000")
    echo "  request $i: → $CODE"
done

# ── Step 7: measure fast-fail time after breaker opens ───────────────────────
echo ""
echo "=== Step 7: measure fast-fail time (should be << dial_timeout 5s) ==="
FAST_FAIL_TIME=$(curl -s -o /dev/null -w "%{time_total}" \
    -X POST http://localhost:8000/v1/events \
    -H 'Content-Type: application/json' \
    -d '{"payload":"fast-fail-probe"}' 2>/dev/null || echo "9.999")
FAIL_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8000/v1/events \
    -H 'Content-Type: application/json' \
    -d '{"payload":"fast-fail-probe2"}' 2>/dev/null || echo "000")
echo ">> POST /v1/events time: ${FAST_FAIL_TIME}s  status: $FAIL_CODE"

# Must be 5xx (fast-fail returns error)
if [[ "$FAIL_CODE" != 5* ]]; then
    echo "FAIL AC-M3: POST /v1/events expected 5xx after rabbit drop, got $FAIL_CODE"
    exit 1
fi
echo ">> PASS POST /v1/events = $FAIL_CODE after rabbit drop"

# Dial timeout is 5s (from config); assert fast-fail is well below that.
# Circuit breaker open => < 0.1s; ECONNREFUSED => < 0.5s.
# HARD bound 2.0s: still far under the 5s dial timeout, but exceeding it means the
# request blocked on the full dial (fast-fail regression). rule-0009 §C forbids a
# WARN/continue passthrough for a claimed guarantee, so FAIL hard here
# (aligned with scen_mq_rocketmq_drop.sh / scen_redis_drop.sh).
if awk "BEGIN {exit ($FAST_FAIL_TIME < 2.0) ? 0 : 1}"; then
    if awk "BEGIN {exit ($FAST_FAIL_TIME < 0.2) ? 0 : 1}"; then
        echo ">> PASS fast-fail: ${FAST_FAIL_TIME}s < 0.2s (circuit breaker open)"
    else
        echo ">> PASS fast-fail: ${FAST_FAIL_TIME}s < 2.0s (connection refused or circuit open, well below 5s dial timeout)"
    fi
else
    echo "FAIL AC-M3: POST /v1/events took ${FAST_FAIL_TIME}s ≥ 2.0s — NOT fast-failing (blocked on ~5s dial timeout regression)"
    exit 1
fi

# ── Step 8: assert /readyz = 503, process alive ───────────────────────────────
echo ""
echo "=== Step 8: assert /readyz = 503 and process alive ==="
READYZ_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz)
echo ">> /readyz → $READYZ_CODE"
if [ "$READYZ_CODE" != "503" ]; then
    echo "FAIL AC-M3: /readyz expected 503 after rabbit drop, got $READYZ_CODE"
    exit 1
fi
echo ">> PASS /readyz = 503 after rabbit drop"

if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-M3: demo process $DEMO_PID died"
    exit 1
fi
echo ">> PASS demo process $DEMO_PID is alive"

# ── Step 9: restore rabbitmq ─────────────────────────────────────────────────
echo ""
echo "=== Step 9: start rabbitmq (restore) ==="
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
        echo "FAIL AC-M3: rabbitmq container did not become healthy after 90s"
        exit 1
    fi
    sleep 1
done

# ── Step 10: poll /readyz until 200 (auto-recovery, no demo restart) ─────────
echo ""
echo "=== Step 10: poll /readyz until 200 (auto-recovery, no demo restart) ==="
RECOVERED=0
for i in $(seq 1 90); do
    RC=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "000")
    echo "  attempt $i: /readyz → $RC"
    if [ "$RC" = "200" ]; then
        RECOVERED=1
        echo ">> PASS /readyz recovered to 200 on attempt $i"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-M3: demo process died during recovery"
        exit 1
    fi
    sleep 1
done

if [ "$RECOVERED" -ne 1 ]; then
    echo "FAIL AC-M3: /readyz did not recover to 200 after 90 attempts"
    exit 1
fi

# ── Step 11: send a second unique payload and verify consumer receives it ─────
echo ""
echo "=== Step 11: send unique payload after recovery, poll consumer receipt ==="
RECOVER_PAYLOAD="post-drop-recover-$$"
echo ">> payload: $RECOVER_PAYLOAD"

# Record current log line count to detect new consumer log after recovery
EVENTS_RESP2=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    -X POST http://localhost:8000/v1/events \
    -H 'Content-Type: application/json' \
    -d "{\"payload\":\"$RECOVER_PAYLOAD\"}" 2>/dev/null || echo "HTTP_STATUS:000")
EVENTS_CODE2=$(echo "$EVENTS_RESP2" | grep "HTTP_STATUS:" | cut -d: -f2)
EVENTS_BODY2=$(echo "$EVENTS_RESP2" | grep -v "HTTP_STATUS:")
echo ">> POST /v1/events → $EVENTS_CODE2  body: $EVENTS_BODY2"

if [ "$EVENTS_CODE2" != "200" ]; then
    echo "FAIL AC-M3: POST /v1/events after recovery expected 200, got $EVENTS_CODE2"
    exit 1
fi
echo ">> PASS POST /v1/events = 200 after recovery"

RECOVER_EVENT_ID=$(echo "$EVENTS_BODY2" | grep -oE '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | grep -oE '"[^"]*"$' | tr -d '"' || echo "")
if [ -z "$RECOVER_EVENT_ID" ]; then
    echo "FAIL AC-M3: could not extract recovery event id from: $EVENTS_BODY2"
    exit 1
fi

# Poll for consumer receipt — anchored on the consumer log line + event id
# (proves the consumer re-subscribed after the drop and actually consumed it;
# a bare payload grep would match the publish request's access log instead).
RECOVER_CONSUMED=0
for i in $(seq 1 60); do
    if grep '"consumer":"received"' /tmp/demo_mq_drop.log 2>/dev/null | grep -q "$RECOVER_EVENT_ID"; then
        RECOVER_CONSUMED=1
        RC_LINE=$(grep '"consumer":"received"' /tmp/demo_mq_drop.log | grep "$RECOVER_EVENT_ID" | tail -1)
        echo ">> PASS consumer logged receipt of recovery event $RECOVER_EVENT_ID on attempt $i"
        echo "   log line: $RC_LINE"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-M3: demo process died while waiting for recovery consumer"
        exit 1
    fi
    sleep 1
done

if [ "$RECOVER_CONSUMED" -ne 1 ]; then
    echo "FAIL AC-M3: consumer did not log receipt of recovery event '$RECOVER_EVENT_ID' (60s)"
    echo "=== demo log tail ==="
    tail -30 /tmp/demo_mq_drop.log || true
    exit 1
fi

# ── Step 12: confirm demo was never restarted ─────────────────────────────────
echo ""
echo "=== Step 12: confirm process identity (no restart) ==="
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-M3: demo process $DEMO_PID is gone"
    exit 1
fi
echo ">> PASS demo pid $DEMO_PID alive (no restart)"

echo ""
echo "=== scen_mq_drop.sh PASSED (AC-M3) ==="
echo "  baseline: /readyz=200, POST /v1/events=200, consumer received"
echo "  rabbit drop → fast-fail (${FAST_FAIL_TIME}s), /readyz=503, process alive"
echo "  rabbit restore → /readyz=200 (auto-recovery)"
echo "  post-recovery: POST /v1/events=200, consumer re-subscribed and received payload"
echo "  demo process never restarted (same pid: $DEMO_PID)"
