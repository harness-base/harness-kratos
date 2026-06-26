# Feature 需求包：F-0006 rocketmq 本机轻量环境 + e2e 销账（S6）

> 验证/补全片：把 S3 留下的 **rocketmq e2e（当时 blocked 等 broker）** 用本机最轻量完整环境跑通。适配器+单测 S3 已写，本片只补"真 broker + 端到端验收"。设计依据：ADR-0002（MQ 监督循环 T2 弹性）。改业务代码前须就绪（rule-0001）。

## 背景 / 目标

S3 的 rocketmq 适配器（v5 SDK 包装：Publisher/Consumer/Start 受 RequestTimeout 约束）+ 单测已落，但 E2E 标 blocked（无 broker）。实测 `apache/rocketmq:5.3.2` **有 arm64 镜像**，本机 docker 可起。本片：起最轻量但完整的 rocketmq 5.x，跑通 publish→consume + 启动期/运行期/恢复三态弹性，对齐 rabbitmq 的 AC-M1~M3。

## 范围

- 包含：
  - sandbox 加 rocketmq **2 容器**（最轻量完整）：`rmqnamesrv`（mqnamesrv，9876）+ `rmqbroker`（mqbroker `--enable-proxy`，broker 进程内嵌 proxy，10911+8081）。**JVM 堆压到 ~256m/512m**（broker 默认要 8G，不压起不来）。broker.conf：`brokerIP1=127.0.0.1`、`autoCreateTopicEnable=true`、`autoCreateSubscriptionGroup=true`（省去 mqadmin 建 topic/group）。
  - bootstrap：`configs/bootstrap.rocketmq-sandbox.yaml`（mq.kind=rocketmq、endpoint=127.0.0.1:8081、topic、consumer_group）。
  - e2e（对齐 AC-M 形状，rule-0009 id 锚定回执）：
    - **AC-MR1** publish→consume 闭环：HTTP 触发发布事件 → 消费者收到，断言锚定**消费方结构化回执 + 业务 id**（不裸串、不取访问日志回显）。
    - **AC-MR2** 启动期 rocketmq 宕：demo 照起、/v1/ping 等本地接口 200、readyz 反映 mq 不可用；rocketmq 起后**自愈**（发布→消费续上，不重启进程）。
    - **AC-MR3** 运行期断：发布**有界失败**（≤request_timeout，goroutine+select 兜住，不被 v5 SDK ~40s 重试拖住）不崩 → broker 恢复 → 监督循环重建 SimpleConsumer、续消费。
  - 纳入 `run_all.sh`；S0~S5 全回归。
- 不包含：rocketmq 集群/多 broker、ACL、事务消息/顺序消息（地基不需要）；k8s（另片，需集群）。

## 用户故事 / 验收目标

- **AC-MR1**：mq.kind=rocketmq 下发布事件 → 消费方收到，回执含事件 id（产出方证据）。
- **AC-MR2**：rocketmq 不可达时 demo 不崩、本地接口照常、readyz 反映；恢复后发布→消费自动续上。
- **AC-MR3**：运行期 broker 断 → 不崩、**有界失败**（≤request_timeout，非 rabbitmq 那种瞬断；rocketmq v5 SDK 自带 gRPC 重试，靠 goroutine+select 限时兜底）→ 恢复后监督循环重建 SimpleConsumer 续消费。
- **AC-REG**：S0~S5（含 rabbitmq AC-M1~M3、nacos/etcd 全套、AC-CF）零回归。

## 影响面

- 被管工程 `projects/kratos-base`：sandbox 加 rocketmq 2 容器 + broker.conf；新增 rocketmq bootstrap + e2e 场景；可能微调 rocketmq 适配器（若真 broker 暴露 topic/group/超时暗坑）。
- 受影响 skill（rule-0007）：无需更新。

## 测试设计

- E2E（容器，本机 docker，含 rule-0009 断言锚定）：`scen_mq_rocketmq.sh`（AC-MR1，发布→消费 id 锚定）、`scen_mq_rocketmq_boot_down.sh`（AC-MR2）、`scen_mq_rocketmq_drop.sh`（AC-MR3）——或合并参数化，复用 rabbitmq 场景结构。
- 断言：消费回执锚定消费方结构化字段 + 事件 id；启动/运行期看 readyz + 本地接口码；恢复看真实消费续上。
- 回归 `run_all.sh`；blocked 项如实（k8s）。

## 状态

- delivery_status: done
- implementation_allowed: true

> 实现见 `projects/kratos-base/`（S6-T1/T2 + 有界失败修复轮）。**第一手 + 独立 eval 双验**：AC-MR1（publish→consume，消费方 id 锚定回执）、AC-MR2（启动期宕→不崩→自愈）、AC-MR3（运行期断→有界失败 ~1s→监督循环重建 SimpleConsumer 续消费）全 EXIT=0；全量 run_all 24 AC PASS（S0~S6）。
> 关键修复（验证中挖出）：①`Publisher.Publish` 用 goroutine+select 把 v5 SDK 的 Send 限在 request_timeout 内（原来 SDK 不理 ctx、断开发布挂 ~40s；现 ~1s 有界失败）；②`init()` 把 SDK 日志重定向到 `/tmp/rocketmq-logs`（macOS /logs 只读）；③Consumer 连续 `maxReceiveErrors` 次 Receive 失败后重建 SimpleConsumer（SDK gRPC 流运行期断后不自愈）+ `consumerLog` 重连日志。连带把 `/readyz` 改 15s 探测 ctx、`pgxpool/redisx.Open` 改吃调用方 ctx、`scen_cc_runtime_down` 的 CR1-b 从脆弱的 redis-flip 重写为 confcenter `retaining previous config` 计数对比（详见 F-0005 注 + lessons 2026-06-23）。
> eval：**green**（`docs/eval/task-reviews/20260623T0600Z-kratos-base-s6/`，零 blocker/warn）。
> blocked 如实：k8s e2e（无集群，另片）。
> 已知有界取舍：被遗弃的 Send goroutine 在持续高流量 outage 下有界累积（≈速率×40s 自然 drain），已在代码注明，可接受。
