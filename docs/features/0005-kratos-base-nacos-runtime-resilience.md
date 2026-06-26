# Feature 需求包：F-0005 nacos 后端补全 + 配置/注册中心运行期弹性验证（S5）

> 验证/补全片：把 nacos 从 blocked 转为真后端验过，并补上 S4 缺口——配置中心/注册中心**运行期后端宕→服务不崩→恢复续上**。设计依据：ADR-0002 弹性模型澄清（配置=急加载、运行期靠内存；注册=非致命、降级跨服务）。改业务代码前须就绪（rule-0001）。

## 背景 / 目标

S4 用 etcd 验过配置+发现，nacos 因当时误判"arm64 无镜像"标了 blocked——实测 `nacos/nacos-server:v2.5.0` **有 arm64 镜像**，本机 docker 可起。本片：①用真 nacos 跑通配置中心+注册中心功能 e2e；②补 S4 未验的**控制面运行期弹性**——后端进程宕掉后服务行为（对 etcd 与 nacos 都验）。

**明确不做**（用户已拍板）：etcd 配置源的启动期本地快照兜底（启动期无配置 fail-fast 可接受）。

## 范围

- 包含：
  - sandbox 加 nacos（`nacos/nacos-server:v2.5.0`，standalone，鉴权关）；nacos 版 bootstrap 配置（config 源 + registry kind=nacos）。
  - **nacos 功能 e2e**：配置加载 → 不重启热更 → 坏配置回滚；注册非致命+重试 → 发现调通（discoveryprobe）。
  - **控制面运行期弹性 e2e（etcd + nacos 各一遍）**：
    - 配置中心：服务起好（/readyz 200、/v1/* 正常）→ **停掉配置中心容器** → 断言**进程不崩、/v1/greet 与 /v1/ping 照常 200、/readyz 仍 200**（配置中心不在 readyz；服务继续用内存里启动时的配置）→ **拉起配置中心** → 推一个配置变更 → 断言**热更又生效**（watch 自重连续上）。
    - 注册中心：服务+注册中心在用、discovery 调通 → **停注册中心** → 断言**跨服务 discovery 调用报错、但 /v1/ping 等本地接口照常 200、进程不崩** → **拉起** → 断言**自动重注册 + discovery 又调通**。
  - 纳入 `run_all.sh`；S0~S4 全回归。
- 不包含：etcd 启动快照（用户否决）；k8s 真集群（无环境）；rocketmq（下一步单独，需用户给 broker 或我 docker 折腾）。

## 用户故事 / 验收目标

- **AC-N1（nacos 配置功能）**：mode=nacos 从 nacos 加载配置启动（/readyz 200）→ 改 nacos 配置不重启热更 → 坏配置被拒保留上版。
- **AC-N2（nacos 注册功能）**：kind=nacos，注册非致命（nacos 不可达时服务照起+重试 WARN）→ nacos 起后自动注册 → discoveryprobe 经 `discovery:///demo` 得 pong。
- **AC-CR1（配置中心运行期宕，etcd+nacos 各验）**：服务运行中停配置中心 → 进程活、`/v1/greet`+`/v1/ping`=200、`/readyz`=200（继续用内存配置）→ 拉起 → 推**非法配置（空 grpc.addr）** → demo 日志出现**新增**的 `retaining previous config`（confcenter 拒绝；该日志只有 watch 重连并把变更送达才出现 ⇒ 热更管线复活）→ 推回好配置 → `/readyz`=200。〔注：原用"坏 redis 地址→503"，但 S6 给 readyz 设了合理的探测超时后发现那条依赖"Open 阻塞耗光请求 ctx"的超时竞态、不稳（self-heal 本就该保留旧好连接、readyz 保持 200 才对），已改为此 ctx 无关的硬证据。详见 S6/F-0006。〕
- **AC-CR2（注册中心运行期宕，etcd+nacos 各验）**：运行中停注册中心 → 跨服务 discovery 调用失败（discoveryprobe 无 pong）、本地 `/v1/ping`=200、进程活 → 拉起 → discovery 又通（discoveryprobe 得真 pong）。恢复机制：注册中心 SDK（etcd contrib / nacos SDK）的**租约 keepalive 自愈**，非 app 层主动重注册（registryx Runner 注册一次即 park，无重注册循环）。
- **AC-REG（回归）**：S0~S4 既有场景全过。

## 影响面

- 被管工程 `projects/kratos-base`：sandbox 加 nacos；新增 nacos bootstrap 配置 + 运行期弹性场景脚本；可能微调 demo 启动期 watch 错误处理（确保后端宕时 watch 不崩进程——若发现暗坑）。
- 受影响 skill（rule-0007）：无需更新。

## 测试设计

- E2E（容器，本机 docker，含 rule-0009 断言锚定）：`scen_conf_nacos.sh`（AC-N1，复用 S4 草稿、本机真跑）、`scen_disc_nacos.sh`（AC-N2，新）、`scen_cc_runtime_down.sh`（AC-CR1，参数化 etcd/nacos）、`scen_reg_runtime_down.sh`（AC-CR2，参数化）。
- 断言锚定产出方证据（rule-0009）：热更看 **redis-addr flip**（推坏地址 6390 → readyz 翻 503，pg/redis 容器均未停，503 只可能来自 watch 落地后重建失败），发现看 discoveryprobe 真 pong，不裸串 grep。AC-CR2 恢复证据只用 CR2-d（probe pong），不 grep 启动期日志行。
- 回归 `run_all.sh`；blocked 项如实（k8s、rocketmq）。

## 状态

- delivery_status: done
- implementation_allowed: true

> 实现见 `projects/kratos-base/`（S5-T1/T2 + eval 复修轮）。**第一手验收通过**：nacos 配置/注册功能（AC-N1/N2）、配置中心运行期宕→不崩→热更续上（AC-CR1，etcd+nacos）、注册中心运行期宕→discovery 失败→恢复续上（AC-CR2，etcd+nacos）全 EXIT=0；全量 run_all 20 AC PASS（S0~S5 回归）。
> eval：**yellow→复修→第一手复验**。考题 012（rule-0009）抓出两处弱锚定，已修：①AC-CR1"热更续上"原用"坏 DSN+停 pg→503"（共因污染，503 区分不了热更落地 vs pg 宕）→ 改 **redis-addr flip**（推坏 redis 地址、pg/redis 容器均不停，503 唯一来源=新配置经 watch 落地后 provider 重建失败，正向无共因）；②AC-CR2"自动重注册"是启动期旧日志假命中（registryx Runner 注册一次即 park，无重注册循环）→ 删该断言，恢复证据只用 **CR2-d（discoveryprobe 真 pong）**，并如实标注恢复机制是注册中心 SDK 租约 keepalive 自愈、非 app 重注册。教训记 `tasks/lessons.md` 2026-06-23。
> blocked 如实：k8s e2e（无集群）、rocketmq e2e（下一步，需 broker）。

## 收尾补强（S5-T3，验证工作流挖出）

用户问"两后端是否真按规格做好测好"，独立核查工作流（3 只读审计 + 实跑复验）触发深查，得两条真改进（均不影响已验的两模型，属健壮性/覆盖补强）：

- **redisx.Open 尊重调用方 context**：原 `Open(cfg)` 用 `context.Background()+DialTimeout` 自建 ping ctx，忽略调用方 ctx。后果：配置热更到坏 redis 地址时，Build 里的 eager ping 阻塞、不被 readyz 请求 ctx 取消，readyz 报成 `context deadline exceeded`（而非干净的"redis 连不上"）。改 `Open(ctx, cfg)`：ping ctx 从调用方派生（仍以 DialTimeout 封顶），随调用方取消/到期及时返回。**注（后续修正）**：当时 AC-CR1 仍用"坏 redis 地址→503"，经实验证实 503 确由新配置落地触发；但该机制依赖"Open 阻塞耗光 readyz 请求 ctx"的超时竞态。S6 给 readyz 设了合理的 15s 探测超时后，self-heal 会回退旧好句柄、readyz 保持 200（这才是对的弹性），该证法失效。故 S6 把 AC-CR1 改为 ctx 无关的硬证据：恢复后推非法配置 → confcenter 新增 `retaining previous config` 日志（BEFORE/AFTER 计数对比，证 watch 重连）。
- **etcd 启动 fail-fast e2e**：新增 `scen_conf_boot_fastfail.sh`——etcd 缺席时 mode=etcd 启动 → demo 快速非零退出（config 加载/probe 失败）+ 不对外服务。把"启动期真没配置又没缓存就 fail-fast（用户认可、不要快照）"从"仅代码+单测"升级为端到端锁定。

AC：**AC-CF**（etcd 配置源启动 fail-fast）；既有 AC-CR1/CR2/N1/N2 回归不变。
