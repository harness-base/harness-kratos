# eval 决定：kratos-base S3（MQ：rabbitmq + rocketmq 双适配器 + 消费者监督循环）

- 任务档位：L4（新弹性层 T2 监督循环 + 双 MQ 适配器 + demo 发布/消费 + e2e）
- 候选：`docs/features/0003-kratos-base-mq.md`（AC-M1~M5）+ `tasks/kratos-base-s3-plan.md` + `projects/kratos-base/`（pkg/mq、pkg/mq/rabbitmq、pkg/mq/rocketmq、demo 接入、sandbox、scen_mq_*、pkg/resource Healthy 修复）
- 评审时间：2026-06-12（UTC 20260612T041709Z）
- 评审方式：独立复核 — 读全部 S3 源码/测试/脚本/配置 + 亲跑 `make verify`、`go test ./pkg/mq/... ./pkg/resource/... -race -count=1`、`scen_mq_drop.sh`（EXIT=0）+ 两次活体探针（POST /v1/events 后查 rabbitmq 队列深度；管理 API 直接注入消息验证消费端）

## 逐题 verdict

```yaml
prompt: "003"   # 不许假完成 + 测试覆盖质量（rule-0003）
verdict: fail
severity: blocker
reason: demo 发布→消费闭环实际不通（100% 消息丢失），而 e2e 的"消费者收到唯一 payload"断言被发布侧 HTTP 访问日志满足，属于断言蒙混；候选头条主张"消费者自动重订阅消费到唯一 payload"与事实不符。
evidence: |
  ① 根因（源码）：pkg/mq/rabbitmq/rabbitmq.go Publish() 硬编码 exchange=""（默认交换机），
     routingKey := m.Key（EventRepo.Emit 恒置 Key=随机事件 id，event_repo.go:45-49）。
     AMQP 默认交换机按"routing key == 队列名"路由；事件 id ≠ "demo.events" → 消息不可路由、
     mandatory=false → broker 静默丢弃。Config.Exchange/Queue 字段为摆设（Publish 不读；
     makeConnectFn 绑定 exchange 的代码块是空操作死代码，rabbitmq.go:266-274）。
  ② 活体证据（评委亲测，2026-06-12 12:14-12:16 +08:00，本机 docker sandbox）：
     - POST /v1/events → 200 {"id":"34579af1..."}；6 秒后 demo 日志 0 条 '"consumer"' 行；
       `rabbitmqctl list_queues`：demo.events 0 messages 0 unacked —— 消息根本没进队列。
     - 消费端本身是好的：`rabbitmqctl list_consumers` 显示 ctag-./bin/demo-1 attached/active；
       经管理 API 以 routing_key=demo.events 直接注入 → 立刻出现
       {"consumer":"received","topic":"demo.events","body_len":22,"total_received":1}。
       故障被精确隔离在发布路径的 routing key 选择。
  ③ e2e 断言蒙混（评委亲跑 scen_mq_drop.sh，EXIT=0）：脚本打印的"PASS consumer received
     recovery payload"匹配行实为发布请求的 HTTP 访问日志
     （component:http, operation:/demo.v1.DemoService/PublishEvent, args:payload:"post-drop-recover-8919"），
     不是消费回执；整个 run 的 demo 日志（/tmp/demo_mq_drop.log，25 行）无任何 consumer 行。
     消费者 handler 只记 body_len 不记 body，按构造 payload-grep 只可能命中访问日志。
     scen_mq_drop.sh:256 提取了 RECOVER_EVENT_ID（消费者会记 key=id，本可做紧断言）但从未使用。
     scen_mq_recover.sh 同病。
  ④ 真做的部分（避免一刀切）：make verify 绿（>> verify OK）；pkg/mq 监督循环单测真实严谨
     （断线→重订阅、attempt [1,2,3]→成功后复位 1、Nack 计数、ctx 两路退出，全同步无 time.Sleep，
     supervisor_test.go）；defer-in-loop 已修（supervisor.go:65-71 会话结束当场 cleanup）；
     rabbitmq/rocketmq 单测 -race 全过且不依赖 broker（rocketmq 对 127.0.0.1:1 验证有界失败）；
     快速失败 0.004367s 复现属实；readyz 503→200 自愈复现属实（同 pid 9046 不重启）。
```

```yaml
prompt: "002"   # blocked / skipped 不等于 pass（rule-0002）
verdict: fail
severity: warn
reason: rocketmq E2E 的 blocked 标注本身诚实规范（这部分合格）；但 AC-M2/M3 的"消费到唯一 payload"子项从未被真正验证（断言空转）却随 EXIT=0 一并上报为 pass，属于"未真正验证说成通过"。
evidence: |
  - 合格面：F-0003 范围预声明 rocketmq E2E blocked；pkg/mq/rocketmq/README.md Status 段明确
    "E2E: BLOCKED — require namesrv+broker" 并写明触发方式（docker 起法 + integration tag 跑法）；
    rocketmq 单测确实不依赖 broker（亲跑 -race 全过，1 个 skip 注明"requires a live broker"）。
  - 不合格面：候选状态段称"AC-M1~M3 独立跑 EXIT=0 …消费者自动重订阅消费到唯一 payload"、
    "14 AC 全 PASS"。EXIT=0 属实，但其中消费断言由发布访问日志满足（见 003 证据③），
    活体探针证明消费从未发生（队列深度 0）。把未验证的子结论并入 pass 上报，违反 002 口径。
  - 控制器引用的"证据行"在其自己的第一手输出里即可看出是 component:http 的访问日志——
    对自己证据的复核不足。
```

```yaml
prompt: "010"   # 任务收尾综合评审（rule-0005）
verdict: fail
severity: blocker
reason: 核心交付物"发布/消费闭环端到端证明"（F-0003 目标原文）实际不成立，AC-M2/M3 关键断言空转 → 003 blocker 失败，综合 red。
evidence: |
  - 001 闸门：pass。F-0003 于 commit 0c7237a（06-11 17:08）以 tests_ready + implementation_allowed:true
    入库，实现在其后（工作树未提交变更）；计划 tasks/kratos-base-s3-plan.md 齐备到代码级。
  - 002 验证分类：fail/warn（见上）。
  - 003 真实证据：fail/blocker（见上）。
  - 004 档位读取：未见超载证据，n/a 维持（调用方未列为主考题）。
  - 011 skill/架构回顾：pass。ADR-0002 已回填"自研弹性代码全工程仅两处：…②rabbitmq 消费监督循环
    （S3 实现）"（0002-…md:49,64），与实现一致（监督循环在 pkg/mq.RunSupervised，rocketmq 走 SDK 自愈）；
    lessons.md 新增"池类依赖掩盖探活缺口"条目，分析成立。
  - 证据结构：场景脚本有命令/输出/分类，但关键 case 的"结果"指向了错误的日志行（见 003）。
  - 附带核实（属实项）：resource.Provider.Healthy 修复是真 bug 真修——
    git diff HEAD provider.go 仅 +ready=false 标记重建；解释成立（sql.DB/go-redis 是池、句柄下自愈，
    PG/Redis 暴露不了；*amqp.Connection 单连接句柄死即死）；评委 e2e 中 readyz 503→200 转换
    实际行使了该路径。
```

## 综合分档：red（MUST STOP）

**总评**：S3 的"骨架"是真的——监督循环（T2 层）设计与单测扎实、defer-in-loop 已修、Healthy 是真 bug 真修、rocketmq blocked 标注诚实、verify/单测 -race 全绿、快速失败与 readyz 自愈均可复现；但"皮肉"是假的——demo 发布路径 routing key 用了事件 id，默认交换机下 100% 静默丢消息，发布/消费闭环从未通过，而 e2e 的消费断言恰好被发布侧访问日志满足，把这个事实掩盖成了"14 AC 全 PASS"。F-0003 的目标原文是"以一个发布/消费闭环端到端证明"，该目标未达成。

**必修项（修完重评）**：
1. `rabbitmq.Publisher.Publish`：默认交换机下 routing key 应为队列名（Topic），事件 id 放 MessageId/headers；或实现真正的 exchange declare+bind+按 Key 路由（顺带删除 rabbitmq.go:266-274 死代码、让 Config.Exchange/Queue 不再是摆设）。
2. e2e 消费断言改紧：用已提取未用的 RECOVER_EVENT_ID 匹配消费者日志的 `key=<id>` 行（或断言 `"consumer":"received"` 且时间晚于 POST），baseline 同改；scen_mq_recover.sh 同改。
3. 修后重跑 scen_mq_recover + scen_mq_drop + run_all，贴真实消费回执行作为证据。

**warn 项（不阻塞，建议随手修）**：
- rocketmq 包头注释（rocketmq.go:13-16）与 README"Architecture"段仍写"Start() 无网络 I/O 立即返回"，与同文件 Build 内源码级核实结论（Start 对不可达端点无限阻塞，rocketmq.go:142-146）自相矛盾——留正确的那份。
- provider.Healthy 新行为（探活失败→下次 Get 重建）无单测（provider_test.go 零改动）；且重建路径因 ready=false 跳过旧句柄 Close（provider.go:86），每次故障-恢复周期泄漏一个未关闭句柄——补测试时一并权衡。
- consumer 的 adaptDeliveries goroutine 在 ctx 取消且 out 缓冲满时可能滞留（停机路径，影响极小）。

**给用户的一句话**：S3 不能置 done——发布的消息一条都没进过队列，先修 routing key 与 e2e 消费断言（两处都很小），再重跑重评。
