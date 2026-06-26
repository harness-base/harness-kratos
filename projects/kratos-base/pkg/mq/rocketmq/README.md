# pkg/mq/rocketmq

RocketMQ v5 adapter for `pkg/mq` using the Apache RocketMQ Go client
(`github.com/apache/rocketmq-clients/golang/v5`).

## Status

Unit tests: **pass** (no broker needed).
E2E tests: **pass** — exercised against a real RocketMQ 5.x namesrv + broker
(broker with `--enable-proxy`) in the sandbox. See `deploy/sandbox/rocketmq`,
`test/resilience/scen_mq_rocketmq*.sh`, and `run_all.sh` (AC-MR1~MR3). Delivered
in F-0006 (S6), which closed the e2e that F-0003/S3 had left blocked on a broker.

## Architecture

- **Publisher** wraps a `resource.Provider[golang.Producer]` with an SRE
  circuit-breaker.  Note (source-verified, v5.1.3): `producer.Start()` blocks
  until the gRPC telemetry settings exchange completes — against an
  unreachable endpoint it blocks indefinitely, so the adapter bounds it with
  `RequestTimeout` (goroutine + GracefulStop on timeout).
- **Consumer** uses `SimpleConsumer` (pull + ack).  The SDK self-heals
  connections internally; the loop backs off only on `Receive` errors using
  `mq.DefaultBackoff`.

## Config

```go
rocketmq.Config{
    Endpoint:       "localhost:8081",     // namesrv or proxy address
    AccessKey:      "ak",                 // empty = no auth
    SecretKey:      "sk",
    ConsumerGroup:  "my-group",           // required for Consumer
    AwaitDuration:  5 * time.Second,      // long-poll window; default 5 s
    RequestTimeout: 3 * time.Second,      // per-RPC timeout; default 3 s
}
```

## Running E2E

The e2e runs against the sandbox broker, not via a separate integration build
tag. From the harness root:

```bash
# brings up the seven-service sandbox, including rmqnamesrv + rmqbroker
# (apache/rocketmq 5.x, broker with --enable-proxy on 8081) and pre-creates the
# demo-events topic
make -C projects/kratos-base sandbox-up

# AC-MR1~MR3: publish→consume e2e + boot-down self-heal + runtime-drop bounded
# failure (these are also part of run_all.sh)
bash projects/kratos-base/test/resilience/scen_mq_rocketmq.sh
bash projects/kratos-base/test/resilience/scen_mq_rocketmq_boot_down.sh
bash projects/kratos-base/test/resilience/scen_mq_rocketmq_drop.sh

make -C projects/kratos-base sandbox-down
```

The demo wires rocketmq via `configs/bootstrap.rocketmq-sandbox.yaml`
(`mq.kind=rocketmq`, `endpoint=127.0.0.1:8081`).

## SDK log noise

By default the SDK writes zap logs to `~/logs/rocketmqlogs/rocketmq_client_go.log`.
In tests this is redirected to `t.TempDir()` via:

```go
t.Setenv(golang.CLIENT_LOG_ROOT, t.TempDir())
golang.ResetLogger()
```

In production set `ROCKETMQ_CLIENT_LOGROOT` or `rocketmq.client.logRoot` env
var (per SDK constants) to a managed log directory.
