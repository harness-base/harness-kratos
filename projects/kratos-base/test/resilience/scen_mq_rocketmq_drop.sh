#!/usr/bin/env bash
# AC-MR3: RocketMQ 运行期断连 → 有界失败(被 Kratos 1s 请求超时闸住，不被 SDK ~40s 挂起) → 恢复续消费
# 流程：sandbox-up + 起 demo(rocketmq) → readyz=200、发一条+消费日志确认基线 →
#   stop rmqbroker + rmqnamesrv → POST /v1/events 有界失败/5xx（handler 不被 SDK ~40s 拖住）、
#   /readyz=503、进程活 →
#   start rmqbroker + rmqnamesrv → 监督循环重连 → 轮询 readyz=200 →
#   再发一条唯一事件 id → 轮询消费日志出现它（consumer re-subscribed，id 锚定）。
# 全程同 pid（不重启 demo）。
# Usage: bash test/resilience/scen_mq_rocketmq_drop.sh
# CWD-independent: cd to project root from this script's location.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.." || { echo "cannot cd to project root"; exit 1; }

DEMO_PID=""
COMPOSE_CMD="docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml"
FAST_FAIL_TIME="9.999"

cleanup() {
    echo ""
    echo "=== [AC-MR3] cleanup ==="
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

echo "=== AC-MR3: scen_mq_rocketmq_drop (runtime drop → fast-fail → reconnect) ==="

# ── Step 1: sandbox-up + start demo ──────────────────────────────────────────
echo ""
echo "=== Step 1: sandbox-up (all services including rocketmq) ==="
make sandbox-down 2>/dev/null || true
make sandbox-up
echo ">> all services healthy"

echo ""
echo "=== Step 2: build and start demo (rocketmq mode) ==="
go build -o bin/demo ./app/demo/cmd
./bin/demo -conf configs/bootstrap.rocketmq-sandbox.yaml >/tmp/demo_rmq_drop.log 2>&1 &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID"

# ── Step 3: poll /readyz until 200 ───────────────────────────────────────────
echo ""
echo "=== Step 3: poll /readyz until 200 ==="
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
        echo "ERROR: demo died during startup"
        cat /tmp/demo_rmq_drop.log || true
        exit 1
    fi
    sleep 1
done
if [ "$READY" -ne 1 ]; then
    echo "FAIL AC-MR3: /readyz never became 200 in step 3"
    cat /tmp/demo_rmq_drop.log || true
    exit 1
fi

# ── Step 4: baseline — send one event, confirm consumer receives it ───────────
echo ""
echo "=== Step 4: baseline — POST /v1/events + confirm consumer receipt ==="
BASELINE_PAYLOAD="rmq-baseline-drop-$$"
EVENTS_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    -X POST http://localhost:8000/v1/events \
    -H 'Content-Type: application/json' \
    -d "{\"payload\":\"$BASELINE_PAYLOAD\"}" 2>/dev/null || echo "HTTP_STATUS:000")
EVENTS_CODE=$(echo "$EVENTS_RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
EVENTS_BODY=$(echo "$EVENTS_RESP" | grep -v "HTTP_STATUS:")
echo ">> POST /v1/events → $EVENTS_CODE  body: $EVENTS_BODY"

if [ "$EVENTS_CODE" != "200" ]; then
    echo "FAIL AC-MR3: baseline POST /v1/events expected 200, got $EVENTS_CODE"
    exit 1
fi

BASELINE_EVENT_ID=$(echo "$EVENTS_BODY" | grep -oE '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | grep -oE '"[^"]*"$' | tr -d '"' || echo "")
if [ -z "$BASELINE_EVENT_ID" ]; then
    echo "FAIL AC-MR3: could not extract baseline event id from: $EVENTS_BODY"
    exit 1
fi

# Poll for consumer receipt — anchored on consumer log line + event id.
BASELINE_CONSUMED=0
for i in $(seq 1 60); do
    if grep '"consumer":"received"' /tmp/demo_rmq_drop.log 2>/dev/null | grep -q "$BASELINE_EVENT_ID"; then
        BASELINE_CONSUMED=1
        BL_LINE=$(grep '"consumer":"received"' /tmp/demo_rmq_drop.log | grep "$BASELINE_EVENT_ID" | tail -1)
        echo ">> PASS baseline consumer logged receipt of $BASELINE_EVENT_ID on attempt $i"
        echo "   log line: $BL_LINE"
        break
    fi
    sleep 1
done

if [ "$BASELINE_CONSUMED" -ne 1 ]; then
    echo "FAIL AC-MR3: consumer did not log receipt of baseline event '$BASELINE_EVENT_ID' after 60s"
    tail -20 /tmp/demo_rmq_drop.log || true
    exit 1
fi
echo ">> PASS baseline: /readyz=200, POST /v1/events=200, consumer received (id-anchored)"

# ── Step 5: stop rocketmq (simulate runtime drop) ────────────────────────────
echo ""
echo "=== Step 5: stop rmqbroker + rmqnamesrv (simulate runtime drop) ==="
$COMPOSE_CMD stop rmqbroker
$COMPOSE_CMD stop rmqnamesrv
echo ">> rmqbroker and rmqnamesrv stopped"

# ── Step 6: warm up the breaker — send several requests to trip it ────────────
echo ""
echo "=== Step 6: send requests to trip the circuit breaker ==="
for i in $(seq 1 15); do
    CODE=$(curl -s -o /dev/null -w "%{http_code}" \
        -X POST http://localhost:8000/v1/events \
        -H 'Content-Type: application/json' \
        -d '{"payload":"breaker-warmup-rmq"}' 2>/dev/null || echo "000")
    echo "  request $i: → $CODE"
done

# ── Step 7: measure bounded-fail time（不是亚秒级 fast-fail，是有界失败）──────────
# 诚实说明：/v1/events 是 proto 生成的 PublishEvent handler，走 Kratos HTTP server
# 用 r 的请求 ctx，受 Kratos 默认 1s per-request 超时管辖（对比 /readyz 故意用独立
# 15s ctx 绕开它，见 http.go）。所以本探针的【真实闸门】是这两者里的较小者：
#   - Kratos 1s 请求超时（先到，binding gate）
#   - publisher 用 goroutine+select 把 SDK Send 限在 request_timeout(10s) 内的界
# rocketmq v5 SDK 的 Send 不限时会自己 SetRequestTimeout×内部重试拖到 ~40s；这里
# 两道闸的较小者（1s）先把 handler 放掉，绝不到 SDK 的 ~40s。HARD 断言它被限在
# 真实闸门内（盖不住"goroutine+select 界失效"或"SDK ~40s 挂起"回归）。
echo ""
echo "=== Step 7: measure bounded-fail time (gated by Kratos 1s request timeout, NOT the ~40s SDK hang) ==="
FAST_FAIL_TIME=$(curl -s -o /dev/null -w "%{time_total}" \
    -X POST http://localhost:8000/v1/events \
    -H 'Content-Type: application/json' \
    -d '{"payload":"fast-fail-probe-rmq"}' 2>/dev/null || echo "99.999")
FAIL_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8000/v1/events \
    -H 'Content-Type: application/json' \
    -d '{"payload":"fast-fail-probe2-rmq"}' 2>/dev/null || echo "000")
echo ">> POST /v1/events time: ${FAST_FAIL_TIME}s  status: $FAIL_CODE"

# Must be 5xx (publish returns error, not hang)
if [[ "$FAIL_CODE" != 5* ]]; then
    echo "FAIL AC-MR3: POST /v1/events expected 5xx after rocketmq drop, got $FAIL_CODE"
    exit 1
fi
echo ">> PASS POST /v1/events = $FAIL_CODE after rocketmq drop"

# HARD assert: handler bounded by the REAL gate = min(Kratos 1s request timeout,
# publisher 10s send bound) = ~1s, + 余量 → 3s. 超界=两道闸都失效（goroutine+select
# 界回归 或 SDK ~40s 挂起回归），直接 FAIL，绝不 WARN 放行。
if awk "BEGIN {exit ($FAST_FAIL_TIME < 3.0) ? 0 : 1}"; then
    echo ">> PASS bounded-fail: ${FAST_FAIL_TIME}s < 3s — handler released by Kratos 1s request timeout, not held by SDK ~40s retry hang"
else
    echo "FAIL AC-MR3: POST took ${FAST_FAIL_TIME}s > 3s — handler NOT bounded by Kratos 1s request timeout nor publisher 10s send bound (goroutine+select界回归 or SDK ~40s 挂起回归)"
    exit 1
fi

# ── Step 8: assert /readyz = 503, process alive ───────────────────────────────
echo ""
echo "=== Step 8: assert /readyz = 503 and process alive ==="
# Poll /readyz until 503 (health probe uses 15s ctx, so give up to 30s)
READYZ_DOWN=0
for i in $(seq 1 30); do
    READYZ_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "000")
    echo "  attempt $i: /readyz → $READYZ_CODE"
    if [ "$READYZ_CODE" = "503" ]; then
        READYZ_DOWN=1
        echo ">> /readyz = 503 (mq down detected on attempt $i)"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-MR3: demo process died"
        exit 1
    fi
    sleep 1
done

if [ "$READYZ_DOWN" -ne 1 ]; then
    echo "FAIL AC-MR3: /readyz did not become 503 after rocketmq drop (30s)"
    exit 1
fi
echo ">> PASS /readyz = 503 after rocketmq drop"

if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-MR3: demo process $DEMO_PID died"
    exit 1
fi
echo ">> PASS demo process $DEMO_PID is alive"

# ── Step 9: restore rocketmq ─────────────────────────────────────────────────
echo ""
echo "=== Step 9: start rmqnamesrv + rmqbroker (restore) ==="
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
        echo "FAIL AC-MR3: rmqnamesrv container did not become healthy after 60s"
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
        echo "FAIL AC-MR3: rmqbroker container did not become healthy after 120s"
        exit 1
    fi
    sleep 1
done

# Ensure topic exists (the consumer supervisor loop will re-subscribe)
echo ">> ensuring topic demo-events + consumer group demo-consumer exist..."
docker exec kratosbase-sandbox-rmqbroker-1 sh mqadmin updateTopic -n rmqnamesrv:9876 -t demo-events -c DefaultCluster 2>&1 | grep -v "^$" || true
docker exec kratosbase-sandbox-rmqbroker-1 sh mqadmin updateSubGroup -n rmqnamesrv:9876 -g demo-consumer -c DefaultCluster 2>&1 | grep -v "^$" || true
echo ">> topic and group ensured"

# ── Step 10: poll /readyz until 200 (auto-recovery, no demo restart) ─────────
echo ""
echo "=== Step 10: poll /readyz until 200 (auto-recovery, no demo restart) ==="
RECOVERED=0
for i in $(seq 1 120); do
    RC=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "000")
    echo "  attempt $i: /readyz → $RC"
    if [ "$RC" = "200" ]; then
        RECOVERED=1
        echo ">> PASS /readyz recovered to 200 on attempt $i"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-MR3: demo process died during recovery"
        exit 1
    fi
    sleep 1
done

if [ "$RECOVERED" -ne 1 ]; then
    echo "FAIL AC-MR3: /readyz did not recover to 200 after 120 attempts"
    exit 1
fi

# ── Step 11: send a second unique event and verify consumer receives it ────────
echo ""
echo "=== Step 11: send unique event after recovery, poll consumer receipt ==="
RECOVER_PAYLOAD="rmq-post-drop-recover-$$"
echo ">> payload: $RECOVER_PAYLOAD"

EVENTS_RESP2=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    -X POST http://localhost:8000/v1/events \
    -H 'Content-Type: application/json' \
    -d "{\"payload\":\"$RECOVER_PAYLOAD\"}" 2>/dev/null || echo "HTTP_STATUS:000")
EVENTS_CODE2=$(echo "$EVENTS_RESP2" | grep "HTTP_STATUS:" | cut -d: -f2)
EVENTS_BODY2=$(echo "$EVENTS_RESP2" | grep -v "HTTP_STATUS:")
echo ">> POST /v1/events → $EVENTS_CODE2  body: $EVENTS_BODY2"

if [ "$EVENTS_CODE2" != "200" ]; then
    echo "FAIL AC-MR3: POST /v1/events after recovery expected 200, got $EVENTS_CODE2"
    exit 1
fi
echo ">> PASS POST /v1/events = 200 after recovery"

RECOVER_EVENT_ID=$(echo "$EVENTS_BODY2" | grep -oE '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | grep -oE '"[^"]*"$' | tr -d '"' || echo "")
if [ -z "$RECOVER_EVENT_ID" ]; then
    echo "FAIL AC-MR3: could not extract recovery event id from: $EVENTS_BODY2"
    exit 1
fi

# Poll for consumer receipt — anchored on consumer log line + event id.
# Proves the consumer supervisor re-subscribed after the drop and consumed it.
RECOVER_CONSUMED=0
for i in $(seq 1 60); do
    if grep '"consumer":"received"' /tmp/demo_rmq_drop.log 2>/dev/null | grep -q "$RECOVER_EVENT_ID"; then
        RECOVER_CONSUMED=1
        RC_LINE=$(grep '"consumer":"received"' /tmp/demo_rmq_drop.log | grep "$RECOVER_EVENT_ID" | tail -1)
        echo ">> PASS consumer logged receipt of recovery event $RECOVER_EVENT_ID on attempt $i"
        echo "   log line: $RC_LINE"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-MR3: demo process died while waiting for recovery consumer"
        exit 1
    fi
    sleep 1
done

if [ "$RECOVER_CONSUMED" -ne 1 ]; then
    echo "FAIL AC-MR3: consumer did not log receipt of recovery event '$RECOVER_EVENT_ID' (60s)"
    echo "=== demo log tail ==="
    tail -30 /tmp/demo_rmq_drop.log || true
    exit 1
fi

# ── Step 12: confirm demo was never restarted ─────────────────────────────────
echo ""
echo "=== Step 12: confirm process identity (no restart) ==="
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-MR3: demo process $DEMO_PID is gone"
    exit 1
fi
echo ">> PASS demo pid $DEMO_PID alive (no restart)"

echo ""
echo "=== scen_mq_rocketmq_drop.sh PASSED (AC-MR3) ==="
echo "  baseline: /readyz=200, POST /v1/events=200, consumer received (id-anchored)"
echo "  rocketmq drop → bounded-fail (${FAST_FAIL_TIME}s, gated by Kratos 1s request timeout), /readyz=503, process alive"
echo "  rocketmq restore → /readyz=200 (auto-recovery)"
echo "  post-recovery: POST /v1/events=200, consumer re-subscribed and received"
echo "  recovery log line: '${RC_LINE}'"
echo "  demo process never restarted (same pid: $DEMO_PID)"
