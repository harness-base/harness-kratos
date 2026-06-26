# rocketmq 就近规约（pkg/mq/rocketmq）

本文件是本包红线；工程全局红线见 `../../../AGENTS.md`，MQ 弹性见 ADR-0002 §4。

## 红线
- **golang.EnableSsl 进程级只写一次**：SDK 拨号在 detached goroutine 里无锁读它，多写 = data race；要不同 TLS 模式只能报错，绝不二次写。（rocketmq.go setEnableTLS:55-113） <!-- rule: kratos/rocketmq-enablessl-set-once | sev: blocker -->
- **Start 前必须 dialReachable 预检**：SDK 对不可达端点 Start 会在 inited-poll 里无限自旋；预检挡住注定失败的 goroutine。（rocketmq.go:115-142,284,726；eval s6 AC-MR3 有界失败 1.007s） <!-- rule: kratos/rocketmq-dial-reachable-precheck | sev: blocker -->
- **空 Topic 守卫必须在 breaker.Allow() 之前**：空 topic 是调用方 wiring bug（返 InvalidArgument/400），不是 broker 故障；放 breaker 后会污染健康判定。（rocketmq.go:461-463；单测有变异自证） <!-- rule: kratos/rocketmq-empty-topic-before-breaker | sev: blocker -->
- **Fingerprint 必须含 AwaitDuration 与 RequestTimeout**：两者 Build 时生效，漏了热更不触发重建、provider 返旧 client。（rocketmq.go:223-229；单测变异自证） <!-- rule: kratos/rocketmq-fingerprint-covers-timeouts | sev: blocker -->
- **Publish 的 Send 必须 goroutine+select 限时**：SDK v5 可能忽略 ctx 内部重试约 4×requestTimeout；不靠 SetRequestTimeout 单兜底，防 HTTP handler 被拖死。（rocketmq.go:488-511） <!-- rule: kratos/rocketmq-publish-bounded-ctx | sev: blocker -->
- **保留 pump(receiver) 可注入缝**：弹性核心（连错重建 / handler 错跳过 ack / 消息映射）要能用 fake receiver 单测，不依赖真 broker。（rocketmq.go:553-594 + export_test.go） <!-- rule: kratos/rocketmq-pump-seam | sev: warn -->

## 指针
- 单测：`./rocketmq_test.go`；可注入缝导出：`./export_test.go`
- e2e：`../../../test/resilience/scen_mq_rocketmq*.sh`
- S6 评审：`../../../../../docs/eval/task-reviews/20260623T0600Z-kratos-base-s6/`
