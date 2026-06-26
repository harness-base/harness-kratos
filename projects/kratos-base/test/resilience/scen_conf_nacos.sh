#!/usr/bin/env bash
# AC-N1: nacos 配置中心闭环（加载 + 热更落地 + 坏配置回滚）。
#
# 验收断言（rule-0009 锚定产出方证据 / 避免共因 §A）：
#   - demo 配置真从 nacos 加载：/readyz=200（进程存活且后端连通）。
#   - 热更"落地"正向硬证：推一个**好**配置变更（仅 log.level info→debug，DSN 不动、
#     pg 不停）→ confcenter 产出方日志 `confcenter: config applied` 的 BEFORE/AFTER
#     计数 +1（只能由 Manager.Publish 成功路径打出，无法被请求入参回显伪造）。
#     不再用"坏 DSN + 停 pg → 503"做归因——那是共因污染（503 纯由停 pg 触发，
#     且 provider self-heal 会吞掉单纯坏 DSN，光改坏 DSN 不停 pg 不会翻 503）。
#   - 坏配置（空 grpc.addr）回滚：`retaining previous config` 的 BEFORE/AFTER 计数 +1
#     （而非裸 grep，防旧行假命中）、进程活、/readyz 仍 200。
#
# 前置：sandbox 含 nacos（8848/9848）。本脚本自己调 sandbox-up，不依赖外部状态。
# CWD 无关（脚本内 cd 到 project root）。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.."

# count_log <substr> — number of lines in $DEMO_LOG containing the literal substr,
# emitted as a single integer. Used for BEFORE/AFTER comparison so a pre-existing
# line can never false-pass. (grep -c already prints 0 on no-match and exits 1;
# `|| true` swallows that exit without appending a second line.)
count_log() { local n; n=$(grep -cF "$1" "$DEMO_LOG" 2>/dev/null || true); echo "${n:-0}"; }

NACOS_ADDR="${NACOS_ADDR:-127.0.0.1:8848}"
NACOS_API="http://$NACOS_ADDR/nacos/v1/cs/configs"
DATA_ID="runtime.yaml"
GROUP="DEFAULT_GROUP"
DEMO_PID=""
DEMO_LOG="/tmp/demo_nacos_$$.log"
COMPOSE_CMD="docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml"

publish_cfg() { # $1 = yaml 内容
    curl -sf -X POST "$NACOS_API" \
        --data-urlencode "dataId=$DATA_ID" \
        --data-urlencode "group=$GROUP" \
        --data-urlencode "content=$1" >/dev/null
}

cleanup() {
    echo ""
    echo "=== [AC-N1] cleanup ==="
    if [ -n "$DEMO_PID" ] && kill -0 "$DEMO_PID" 2>/dev/null; then
        echo ">> stopping demo (SIGTERM)"
        kill -TERM "$DEMO_PID" 2>/dev/null || true
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

echo "=== AC-N1: scen_conf_nacos (nacos config center e2e) ==="

# ── Step 1: sandbox-up（含 nacos）─────────────────────────────────────────────
echo ""
echo "=== Step 1: sandbox-up (pg + redis + etcd + rabbitmq + nacos) ==="
make sandbox-down 2>/dev/null || true
make sandbox-up
echo ">> sandbox healthy"

# ── Step 2: nacos 可达性验证 ──────────────────────────────────────────────────
echo ""
echo "=== Step 2: verify nacos reachable ==="
NACOS_HC=$(curl -s -o /dev/null -w "%{http_code}" "http://$NACOS_ADDR/nacos/v1/console/health/readiness")
if [ "$NACOS_HC" != "200" ]; then
    echo "BLOCKED: nacos not reachable at $NACOS_ADDR (HTTP $NACOS_HC)"
    docker logs kratosbase-sandbox-nacos-1 2>&1 | tail -30 || true
    exit 1
fi
echo ">> nacos reachable (HTTP 200)"

# ── Step 3: 发布初始配置到 nacos ──────────────────────────────────────────────
echo ""
echo "=== Step 3: publish initial config to nacos (dataId=$DATA_ID group=$GROUP) ==="
GOOD_CFG=$(cat <<'YAML'
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
  mq:
    kind: "rabbitmq"
    topic: "demo.events"
    rabbitmq:
      url: "amqp://guest:guest@localhost:5672/"
      dial_timeout: "5s"
    rocketmq:
      endpoint: ""
      access_key: ""
      secret_key: ""
      consumer_group: "demo-consumer"
      await_duration: "5s"
      request_timeout: "3s"
trace:
  endpoint: ""
  sample_ratio: 0.0
YAML
)
publish_cfg "$GOOD_CFG"
echo ">> config published"

# ── Step 4: build demo ────────────────────────────────────────────────────────
echo ""
echo "=== Step 4: build demo ==="
go build -o bin/demo ./app/demo/cmd
echo ">> built bin/demo"

# ── Step 5: 起 demo（mode=nacos）→ 轮询 /readyz=200 ──────────────────────────
echo ""
echo "=== Step 5: start demo (mode=nacos) → poll /readyz until 200 ==="
./bin/demo -conf configs/bootstrap.nacos-sandbox.yaml >"$DEMO_LOG" 2>&1 &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID"

STARTED=0
for i in $(seq 1 40); do
    RC=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "000")
    echo "  attempt $i: /readyz → $RC"
    if [ "$RC" = "200" ]; then
        STARTED=1
        echo ">> PASS /readyz=200 (config loaded from nacos, pg healthy)"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL: demo exited early"
        tail -30 "$DEMO_LOG"
        exit 1
    fi
    sleep 1
done
if [ "$STARTED" -ne 1 ]; then
    echo "FAIL AC-N1 Step 5: /readyz never reached 200 (nacos config not loaded?)"
    tail -30 "$DEMO_LOG"
    exit 1
fi

# ── Step 6: 热更落地正向硬证 — publish a GOOD config change, assert producer evidence ─
# rule-0009 §A：不再用"坏 DSN + 停 pg → 503"做归因（共因污染）。改为推一个**好**配置
# 变更（仅 log.level info→debug，DSN 不动、pg 不停），用 confcenter 产出方日志
# `confcenter: config applied` 的 BEFORE/AFTER 计数 +1 证明热更真落地。该行只在
# Manager.Publish 成功路径打出，无法被任何请求访问日志/入参回显伪造。
echo ""
echo "=== Step 6: hot-reload LANDING — publish good config change, assert 'config applied' +1 ==="

APPLIED_BEFORE=$(count_log "confcenter: config applied")
echo ">> 'config applied' count BEFORE: $APPLIED_BEFORE"

GOOD_FLIP_CFG=$(cat <<'YAML'
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
  mq:
    kind: "rabbitmq"
    topic: "demo.events"
    rabbitmq:
      url: "amqp://guest:guest@localhost:5672/"
      dial_timeout: "5s"
    rocketmq:
      endpoint: ""
      access_key: ""
      secret_key: ""
      consumer_group: "demo-consumer"
      await_duration: "5s"
      request_timeout: "3s"
trace:
  endpoint: ""
  sample_ratio: 0.0
YAML
)
publish_cfg "$GOOD_FLIP_CFG"
echo ">> good config change published (log.level info→debug)"

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
        echo "FAIL: demo died during hot-reload landing wait"
        tail -20 "$DEMO_LOG"
        exit 1
    fi
    sleep 1
done
if [ "$APPLIED_OK" -ne 1 ]; then
    echo ">> Log tail (last 20 lines):"
    tail -20 "$DEMO_LOG" || true
    echo "FAIL AC-N1 Step 6: 'config applied' count did not increase after good config change"
    exit 1
fi

# Sanity: a real config change with a valid DSN must keep the app healthy.
READYZ_AFTER_FLIP=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "000")
echo ">> /readyz after good flip: $READYZ_AFTER_FLIP"
if [ "$READYZ_AFTER_FLIP" != "200" ]; then
    echo "FAIL AC-N1 Step 6: /readyz should stay 200 after good config change, got $READYZ_AFTER_FLIP"
    exit 1
fi
echo ">> PASS /readyz=200 (valid hot-reload, app stays healthy)"

# ── Step 7: 坏配置回滚 — empty grpc.addr, assert 'retaining previous config' +1 ─
echo ""
echo "=== Step 7: bad config rollback — empty grpc.addr, assert 'retaining previous config' +1 ==="

RETAIN_BEFORE=$(count_log "retaining previous config")
echo ">> 'retaining previous config' count BEFORE: $RETAIN_BEFORE"

INVALID_CFG=$(cat <<'YAML'
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
  mq:
    kind: "rabbitmq"
    topic: "demo.events"
    rabbitmq:
      url: "amqp://guest:guest@localhost:5672/"
      dial_timeout: "5s"
    rocketmq:
      endpoint: ""
      access_key: ""
      secret_key: ""
      consumer_group: "demo-consumer"
      await_duration: "5s"
      request_timeout: "3s"
trace:
  endpoint: ""
  sample_ratio: 0.0
YAML
)
publish_cfg "$INVALID_CFG"
echo ">> invalid config published (empty grpc.addr)"

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
        echo "FAIL: demo died while waiting for config rejection"
        exit 1
    fi
    sleep 1
done
if [ "$RETAIN_OK" -ne 1 ]; then
    echo ">> Log tail (last 20 lines):"
    tail -20 "$DEMO_LOG" || true
    echo "FAIL AC-N1 Step 7: 'retaining previous config' count did not increase after bad config"
    exit 1
fi

# 进程仍活、/readyz 仍 200
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL AC-N1 Step 7: demo process died after invalid config"
    exit 1
fi
FINAL_RC=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "000")
if [ "$FINAL_RC" != "200" ]; then
    echo "FAIL AC-N1 Step 7: /readyz=$FINAL_RC after invalid config (expected 200)"
    exit 1
fi
echo ">> PASS invalid config rejected: process alive, /readyz=200"

echo ""
echo "=== scen_conf_nacos.sh PASSED (AC-N1) ==="
echo "  AC-N1-a: config loaded from nacos → /readyz=200"
echo "  AC-N1-b: hot-reload landing → good config change → 'config applied' BEFORE/AFTER +1 (producer evidence), /readyz=200"
echo "  AC-N1-c: bad config rollback → empty grpc.addr → 'retaining previous config' BEFORE/AFTER +1, process alive, /readyz=200"
