# Candidate — kratos-base S6（rocketmq 本机轻量环境 + e2e 销账 + 连带弹性修正）

档位：L4（验证/补全片）。

## 主文档 / 计划
- `docs/features/0006-kratos-base-rocketmq-e2e.md`（AC-MR1~3、AC-REG；delivery_status: tests_ready）
- `tasks/kratos-base-s6-plan.md`

## 实现
- rocketmq 2 容器：`deploy/sandbox/docker-compose.yaml`（rmqnamesrv + rmqbroker，HEAP_OPTS 压堆 256m/512m，proxy 8081）+ `deploy/sandbox/rocketmq/broker.conf`（brokerIP1=127.0.0.1、autoCreateTopic/SubGroup）+ Makefile `sandbox-up` 等双 healthy + 预建 topic demo-events。
- 配置：`configs/bootstrap.rocketmq-sandbox.yaml` + `configs/runtime.rocketmq-sandbox.yaml`（kind=rocketmq、endpoint=127.0.0.1:8081、request_timeout=10s、await=5s）。
- 适配器：`pkg/mq/rocketmq/rocketmq.go`（Publish goroutine+select 限时；Consumer maxReceiveErrors=3 后重建 SimpleConsumer 外层循环）。
- 消费回执来源：`app/demo/internal/data/consumer_runner.go`（handler 打 `"consumer":"received"` + `"key":"<event id>"`，slog JSON）。
- 事件 id：`app/demo/internal/data/event_repo.go`（crypto/rand 32-hex，作 mq.Message.Key）。
- 三场景：`test/resilience/scen_mq_rocketmq{,_boot_down,_drop}.sh`；纳入 `run_all.sh`（AC-MR1~MR3 + S0~S5 + AC-CF，共 23 项矩阵）。

## 连带改动
- `app/demo/internal/server/http.go`：/readyz 改用独立 15s ctx（绕开 kratos 默认 1s 请求超时，让 mq 探活跑完）。
- `pkg/pgxpool/pool.go` + `pkg/redisx/client.go`：Open(ctx, cfg) 吃调用方 ctx（committed bff9491）。
- `test/resilience/scen_cc_runtime_down.sh`：CR1-b 从 redis-flip 重写为"恢复后推非法配置（空 grpc.addr）→ confcenter 新增 `retaining previous config`（BEFORE/AFTER 计数对比）"。
- `docs/features/0005-...md` + `tasks/lessons.md`：如实记录旧 redis-flip 证法因 readyz 超时改动失效、改为 ctx 无关硬证据。
