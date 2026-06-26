# 实施计划：kratos-base S4（配置中心 + 服务发现，四后端适配器框架）

> 临时工作产物。设计源：`docs/features/0004-kratos-base-conf-registry.md` + ADR-0002（弹性模型澄清）。
> 关键语义：配置**急加载**（不走 resource.Provider）；注册**非致命+后台重试**；同后端**共享接入**；接口=Kratos `config.Source` / `registry.*`。

## 1. 概述

- 现状：`bootstrap.NewConfigSource` 对 etcd/nacos/k8s 显式报 "not implemented until S5"；无注册/发现（直连地址）。
- 本片产出：四后端配置源 + 四种发现（local/etcd/nacos/k8s）+ bootstrap/env 选择 + 共享接入层。
- contrib 全部锁伪版本 `v2.0.0-20260404020628-f149714c1d54`，**禁 `go get -u`**；不可用才自写薄适配（记录原因）。

## 2. 文件结构（S4 增量）

```
projects/kratos-base/
├── pkg/backends/          etcd.go / nacos.go / k8s.go：客户端构造（接入层，连接细节仅此一层）← 新
├── pkg/bootstrap/         Bootstrap 结构扩展（per-backend 配置节）+ NewConfigSource 四源实现
├── pkg/registryx/         registry.go(选择器+非致命注册 runner) + local.go(静态直连) ← 新
├── app/demo/cmd/main.go   接注册 runner（kind≠local 注册自身）
├── configs/               bootstrap*.yaml 扩展（etcd/nacos/k8s 节示例）
├── deploy/sandbox/        e2e 期加 etcd 容器（bitnami/etcd 或 quay coreos etcd）
└── test/resilience/       scen_conf_etcd.sh / scen_disc_etcd.sh + run_all 纳入
```

## 3. 步骤

### S4-T1 配置源四后端 + 接入层（pkg/backends + bootstrap 扩展）
- `pkg/backends`：`NewEtcdClient(EtcdConfig)(*clientv3.Client,error)`、`NewNacosConfigClient/NewNacosNamingClient(NacosConfig)`、`NewK8sClient(K8sConfig)`；各 Config 带 json+yaml 双 tag。
- `pkg/bootstrap`：Bootstrap 加 `Etcd/Nacos/K8s` 配置节（保持 `Mode`/`Path` 兼容）；`NewConfigSource` 实现 etcd/nacos/k8s 分支（contrib `config/{etcd,nacos,kubernetes}` 接收注入的 client；k8s 含 configmap+secret）。急加载语义不变（Load 失败返回 err，由 main fail-fast）。
- 单测：四源选择/非法 mode；各分支不可达 endpoint→error 不 hang（短超时）；Bootstrap 解析（含 INFRA_MODE 覆盖）。无需真服务。
- 验证：`make verify` 绿；`go mod tidy` 干净（contrib 伪版本锁定；**报告 client-go 是否冲突**）。

### S4-T2 注册/发现（pkg/registryx + demo 接入）
- `pkg/registryx`：`Config{Kind string; Etcd/Nacos/K8s 节}`；`New(cfg, backends...)(registry.Registrar, registry.Discovery, error)`（kind=local 返回 nil registrar + 直连 discovery/无 discovery）；**非致命注册 runner**：`StartRegister(ctx, reg, instance, backoff)`——注册失败退避重试、日志告警、不阻断启动；`Deregister` 优雅退出挂 AfterStop。
- demo：main/wire 接 runner（kind≠local 注册自身 gRPC 端点）；bootstrap/runtime 配置节。
- 单测：local 直连等价现状；注册 runner 用假 Registrar 验"失败→退避重试→成功"（可注入退避，无 sleep）；etcd/nacos/k8s 适配构造+不可达 error。
- 验证：`make verify` 绿；冒烟：kind=etcd 且 etcd 不可达 → 服务照常起、/v1/ping 200、日志见重试（AC-D1 的单机版）。

### S4-T3 e2e（etcd 容器）+ nacos 尽力 + 回归 + 收尾
- sandbox 加 etcd 容器（healthcheck）；`sandbox-up` 等 pg+redis+etcd。
- `scen_conf_etcd.sh`（AC-C2）：配置写入 etcd → 服务从 etcd 启动 → 改 key 热更 → 坏配置拒绝 → config-flip（坏 DSN→readyz 503→改回→200）。
- `scen_disc_etcd.sh`（AC-D1/D2）：etcd 宕起服务（不崩+重试日志）→ 起 etcd → 注册成功 → 测试客户端 `discovery:///demo` 调 Ping 成功。
- nacos：standalone 容器尽力跑一条配置热更；不济 blocked 如实记。
- 回归 run_all（S0+S1 场景）；更新 `workspace/verification.yaml`；CWD 无关；亲跑取证；收尾 eval（slug `kratos-base-s4`）。

## 4. 验证 runbook（映射 AC）

- [ ] `make -C projects/kratos-base verify` 绿（含全部新单测 -race）
- [ ] AC-C1 四源选择 + 非法值报错（单测）
- [ ] AC-C2 etcd 配置 e2e：加载/热更/坏配置回滚/config-flip
- [ ] AC-C3 nacos 构建+单测；e2e 尽力（pass 或 blocked 如实）
- [ ] AC-C4 k8s 构建+单测；e2e blocked（预声明）
- [ ] AC-D1 注册非致命：etcd 宕服务照常起+重试日志+恢复后注册成功
- [ ] AC-D2 `discovery:///demo` 调通 Ping
- [ ] AC-D3 local 直连等价现状
- [ ] AC-D4 nacos/k8s 注册构建+单测；e2e 尽力/blocked
- [ ] AC-REG 回归 S0+S1 全过
- [ ] 收尾 eval green（kratos-base-s4）

## 5. 失败模式与回滚

- contrib k8s 的 client-go 版本冲突 → 先报告，必要时 k8s 适配自写（仅 configmap/secret + endpoints watch 的薄实现）。
- contrib nacos SDK 过老不可用 → 对 nacos-sdk-go/v2 自写薄适配，记录进 ADR。
- 回滚：S4 全在工程内新增/扩展 + verification.yaml；删 backends/registryx + 还原 bootstrap/demo 即回滚。

## 6. 受影响 skill / 文档（rule-0007）

- feature-delivery / context-loading：无需更新。收尾核对 ADR-0002 澄清段与实现一致。
