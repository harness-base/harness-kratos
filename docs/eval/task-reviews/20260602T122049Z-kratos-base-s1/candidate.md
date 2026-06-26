# candidate 副本 — kratos-base S1（Redis 接入）

> 评审对象主文档：`docs/features/0002-kratos-base-redis.md`
> 配套：`tasks/kratos-base-s1-plan.md`、`projects/kratos-base/`（`pkg/redisx`、demo `/v1/hits` + redis 数据层 + readiness 含 redis、sandbox 加 redis、`test/resilience/scen_redis_*.sh`）
> ADR：`docs/decisions/0002-kratos-base-architecture-and-resilience.md`

## 主文档原文（docs/features/0002-kratos-base-redis.md）

# Feature 需求包：F-0002 kratos-base Redis 接入（S1）

> 沿 S0 懒加载脊柱（`pkg/resource`）挂 Redis。架构见 `docs/decisions/0002-kratos-base-architecture-and-resilience.md`。改业务代码前须就绪（rule-0001）。

## 背景 / 目标

给基座接 Redis（go-redis v9 UniversalClient，自适应单机/集群/分片），走**同一套 `resource.Provider` 懒加载 + 自愈**，并以一个 Redis 支撑的接口端到端证明"杀 Redis 不宕、恢复自愈"——与 S0 对 PostgreSQL 的闭环一致。验证"地基立对了，加适配器就是照脊柱插上去"。

## 范围

- 包含：
  - `pkg/redisx`：go-redis `UniversalClient`（addrs 多个=cluster、单个=single；预留 sentinel/TLS 配置位）封进 `resource.Provider`（懒加载、配置变更重建、`Health`=PING、`Fingerprint`）。
  - demo 接一个 **Redis 支撑的接口**（`/v1/hits`：`INCR` 计数）演示弹性。
  - Redis 健康注册进 readiness（`/readyz` 同时反映 PG + Redis）。
  - `conf`/`runtime.yaml` 加 redis 配置；sandbox docker-compose 加 redis 容器。
  - 弹性场景脚本（boot-redis-down、kill-redis/recover）。
- 不包含：真集群多节点 sandbox（cluster 仅验证配置兼容，不起多节点）；缓存旁路/降级高级策略；MQ / 服务发现 / 远程配置（S3+）。

## 用户故事 / 验收目标

- **AC-R1（启动期 Redis 宕、不崩）**：Redis 关 → 服务起、进程活；`/readyz`=503（redis 项 unhealthy）；`/v1/hits` 返结构化错误；不依赖 redis 的接口（ping、greet）正常。
- **AC-R2（按需连 + 自愈）**：起 Redis（不重启服务）→ `/readyz` 转 200、`/v1/hits` 开始工作、计数递增。
- **AC-R3（运行中断连 + 恢复）**：服务 + Redis 在用 → 杀 Redis → `/v1/hits` 快速失败、`/readyz`=503、进程活 → 恢复 Redis → 自动续上、`/readyz`=200。
- **AC-R4（多模式兼容）**：单机模式 sandbox 实测通过；集群/分片模式经配置（多 addrs）构造客户端不报错（UniversalClient 自适应）——单测覆盖"addrs→模式选择"。
- **AC-R5（PG 不回归）**：S0 的 AC1–AC6 仍全过（加 Redis 不破坏既有）。

## 状态

- delivery_status: verified
- implementation_allowed: true

> 实现见 `projects/kratos-base/`（S1-T1~T3）。**AC-R1~R3 + S0 回归 AC1~AC6 第一手验收通过**：`bash projects/kratos-base/test/resilience/run_all.sh` 全 PASS；三个 redis 场景单独从 harness 根跑也全过（CWD 无关）。证据：redis 宕→`/readyz` 503(含 redis)+`/v1/hits` 503+`/v1/greet` 仍 200；恢复→不重启自愈、`/v1/hits` 计数递增；杀 redis→~1s 快速失败（ECONNREFUSED，远小于 dial 超时）、进程活。
> 注：AC-R3 快速失败走 ECONNREFUSED（~1s）而非熔断开路（<0.2s）；想强化可把 redis DialTimeout 调小。收尾过 eval（rule-0005）后置 done。

---

（计划 `tasks/kratos-base-s1-plan.md`、实现源码不在此全文复制，评审时已逐文件独立核对，见 decision.md 证据段。）
