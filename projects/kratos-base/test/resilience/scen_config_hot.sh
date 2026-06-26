#!/usr/bin/env bash
# AC4: 配置热更落地 + 坏配置回滚（本地文件 config source，mode=local）。
#
# 验收断言（rule-0009 锚定产出方证据 / 避免共因 §A）：
#   - 热更"落地"正向硬证：改一个**好**配置（仅 log.level info→debug，DSN 不动、PG 不停）
#     → confcenter 产出方日志 `confcenter: config applied` 计数 BEFORE→AFTER +1。
#     该行只在 BindKratosWatch 的 observer 走完 reload+Publish 成功路径才打出
#     （main.go BindKratosWatch / confcenter manager.go:147），无法被任何请求访问
#     日志或入参回显伪造。/readyz 同时仍 200（好配置不该让服务掉线）。
#   - 不再用"改坏 DSN + 同时停 PG → /readyz=503"做热更归因——那是共因污染（§A）：
#     503 由停 PG 本身即可触发；且 resource.Provider 的 self-heal 会回退旧好句柄、
#     吞掉单纯的坏 DSN，根本区分不了热更是否真生效。
#   - 坏配置回滚：空 grpc.addr → `retaining previous config` 计数 BEFORE→AFTER +1
#     （计数对比，防旧行假命中）、进程活、/readyz 仍 200。
# Usage: bash test/resilience/scen_config_hot.sh
# Must be run from the project root (projects/kratos-base/).
set -euo pipefail

# count_log <substr> — number of $DEMO_LOG lines containing the literal substr,
# as a single integer, for BEFORE/AFTER comparison (a pre-existing line can never
# false-pass). grep -c prints 0 and exits 1 on no-match; `|| true` swallows that.
count_log() { local n; n=$(grep -cF "$1" /tmp/demo_config_hot.log 2>/dev/null || true); echo "${n:-0}"; }

DEMO_PID=""
COMPOSE_CMD="docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml"
# Work on a mutable copy of the runtime config so we don't corrupt the original.
ORIG_RUNTIME="configs/runtime.sandbox.yaml"
WORK_RUNTIME="/tmp/runtime_hot_test_$$.yaml"
WORK_BOOTSTRAP="/tmp/bootstrap_hot_test_$$.yaml"

cleanup() {
    echo ""
    echo "=== [AC4] cleanup ==="
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
    # Idempotent safety net (this scenario no longer stops postgres).
    $COMPOSE_CMD start postgres 2>/dev/null || true
    echo ">> make sandbox-down"
    make sandbox-down 2>/dev/null || true
    rm -f "$WORK_RUNTIME" "$WORK_BOOTSTRAP"
}
trap cleanup EXIT

echo "=== AC4: scen_config_hot ==="

# ── Step 1: sandbox-up ────────────────────────────────────────────────────────
echo ""
echo "=== Step 1: sandbox-up ==="
make sandbox-down 2>/dev/null || true
make sandbox-up
echo ">> PG healthy"

# ── Step 2: build + create mutable config copy ───────────────────────────────
echo ""
echo "=== Step 2: build demo and prepare mutable config ==="
go build -o bin/demo ./app/demo/cmd
echo ">> built bin/demo"

# Copy runtime config to a writable temp path
cp "$ORIG_RUNTIME" "$WORK_RUNTIME"

# Create bootstrap that points to the mutable runtime
cat > "$WORK_BOOTSTRAP" <<EOF
infra:
  mode: local
  path: $WORK_RUNTIME
EOF
echo ">> work runtime: $WORK_RUNTIME"
echo ">> work bootstrap: $WORK_BOOTSTRAP"

# ── Step 3: start demo with mutable config ────────────────────────────────────
echo ""
echo "=== Step 3: start demo ==="
./bin/demo -conf "$WORK_BOOTSTRAP" >/tmp/demo_config_hot.log 2>&1 &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID"

# Poll /readyz until 200
echo ""
echo "=== Step 4: poll /readyz until 200 ==="
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
        cat /tmp/demo_config_hot.log || true
        exit 1
    fi
    sleep 1
done
if [ "$READY" -ne 1 ]; then
    echo "FAIL AC4: /readyz never became 200"
    exit 1
fi
echo ">> PASS baseline: /readyz=200"

# ── Step 5: 热更落地正向硬证 — 改一个好配置，断言产出方 'config applied' +1 ──────
# rule-0009 §A：不再用"改坏 DSN + 停 PG → 503"做归因（共因污染：503 由停 PG 即触发，
# 且 provider self-heal 会吞坏 DSN）。改为推一个**好**配置变更（仅 log.level info→debug，
# DSN 不动、PG 不停），用 confcenter 产出方日志 `confcenter: config applied` 的
# BEFORE/AFTER 计数 +1 证明热更真落地——该行只在 observer reload+Publish 成功路径打出。
echo ""
echo "=== Step 5: hot-reload LANDING — flip log.level (good change), assert 'config applied' +1 ==="

APPLIED_BEFORE=$(count_log "confcenter: config applied")
echo ">> 'config applied' count BEFORE: $APPLIED_BEFORE"

# Flip log.level info→debug — a valid change (Validate passes, DSN/PG untouched).
sed -i.bak 's|level: info|level: debug|' "$WORK_RUNTIME"
echo ">> runtime log.level changed info→debug (valid hot-reload, DSN unchanged, PG up)"
grep -n "level:" "$WORK_RUNTIME" || true

# Poll until the producer-side 'config applied' count strictly increases.
APPLIED_OK=0
for i in $(seq 1 30); do
    APPLIED_NOW=$(count_log "confcenter: config applied")
    echo "  attempt $i: 'config applied' count → $APPLIED_NOW"
    if [ "$APPLIED_NOW" -gt "$APPLIED_BEFORE" ]; then
        APPLIED_OK=1
        echo ">> PASS hot-reload landed: 'config applied' $APPLIED_BEFORE → $APPLIED_NOW (+$((APPLIED_NOW - APPLIED_BEFORE)))"
        grep -F "confcenter: config applied" /tmp/demo_config_hot.log | tail -1 || true
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC4: demo died during hot-reload landing wait"
        exit 1
    fi
    sleep 1
done
if [ "$APPLIED_OK" -ne 1 ]; then
    echo ">> Log tail (last 30 lines):"
    tail -30 /tmp/demo_config_hot.log || true
    echo "FAIL AC4: 'config applied' count did not increase after good config change"
    exit 1
fi

# Sanity: a valid hot-reload (DSN intact, PG up) must keep the app healthy.
echo ""
echo "=== Step 6: assert /readyz=200 after valid hot-reload (app stays healthy) ==="
READYZ_AFTER_FLIP=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz)
echo ">> /readyz after good flip: $READYZ_AFTER_FLIP"
if [ "$READYZ_AFTER_FLIP" != "200" ]; then
    echo "FAIL AC4: /readyz should stay 200 after good config change, got $READYZ_AFTER_FLIP"
    exit 1
fi
echo ">> PASS /readyz=200 (valid hot-reload, app stays healthy)"

# ── Step 7: 坏配置回滚 — 清空 grpc.addr → 'retaining previous config' +1 ──────────
echo ""
echo "=== Step 7: bad config rollback — empty grpc.addr, assert 'retaining previous config' +1 ==="
RETAIN_BEFORE=$(count_log "retaining previous config")
echo ">> 'retaining previous config' count BEFORE: $RETAIN_BEFORE"

# Replace grpc addr with empty string to trigger Validate failure
sed -i.bak 's|addr: ":9000"|addr: ""|g' "$WORK_RUNTIME"
echo ">> grpc.addr set to empty — should fail Validate"
grep -A3 "grpc:" "$WORK_RUNTIME" || true

# Poll until the producer-side 'retaining previous config' count strictly increases
# (BEFORE/AFTER, not bare grep — a pre-existing line cannot false-pass).
RETAIN_OK=0
for i in $(seq 1 30); do
    RETAIN_NOW=$(count_log "retaining previous config")
    echo "  attempt $i: 'retaining previous config' count → $RETAIN_NOW"
    if [ "$RETAIN_NOW" -gt "$RETAIN_BEFORE" ]; then
        RETAIN_OK=1
        echo ">> PASS bad config rejected: 'retaining previous config' $RETAIN_BEFORE → $RETAIN_NOW (+$((RETAIN_NOW - RETAIN_BEFORE)))"
        grep -F "retaining previous config" /tmp/demo_config_hot.log | tail -1 || true
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC4: demo died while waiting for bad config rejection"
        exit 1
    fi
    sleep 1
done
if [ "$RETAIN_OK" -ne 1 ]; then
    echo ">> Log tail (last 20 lines):"
    tail -20 /tmp/demo_config_hot.log || true
    echo "FAIL AC4: 'retaining previous config' count did not increase after bad config"
    exit 1
fi

# ── Step 8: assert process alive and /readyz unaffected ─────────────────────
echo ""
echo "=== Step 8: assert process alive and /readyz still 200 ==="
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC4: demo process died after bad config push"
    exit 1
fi
echo ">> PASS demo process alive"

READYZ_AFTER_BAD=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz)
echo ">> /readyz after bad config: $READYZ_AFTER_BAD"
if [ "$READYZ_AFTER_BAD" != "200" ]; then
    echo "FAIL AC4: /readyz should stay 200 after bad config was rejected, got $READYZ_AFTER_BAD"
    exit 1
fi
echo ">> PASS /readyz=200 after bad config (retained previous)"

echo ""
echo "=== scen_config_hot.sh PASSED (AC4) ==="
echo "  热更落地: good change (log.level info→debug, PG up) → 'config applied' BEFORE/AFTER +1 (producer evidence), /readyz=200"
echo "  坏配置回滚: empty grpc.addr → 'retaining previous config' BEFORE/AFTER +1 → process alive, /readyz=200"
