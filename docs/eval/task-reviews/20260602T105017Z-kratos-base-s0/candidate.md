# Feature 需求包：F-0001 kratos-base 地基与依赖弹性闭环（S0）

> 用本模板建需求包，登记进 `docs/features/index.yaml`。改业务代码前必须就绪（rule-0001）。
> 架构与弹性路线见 `docs/decisions/0002-kratos-base-architecture-and-resilience.md`。

## 背景 / 目标

搭起 `projects/kratos-base` 的骨架地基，并**以 PostgreSQL 为首个真实依赖，端到端证明"依赖懒加载 + 断线自愈 + 不宕服务 + 平滑恢复"闭环**。地基立住后，Redis / MQ / 服务发现 / 远程配置按**同一套标准模式**增量挂（S1-S6）。

## 范围

- 包含：
  - 工程脚手架：`go.mod`、`Makefile`、codegen 工具链（buf + protoc + kratos proto、ent、wire）、目录布局（`app/`、`api/`、`internal/pkg/`、`pkg/`、`configs/`、`deploy/`）。
  - 配置两段式：bootstrap（启动期，决定配置源）+ runtime（运行期，**file 源**，Watch 热更 + 坏配置校验拒绝并保留上一版）。
  - 日志（结构化 JSON + trace_id 字段）+ 错误模型（Kratos errors / apperr 规范）。
  - Kratos 中间件链：recovery / logging / tracing(otel) / metrics(prom) / ratelimit / circuitbreaker / metadata / validate。
  - 薄 provider 层 + 健康注册；`/healthz`(liveness)、`/readyz`(readiness)、`/metrics`。
  - `app/gateway`（HTTP→gRPC 统一网关）+ 1 个 demo 服务（gRPC，含一个依赖 PG 的接口 + 一个不依赖 PG 的接口）。
  - PostgreSQL via ent（自愈池 + 池参数可配 + atlas 迁移）接入薄 provider。
  - sandbox：docker-compose（postgres）用于弹性场景验收。
- 不包含（后续分片）：Redis、消息队列、服务发现、配置远程适配器（nacos/etcd/k8s configmap·secret）、降级/舱壁高级策略、生产级 TLS。

## 用户故事 / 验收目标

> 目的即验收：下列每条都可观察、可验证（不是"做完了"）。

- **AC1（启动期懒加载 / 不宕）**：PG **关闭**时服务进程正常启动并保持运行；`/healthz`=200，`/readyz`=503；不依赖 PG 的 demo 接口正常返回；依赖 PG 的接口返回**清晰错误**（非 panic、非进程退出）。
- **AC2（按需连 + 自愈）**：随后**启动 PG，不重启服务**；`/readyz` 在数秒内转 200；依赖 PG 的接口开始成功。
- **AC3（运行中断连 + 恢复）**：服务与 PG 均在用 →**杀掉 PG**→ 依赖 PG 的接口**快速失败**（熔断打开，不挂起）、`/readyz`=503、进程不退；**恢复 PG**→ 自动续上、`/readyz`=200、接口再次成功。
- **AC4（配置热更 + 坏配置回滚）**：改 runtime 配置项 →**不重启**生效；推入非法配置 → 被校验拒绝、保留上一版、服务照常。
- **AC5（可观测）**：日志为结构化 JSON 且带 trace_id；`/metrics` 暴露 Prometheus 指标；otel 产出 trace（本地 exporter）。
- **AC6（网关链路）**：HTTP 经 gateway 到达 demo gRPC 服务并正确返回；中间件链（recovery/log/trace/metric/breaker/ratelimit）生效。

## 影响面

- 被管工程：`projects/kratos-base`（新建）
- 接口 / 数据 / 权限：gateway HTTP 入口 + demo gRPC + `/healthz`·`/readyz`·`/metrics`；PG 一张 demo 表（ent schema + 迁移）；本片不涉权限/多租户。
- 受影响 skill（rule-0007）：feature-delivery（否，流程未变）、context-loading（否）。

## 测试设计

- API：
  - `ping_no_dep`：gateway→demo 不依赖 PG 接口 / 无前置 / 断言 200+payload / 反向 n/a / 持久化 n/a
  - `demo_read_db`：依赖 PG 的读接口 / 前置：PG up + 迁移 + 种子 / 权限·租户 n/a / 断言 200+数据 / 反向：PG down 返回结构化错误码 / 持久化：从 PG 读到
- 弹性 / E2E（具名 sandbox，docker-compose）：
  - `scen_boot_dep_down`（AC1）：先起服务后起 PG，断言 readiness 流转 + 接口行为
  - `scen_runtime_drop`（AC3）：用中杀 PG 再恢复，断言快速失败→自动恢复、进程存活
  - `scen_config_hot`（AC4）：改/坏配置，断言热更与回滚
  - 观测断言（AC5）：日志含 trace_id、`/metrics` 有计数、trace 产出
- 证据按 `workspace/verification.yaml` 路由记录（命令 / 时间 / 环境 / 结果 / 分类 / 对应 case id）。

## 状态

- delivery_status: verified
- implementation_allowed: true

> 实现见 `projects/kratos-base/`（T1–T9，子代理逐任务交付+双审）。**AC1–AC6 第一手验收通过**：`bash projects/kratos-base/test/resilience/run_all.sh`（从 harness 根运行）输出 ALL AC1-AC6 PASSED；`make -C projects/kratos-base verify` 绿。证据：`/v1/greet/1`→`{"id":"1","content":"hello from sandbox"}`（真 PG 经 ent）、杀库后熔断快速失败 0.002s、恢复后 /readyz 自动转绿、配置热更生效+坏配置回滚、/metrics 有计数、日志含 trace_id。
> 收尾过 eval（rule-0005）后置 done。
