# 实施计划：kratos-base S3（消息队列：rabbitmq + rocketmq）

> 临时工作产物。设计源：需求包 `docs/features/0003-kratos-base-mq.md` + ADR-0002。
> **首次落"T2 监督循环"层**：生产者=`Resource[T]`，消费者=后台监督 goroutine（断了自重订阅）。

## 1. 概述

- 背景：S0/S1 证明了 `Resource[T]`（T1 层，PG/Redis）。MQ 引入消费者这个 `Get()→句柄` 装不下的形状，用监督循环（T2 层）解决。
- 关键约束：rabbitmq 用 `amqp091-go`（**裸 AMQP 不自愈，连接级走 resource.Provider，消费走监督循环**）；rocketmq 用 `rocketmq-clients/golang/v5`（push consumer 自带重连，包成同一接口）。版本 amqp091-go v1.11.0 / rocketmq-clients golang v5.1.3（ADR 矩阵，已预热）。bootstrap 选后端。
- 产出即验收：AC-M1~M5（见需求包）。**rocketmq E2E 预声明 blocked**（沙箱重，待 broker 配置）。

## 2. 文件结构（S3 增量）

```
projects/kratos-base/
├── pkg/mq/
│   ├── mq.go              接口：Publisher/Consumer/Message/Handler                    ← 新
│   ├── rabbitmq/          conn(resource.Provider[*amqp.Connection]) + publisher + consumer(监督循环) + *_test.go
│   └── rocketmq/          producer + push-consumer 包装 + *_test.go（E2E 后续）
├── app/demo/internal/
│   ├── conf/conf.go       加 MQ 配置 + SelectMQ
│   ├── biz/、data/、service/  发布端点 + 消费处理（计数）
│   └── cmd/{main.go,wire.go}  装配 mq publisher + 启动 consumer（App.BeforeStart）+ readiness 加 mq
├── pkg/bootstrap 或 conf  mq.kind: rabbitmq|rocketmq 选择
├── configs/runtime*.yaml  加 data.mq.*
├── deploy/sandbox/        docker-compose 加 rabbitmq:3-management
└── test/resilience/       scen_mq_*.sh + run_all 纳入
```

## 3. 步骤（到代码级）

### S3-T1 `pkg/mq` 接口 + rabbitmq 适配器（核心，含监督循环）
- `pkg/mq/mq.go`：
  ```go
  type Message struct { Topic, Key string; Body []byte; Headers map[string]string }
  type Handler func(ctx context.Context, m Message) error
  type Publisher interface { Publish(ctx context.Context, m Message) error; Close() error }
  type Consumer  interface { Subscribe(ctx context.Context, topic string, h Handler) error; Close() error } // Subscribe 阻塞直到 ctx done
  ```
- `pkg/mq/rabbitmq`：
  - `Config{ URL string; Exchange string; ... }`、`Fingerprint()`。连接封 `resource.Provider[*amqp091.Connection]`（Build=Dial+确认 open；Close=conn.Close；Health=!conn.IsClosed()）。
  - `Publisher`：`Publish` 经 provider `Get()` 取连接→开 channel→`PublishWithContext`；sre 熔断包；失败→`errs.DBUnavailable`（依赖不可用 503）。
  - **`Consumer`（监督循环）**：`Subscribe(ctx, topic, h)`：`for { conn := provider.Get(ctx)（拿不到→退避 continue）; ch := conn.Channel(); 声明 queue; deliveries := ch.Consume(...); for d := range deliveries { h(ctx, msg); d.Ack() }; // deliveries 关=断线 → 退避后回到 for 顶重连重订阅; select ctx.Done()→return }`。退避用可注入的 backoff（测试不 sleep）。
- 单测：Config Fingerprint；不可达 URL→Publisher.Publish 返 error（短超时不 hang）；**监督循环用可注入的假连接源**验证"deliveries 关闭→重新 Get()→重订阅"（计数断言重连发生，channel 同步，**无 time.Sleep**）；Handler 错误处理（nack/重投策略）。
- 验证：`go test ./pkg/mq/... -race`、`make verify` 绿。go.mod 加 amqp091-go v1.11.0（已预热）。

### S3-T2 rocketmq 适配器（构建+单测，E2E 后续）
- `pkg/mq/rocketmq`：`Config`；`Publisher` 包 `rmq_client.Producer`（v5）；`Consumer` 包 push consumer（`SimpleConsumer` 或 `PushConsumer`，自带重连），把 v5 回调桥接到 `mq.Handler`。不可达→构造/收发返 error。
- 单测：Config→client options 映射；Publisher/Consumer 接口契约（用 v5 的可构造但不连真 broker 的部分，或对不可达 endpoint 断言 error）。**E2E 标 blocked**：在 `pkg/mq/rocketmq/README.md` 写明"需 nameserver+broker，用户给配置后跑 scen_rocketmq_*"。
- 验证：`go test ./pkg/mq/rocketmq/... -race`、`make verify` 绿。go.mod 加 rocketmq v5.1.3（已预热）。

### S3-T3 接入 demo（发布端点 + 消费者 + bootstrap 选择 + readiness）
- conf：`Data.MQ{ Kind string("rabbitmq"|"rocketmq"); Rabbitmq RabbitmqCfg; Rocketmq RocketmqCfg }`（json+yaml tag）；`SelectMQ(cfg)→mq.Publisher/Consumer 工厂`（按 kind 选适配器）。runtime/sandbox 配置加 `data.mq`。
- proto：加 `Publish(PublishRequest{payload})→PublishReply` [POST /v1/events]。
- biz/data/service：`EventRepo.Emit(ctx, payload)`→publisher.Publish（熔断+errs 映射）；service 实现 Publish。后台消费者：`ConsumerRunner`，在 `kratos.App` 的 `BeforeStart` 起 `go consumer.Subscribe(ctx, topic, handler)`，handler 处理消息（计数到内存/redis + 结构化日志）；`AfterStop` 关闭。
- wire：装配 publisher + consumer runner；`provideRegistry` 加注册 mq 生产者健康（`r.Register("mq", mqHealthy)`）。
- 单测：event repo（不可达→错误、熔断）；service Publish。
- 验证：`make wire`、`make verify` 绿；冒烟（无 mq）`/readyz`=503 含 mq、`/v1/events` 503、其它正常。

### S3-T4 sandbox rabbitmq + 弹性验收 + 回归
- `deploy/sandbox/docker-compose.yaml` 加 `rabbitmq:3-management`（healthcheck `rabbitmq-diagnostics -q ping`）；`sandbox-up` 等三者（pg+redis+rabbitmq）healthy。sandbox 配置 mq.kind=rabbitmq 指向它。
- `test/resilience/`：`scen_mq_boot_down.sh`(AC-M1)、`scen_mq_recover.sh`(AC-M2)、`scen_mq_drop.sh`(AC-M3：杀 rabbitmq→发布快速失败+/readyz 503+**消费者日志显示重连退避**+进程活→恢复→发布+消费续上)。纳入 `run_all.sh`。
- **回归**：S0(AC1-6)+S1(AC-R1-3) 仍全过（sandbox-up 现在带 rabbitmq，pg/redis 场景的 /readyz=200 需三者健康）。脚本 CWD 无关。
- 更新 `workspace/verification.yaml`。
- 验证：亲跑 `run_all.sh`（PG+Redis+MQ 全场景）E2E_EXIT=0；`make verify` 绿；无残留容器。

## 4. 验证 runbook（映射 AC-M）

- [ ] `make -C projects/kratos-base verify`（build+vet+lint+test -race 绿，含 rabbitmq+rocketmq 单测）
- [ ] **AC-M1** mq 关→起服务→/readyz 503(mq)、/v1/events 5xx、其它正常、进程活
- [ ] **AC-M2** 起 rabbitmq(不重启)→/readyz 200、发布成功、消费者连上处理
- [ ] **AC-M3** 杀 rabbitmq→发布快速失败+/readyz 503+消费者监督循环重试日志+进程活→恢复→发布+消费续上
- [ ] **AC-M4** rabbitmq+rocketmq 单测过；rabbitmq E2E 过；**rocketmq E2E blocked（如实标注）**
- [ ] **AC-M5** S0+S1 场景回归全 PASS
- [ ] 证据按 `workspace/verification.yaml` 记录

## 5. 失败模式与回滚

- rocketmq v5 依赖树大/构造复杂：先确保编译+不可达 error，E2E 留 blocked，别硬凑。
- 监督循环测试别用 time.Sleep；退避可注入、channel 同步。
- 回滚：S3 全在工程内新增 + verification.yaml；删 pkg/mq + 还原 demo/sandbox 即回滚，不影响 S0/S1。

## 6. 受影响 skill / 文档（rule-0007）

- feature-delivery / context-loading：无需更新。
- **ADR-0002**：本片首次落"T2 监督循环"，收尾确认 ADR 弹性分层描述与实现一致（如有出入补一句）。
- 收尾：L2+ 跑 eval（rule-0005）。
