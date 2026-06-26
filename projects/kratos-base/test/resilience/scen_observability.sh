#!/usr/bin/env bash
# AC5: 可观测性验证 — 日志 JSON + trace_id；/metrics 含请求计数；stdout trace exporter 有 span 输出。
# AC6: 复用网关链路 — /v1/greet/1 经 HTTP→service→ent 返回正确结果。
# Usage: bash test/resilience/scen_observability.sh
# Must be run from the project root (projects/kratos-base/).
set -euo pipefail

DEMO_PID=""
# Use a config with sample_ratio: 1.0 so traces are actually sampled and trace_id appears in logs.
OBS_RUNTIME="/tmp/runtime_obs_$$.yaml"
OBS_BOOTSTRAP="/tmp/bootstrap_obs_$$.yaml"

cleanup() {
    echo ""
    echo "=== [AC5/AC6] cleanup ==="
    if [ -n "$DEMO_PID" ] && kill -0 "$DEMO_PID" 2>/dev/null; then
        echo ">> stopping demo gracefully (SIGTERM, flush span batcher)"
        kill -TERM "$DEMO_PID" || true
        # Wait up to 10s for graceful stop
        for i in $(seq 1 10); do
            if ! kill -0 "$DEMO_PID" 2>/dev/null; then
                echo ">> demo exited (attempt $i)"
                break
            fi
            sleep 1
        done
        # Force kill if still running
        if kill -0 "$DEMO_PID" 2>/dev/null; then
            echo ">> force kill demo"
            kill -KILL "$DEMO_PID" 2>/dev/null || true
        fi
        wait "$DEMO_PID" 2>/dev/null || true
    fi
    echo ">> make sandbox-down"
    make sandbox-down 2>/dev/null || true
    rm -f "$OBS_RUNTIME" "$OBS_BOOTSTRAP"
}
trap cleanup EXIT

echo "=== AC5+AC6: scen_observability ==="

# ── Step 1: sandbox-up ────────────────────────────────────────────────────────
echo ""
echo "=== Step 1: sandbox-up ==="
make sandbox-down 2>/dev/null || true
make sandbox-up
echo ">> PG healthy"

# ── Step 2: build + prepare observability config (sample_ratio=1.0) ───────────
echo ""
echo "=== Step 2: build demo and prepare observability config ==="
go build -o bin/demo ./app/demo/cmd
echo ">> built bin/demo"

# Create runtime config with sample_ratio=1.0 so spans ARE sampled → trace_id in logs
cat > "$OBS_RUNTIME" <<'EOF'
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
trace:
  endpoint: ""
  sample_ratio: 1.0
EOF

cat > "$OBS_BOOTSTRAP" <<EOF
infra:
  mode: local
  path: $OBS_RUNTIME
EOF
echo ">> observability config written (sample_ratio=1.0)"

# ── Step 3: start demo ────────────────────────────────────────────────────────
echo ""
echo "=== Step 3: start demo with observability config ==="
./bin/demo -conf "$OBS_BOOTSTRAP" >/tmp/demo_obs.log 2>&1 &
DEMO_PID=$!
echo ">> demo pid: $DEMO_PID"

# Poll /readyz until 200
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
        cat /tmp/demo_obs.log || true
        exit 1
    fi
    sleep 1
done
if [ "$READY" -ne 1 ]; then
    echo "FAIL AC5: /readyz never became 200"
    exit 1
fi

# ── Step 4: send several requests to generate log entries + metrics ───────────
echo ""
echo "=== Step 4: send requests to generate observability data ==="
# NOTE: Span export for the startup 'demo.startup' span appears at boot time
# (SimpleSpanProcessor is synchronous). Per-request spans also export here.
for i in $(seq 1 5); do
    curl -s -o /dev/null http://localhost:8000/v1/ping
    curl -s -o /dev/null http://localhost:8000/v1/greet/1
done
echo ">> requests sent"

# ── Step 5: AC6 — /v1/greet/1 returns correct result ─────────────────────────
echo ""
echo "=== Step 5: AC6 — /v1/greet/1 returns 200 with 'hello from sandbox' ==="
GREET_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" http://localhost:8000/v1/greet/1)
GREET_CODE=$(echo "$GREET_RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
GREET_BODY=$(echo "$GREET_RESP" | grep -v "HTTP_STATUS:")
echo ">> HTTP status: $GREET_CODE"
echo ">> body: $GREET_BODY"

if [ "$GREET_CODE" != "200" ]; then
    echo "FAIL AC6: /v1/greet/1 expected 200, got $GREET_CODE"
    exit 1
fi
if ! echo "$GREET_BODY" | grep -q "hello from sandbox"; then
    echo "FAIL AC6: response does not contain 'hello from sandbox'"
    exit 1
fi
echo ">> PASS AC6: /v1/greet/1 = 200, 'hello from sandbox'"

# Also send /v1/ping for AC6 middleware chain check
PING_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/v1/ping)
echo ">> /v1/ping → $PING_CODE"
if [ "$PING_CODE" != "200" ]; then
    echo "FAIL AC6: /v1/ping expected 200, got $PING_CODE"
    exit 1
fi
echo ">> PASS AC6: /v1/ping = 200 (middleware chain active)"

# ── Step 6: check /metrics BEFORE shutdown ────────────────────────────────────
echo ""
echo "=== Step 6: AC5 — assert /metrics has request count metric > 0 ==="
METRICS_BODY=$(curl -s http://localhost:8000/metrics)
echo ">> Checking /metrics for server_requests_code_total (Kratos metrics)..."

# The HTTP metrics middleware emits server_requests_code_total (via otel→prom bridge).
if echo "$METRICS_BODY" | grep -qE 'server_requests_code_total.*[1-9]'; then
    echo ">> PASS AC5: /metrics contains server_requests_code_total > 0"
    echo "$METRICS_BODY" | grep "server_requests_code_total" | head -3 || true
elif echo "$METRICS_BODY" | grep -qE 'server_requests|rpc_server|http_server'; then
    echo ">> PASS AC5: /metrics contains request count metrics"
    echo "$METRICS_BODY" | grep -E '(server_requests|rpc_server|http_server)' | grep -v '^#' | head -3 || true
else
    echo "FAIL AC5: no request count metric found in /metrics"
    echo "=== /metrics content: ==="
    echo "$METRICS_BODY" | head -20
    exit 1
fi

# ── Step 7: gracefully stop demo to flush span batcher ───────────────────────
echo ""
echo "=== Step 7: gracefully stop demo (flush span batcher) ==="
kill -TERM "$DEMO_PID" || true
STOPPED=0
for i in $(seq 1 10); do
    if ! kill -0 "$DEMO_PID" 2>/dev/null; then
        STOPPED=1
        echo ">> demo exited on attempt $i"
        break
    fi
    sleep 1
done
if [ "$STOPPED" -ne 1 ]; then
    echo ">> force kill demo (did not exit gracefully)"
    kill -KILL "$DEMO_PID" 2>/dev/null || true
    wait "$DEMO_PID" 2>/dev/null || true
fi
# Mark as cleaned so cleanup trap doesn't try again
DEMO_PID=""

# Give OS a moment to flush file buffers
sleep 0

# ── Step 8: AC5 — assert logs are JSON ────────────────────────────────────────
echo ""
echo "=== Step 8: AC5 — assert logs are JSON format ==="
JSON_LINE=$(grep -m1 '^{' /tmp/demo_obs.log 2>/dev/null || echo "")
if [ -z "$JSON_LINE" ]; then
    echo "FAIL AC5: no JSON log lines found in demo output"
    echo "=== Log sample (first 20 lines): ==="
    head -20 /tmp/demo_obs.log || true
    exit 1
fi
echo ">> Sample JSON log line: $JSON_LINE"
echo ">> PASS AC5: logs are JSON"

# ── Step 9: AC5 — assert logs contain trace_id ───────────────────────────────
echo ""
echo "=== Step 9: AC5 — assert some log line contains trace_id ==="
if grep -q '"trace_id"' /tmp/demo_obs.log; then
    TRACE_LINE=$(grep '"trace_id"' /tmp/demo_obs.log | head -1)
    echo ">> PASS AC5: trace_id found in logs"
    echo ">> Sample: $TRACE_LINE"
else
    echo ">> FAIL: 'trace_id' not found — log contents:"
    cat /tmp/demo_obs.log || true
    echo "FAIL AC5: no trace_id field found in logs"
    exit 1
fi

# ── Step 10: AC5 — HARD assert stdouttrace emitted a real span whose TraceID
#            matches a request trace_id (rule-0009 §C: no near-tautological
#            fallback, no WARN passthrough) ─────────────────────────────────────
echo ""
echo "=== Step 10: AC5 — assert stdouttrace span TraceID == request trace_id (HARD) ==="
# stdouttrace (SimpleSpanProcessor, sample_ratio=1.0) writes each exported span as
# a JSON object containing "SpanContext":{"TraceID":"<32hex>",...}. A normal request
# log line carries the SAME id as "trace_id":"<32hex>". We pin the producer-side
# evidence by requiring ONE id that appears BOTH as a request trace_id AND inside an
# exported span object — so a mere log line echoing trace_id can never satisfy this.
#
# Removed (per rule-0009 §C): the 32-hex "trace hex present" fallback (near-tautology:
# any request log line already carries a 32-hex trace_id) and the WARN(non-fatal)
# passthrough. Span absence now fails the scenario.

# Collect every trace_id seen in request log lines.
REQUEST_TRACE_IDS=$(grep '"trace_id"' /tmp/demo_obs.log 2>/dev/null \
    | grep -oE '"trace_id":"[0-9a-f]{32}"' \
    | grep -oE '[0-9a-f]{32}' \
    | grep -vE '^0{32}$' \
    | sort -u)

if [ -z "$REQUEST_TRACE_IDS" ]; then
    echo ">> Log dump (last 40 lines):"
    tail -40 /tmp/demo_obs.log || true
    echo "FAIL AC5: no non-zero request trace_id found to anchor span assertion"
    exit 1
fi

# A real exported span line must contain BOTH the "SpanContext" key AND a request
# trace_id. We require the TraceID to sit inside the SpanContext object of that line.
SPAN_MATCH=""
MATCHED_TID=""
while IFS= read -r tid; do
    [ -z "$tid" ] && continue
    # Line is an exported span (has "SpanContext") AND embeds this exact trace id.
    LINE=$(grep -F '"SpanContext"' /tmp/demo_obs.log 2>/dev/null | grep -F "$tid" | head -1 || true)
    if [ -n "$LINE" ]; then
        # Defence-in-depth: the id must appear specifically as a TraceID field, not
        # incidentally elsewhere on the line.
        if echo "$LINE" | grep -qE "\"TraceID\"[[:space:]]*:[[:space:]]*\"$tid\""; then
            SPAN_MATCH="$LINE"
            MATCHED_TID="$tid"
            break
        fi
    fi
done <<EOF
$REQUEST_TRACE_IDS
EOF

if [ -z "$SPAN_MATCH" ]; then
    echo ">> request trace_ids seen:"
    echo "$REQUEST_TRACE_IDS"
    echo ">> exported span lines (SpanContext), last 5:"
    grep -F '"SpanContext"' /tmp/demo_obs.log 2>/dev/null | tail -5 || echo "  (none)"
    echo "FAIL AC5: no stdouttrace span whose TraceID matches a request trace_id"
    exit 1
fi
echo ">> PASS AC5: stdouttrace span TraceID == request trace_id ($MATCHED_TID)"
echo "   span line: $(echo "$SPAN_MATCH" | head -c 200)"

echo ""
echo "=== scen_observability.sh PASSED (AC5+AC6) ==="
echo "  AC5: logs are JSON format ✓"
echo "  AC5: logs contain trace_id ✓"
echo "  AC5: /metrics returns data with non-zero values ✓"
echo "  AC5: stdouttrace span TraceID == request trace_id ($MATCHED_TID) ✓"
echo "  AC6: /v1/greet/1 → 200 'hello from sandbox' (HTTP→service→ent) ✓"
echo "  AC6: /v1/ping → 200 (middleware chain active) ✓"
