#!/usr/bin/env bash
# AC-R3: redis 运行中断连 → 快速失败（连接拒绝/熔断）→ 恢复续上。全程不重启 demo。
# 流程：sandbox-up + 起 demo → 轮询 /readyz=200 + /v1/hits 工作
#   → stop redis → 连发几次 /v1/hits
#   → 断言请求快速失败（实测 ~1s，走 ECONNREFUSED；远小于 dial 超时 5s）、/readyz=503、进程活
#   → start redis → 轮询 /readyz→200 + /v1/hits 计数恢复递增
#   （全程未重启 demo）
# Usage: bash test/resilience/scen_redis_drop.sh
# CWD-independent: cd to project root from this script's location.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.." || { echo "cannot cd to project root"; exit 1; }

DEMO_PID=""
COMPOSE_CMD="docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml"
TOTAL_TIME="9.999"

cleanup() {
    echo ""
    echo "=== [AC-R3] cleanup ==="
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

echo "=== AC-R3: scen_redis_drop ==="

# ── Step 1: sandbox-up + start demo ──────────────────────────────────────────
echo ""
echo "=== Step 1: sandbox-up (pg + redis) ==="
make sandbox-down 2>/dev/null || true
make sandbox-up
echo ">> pg + redis are healthy"

echo ""
echo "=== Step 2: build and start demo ==="
go build -o bin/demo ./app/demo/cmd
./bin/demo -conf configs/bootstrap.sandbox.yaml >/tmp/demo_redis_drop.log 2>&1 &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID"

# Poll /readyz until 200
echo ""
echo "=== Step 3: poll /readyz until 200 ==="
READY=0
for i in $(seq 1 30); do
    RC=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "000")
    echo "  attempt $i: /readyz → $RC"
    if [ "$RC" = "200" ]; then
        READY=1
        echo ">> /readyz = 200 (attempt $i)"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "ERROR: demo died during startup"
        cat /tmp/demo_redis_drop.log || true
        exit 1
    fi
    sleep 1
done
if [ "$READY" -ne 1 ]; then
    echo "FAIL AC-R3: /readyz never became 200 in step 3"
    exit 1
fi

# Assert /v1/hits works (baseline)
HITS_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/v1/hits)
echo ">> /v1/hits → $HITS_CODE (must be 200 before drop)"
if [ "$HITS_CODE" != "200" ]; then
    echo "FAIL AC-R3: /v1/hits should be 200 before redis drop, got $HITS_CODE"
    exit 1
fi
echo ">> PASS baseline: /readyz=200, /v1/hits=200"

# ── Step 4: stop redis (simulate runtime drop) ───────────────────────────────
echo ""
echo "=== Step 4: stop redis (simulate runtime drop) ==="
$COMPOSE_CMD stop redis
echo ">> redis stopped"

# ── Step 5: send several requests to trip the breaker ────────────────────────
echo ""
echo "=== Step 5: send requests while redis is down ==="
for i in $(seq 1 20); do
    CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/v1/hits 2>/dev/null || echo "000")
    echo "  request $i: /v1/hits → $CODE"
done

# Brief pause to let breaker stabilize
sleep 1

# ── Step 6: assert fast-fail after breaker opens ─────────────────────────────
echo ""
echo "=== Step 6: assert fast-fail (refused/breaker) — request time << dial timeout ==="
TOTAL_TIME=$(curl -s -o /dev/null -w "%{time_total}" http://localhost:8000/v1/hits 2>/dev/null || echo "9.999")
HITS_CODE_OPEN=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/v1/hits 2>/dev/null || echo "000")
echo ">> /v1/hits time: ${TOTAL_TIME}s  status: $HITS_CODE_OPEN"

# Must be 5xx (fast-fail returns error)
if [[ "$HITS_CODE_OPEN" != 5* ]]; then
    echo "FAIL AC-R3: /v1/hits expected 5xx after redis drop, got $HITS_CODE_OPEN"
    exit 1
fi

# HARD fast-fail bound: request must return well under the 5s dial timeout.
# If circuit is open (breaker fired), expect < 0.2s; connection-refused is also < 0.5s.
# Threshold 2.0s is conservative — still far below 5s dial. Exceeding it means the
# request blocked on the full dial timeout (fast-fail regression), so FAIL hard
# (no WARN/continue passthrough — rule-0009 §C requires a guarding assertion).
if awk "BEGIN {exit ($TOTAL_TIME < 2.0) ? 0 : 1}"; then
    if awk "BEGIN {exit ($TOTAL_TIME < 0.2) ? 0 : 1}"; then
        echo ">> PASS fast-fail: ${TOTAL_TIME}s < 0.2s (circuit breaker open)"
    else
        echo ">> PASS fast-fail: ${TOTAL_TIME}s < 2.0s (connection refused or circuit open)"
    fi
else
    echo "FAIL AC-R3: /v1/hits took ${TOTAL_TIME}s ≥ 2.0s — NOT fast-failing (blocked on ~5s dial timeout regression)"
    exit 1
fi

# ── Step 7: assert /readyz = 503, process alive ───────────────────────────────
echo ""
echo "=== Step 7: assert /readyz = 503 and process alive ==="
READYZ_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz)
echo ">> /readyz → $READYZ_CODE"
if [ "$READYZ_CODE" != "503" ]; then
    echo "FAIL AC-R3: /readyz expected 503 after redis drop, got $READYZ_CODE"
    exit 1
fi
echo ">> PASS /readyz = 503 after redis drop"

if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-R3: demo process $DEMO_PID died"
    exit 1
fi
echo ">> PASS demo process $DEMO_PID is alive"

# ── Step 8: restore redis ────────────────────────────────────────────────────
echo ""
echo "=== Step 8: start redis (restore) ==="
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
        echo "FAIL AC-R3: redis container did not become healthy after 30s"
        exit 1
    fi
    sleep 1
done

# ── Step 9: poll /readyz until 200 (auto-recovery, no demo restart) ──────────
echo ""
echo "=== Step 9: poll /readyz until 200 (auto-recovery, no demo restart) ==="
RECOVERED=0
for i in $(seq 1 45); do
    RC=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "000")
    echo "  attempt $i: /readyz → $RC"
    if [ "$RC" = "200" ]; then
        RECOVERED=1
        echo ">> PASS /readyz recovered to 200 on attempt $i"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-R3: demo process died during recovery"
        exit 1
    fi
    sleep 1
done

if [ "$RECOVERED" -ne 1 ]; then
    echo "FAIL AC-R3: /readyz did not recover to 200 after 45 attempts"
    exit 1
fi

# ── Step 10: assert /v1/hits count recovers and increments ───────────────────
echo ""
echo "=== Step 10: assert /v1/hits count recovers and increments after recovery ==="
HITS_OK=0
COUNT_A=0
COUNT_B=0
for i in $(seq 1 15); do
    HITS_RESP_A=$(curl -s -w "\nHTTP_STATUS:%{http_code}" http://localhost:8000/v1/hits 2>/dev/null || echo "HTTP_STATUS:000")
    HITS_CODE_A=$(echo "$HITS_RESP_A" | grep "HTTP_STATUS:" | cut -d: -f2)
    HITS_BODY_A=$(echo "$HITS_RESP_A" | grep -v "HTTP_STATUS:")
    echo "  attempt $i: /v1/hits HTTP $HITS_CODE_A  body: $HITS_BODY_A"
    if [ "$HITS_CODE_A" = "200" ]; then
        COUNT_A=$(echo "$HITS_BODY_A" | grep -oE '"count"[[:space:]]*:[[:space:]]*"?[0-9]+"?' | grep -oE '[0-9]+' | tail -1 || echo "0")
        # Make a second call to verify increment
        HITS_RESP_B=$(curl -s -w "\nHTTP_STATUS:%{http_code}" http://localhost:8000/v1/hits 2>/dev/null || echo "HTTP_STATUS:000")
        HITS_CODE_B=$(echo "$HITS_RESP_B" | grep "HTTP_STATUS:" | cut -d: -f2)
        HITS_BODY_B=$(echo "$HITS_RESP_B" | grep -v "HTTP_STATUS:")
        echo "  second call: /v1/hits HTTP $HITS_CODE_B  body: $HITS_BODY_B"
        if [ "$HITS_CODE_B" = "200" ]; then
            COUNT_B=$(echo "$HITS_BODY_B" | grep -oE '"count"[[:space:]]*:[[:space:]]*"?[0-9]+"?' | grep -oE '[0-9]+' | tail -1 || echo "0")
            if awk "BEGIN { exit ($COUNT_B > $COUNT_A) ? 0 : 1 }"; then
                HITS_OK=1
                echo ">> PASS /v1/hits count increments: $COUNT_A → $COUNT_B (attempt $i)"
                break
            fi
        fi
    fi
    sleep 1
done

if [ "$HITS_OK" -ne 1 ]; then
    echo "FAIL AC-R3: /v1/hits did not recover with incrementing count after redis restore"
    exit 1
fi

# ── Step 11: confirm demo was never restarted ─────────────────────────────────
echo ""
echo "=== Step 11: confirm process identity (no restart) ==="
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-R3: demo process $DEMO_PID is gone"
    exit 1
fi
echo ">> PASS demo pid $DEMO_PID alive (no restart)"

echo ""
echo "=== scen_redis_drop.sh PASSED (AC-R3) ==="
echo "  redis drop → fast-fail (${TOTAL_TIME}s), /readyz=503, process alive"
echo "  redis restore → /readyz=200, /v1/hits count increments: $COUNT_A → $COUNT_B"
echo "  demo process never restarted (same pid: $DEMO_PID)"
