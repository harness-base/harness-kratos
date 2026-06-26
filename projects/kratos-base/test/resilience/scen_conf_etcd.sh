#!/usr/bin/env bash
# AC-C2: 配置来自 etcd + 热更落地 + 坏配置回滚 闭环。
#
# 验收断言（rule-0009 锚定产出方证据 / 避免共因 §A）：
#   - 配置真从 etcd 加载：起 demo → /readyz=200。
#   - 热更"落地"正向硬证：put 一个**好**配置变更 → confcenter 产出方日志
#     `confcenter: config applied` 的计数 BEFORE→AFTER +1（只能由 Publish 成功路径打出）。
#     不再用"坏 DSN + 停 pg → 503"做归因——那是共因污染（503 纯由停 pg 触发，
#     且 provider self-heal 会吞掉单纯坏 DSN），证明不了热更是否真生效。
#   - 坏配置回滚：put 空 grpc.addr → `retaining previous config` 计数 BEFORE→AFTER +1
#     （而非裸 grep，防旧行假命中）、进程活、/readyz 仍 200。
#
# Usage: bash test/resilience/scen_conf_etcd.sh
# Must be run from the project root (projects/kratos-base/).
#
# Key convention:
#   etcd contrib config/etcd uses filepath.Ext(key) to detect format.
#   Key="/configs/demo/runtime.yaml" → ext=".yaml" → parsed as YAML.
set -euo pipefail

# count_log <substr> — number of lines in $DEMO_LOG containing the literal substr,
# emitted as a single integer. Used for BEFORE/AFTER comparison so a pre-existing
# line can never false-pass. (grep -c already prints 0 on no-match and exits 1;
# `|| true` swallows that exit without appending a second line.)
count_log() { local n; n=$(grep -cF "$1" "$DEMO_LOG" 2>/dev/null || true); echo "${n:-0}"; }

# CWD-independent: always run from project root
cd "$(dirname "$0")/../.."

DEMO_PID=""
COMPOSE_CMD="docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml"
ETCD_CONTAINER="kratosbase-sandbox-etcd-1"
ETCD_KEY="/configs/demo/runtime.yaml"
DEMO_LOG="/tmp/demo_conf_etcd_$$.log"

cleanup() {
    echo ""
    echo "=== [AC-C2] cleanup ==="
    if [ -n "$DEMO_PID" ] && kill -0 "$DEMO_PID" 2>/dev/null; then
        echo ">> stopping demo (SIGTERM)"
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
    echo ">> sandbox-down"
    make sandbox-down 2>/dev/null || true
    rm -f "$DEMO_LOG"
}
trap cleanup EXIT

echo "=== AC-C2: scen_conf_etcd (etcd config hot-reload) ==="

# ── Step 1: sandbox-up ────────────────────────────────────────────────────────
echo ""
echo "=== Step 1: sandbox-up (pg + redis + etcd) ==="
make sandbox-down 2>/dev/null || true
make sandbox-up
echo ">> sandbox healthy"

# ── Step 2: seed runtime config into etcd ────────────────────────────────────
echo ""
echo "=== Step 2: seed runtime config into etcd ==="

RUNTIME_YAML=$(cat <<'YAML'
server:
  grpc:
    addr: ":9000"
  http:
    addr: ":8000"
log:
  level: info
data:
  database:
    dsn: "postgres://demo:demo@localhost:5432/demo?sslmode=disable"
    max_open: 10
    max_idle: 5
    conn_max_lifetime: 300s
    conn_max_idle_time: 60s
    connect_timeout: 2s
  redis:
    addrs:
      - "localhost:6379"
trace:
  endpoint: ""
  sample_ratio: 0.0
YAML
)

docker exec "$ETCD_CONTAINER" etcdctl --endpoints=http://127.0.0.1:2379 put "$ETCD_KEY" "$RUNTIME_YAML"
echo ">> seeded etcd key: $ETCD_KEY"

# ── Step 3: build + start demo with etcd bootstrap ───────────────────────────
echo ""
echo "=== Step 3: build + start demo (etcd config source) ==="
go build -o bin/demo ./app/demo/cmd
echo ">> built bin/demo"

./bin/demo -conf configs/bootstrap.etcd-sandbox.yaml >"$DEMO_LOG" 2>&1 &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID"

# Poll /readyz until 200 — proves config was loaded from etcd
echo ""
echo "=== Step 4: poll /readyz until 200 (config from etcd) ==="
READY=0
for i in $(seq 1 40); do
    RC=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "000")
    echo "  attempt $i: /readyz → $RC"
    if [ "$RC" = "200" ]; then
        READY=1
        echo ">> PASS /readyz=200 (etcd config loaded)"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "ERROR: demo died during startup"
        cat "$DEMO_LOG" || true
        exit 1
    fi
    sleep 1
done
if [ "$READY" -ne 1 ]; then
    echo "FAIL AC-C2: /readyz never became 200"
    cat "$DEMO_LOG" || true
    exit 1
fi

# ── Step 5: 热更落地正向硬证 — put a GOOD config change, assert producer evidence ─
# rule-0009 §A：不再用"坏 DSN + 停 pg → 503"做归因（共因污染）。改为推一个**好**配置
# 变更（仅 log.level info→debug，DSN 不动、pg 不停），用 confcenter 产出方日志
# `confcenter: config applied` 的 BEFORE/AFTER 计数 +1 证明热更真落地。该行只在
# Manager.Publish 成功路径打出，无法被任何请求访问日志/入参回显伪造。
echo ""
echo "=== Step 5: hot-reload LANDING — put good config change, assert 'config applied' +1 ==="

APPLIED_BEFORE=$(count_log "confcenter: config applied")
echo ">> 'config applied' count BEFORE: $APPLIED_BEFORE"

GOOD_FLIP_YAML=$(cat <<'YAML'
server:
  grpc:
    addr: ":9000"
  http:
    addr: ":8000"
log:
  level: debug
data:
  database:
    dsn: "postgres://demo:demo@localhost:5432/demo?sslmode=disable"
    max_open: 10
    max_idle: 5
    conn_max_lifetime: 300s
    conn_max_idle_time: 60s
    connect_timeout: 2s
  redis:
    addrs:
      - "localhost:6379"
trace:
  endpoint: ""
  sample_ratio: 0.0
YAML
)

docker exec "$ETCD_CONTAINER" etcdctl --endpoints=http://127.0.0.1:2379 put "$ETCD_KEY" "$GOOD_FLIP_YAML"
echo ">> put good config change (log.level info→debug) into etcd"

# Poll until the producer-side 'config applied' count strictly increases.
APPLIED_OK=0
for i in $(seq 1 30); do
    APPLIED_NOW=$(count_log "confcenter: config applied")
    echo "  attempt $i: 'config applied' count → $APPLIED_NOW"
    if [ "$APPLIED_NOW" -gt "$APPLIED_BEFORE" ]; then
        APPLIED_OK=1
        echo ">> PASS hot-reload landed: 'config applied' $APPLIED_BEFORE → $APPLIED_NOW (+$((APPLIED_NOW - APPLIED_BEFORE)))"
        grep -F "confcenter: config applied" "$DEMO_LOG" | tail -1 || true
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-C2: demo died during hot-reload landing wait"
        exit 1
    fi
    sleep 1
done
if [ "$APPLIED_OK" -ne 1 ]; then
    echo ">> Log tail (last 30 lines):"
    tail -30 "$DEMO_LOG" || true
    echo "FAIL AC-C2: 'config applied' count did not increase after good config change"
    exit 1
fi

# Sanity: a real config change with a valid DSN must keep the app healthy.
READYZ_AFTER_FLIP=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz)
echo ">> /readyz after good flip: $READYZ_AFTER_FLIP"
if [ "$READYZ_AFTER_FLIP" != "200" ]; then
    echo "FAIL AC-C2: /readyz should stay 200 after good config change, got $READYZ_AFTER_FLIP"
    exit 1
fi
echo ">> PASS /readyz=200 (valid hot-reload, app stays healthy)"

# ── Step 6: 坏配置回滚 — put empty grpc.addr, assert 'retaining previous config' +1 ─
echo ""
echo "=== Step 6: bad config rollback — put empty grpc.addr, assert 'retaining previous config' +1 ==="

RETAIN_BEFORE=$(count_log "retaining previous config")
echo ">> 'retaining previous config' count BEFORE: $RETAIN_BEFORE"

BAD_GRPC_YAML=$(cat <<'YAML'
server:
  grpc:
    addr: ""
  http:
    addr: ":8000"
log:
  level: info
data:
  database:
    dsn: "postgres://demo:demo@localhost:5432/demo?sslmode=disable"
    max_open: 10
    max_idle: 5
    conn_max_lifetime: 300s
    conn_max_idle_time: 60s
    connect_timeout: 2s
  redis:
    addrs:
      - "localhost:6379"
trace:
  endpoint: ""
  sample_ratio: 0.0
YAML
)

docker exec "$ETCD_CONTAINER" etcdctl --endpoints=http://127.0.0.1:2379 put "$ETCD_KEY" "$BAD_GRPC_YAML"
echo ">> put empty grpc.addr into etcd"

# Poll until the 'retaining previous config' count strictly increases
# (BEFORE/AFTER, not bare grep — a pre-existing line cannot false-pass).
RETAIN_OK=0
for i in $(seq 1 30); do
    RETAIN_NOW=$(count_log "retaining previous config")
    echo "  attempt $i: 'retaining previous config' count → $RETAIN_NOW"
    if [ "$RETAIN_NOW" -gt "$RETAIN_BEFORE" ]; then
        RETAIN_OK=1
        echo ">> PASS bad config rejected: 'retaining previous config' $RETAIN_BEFORE → $RETAIN_NOW (+$((RETAIN_NOW - RETAIN_BEFORE)))"
        grep -F "retaining previous config" "$DEMO_LOG" | tail -1 || true
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL AC-C2: demo died while waiting for bad config rejection"
        exit 1
    fi
    sleep 1
done
if [ "$RETAIN_OK" -ne 1 ]; then
    echo ">> Log tail (last 30 lines):"
    tail -30 "$DEMO_LOG" || true
    echo "FAIL AC-C2: 'retaining previous config' count did not increase after bad config"
    exit 1
fi

# ── Step 7: assert process alive + /readyz still 200 (previous config retained) ─
echo ""
echo "=== Step 7: assert process alive + /readyz=200 after bad config ==="
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-C2: demo process died after bad config push"
    exit 1
fi
echo ">> PASS demo process alive"

READYZ_FINAL=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz)
echo ">> /readyz after bad config: $READYZ_FINAL"
if [ "$READYZ_FINAL" != "200" ]; then
    echo "FAIL AC-C2: /readyz should stay 200 after bad config rejected, got $READYZ_FINAL"
    exit 1
fi
echo ">> PASS /readyz=200 (previous config retained)"

echo ""
echo "=== scen_conf_etcd.sh PASSED (AC-C2) ==="
echo "  etcd config load:    /readyz=200 (config from etcd)"
echo "  hot-reload landing:  good config change → 'config applied' BEFORE/AFTER +1 (producer evidence), /readyz=200"
echo "  bad config rollback: empty grpc.addr → 'retaining previous config' BEFORE/AFTER +1, process alive, /readyz=200"
