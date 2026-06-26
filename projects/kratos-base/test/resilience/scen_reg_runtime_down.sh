#!/usr/bin/env bash
# AC-CR2: 注册中心运行期宕 → discovery 调用失败、/v1/ping 照常、进程不崩 →
#          恢复后 discovery 又通。
#
# 参数: $1 = etcd | nacos
#
# 设计前提（已与用户对齐）：
#   - 使用 disc-bootstrap（mode=local + registry=backend），配置来自文件。
#   - 注册中心宕：注册中心不在 readyz，所以 /readyz 继续 200；
#     但 discoveryprobe 会因为解析不到服务而失败（退出非 0 或无 "pong" 输出）。
#   - 恢复后：注册中心 SDK（etcd contrib / nacos SDK）的租约 keepalive 自愈，
#     服务记录在注册中心侧重新生效；discoveryprobe 再次得到真 pong。
#   - 注意：registryx Runner 注册一次就 park（无重注册循环）；"自愈"靠 SDK 层
#     keepalive，而非 app 层主动重注册。因此删去 CR2-c（grep 日志"service registered"
#     命中的是启动期那行，不是恢复期的重注册——该行为不存在）。
#
# 验收断言（rule-0009 锚定产出方证据）：
#   CR2-a: discovery probe 在后端宕时失败（输出不含 "pong" 或退出非 0）
#   CR2-b: /v1/ping=200（本地接口正常）、进程活（kill -0 成功）
#   CR2-d: recoveryprobe 恢复后重跑输出含 "pong"（经 discovery 真 RPC 得 Ping 返回，干净充分）
#
# CWD 无关（脚本内 cd 到 project root）。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.."

# ── 参数解析 ──────────────────────────────────────────────────────────────────
BACKEND="${1:-}"
if [ "$BACKEND" != "etcd" ] && [ "$BACKEND" != "nacos" ]; then
    echo "Usage: $0 etcd|nacos"
    echo "  etcd  → uses configs/bootstrap.disc-sandbox.yaml (mode=local, registry=etcd)"
    echo "  nacos → uses configs/bootstrap.nacos-disc.yaml (mode=local, registry=nacos)"
    exit 1
fi

COMPOSE_CMD="docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml"
NACOS_ADDR="${NACOS_ADDR:-127.0.0.1:8848}"

DEMO_PID=""
DEMO_LOG="/tmp/demo_reg_runtime_down_${BACKEND}_$$.log"
PROBE_BIN="/tmp/discoveryprobe_reg_down_${BACKEND}_$$"

select_bootstrap() {
    if [ "$BACKEND" = "etcd" ]; then
        echo "configs/bootstrap.disc-sandbox.yaml"
    else
        echo "configs/bootstrap.nacos-disc.yaml"
    fi
}

backend_service_name() {
    if [ "$BACKEND" = "etcd" ]; then
        echo "etcd"
    else
        echo "nacos"
    fi
}

# discovery endpoint 以 nacos 注册时追加 ".grpc" 而得名
discovery_endpoint() {
    if [ "$BACKEND" = "etcd" ]; then
        echo "discovery:///demo"
    else
        echo "discovery:///demo.grpc"
    fi
}

# ── cleanup ───────────────────────────────────────────────────────────────────
cleanup() {
    echo ""
    echo "=== [AC-CR2-${BACKEND}] cleanup ==="
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
    # Restart backend in case it was stopped during the test
    $COMPOSE_CMD start "$(backend_service_name)" 2>/dev/null || true
    echo ">> sandbox-down"
    make sandbox-down 2>/dev/null || true
    rm -f "$DEMO_LOG" "$PROBE_BIN"
}
trap cleanup EXIT

echo "=== AC-CR2: scen_reg_runtime_down (backend=$BACKEND) ==="

# ── Step 1: sandbox-up ────────────────────────────────────────────────────────
echo ""
echo "=== Step 1: sandbox-up ==="
make sandbox-down 2>/dev/null || true
make sandbox-up
echo ">> sandbox healthy"

# ── Step 2: build demo + discoveryprobe ───────────────────────────────────────
echo ""
echo "=== Step 2: build demo + discoveryprobe ==="
go build -o bin/demo ./app/demo/cmd
echo ">> built bin/demo"
go build -o "$PROBE_BIN" ./test/discoveryprobe
echo ">> built discoveryprobe at $PROBE_BIN"

# ── Step 3: 起 demo（registry=backend）+ 轮询直到 discovery probe 得 pong ────
echo ""
echo "=== Step 3: start demo (registry=$BACKEND) ==="
BOOTSTRAP="$(select_bootstrap)"
./bin/demo -conf "$BOOTSTRAP" >"$DEMO_LOG" 2>&1 &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID  bootstrap: $BOOTSTRAP"

# 先轮询 /v1/ping=200（确认服务起来）
echo ""
echo "=== Step 3a: poll /v1/ping until 200 ==="
STARTED=0
for i in $(seq 1 40); do
    RC=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/v1/ping 2>/dev/null || echo "000")
    echo "  attempt $i: /v1/ping → $RC"
    if [ "$RC" = "200" ]; then
        STARTED=1
        echo ">> PASS /v1/ping=200"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "ERROR: demo died during startup"
        tail -30 "$DEMO_LOG" || true
        exit 1
    fi
    sleep 1
done
if [ "$STARTED" -ne 1 ]; then
    echo "FAIL AC-CR2: demo never responded to /v1/ping"
    tail -30 "$DEMO_LOG" || true
    exit 1
fi

# 轮询 discoveryprobe 直到 "pong"（证明注册 + 发现已建立）
echo ""
echo "=== Step 3b: poll discoveryprobe until 'pong' (baseline) ==="
PROBE_OK=0
for i in $(seq 1 60); do
    if [ "$BACKEND" = "etcd" ]; then
        PROBE_OUT=$("$PROBE_BIN" 2>&1 || true)
    else
        PROBE_OUT=$(DISCOVERY_BACKEND=nacos NACOS_ADDR="$NACOS_ADDR" "$PROBE_BIN" 2>&1 || true)
    fi
    echo "  attempt $i: probe output: $PROBE_OUT"
    if echo "$PROBE_OUT" | grep -q "pong"; then
        PROBE_OK=1
        echo ">> PASS baseline discoveryprobe returned 'pong'"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL: demo died while waiting for discoveryprobe baseline"
        exit 1
    fi
    sleep 2
done
if [ "$PROBE_OK" -ne 1 ]; then
    echo "FAIL AC-CR2 Step 3b: discoveryprobe never returned 'pong' at baseline"
    tail -30 "$DEMO_LOG" || true
    exit 1
fi

# ── Step 4: 停注册中心 ────────────────────────────────────────────────────────
echo ""
echo "=== Step 4: stop $BACKEND (registry goes down) ==="
$COMPOSE_CMD stop "$(backend_service_name)"
echo ">> $BACKEND stopped; waiting 4s for clients to detect disconnection"
sleep 4

# ── Step 5: 断言 CR2-a：discoveryprobe 失败（解析不到 / 调用错）────────────
echo ""
echo "=== Step 5: assert discoveryprobe FAILS while $BACKEND is down (CR2-a) ==="
if [ "$BACKEND" = "etcd" ]; then
    PROBE_DOWN_OUT=$("$PROBE_BIN" 2>&1 || true)
else
    PROBE_DOWN_OUT=$(DISCOVERY_BACKEND=nacos NACOS_ADDR="$NACOS_ADDR" "$PROBE_BIN" 2>&1 || true)
fi
echo ">> probe output while $BACKEND down: $PROBE_DOWN_OUT"

if echo "$PROBE_DOWN_OUT" | grep -q "pong"; then
    echo "FAIL CR2-a: discoveryprobe should FAIL when $BACKEND is down but returned 'pong'"
    echo "  (This is unexpected — the probe may have cached a resolved address.)"
    exit 1
fi
echo ">> PASS CR2-a: discoveryprobe did NOT return 'pong' — discovery failure confirmed"

# ── Step 6: 断言 CR2-b：/v1/ping=200 + 进程活 ───────────────────────────────
echo ""
echo "=== Step 6: assert /v1/ping=200 and process alive while $BACKEND down (CR2-b) ==="

# 进程活
if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    echo "FAIL CR2-b: demo DIED after $BACKEND stopped"
    tail -30 "$DEMO_LOG" || true
    exit 1
fi
echo ">> PASS CR2-b: kill -0 $DEMO_PID succeeded — process alive"

# /v1/ping 照常（本地 handler，不依赖注册中心）
PING_DOWN=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/v1/ping 2>/dev/null || echo "000")
echo ">> /v1/ping while $BACKEND down → $PING_DOWN"
if [ "$PING_DOWN" != "200" ]; then
    echo "FAIL CR2-b: /v1/ping should be 200 while $BACKEND is down, got $PING_DOWN"
    exit 1
fi
echo ">> PASS CR2-b: /v1/ping=200 while $BACKEND down (local endpoint unaffected)"

# ── Step 7: 恢复注册中心 ──────────────────────────────────────────────────────
echo ""
echo "=== Step 7: start $BACKEND (registry recovers) ==="
$COMPOSE_CMD start "$(backend_service_name)"
echo ">> $BACKEND started, waiting for healthy..."

if [ "$BACKEND" = "etcd" ]; then
    for i in $(seq 1 40); do
        HC=$(docker inspect --format='{{.State.Health.Status}}' kratosbase-sandbox-etcd-1 2>/dev/null || echo "unknown")
        if [ "$HC" = "healthy" ]; then echo ">> etcd healthy (attempt $i)"; break; fi
        if [ "$i" -eq 40 ]; then echo "FAIL: etcd did not become healthy"; exit 1; fi
        sleep 1
    done
else
    for i in $(seq 1 60); do
        HC=$(curl -s -o /dev/null -w "%{http_code}" "http://$NACOS_ADDR/nacos/v1/console/health/readiness" 2>/dev/null || echo "000")
        if [ "$HC" = "200" ]; then echo ">> nacos healthy (attempt $i)"; break; fi
        if [ "$i" -eq 60 ]; then echo "FAIL: nacos did not become healthy after 120s"; exit 1; fi
        sleep 2
    done
fi

# ── Step 8 (CR2-c 已删): 恢复机制说明 ───────────────────────────────────────
# CR2-c（grep 日志"service registered"）已删除：registryx Runner 注册一次就 park，
# 无重注册循环；grep 命中的是启动期那行，不是恢复期的重注册——该声称行为不存在。
# 恢复靠注册中心 SDK（etcd contrib / nacos SDK）的租约 keepalive 自愈，非 app 层重注册。
# 恢复证据只用 CR2-d：discoveryprobe 得真 pong（经 discovery 真 RPC，干净充分）。

# ── Step 9: 断言 CR2-d：discoveryprobe 重跑得 pong ──────────────────────────
echo ""
echo "=== Step 9: poll discoveryprobe until 'pong' after recovery (CR2-d) ==="
PROBE_RECOVERED=0
for i in $(seq 1 30); do
    if [ "$BACKEND" = "etcd" ]; then
        PROBE_REC_OUT=$("$PROBE_BIN" 2>&1 || true)
    else
        PROBE_REC_OUT=$(DISCOVERY_BACKEND=nacos NACOS_ADDR="$NACOS_ADDR" "$PROBE_BIN" 2>&1 || true)
    fi
    echo "  attempt $i: probe output: $PROBE_REC_OUT"
    if echo "$PROBE_REC_OUT" | grep -q "pong"; then
        PROBE_RECOVERED=1
        echo ">> PASS CR2-d: discoveryprobe returned 'pong' after $BACKEND recovery"
        break
    fi
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        echo "FAIL CR2-d: demo died during recovery probe"
        exit 1
    fi
    sleep 2
done
if [ "$PROBE_RECOVERED" -ne 1 ]; then
    echo "FAIL CR2-d: discoveryprobe did not return 'pong' after $BACKEND recovered"
    tail -30 "$DEMO_LOG" || true
    exit 1
fi

echo ""
echo "=== scen_reg_runtime_down.sh PASSED (AC-CR2, backend=$BACKEND) ==="
echo "  CR2-a: $BACKEND down → discoveryprobe failed (no 'pong')"
echo "  CR2-b: /v1/ping=200, process alive while $BACKEND down"
echo "  CR2-d: $BACKEND recovered → discoveryprobe returned 'pong' (SDK keepalive 自愈，非 app 层重注册)"
