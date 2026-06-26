#!/usr/bin/env bash
# AC-CR1: 配置中心运行期宕 → 服务不崩、本地接口照常 → 恢复后热更续上。
#
# 参数: $1 = etcd | nacos
#
# 设计前提（已与用户对齐）：
#   - 配置已在启动时加载进内存；配置中心宕机后进程不崩、/readyz 仍 200
#     （readyz 只查 pg/redis/mq，不查配置中心）。
#   - 恢复后 watch 自重连，热更管线复活。
#
# 为什么不用"推坏 redis 地址 → readyz 翻 503"证热更：那其实是脆弱的假象——
#   有了 resource.Provider 的 self-heal（坏地址 Build 失败→回退旧好句柄）+ Open 尊重
#   调用方 ctx + readyz 合理超时后，推坏 redis 地址 readyz **本就该保持 200**（继续用
#   旧好连接，这才是对的弹性）。原先能翻 503 全靠"Open 阻塞耗光 readyz 那点 ctx"的
#   超时竞态，机制不稳、随 readyz 超时改动即失效。改用下面与 ctx/self-heal 无关的硬证据。
#
# 验收断言（rule-0009 锚定产出方证据）：
#   CR1-a: 配置中心停后 kill -0 $PID 成功（进程存活），/v1/greet=200，/readyz=200
#   CR1-b: 恢复后推【非法配置（空 grpc.addr）】→ demo 日志出现【新增】的
#          "retaining previous config"（confcenter 产出方日志）。该日志只有 watch 重连
#          并把变更送达 confcenter、校验拒绝才会出现 ⇒ 证明热更管线已复活。
#          （用计数 BEFORE/AFTER 对比，杜绝命中恢复前的旧日志行。）
#   CR1-c: 推回好配置 → /readyz 保持 200（拒绝非法后服务仍健康、好配置被接受）
#
# CWD 无关（脚本内 cd 到 project root）。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.."

# ── 参数解析 ──────────────────────────────────────────────────────────────────
BACKEND="${1:-}"
if [ "$BACKEND" != "etcd" ] && [ "$BACKEND" != "nacos" ]; then
    echo "Usage: $0 etcd|nacos"
    echo "  etcd  → uses configs/bootstrap.etcd-sandbox.yaml (mode=etcd)"
    echo "  nacos → uses configs/bootstrap.nacos-sandbox.yaml (mode=nacos)"
    exit 1
fi

COMPOSE_CMD="docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml"
ETCD_CONTAINER="kratosbase-sandbox-etcd-1"
ETCD_KEY="/configs/demo/runtime.yaml"
NACOS_ADDR="${NACOS_ADDR:-127.0.0.1:8848}"
NACOS_API="http://$NACOS_ADDR/nacos/v1/cs/configs"
NACOS_DATA_ID="runtime.yaml"
NACOS_GROUP="DEFAULT_GROUP"

DEMO_PID=""
DEMO_LOG="/tmp/demo_cc_runtime_down_${BACKEND}_$$.log"

# ── 配置模板 ──────────────────────────────────────────────────────────────────
GOOD_RUNTIME_YAML=$(cat <<'YAML'
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

# 非法配置：空 grpc.addr → confcenter 校验拒绝（其余字段保持合法，确保唯一拒绝原因是空 addr）
INVALID_YAML=$(cat <<'YAML'
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

# ── 辅助函数：发布配置 ────────────────────────────────────────────────────────
publish_cfg() {
    local yaml_content="$1"
    if [ "$BACKEND" = "etcd" ]; then
        docker exec "$ETCD_CONTAINER" etcdctl --endpoints=http://127.0.0.1:2379 put "$ETCD_KEY" "$yaml_content"
    else
        curl -sf -X POST "$NACOS_API" \
            --data-urlencode "dataId=$NACOS_DATA_ID" \
            --data-urlencode "group=$NACOS_GROUP" \
            --data-urlencode "content=$yaml_content" >/dev/null
    fi
}

select_bootstrap() {
    if [ "$BACKEND" = "etcd" ]; then
        echo "configs/bootstrap.etcd-sandbox.yaml"
    else
        echo "configs/bootstrap.nacos-sandbox.yaml"
    fi
}

backend_service_name() {
    if [ "$BACKEND" = "etcd" ]; then
        echo "etcd"
    else
        echo "nacos"
    fi
}

# ── cleanup ───────────────────────────────────────────────────────────────────
cleanup() {
    echo ""
    echo "=== [AC-CR1-${BACKEND}] cleanup ==="
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
    # Restore backend if it was stopped during the test (pg is never stopped in this scenario)
    $COMPOSE_CMD start "$(backend_service_name)" 2>/dev/null || true
    echo ">> sandbox-down"
    make sandbox-down 2>/dev/null || true
    rm -f "$DEMO_LOG"
}
trap cleanup EXIT

echo "=== AC-CR1: scen_cc_runtime_down (backend=$BACKEND) ==="

# ── Step 1: sandbox-up ────────────────────────────────────────────────────────
echo ""
echo "=== Step 1: sandbox-up ==="
make sandbox-down 2>/dev/null || true
make sandbox-up
echo ">> sandbox healthy"

# ── Step 2: nacos 可达性验证（仅 nacos 模式）─────────────────────────────────
if [ "$BACKEND" = "nacos" ]; then
    echo ""
    echo "=== Step 2: verify nacos reachable ==="
    NACOS_HC=$(curl -s -o /dev/null -w "%{http_code}" "http://$NACOS_ADDR/nacos/v1/console/health/readiness" 2>/dev/null || echo "000")
    if [ "$NACOS_HC" != "200" ]; then
        echo "BLOCKED: nacos not reachable at $NACOS_ADDR (HTTP $NACOS_HC)"
        docker logs kratosbase-sandbox-nacos-1 2>&1 | tail -30 || true
        exit 1
    fi
    echo ">> nacos reachable (HTTP 200)"
fi

# ── Step 3: 发布初始配置 ──────────────────────────────────────────────────────
echo ""
echo "=== Step 3: publish initial config to $BACKEND ==="
publish_cfg "$GOOD_RUNTIME_YAML"
echo ">> initial config published to $BACKEND"

# ── Step 4: build + 起 demo ───────────────────────────────────────────────────
echo ""
echo "=== Step 4: build + start demo ($BACKEND config) ==="
go build -o bin/demo ./app/demo/cmd
echo ">> built bin/demo"

BOOTSTRAP="$(select_bootstrap)"
./bin/demo -conf "$BOOTSTRAP" >"$DEMO_LOG" 2>&1 &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID  bootstrap: $BOOTSTRAP"

# 轮询 /readyz=200（证明配置从后端加载）
echo ""
echo "=== Step 4 (cont): poll /readyz until 200 ==="
READY=0
for i in $(seq 1 40); do
    RC=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "000")
    echo "  attempt $i: /readyz → $RC"
    if [ "$RC" = "200" ]; then
        READY=1
        echo ">> PASS /readyz=200 (config loaded from $BACKEND)"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "ERROR: demo died during startup"
        tail -30 "$DEMO_LOG" || true
        exit 1
    fi
    sleep 1
done
if [ "$READY" -ne 1 ]; then
    echo "FAIL AC-CR1: /readyz never became 200"
    tail -30 "$DEMO_LOG" || true
    exit 1
fi

# 也确认 /v1/greet 正常（任意 id 走 DB）
GREET_RC=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8000/v1/greet/1" 2>/dev/null || echo "000")
echo ">> baseline /v1/greet/1 → $GREET_RC"
# 200 或 404 均说明服务正常（DB 里可能没有 id=1）
if [ "$GREET_RC" = "200" ] || [ "$GREET_RC" = "404" ] || [ "$GREET_RC" = "400" ]; then
    echo ">> PASS baseline /v1/greet OK (HTTP $GREET_RC)"
else
    echo "FAIL AC-CR1: /v1/greet baseline unexpected code $GREET_RC"
    exit 1
fi

# ── Step 5: 停配置中心 ────────────────────────────────────────────────────────
echo ""
echo "=== Step 5: stop $BACKEND (config center goes down) ==="
$COMPOSE_CMD stop "$(backend_service_name)"
echo ">> $BACKEND stopped"

# 等几拍让 watch goroutine 感知断开
sleep 5

# ── Step 6: 核心断言 CR1-a ————进程活 + 本地接口照常 + /readyz=200 ───────────
echo ""
echo "=== Step 6: assert process alive + local endpoints still up + /readyz=200 (CR1-a) ==="

# 6a: 进程活（产出方证据：kill -0 成功）
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL CR1-a: demo process DIED after $BACKEND stopped (watch goroutine panic?)"
    tail -30 "$DEMO_LOG" || true
    exit 1
fi
echo ">> PASS CR1-a: kill -0 $DEMO_PID succeeded — process alive after $BACKEND down"

# 6b: /v1/greet 照常（配置在内存，路由正常）
GREET_DOWN=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8000/v1/greet/1" 2>/dev/null || echo "000")
echo ">> /v1/greet/1 while $BACKEND down → $GREET_DOWN"
if [ "$GREET_DOWN" = "200" ] || [ "$GREET_DOWN" = "404" ] || [ "$GREET_DOWN" = "400" ]; then
    echo ">> PASS CR1-a: /v1/greet OK while $BACKEND down (HTTP $GREET_DOWN)"
else
    echo "FAIL CR1-a: /v1/greet should work while $BACKEND is down, got $GREET_DOWN"
    tail -20 "$DEMO_LOG" || true
    exit 1
fi

# 6c: /readyz=200（readyz 只查 pg/redis/mq，不查配置中心）
READYZ_DOWN=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || echo "000")
echo ">> /readyz while $BACKEND down → $READYZ_DOWN"
if [ "$READYZ_DOWN" != "200" ]; then
    echo "FAIL CR1-a: /readyz should be 200 while $BACKEND is down, got $READYZ_DOWN"
    tail -20 "$DEMO_LOG" || true
    exit 1
fi
echo ">> PASS CR1-a: /readyz=200 while $BACKEND down (config center not in readyz)"

# ── Step 7: 恢复配置中心 ──────────────────────────────────────────────────────
echo ""
echo "=== Step 7: start $BACKEND (config center recovers) ==="
$COMPOSE_CMD start "$(backend_service_name)"
echo ">> $BACKEND started, waiting for healthy..."

# 等待后端 healthy
if [ "$BACKEND" = "etcd" ]; then
    for i in $(seq 1 40); do
        HC=$(docker inspect --format='{{.State.Health.Status}}' kratosbase-sandbox-etcd-1 2>/dev/null || echo "unknown")
        if [ "$HC" = "healthy" ]; then echo ">> etcd healthy (attempt $i)"; break; fi
        if [ "$i" -eq 40 ]; then echo "FAIL: etcd did not become healthy"; exit 1; fi
        sleep 1
    done
else
    # nacos 启动较慢，轮询 HTTP health endpoint
    for i in $(seq 1 60); do
        HC=$(curl -s -o /dev/null -w "%{http_code}" "http://$NACOS_ADDR/nacos/v1/console/health/readiness" 2>/dev/null || echo "000")
        if [ "$HC" = "200" ]; then echo ">> nacos healthy (attempt $i)"; break; fi
        if [ "$i" -eq 60 ]; then echo "FAIL: nacos did not become healthy after 60s"; exit 1; fi
        sleep 2
    done
fi

# 给 watch 重连一点时间
sleep 3

# ── Step 8: 推非法配置 → confcenter 拒绝并打 "retaining previous config"（CR1-b）─────
# 证明配置中心恢复后 watch 已重连：该日志只有 watch 把变更送达 confcenter、校验拒绝才出现。
# 产出方证据（confcenter 日志），与 self-heal/ctx 时序无关，不依赖 readyz 状态码。
# 用 BEFORE/AFTER 计数对比，杜绝命中恢复前的旧日志行（错题本：过期假命中）。
echo ""
echo "=== Step 8: push invalid config (empty grpc.addr) → expect NEW confcenter reject log (CR1-b) ==="
BEFORE=$(grep -c "retaining previous config" "$DEMO_LOG" 2>/dev/null || true)
publish_cfg "$INVALID_YAML"
echo ">> invalid config published (empty grpc.addr); prior 'retaining' count=$BEFORE"

LANDED=0
for i in $(seq 1 30); do
    AFTER=$(grep -c "retaining previous config" "$DEMO_LOG" 2>/dev/null || true)
    if [ "$AFTER" -gt "$BEFORE" ]; then
        LANDED=1
        echo ">> PASS CR1-b: watch reconnected — NEW 'retaining previous config' after recovery (count $BEFORE→$AFTER):"
        grep "retaining previous config" "$DEMO_LOG" | tail -1
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL CR1-b: demo died during invalid-config test"
        tail -20 "$DEMO_LOG" || true
        exit 1
    fi
    sleep 1
done
if [ "$LANDED" -ne 1 ]; then
    echo "FAIL CR1-b: no NEW 'retaining previous config' — watch did not deliver change post-recovery"
    tail -30 "$DEMO_LOG" || true
    exit 1
fi

# ── Step 9: 推回好配置 → 'config applied' +1 + /readyz 保持 200（CR1-c：好配置被接受、服务仍健康）──
# rule-0009（近恒真断言）：单看 /readyz=200 证不了"好配置被接受"——配置中心不在 readyz，
# 整个 CR1 期间 readyz 本就一直 200。所以主断言改为产出方硬证：confcenter 日志
# `config applied` 计数 BEFORE→AFTER +1（只有 observer reload+Publish 成功路径才打出，
# manager.go:147），证明这条好配置真被 watch 送达、校验通过、落地应用。
# 此刻后端持有的是 Step 8 推入的 INVALID（空 grpc.addr），推 GOOD 是真内容变更，watch 必触发。
# /readyz=200 降为旁证（好配置不该让服务掉线）。
echo ""
echo "=== Step 9: restore good config → 'config applied' +1 + /readyz stays 200 (CR1-c) ==="
APPLIED_BEFORE=$(grep -cF "confcenter: config applied" "$DEMO_LOG" 2>/dev/null || true)
echo ">> 'config applied' count BEFORE: $APPLIED_BEFORE"

publish_cfg "$GOOD_RUNTIME_YAML"
echo ">> good config restored in $BACKEND"

# Poll until the producer-side 'config applied' count strictly increases.
APPLIED_OK=0
for i in $(seq 1 30); do
    APPLIED_NOW=$(grep -cF "confcenter: config applied" "$DEMO_LOG" 2>/dev/null || true)
    echo "  attempt $i: 'config applied' count → $APPLIED_NOW"
    if [ "$APPLIED_NOW" -gt "$APPLIED_BEFORE" ]; then
        APPLIED_OK=1
        echo ">> PASS CR1-c: good config accepted — 'config applied' $APPLIED_BEFORE → $APPLIED_NOW (+$((APPLIED_NOW - APPLIED_BEFORE)))"
        grep -F "confcenter: config applied" "$DEMO_LOG" | tail -1 || true
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL CR1-c: demo died during recovery"
        tail -20 "$DEMO_LOG" || true
        exit 1
    fi
    sleep 1
done
if [ "$APPLIED_OK" -ne 1 ]; then
    echo ">> Log tail (last 30 lines):"
    tail -30 "$DEMO_LOG" || true
    echo "FAIL CR1-c: 'config applied' count did not increase after restoring good config"
    exit 1
fi

# Corroborating sign: a good config must not knock the service offline.
RC=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/readyz 2>/dev/null || true)
echo ">> /readyz after good config: $RC"
if [ "$RC" != "200" ]; then
    echo "FAIL CR1-c: /readyz should stay 200 after good config, got $RC"
    tail -30 "$DEMO_LOG" || true
    exit 1
fi
echo ">> PASS CR1-c: /readyz=200 (service stays healthy)"

echo ""
echo "=== scen_cc_runtime_down.sh PASSED (AC-CR1, backend=$BACKEND) ==="
echo "  CR1-a: $BACKEND stopped → kill -0 alive, /v1/greet OK, /readyz=200（继续用内存配置）"
echo "  CR1-b: $BACKEND recovered → 推非法配置触发 NEW 'retaining previous config'（watch 重连、热更管线复活，confcenter 产出方日志）"
echo "  CR1-c: 推回好配置 → 'config applied' BEFORE/AFTER +1（产出方硬证：好配置真被接受落地）+ /readyz=200"
