# decision：kratos-base S4（配置中心 + 服务发现，四后端适配器框架）收尾评审

- 任务档位：L4（跨模块基建片）
- 评审时间：2026-06-11T10:04Z（UTC）
- 评审环境：macOS arm64（Darwin 25.4.0），docker 29.4.3 aarch64，被评工程 `projects/kratos-base`
- 评审方式：文档/代码独立通读 + 第一手复跑（make verify、三包 -race -count=1 单测、两条新 e2e 场景、急加载 fail-fast 单点实验、nacos 镜像 arm64 阻塞复现）

## 逐题 verdict

```yaml
prompt: "010"
verdict: pass
severity: warn
reason: 闸门/验证/证据结构齐备且经独立复核成立，但有两处 warn 级一致性缺口（见下），可有条件收尾
evidence: 见"独立复核记录"与"warn 发现"两节
```

```yaml
prompt: "003"
verdict: pass
severity: warn
reason: 完成声明全部有真实运行证据且评委独立复跑全过；测试断言真实业务值（日志串、RPC 响应内容、行为断言），非只测 200
evidence: scen_conf_etcd.sh 独立复跑 EXIT=0；scen_disc_etcd.sh 独立复跑 PASSED（"service registered" INFO + 探针输出 "pong: pong"）；go test ./pkg/{backends,bootstrap,registryx}/... -race -count=1 全 ok（EXIT=0）；make verify ">> verify OK"
```

```yaml
prompt: "002"
verdict: pass
severity: warn
reason: nacos/k8s e2e 如实标 blocked 未粉饰，阻塞原因经评委独立复现属实；blocked 项未混入任何 pass 矩阵
evidence: docker pull --platform linux/arm64 nacos/nacos-server:v2.3.2 → "no matching manifest for linux/arm64"（与 test/resilience/README.md 记录一致）；run_all.sh 11 项矩阵不含 nacos/k8s e2e；feature/README/CURRENT_STATUS 三处口径一致
```

## 综合分档：**yellow**

全部相关考题 pass，但有两处 warn 级问题需说明（不阻塞收尾，建议修复后置 done）。

## 独立复核记录（第一手，非采信声称）

1. `make -C projects/kratos-base verify` → build+vet+lint+test，输出 ">> verify OK"、lint "0 issues."。
2. `go test ./pkg/backends/... ./pkg/bootstrap/... ./pkg/registryx/... -race -count=1` → 三包全 ok，EXIT=0（非缓存）。
3. `bash test/resilience/scen_conf_etcd.sh` → **EXIT=0**：etcd 注入配置启动 /readyz=200 → 坏 DSN+停 PG → 503 → 改回+起 PG → 200 → 空 grpc.addr → 日志断言 "retaining previous config" + 进程活 + /readyz=200（AC-C2）。
4. `bash test/resilience/scen_disc_etcd.sh` → 终局 PASSED：etcd 全宕下 demo 启动、/v1/ping=200、注册重试 WARN → sandbox-up 后日志 "registryx: service registered"（不重启）→ discoveryprobe 经 `discovery:///demo` 输出 "pong: pong"（AC-D1/D2）。
5. **急加载语义（诚实点③）**：etcd 宕时 `./bin/demo -conf configs/bootstrap.etcd-sandbox.yaml` → **DEMO_EXIT=1**，错误链 `demo: new config source: bootstrap: etcd backend: backends/etcd: probe "127.0.0.1:2379": context deadline exceeded`——mode=etcd 确为 Load fail-fast，未硬套懒加载。配套单测 `TestNewConfigSource_EtcdUnreachable` 亦覆盖。
6. **注册非致命（诚实点④）**：grep 全工程非测试码无 `kratos.Registrar(` 内置选项；自研 `registryx.Runner`（main.go Phase 6b，构造失败仅 Warn 继续启动），单测以注入式退避真验"失败 N 次→重试 N+1 次→成功""Stop→Deregister 恰一次""nil registrar no-op""ctx cancel 干净退出"。
7. **blocked 复现（诚实点①②）**：nacos 镜像 arm64 拉取失败独立复现（同 README 记录串）；k8s e2e 无本地集群预声明，适配器构造/错误路径单测真过（`TestNew_K8sKind_NoKubeconfig`、`TestNewConfigSource_K8sNonExistentKubeconfig`、`TestNewConfigSource_K8sInClusterConstruction`）。
8. **回归向量（AC-REG）**：run_all.sh 共 11 场景（S0 六 + S1 三 + 新二）；默认 `bootstrap.sandbox.yaml` 无 registry 节 → kind="" → local → Runner no-op，S4 改动对旧场景无结构性影响；控制器声称 11 AC 全 PASS，评委抽测两条新场景 + verify 全绿予以采信（未全量重跑，时长原因）。
9. 状态一致性：F-0004 `delivery_status: verified` + "收尾过 eval 后置 done"；index.yaml、CURRENT_STATUS 口径一致。

## warn 发现（不阻塞，建议修复）

- **W1 悬空 runbook 引用**：`test/resilience/README.md`（NACOS_E2E=1 一步）与 `workspace/verification.yaml`（nacos 可选 E2E 注释）均指向 `test/resilience/scen_conf_nacos.sh`，**该文件不存在**；README 引用的 `configs/bootstrap.nacos-sandbox.yaml` 同样不存在（措辞"需配置"勉强算用户自建）。blocked 分类本身诚实，但"环境解锁后怎么跑"的指引不可执行——要么补脚本，要么改为纯手工步骤并删悬空引用。
- **W2 "共享 client"承诺与实现有出入（ADR 一致性核对未做彻底）**：ADR-0002 澄清段写"etcd/k8s：contrib 适配器接收注入的**同一 client**"。实际：(a) k8s contrib 自建连接不收注入（backends.go 注释与 feature 状态注已如实说明，但 **ADR 文本未回填修正**）；(b) etcd 配置角色（`backends.NewEtcdClient`，带探活）与注册角色（`registryx.New` 内 `clientv3.New`，懒连）**各建一个 client 实例**，并非"一份 client 喂两个角色"（feature 范围段原话），且 registryx 直接构造 clientv3 也突破了"连接细节只在 backends 一层"的分层声明。两角色探活语义不同（急 vs 非致命）是合理工程理由，但设计文本应回填澄清而非保留与实现不符的表述。
- 备注（不计分）：AC-C2 文字含"confcenter 版本++"，脚本未直接断版本号，且 503 翻转同时停了 PG（热更"应用"路径证据与 PG 宕机混杂）；watch→校验→拒绝路径有直接日志断言，应用路径由 S0 file 源场景背书，够用但可更严。

## 一句总评

S4 交付货真价实：两条 etcd e2e、急加载 fail-fast、非致命自研 Runner 全部经评委第一手复跑成立，nacos/k8s blocked 如实——但 nacos 跑法悬空引用与 ADR"共享 client"表述未对齐实现，修掉这两处再置 done。

## 给用户的提示

可有条件收尾（yellow）：功能与证据可信，建议在置 done 前补 `scen_conf_nacos.sh`（或改 README/verification.yaml 为纯手工 runbook）并回填 ADR-0002 澄清段的"共享 client"措辞（etcd 双实例、k8s 自建连接的实情）。
