# Feature 需求包：F-0003 kratos-base 消息队列接入（S3：rabbitmq + rocketmq）

> 沿弹性脊柱挂 MQ。**首次落地设计中的"T2 监督循环"层**（消费者=后台常驻+自重连，非 `Resource[T]` 句柄）。架构见 `docs/decisions/0002-...`。改业务代码前须就绪（rule-0001）。

## 背景 / 目标

给基座接消息队列，**rabbitmq + rocketmq 双适配器**，统一接口、`bootstrap.yaml` 选后端（和配置/发现适配器一个套路）。生产者走 `resource.Provider`（懒加载+自愈，连接级）；**消费者走监督循环**（断线自己重订阅、不重启服务）。以一个发布/消费闭环端到端证明"杀 MQ 不宕、恢复自愈"。

## 范围

- 包含：
  - `pkg/mq`：适配器无关接口 `Publisher`（Publish/Close）+ `Consumer`（Subscribe 启动监督消费循环/Close）+ `Message`。
  - `pkg/mq/rabbitmq`：基于 `amqp091-go`（**裸 AMQP 不自愈**）——连接封进 `resource.Provider`；Publisher 经 provider 取连接发布、sre 熔断；**Consumer = 监督 goroutine**（`Get()` 连接→声明队列→消费 delivery→`NotifyClose`/error 触发退避重连重订阅）。
  - `pkg/mq/rocketmq`：基于 `rocketmq-clients/golang/v5`（push consumer 自带重连/rebalance）——Producer/Consumer 包成同一接口。
  - bootstrap 选 `mq.kind: rabbitmq|rocketmq`；conf/runtime 加 mq 配置。
  - demo 接入：发布端点（如 `POST /v1/events` 或 `/v1/hits` 触发发事件）+ 后台消费者（处理消息：计数/日志）；MQ 生产者健康进 readiness。
  - sandbox 加 **rabbitmq** 容器；rabbitmq 弹性场景（boot-down / kill-recover）。
- 不包含：
  - **rocketmq E2E**（需 nameserver+broker，沙箱重）——S3 只**构建适配器 + 单测**，E2E **预声明 blocked**，待用户提供 broker 配置再验（rule-0002 如实标注，不冒充 pass）。
  - 事务消息 / 死信高级策略 / 顺序消费（后续）。

## 用户故事 / 验收目标

- **AC-M1（启动期 MQ 宕、不崩）**：MQ 关 → 服务起、进程活；`/readyz`=503（mq 项 unhealthy）；发布端点返结构化错误；不依赖 MQ 的接口（ping/greet/hits 视依赖）正常。
- **AC-M2（按需连 + 自愈）**：起 MQ（不重启服务）→ `/readyz` 转 200、发布成功、**消费者自动连上并处理**消息。
- **AC-M3（运行中断连 + 恢复）**：服务+MQ 在用 → 杀 MQ → 发布快速失败、**消费者监督循环退避重试**（日志可见）、`/readyz`=503、进程活 → 恢复 MQ → 发布恢复、**消费者自动重订阅续上**（全程不重启）。
- **AC-M4（双适配器）**：rabbitmq + rocketmq 适配器**都编译 + 单测过**；`bootstrap.yaml` 切后端；rabbitmq **E2E 实测通过**；rocketmq **E2E 标 blocked（待 broker 配置）**，不冒充。
- **AC-M5（回归）**：S0（AC1–6）+ S1（AC-R1–3）仍全过。

## 影响面

- 被管工程 `projects/kratos-base`：新增 `pkg/mq`(+rabbitmq/rocketmq)；demo 加发布/消费；conf/runtime/bootstrap/sandbox 扩展；readiness 加 mq。
- 受影响 skill（rule-0007）：feature-delivery/context-loading 无需更新。**注意**：本片首次落"监督循环"层，收尾回顾 ADR-0002 的弹性分层描述是否需补充。

## 测试设计

- 单测：mq 接口契约；rabbitmq（连接 provider 懒加载/不可达→error、Publisher 熔断、Consumer 监督循环用假连接验证"断→退避→重订阅"——可注入的 reconnect，不依赖真 broker、无 time.Sleep）；rocketmq（配置→client 构造、Publisher/Consumer 包装，不可达→error）。
- E2E（sandbox 加 rabbitmq）：`scen_mq_boot_down`(AC-M1)、`scen_mq_recover`(AC-M2)、`scen_mq_drop`(AC-M3，含消费者重订阅断言)；回归 `run_all.sh`(AC-M5)。
- rocketmq E2E：blocked，文档写明触发方式（用户给 broker 配置后跑）。
- 证据按 `workspace/verification.yaml` 记录。

## 状态

- delivery_status: verified
- implementation_allowed: true

> 实现见 `projects/kratos-base/`（S3-T1~T4）。**第一手验收通过**：AC-M1~M3 独立跑 EXIT=0（MQ 宕→/readyz 503(含 mq)+/v1/events 503+pg/redis 接口不受影响；恢复→不重启发布 200+**消费者自动重订阅消费到唯一 payload**；杀 rabbitmq→0.005s 快速失败→恢复续消费，全程同 pid）；全量 run_all **14 AC 全 PASS**（S0/S1/S4 零回归）。
> 关键修复：S3 e2e 暴露 `resource.Provider.Healthy()` 探活失败不标记重建的缺口（池类依赖掩盖、单连接句柄炸出），已修+记 lessons。监督循环（T2 弹性层）已落地：`pkg/mq.RunSupervised`（断线重订阅、退避复位、Nack、ctx 干净退出，全部表驱动单测）。
> 如实 blocked：rocketmq E2E（需 namesrv+broker，本机未起；适配器+单测已过，SDK Start 对不可达端点无限阻塞已用超时兜住——见 `pkg/mq/rocketmq/README.md` 跑法）。
> 收尾过 eval（rule-0005）后置 done。
