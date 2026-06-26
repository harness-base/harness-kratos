#!/usr/bin/env bash
# AC3: 运行中断连 → 快速失败（熔断）→ 恢复续上。全程不重启 demo。
# Usage: bash test/resilience/scen_runtime_drop.sh
# Must be run from the project root (projects/kratos-base/).
set -euo pipefail

DEMO_PID=""
COMPOSE_CMD="docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml"

cleanup() {
    echo ""
    echo "=== [AC3] cleanup ==="
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
    echo ">> ensure postgres is started before sandbox-down (restore in case it was stopped)"
    $COMPOSE_CMD start postgres 2>/dev/null || true
    echo ">> make sandbox-down"
    make sandbox-down 2>/dev/null || true
}
trap cleanup EXIT

echo "=== AC3: scen_runtime_drop ==="

# ── Step 1: sandbox-up + start demo ──────────────────────────────────────────
echo ""
echo "=== Step 1: sandbox-up ==="
make sandbox-down 2>/dev/null || true
make sandbox-up
echo ">> PG is healthy"

echo ""
echo "=== Step 2: build and start demo ==="
go build -o bin/demo ./app/demo/cmd
./bin/demo -conf configs/bootstrap.sandbox.yaml >/tmp/demo_runtime_drop.log 2>&1 &
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
        cat /tmp/demo_runtime_drop.log || true
        exit 1
    fi
    sleep 1
done
if [ "$READY" -ne 1 ]; then
    echo "FAIL AC3: /readyz never became 200 in step 3"
    exit 1
fi

# Assert /v1/greet/1 = 200
GREET_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/v1/greet/1)
echo ">> /v1/greet/1 → $GREET_CODE (must be 200 before drop)"
if [ "$GREET_CODE" != "200" ]; then
    echo "FAIL AC3: /v1/greet/1 should be 200 before PG drop, got $GREET_CODE"
    exit 1
fi
echo ">> PASS baseline: /readyz=200, /v1/greet/1=200"

# ── Step 4: kill PG (stop postgres container) ────────────────────────────────
echo ""
echo "=== Step 4: stop postgres (simulate runtime drop) ==="
$COMPOSE_CMD stop postgres
echo ">> postgres stopped"

# ── Step 5: send several requests to trip the breaker ────────────────────────
echo ""
echo "=== Step 5: trigger circuit breaker (send requests to trip it) ==="
for i in $(seq 1 8); do
    CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/v1/greet/1 2>/dev/null || echo "000")
    echo "  request $i: /v1/greet/1 → $CODE"
done

# Brief pause to let breaker stabilize
sleep 1

# ── Step 6: assert fast-fail after breaker opens ─────────────────────────────
echo ""
echo "=== Step 6: assert fast-fail (open circuit) — request time < 0.2s ==="
TOTAL_TIME=$(curl -s -o /dev/null -w "%{time_total}" http://localhost:8000/v1/greet/1 2>/dev/null || echo "9.999")
GREET_CODE_OPEN=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/v1/greet/1 2>/dev/null || echo "000")
echo ">> /v1/greet/1 time: ${TOTAL_TIME}s  status: $GREET_CODE_OPEN"

# Check that request is not 200 (fast-fail returns error)
if [ "$GREET_CODE_OPEN" = "200" ]; then
    echo "FAIL AC3: /v1/greet/1 unexpectedly returned 200 after PG drop"
    exit 1
fi

# Check fast-fail: breaker open => ~0.1s; connection-refused (PG port closed) => < 0.5s.
# HARD bound 1.0s: comfortably above the ECONNREFUSED/breaker case yet well below the
# 2s connect_timeout. Exceeding it means the request blocked on the full connect
# timeout — the fast-fail regression. rule-0009 §C forbids a WARN/continue passthrough
# for a claimed guarantee, so FAIL hard here (aligned with scen_redis_drop.sh /
# scen_mq_rocketmq_drop.sh).
if awk "BEGIN {exit ($TOTAL_TIME < 1.0) ? 0 : 1}"; then
    echo ">> PASS fast-fail: ${TOTAL_TIME}s < 1.0s (breaker open or connection refused, under connect_timeout 2s)"
else
    echo "FAIL AC3: /v1/greet/1 took ${TOTAL_TIME}s ≥ 1.0s — NOT fast-failing (blocked on ~2s connect_timeout regression)"
    exit 1
fi

# ── Step 7: assert /readyz = 503, process alive ───────────────────────────────
echo ""
echo "=== Step 7: assert /readyz = 503 and process alive ==="
READYZ_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz)
echo ">> /readyz → $READYZ_CODE"
if [ "$READYZ_CODE" != "503" ]; then
    echo "FAIL AC3: /readyz expected 503 after PG drop, got $READYZ_CODE"
    exit 1
fi
echo ">> PASS /readyz = 503 after PG drop"

if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC3: demo process $DEMO_PID died"
    exit 1
fi
echo ">> PASS demo process $DEMO_PID is alive"

# ── Step 8: restore PG ───────────────────────────────────────────────────────
echo ""
echo "=== Step 8: start postgres (restore PG) ==="
$COMPOSE_CMD start postgres
echo ">> postgres started"

# Wait for postgres to be healthy
for i in $(seq 1 30); do
    HC=$($COMPOSE_CMD ps --format json 2>/dev/null | grep -o '"Health":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "unknown")
    # Alternative: use docker inspect directly
    HC2=$(docker inspect --format='{{.State.Health.Status}}' kratosbase-sandbox-postgres-1 2>/dev/null || echo "unknown")
    echo "  pg health attempt $i: $HC2"
    if [ "$HC2" = "healthy" ]; then
        echo ">> postgres healthy"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "FAIL AC3: postgres did not become healthy after 30s"
        exit 1
    fi
    sleep 1
done

# ── Step 9: poll /readyz until 200 (auto-recovery) ───────────────────────────
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
        echo "FAIL AC3: demo process died during recovery"
        exit 1
    fi
    sleep 1
done

if [ "$RECOVERED" -ne 1 ]; then
    echo "FAIL AC3: /readyz did not recover to 200 after 45 attempts"
    exit 1
fi

# ── Step 10: assert /v1/greet/1 = 200 again ──────────────────────────────────
echo ""
echo "=== Step 10: assert /v1/greet/1 = 200 after recovery ==="
# Poll a few times since breaker may still be half-open
GREET_OK=0
for i in $(seq 1 15); do
    GREET_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/v1/greet/1 2>/dev/null || echo "000")
    echo "  attempt $i: /v1/greet/1 → $GREET_CODE"
    if [ "$GREET_CODE" = "200" ]; then
        GREET_OK=1
        echo ">> PASS /v1/greet/1 = 200 after recovery (attempt $i)"
        break
    fi
    sleep 1
done

if [ "$GREET_OK" -ne 1 ]; then
    echo "FAIL AC3: /v1/greet/1 did not return 200 after recovery"
    exit 1
fi

# ── Step 11: confirm demo was never restarted ─────────────────────────────────
echo ""
echo "=== Step 11: confirm process identity (no restart) ==="
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC3: demo process $DEMO_PID is gone"
    exit 1
fi
echo ">> PASS demo pid $DEMO_PID alive (no restart)"

echo ""
echo "=== scen_runtime_drop.sh PASSED (AC3) ==="
echo "  PG drop → fast-fail (${TOTAL_TIME}s), /readyz=503, process alive"
echo "  PG restore → /readyz=200, /v1/greet/1=200"
echo "  demo process never restarted (same pid: $DEMO_PID)"
