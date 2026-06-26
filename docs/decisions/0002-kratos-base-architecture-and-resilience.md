---
title: ADR-0002 Kratos 基础骨架技术选型与依赖弹性策略
status: accepted
date: 2026-06-02
last_updated: 2026-06-11
source_files: []
related_docs:
  - ../features/0001-kratos-base-spine.md
---

# ADR-0002：Kratos 基础骨架技术选型与依赖弹性策略

## 背景

首个被管工程挂入 `projects/kratos-base`：从 0 搭 Kratos 微服务基础骨架。

核心诉求（用户）：所有外部依赖（配置中心、服务发现、PostgreSQL、Redis、消息队列）必须**懒加载**——启动时依赖未就绪不报错、不退出；首次使用才连；连不上则该操作清晰报错但**服务不宕**；依赖恢复后**无需重启、自动平滑续上**；依赖故障期间**快速失败不雪崩**。

参考工程 `z-mate-control/projects/backend-service` 只对 PostgreSQL 用自研 `dynresource` 做了懒加载，且抽象做大、其余依赖（Redis/gRPC/MQ）启动即连。需为本工程定技术选型与弹性落地路线。

## 决策

**弹性不靠自研大框架，靠标准分层组合**（业界惯例）：

1. **工程形态**：monorepo + 多应用（`app/gateway` 统一网关 + `app/<svc>` 业务服务）+ 共享 `pkg/`、`internal/pkg/`。

2. **技术选型**（确切版本与兼容性见下文『版本与兼容矩阵』）：
   - Go 1.26.x；Kratos v2.9.2（gRPC + HTTP）
   - DI：**wire**（Kratos 官方脚手架标准，编译期注入；代价是多一道 codegen → 封装成 `make wire`/`go generate`，改装配后重跑、生成物入库）
   - codegen：buf + protoc + kratos proto 插件；ORM：**ent**（+ atlas 迁移）
   - Redis：go-redis v9（UniversalClient，自适应单机/集群/分片）
   - 可观测：OpenTelemetry（trace）+ Prometheus（metrics）

3. **依赖生命周期 = 标准件组合**：
   - **自愈连接池**：`database/sql`/pgxpool、go-redis、gRPC 客户端本身就是惰性连接 + 自动重连的池；落地 = **配置对池参数/超时/重试**，不自己写重连。
   - **入站 Kratos 中间件链**：recovery / tracing(otel) / logging / metadata / metrics(prom) / ratelimit(BBR) / validate。**熔断不在入站中间件**：本工程的熔断是**数据/客户端层** `sre.Breaker`（包住 PG/Redis/MQ 调用，见策略表"breaker（client 侧包裹）"与 `app/demo/internal/data` repo），不是入站请求中间件；`circuitbreaker` 入站中间件仅在策略表中作为**未来服务间 gRPC 调用**的选项列出，本片不挂。
   - **k8s 探针**：liveness = 进程存活（依赖故障**不**判失败、不重启）；readiness = 依赖健康聚合（依赖挂→摘流量，恢复→自动回流）。这是"不宕服务、平滑恢复"的落地点。
   - **薄 provider 层**（自研仅此一层）：只补库不管的两件事——(a) 运行期配置变更时**原子换池**；(b) **监督无自愈能力的客户端**（主要是 MQ 裸消费者的重连订阅循环）。`Get()` 返回**原生 client**，不统一各中间件用法。
   - **服务发现 / 配置热更**：以 Kratos `registry` / `config` + Watch 接口为**插槽**，`bootstrap.yaml` 选后端；后端实现优先用 contrib，contrib 不稳的就对原生 SDK 自写薄适配器（contrib 仅 commit 伪版本、v3 未发布，见版本矩阵）。

4. **逐依赖弹性策略表**（每依赖：懒加载/重连从哪来 / 关键参数 / 挂哪个中间件 / 是否进 readiness）：

   | 依赖 | 懒加载+重连来源 | 关键参数/配置 | Kratos 中间件 | 进 readiness |
   |---|---|---|---|---|
   | PostgreSQL/ent | `sql.DB`/pgxpool 自愈池 | MaxOpen/Idle、ConnMaxLifetime、连接/语句超时 | breaker（client 侧包裹）| 是（ping）|
   | Redis | go-redis 自愈池 | PoolSize、MaxRetries、Dial/Read/Write 超时、集群路由 | breaker | 是（ping）|
   | gRPC 服务间 | grpc 惰性连接 + 自动重连 | keepalive、退避、超时（可选 mesh）| circuitbreaker + ratelimit | 否（按需）|
   | MQ 生产 | 客户端连接（视库）| 确认/超时、重试（事务/outbox 二期）| 非 RPC，n/a | 可选 |
   | MQ 消费 | **自研监督循环**（裸 AMQP 不自愈）| 重连退避、prefetch、幂等、死信 | 非 RPC，n/a | 可选 |
   | 配置中心 | Kratos config + Watch | 源适配器、校验、坏配置回滚 | n/a | 启动期就绪即可 |
   | 服务发现 | Kratos registry + Watch（contrib）| 注册/心跳 TTL、选择器 | selector/balancer | 否 |

   > **弹性模型澄清（2026-06-11 补，与用户对齐后定稿）**——上表三类依赖的弹性模型不同，**不混用**：
   > - **数据面**（PG / Redis / MQ 连接）：懒加载 + 断线自愈 + 熔断，`resource.Provider` 管"配置变更→换池"。
   > - **配置中心**：**急加载，不走懒加载**（没有配置 app 无法装配）。启动 `Load()` 失败默认 fail-fast；远程源的兜底是**本地快照缓存**（nacos SDK 自带；etcd/file 无，需要再补）。运行期弹性 = Watch 热更 + 坏配置校验回滚（S0 已实现）；watch 断线由客户端 SDK 自愈。
   > - **注册中心**：解析按需 + watch；**服务注册失败应非致命 + 后台重试**（不阻断启动）；注册中心挂掉只降级跨服务调用，不影响本地依赖接口。
   >
   > **共享接入（S4 实现回填，按实际修正）**："共享"兑现在**配置节 + `pkg/backends` 构造层**（连接细节仅在 backends），而非单一 client 对象：
   > - **etcd**：contrib 两侧都收注入 client，但本工程**有意**给两角色各建一个实例——配置源用 `NewEtcdClient`（带探活，急加载 fail-fast），注册用 `NewEtcdClientLazy`（懒连接，非致命）——语义不同，共享同一 `infra.etcd` 配置节；
   > - **nacos**：SDK 分 v1(config)/v2(naming) 两个对象，同一 `infra.nacos` 配置节构造；
   > - **k8s**：contrib config 源**自建连接**（不收注入，仅 kubeconfig/in-cluster 选项）；registry 收注入 clientset。
   > 适配器的标准化接口就是 Kratos 的 `config.Source` 与 `registry.Registrar/Discovery`，不另造。
   >
   > **自研弹性代码全工程仅两处**：①配置变更→换池编排（confcenter+provider 指纹，S0 已有）；②rabbitmq 消费监督循环（裸 AMQP 不自愈，S3 实现）。其余（各库懒连接、池自愈、SDK 重连、watch 检测）全部来自标准件。：配置中心与服务发现都支持 **local / nacos / etcd / k8s** 四套，由 **`bootstrap.yaml` 选择**；MQ 支持 **rabbitmq + rocketmq** 两套。k8s 适配器照常实现，但**本地无集群 → 其 E2E 实测标记 blocked（待真实集群），本地仅做单元/契约测试（fake client）**（rule-0002：blocked 不当 pass）。

6. **分片交付**：S0 地基（含以 PostgreSQL 验证弹性闭环）→ S1 Redis → S2 trace/metrics 完善 → S3 MQ（rabbitmq + rocketmq）→ S4 服务发现（local/nacos/etcd/k8s）→ S5 配置远程适配器（nacos/etcd/k8s，含 configmap·secret）→ S6 熔断限流/降级打磨。每片独立需求包。

## 版本与兼容矩阵（2026-06 锁定）

> 基线取自参考工程 `backend-service/go.mod`（Go 1.26.1 / Kratos v2.9.2 / ent v0.14.6 / go-redis v9.18.0 / protobuf v1.36.11）；新增中间件版本经 goproxy + 各项目 release 调研确定。**意向版本如下，最终以脚手架时 `go mod tidy` 干净解析为准并回填**（rule-0003 / rule-0008）。

**go.mod 直接引入：**

| 依赖 | 模块 | 版本 |
|---|---|---|
| Kratos | `github.com/go-kratos/kratos/v2` | v2.9.2 |
| Kratos contrib（config/registry × nacos/etcd/k8s）| `.../contrib/{config,registry}/{nacos,etcd,kubernetes}/v2` | **伪版本 `v2.0.0-20260404020628-f149714c1d54`** |
| otel（API+SDK+exporters，全部统一）| `go.opentelemetry.io/otel...` | **v1.44.0**（prometheus exporter `v0.66.0`）|
| Prometheus | `github.com/prometheus/client_golang` | v1.23.2 |
| ent | `entgo.io/ent` | v0.14.6 |
| PG driver | `github.com/jackc/pgx/v5`（走 stdlib 适配器）| v5.9.2 |
| Redis | `github.com/redis/go-redis/v9` | v9.18.0 |
| wire | `github.com/google/wire` | v0.7.0 |
| protobuf | `google.golang.org/protobuf` | v1.36.11 |
| MQ（二期）| `rabbitmq/amqp091-go` ／ `apache/rocketmq-clients/golang/v5` | v1.11.0 ／ v5.1.3 |

**工具链（`go install`，不进 require）：** buf v1.70.0（CLI 仍是 v1，用 **v2 配置格式**）、protoc-gen-go v1.36.11、protoc-gen-go-grpc v1.6.2、protoc-gen-go-http（kratos）、protoc-gen-openapi（google/gnostic v0.7.1）、wire v0.7.0、Atlas CLI（独立装）。

**关键兼容性坑（必须照做）：**
1. **otel 全锁 v1.44.0**：Kratos v2.9.2 自带 otel API v1.24.0，我们直接引的 SDK 是 v1.44.0；二者向后兼容，但要在自己 `go.mod` 把所有 otel 模块**显式锁到同一个 v1.44.0**，让 MVS 干净收敛，别让两截版本打架。
2. **contrib 锁 commit 伪版本（为可复现，不是怕 v3）**：经 goproxy 实证——Kratos **v3 未发布**（无 tag，仅 dev 伪版本 `v3.0.0-20260526…`）；我们用的 `…/v2 @ v2.0.0-20260404…` 其 `go.mod` **明确 require `kratos/v2 v2.9.2`**，且**不存在 `/v3` 路径的 contrib**。contrib 模块**没有正式 release tag、只能用伪版本消费**，故锁该 commit 保证可复现。Go 大版本 = 不同 import 路径，`go get -u` **不会**误升到 v3。长期注意：contrib `/v2` 最新停在 2026-04-04、core 已在开发 v3，待 v3 GA + contrib 出 v3 版再评估迁移。
3. **metrics 中间件已改用 otel metric API**（v2.8.0+）：老的 prometheus 直接计数写法废弃，按新接口写。
4. **ent 走 `database/sql` + pgx stdlib**（`sql.Open("pgx", dsn)` → `entsql.OpenDB`），不直连 pgx。
5. **atlas SDK（`ariga.io/atlas`）≠ Atlas CLI（`github.com/ariga/atlas`）**，版本号体系不同；用 CLI 迁移方案则 atlas 不进 `go.mod`。

**待脚手架时验证（不预设、不编造）：**
- k8s contrib 的 `client-go` 版本是否与其他 k8s 库冲突；
- `protoc-gen-go-http` 用 `@v2.9.2` 子模块 tag 还是伪版本；
- buf v2 配置在本工程 proto 布局下的 managed mode override 细则。

**S0 实现回填（实测，以 `go mod tidy` 为准）：**
- otel 全锁 **v1.44.0** ✓、ent **v0.14.6** ✓、pgx/v5 **v5.9.2** ✓、wire **v0.7.0** ✓、prometheus client_golang **v1.23.2** ✓、aegis **v0.2.0**（熔断，promote 为直接依赖）。
- 协议链：protobuf **v1.36.11** + **grpc v1.80.0**（ADR 未锁，取与参考工程一致以满足 protoc-gen-go-grpc 生成码运行时）+ 本机 protoc-gen-go-grpc **v1.6.1**（ADR 意向 v1.6.2，差一 patch）。
- **Kratos `config.Scan` 走 `json.Unmarshal`** → 配置结构体字段须带 **json tag**（实现期 e2e 才暴露：缺 json tag 导致 snake_case 字段静默解析为零值）。
- contrib（nacos/etcd/k8s）S0 未引入，留 S4/S5。
- **架构偏离（S0）**：单服务下统一网关无意义且 `/readyz` 需读进程内 resource.Registry，故 S0 由 **demo 自身 Kratos HTTP transport** 承担入口 + 健康/指标端点；独立统一网关待多服务时再拆（见 feature 后续片）。
- atlas 迁移因 ent schema 在 Go `internal` 包下被 atlas loader 拒绝，S0 改**手写初始迁移 SQL**（与 schema 一致，sandbox initdb 应用）；proper atlas 生成（schema 移出 internal 或 entc shim）留后续。

## 受影响的 skill（rule-0007）

- skill：feature-delivery ／ 是否已更新：否（新工程按既有交付流程走，流程未变）
- skill：context-loading ／ 是否已更新：否（加载档位规则未变）

## 备选方案

1. **自研统一资源框架（"罩住一切"）**：漏抽象、最小公分母、重复造轮子（库/网格已做的事再做一遍）。否决。
2. **纯服务网格（Istio/Linkerd）兜弹性**：只覆盖服务间 RPC，不覆盖 PG/Redis/MQ 等有状态依赖；且强绑 k8s+mesh。作为**可选增强**，不作基线。
3. **标准分层组合（采用）**：库自愈 + Kratos 中间件 + k8s 探针 + 薄 provider；覆盖全、依赖标准件、可被网格进一步增强。

## 影响

- 首个被管工程落 `projects/kratos-base`；按 `docs/harness/PROJECT_ONBOARDING.md` 接入（工程 `AGENTS.md`、verification 路由随 S0 实现补）。
- S0 用"杀 DB→服务不宕→readiness 转红→恢复 DB→自动转绿→请求续上"作为弹性闭环的可验证证据。
