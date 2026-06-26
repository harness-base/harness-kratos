# rabbitmq 就近规约（pkg/mq/rabbitmq）

本文件是本包红线；工程全局红线见 `../../../AGENTS.md`，MQ 弹性见 ADR-0002 §4。

## 红线
- **主队列必须 quorum 型 + x-delivery-limit**：经典队列不维护投递计数，`Nack(requeue=true)` 不累加 x-death，毒消息死循环永不进 DLQ。（rabbitmq.go:59-65；lessons 2026-06-24） <!-- rule: kratos/rabbitmq-quorum-dlq | sev: blocker -->
- **Nack 一律 requeue=true**：毒消息死信交给 broker 端 x-delivery-limit，禁止应用侧按 attempt 上限 `Nack(requeue=false)`（否则 quorum 死信成死代码、毒消息热循环）。（rabbitmq.go:632-638 + 单测） <!-- rule: kratos/rabbitmq-nack-requeue-true | sev: blocker -->
- **默认交换机 routing key 必须 == Topic（队列名）**：业务 id 放 MessageId 不放 Key；用 Key 当 routing key 会让消息 unroutable 被静默丢（S3 根因）。（rabbitmq.go:420-425,462-463；eval s3） <!-- rule: kratos/rabbitmq-routing-key-equals-topic | sev: blocker -->
- **Publish 前必须 declareTopicQueue**：默认交换机下队列没声明 = 消息静默丢（消费者还没连时发布"成功"却消失，无错、无队列深度、无恢复）。（rabbitmq.go:445-452；eval s3） <!-- rule: kratos/rabbitmq-declare-before-publish | sev: blocker -->
- **DLQ 必须先于主队列声明**：x-delivery-limit 超限要死信时 DLQ 不存在，则消息静默蒸发、无审计。（rabbitmq.go:544-551） <!-- rule: kratos/rabbitmq-dlq-declared-first | sev: blocker -->
- **Qos(prefetch) 必须在 Consume 之前**：否则 broker 把整个 backlog 一次推给单消费者，未确认堆积致 OOM。（rabbitmq.go:566-573） <!-- rule: kratos/rabbitmq-qos-before-consume | sev: warn -->

## 指针
- 监督/重连：`../supervisor.go`
- 单测（拓扑/毒消息/快失败）：`./rabbitmq_test.go`
- S3 评审复盘：`../../../../../docs/eval/task-reviews/20260612T041709Z-kratos-base-s3/`
