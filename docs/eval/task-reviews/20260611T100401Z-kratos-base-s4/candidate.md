# candidate 副本：kratos-base S4 收尾评审

> 评审对象快照（2026-06-11，eval kratos-base-s4）。主文档全文如下；配套产出以指针列出。

## 配套产出（指针）

- 计划：`tasks/kratos-base-s4-plan.md`
- ADR：`docs/decisions/0002-kratos-base-architecture-and-resilience.md`（弹性模型澄清段，last_updated 2026-06-11）
- 实现：`projects/kratos-base/`（pkg/backends、pkg/registryx、pkg/bootstrap 四源、configs/bootstrap.{etcd,disc}-sandbox.yaml、test/resilience/scen_conf_etcd.sh + scen_disc_etcd.sh、test/discoveryprobe/）
- 验证路由：`workspace/verification.yaml`；状态：`docs/context/CURRENT_STATUS.md`、`docs/features/index.yaml`（F-0004 delivery_status: verified）

## 主文档全文：docs/features/0004-kratos-base-conf-registry.md

```markdown
# Feature 需求包：F-0004 kratos-base 配置中心 + 服务发现（S4：四后端适配器框架）

> 基建片：不为单一功能，而是建「配置/注册的适配器框架」。设计依据：ADR-0002 弹性模型澄清（配置**急加载**不懒、注册**非致命+后台重试**、同后端**共享一份客户端接入**、标准接口=Kratos `config.Source` / `registry.*`，不另造）。改业务代码前须就绪（rule-0001）。

## 背景 / 目标

把 S0 配置两段式预留的远程源槽位（etcd/nacos/k8s 现在显式报 "not implemented"）真正补齐，并新增服务注册/发现能力。产出是**框架**：标准化接口 + 四种预制适配 + `bootstrap.yaml`/环境变量选择 + 「中间件接入」与「配置/注册角色」分层（共享 client）。未来任何后端按同样模式扩展。

## 范围

- 包含：
  - **统一接入层** `pkg/backends`：etcd / nacos / k8s 客户端的构造与配置（连接细节只在这一层；etcd/k8s 一份 client 喂两个角色，nacos 同配置构造 config+naming 两个 SDK 对象）。
  - **配置源适配**（实现 Kratos `config.Source`）：file（已有）+ **etcd + nacos + k8s configmap/secret**；接入 S0 的 `bootstrap.NewConfigSource` 选择器（`infra.mode` / `INFRA_MODE`）。**急加载语义**：启动 `Load()` 失败 fail-fast；热更走既有 `confcenter.Manager`（watch→校验→换池）。
  - **服务注册/发现** `pkg/registryx`（实现/复用 Kratos `registry.Registrar`/`Discovery`）：**local（静态直连，即现状）+ etcd + nacos + k8s**，`bootstrap.yaml` 选择；**注册非致命**——注册中心不可达时服务照常启动、后台退避重试注册（日志可见）、本地接口不受影响。
  - demo 接入：启动时注册自身（kind≠local 时）；e2e 用 Kratos gRPC client + `discovery:///demo` 按服务名调通 Ping（服务发现闭环）。
  - 后端实现优先用 **contrib（锁 2026-04-04 伪版本，禁 `-u`）**；某 contrib 不可用/太旧则按 ADR 对原生 SDK 自写薄适配并记录原因。
- 不包含：多实例负载均衡策略调优、配置加密、k8s 真集群验证（无本地集群）、nacos 鉴权进阶。

## 用户故事 / 验收目标

- **AC-C1（选择器）**：`bootstrap.yaml`/`INFRA_MODE` 可选 file/etcd/nacos/k8s 四种配置源；非法值显式报错。
- **AC-C2（etcd 配置 e2e）**：etcd 容器；服务从 etcd 加载配置启动；**改 etcd 里的 key → 不重启热更生效**（confcenter 版本++）；坏配置被拒保留上版；**config-flip 弹性**（经 etcd 改坏 DSN→`/readyz` 503→改回→200）。
- **AC-C3（nacos 配置）**：适配器构建+单测过；e2e 用 nacos 容器（standalone）尽力——跑通则 pass，环境不济**如实标 blocked**。
- **AC-C4（k8s 配置）**：configmap/secret 适配器构建+单测（fake client / 契约）；**e2e blocked（无本地集群，预声明）**。
- **AC-D1（注册非致命）**：kind=etcd 但 etcd 不可达 → 服务**照常启动**、`/v1/ping` 等正常、日志可见注册退避重试；etcd 恢复后注册成功（不重启）。
- **AC-D2（发现闭环 e2e）**：etcd 容器；demo 注册自身；客户端经 `discovery:///demo` 解析并成功调用 gRPC Ping。
- **AC-D3（local 发现）**：kind=local 行为与现状一致（静态直连，零注册）。
- **AC-D4（nacos/k8s 注册）**：适配器构建+单测；nacos e2e 尽力、k8s e2e blocked（同上口径）。
- **AC-REG（回归）**：S0（AC1-6）+ S1（AC-R1-3）仍全过。

## 影响面

- 被管工程 `projects/kratos-base`：新增 `pkg/backends`、`pkg/registryx`；扩展 `pkg/bootstrap`（Bootstrap 结构 + 四源选择）；demo 启动注册；sandbox 加 etcd（e2e 期，nacos 尽力）；conf/runtime 扩展。
- 受影响 skill（rule-0007）：feature-delivery / context-loading 无需更新。
- 风险：contrib 伪版本的 k8s `client-go` 依赖冲突（ADR 已标"待验证"）；nacos contrib 锁的 SDK 较老（v1.x），不济则自写（记录决策）。

## 测试设计

- 单测：bootstrap 四源选择/非法值；各适配器构造与错误路径（不可达 endpoint→error 不 hang）；注册重试 runner（可注入退避，无 time.Sleep）；local 发现直连。
- E2E（容器，用户已允许）：`scen_conf_etcd`（AC-C2 含 config-flip）、`scen_disc_etcd`（AC-D1/D2）、nacos 尽力、回归 `run_all.sh`。
- 证据按 `workspace/verification.yaml` 记录；blocked 项如实分类（rule-0002）。

## 状态

- delivery_status: verified
- implementation_allowed: true

> 实现见 `projects/kratos-base/`（S4-T1~T3）。**第一手验收通过**：`scen_conf_etcd`（配置从 etcd 加载、改坏 DSN→/readyz 503、改回→200、坏配置拒绝保留上版）与 `scen_disc_etcd`（etcd 宕启动不崩+重试 WARN、恢复自动注册、探针经 `discovery:///demo` 拿到 pong）独立跑 EXIT=0；全量 run_all 11 AC 全 PASS（S0+S1 零回归）。
> 如实 blocked：nacos e2e（macOS arm64 无官方镜像，适配器+单测已过，跑法见 `test/resilience/README.md`）；k8s e2e（无本地集群，构建+单测已过）。contrib 关键核实：config/etcd 与 nacos 收注入 client（共享接入成立）、k8s contrib 自建连接（已注明）、client-go v0.26.3 无冲突、etcd 配置 key 须 `.yaml` 后缀（格式探测按扩展名）。
> 收尾过 eval（rule-0005）后置 done。
```
