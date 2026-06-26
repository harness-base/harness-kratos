# confcenter 就近规约（pkg/confcenter）

配置中心：急加载 fail-fast + watch 热更 + 坏配置回滚。本文件是本包红线；工程全局红线见 `../../AGENTS.md`，弹性模型见 ADR-0002 §4。

## 红线
- **启动期急加载 + fail-fast**：配置源（etcd/nacos/k8s）启动连不上就直接报错退出，绝不延迟加载——没配置装配不出服务（如 grpc.addr 空）。（../bootstrap/bootstrap.go:88-90；ADR-0002；lessons 2026-06-02） <!-- rule: kratos/confcenter-eager-load-fail-fast | sev: blocker -->
- **Publish 前必须 validate，坏配置原子保留上一版**：校验失败返 error、Version/Value 不变；坏配置绝不进运行态。（manager.go:61-70；manager_test.go TestBindKratosWatch_PublishRejectRetainsPrev） <!-- rule: kratos/confcenter-validate-atomic-rollback | sev: blocker -->
- **Snapshot.Version 从 1 起严格 +1，禁手改/重置**：Version 是热更硬证据（BEFORE/AFTER 计数对比）；乱序/重置会让消费方检测不到真更新。（manager.go:45,68） <!-- rule: kratos/confcenter-version-monotonic | sev: blocker -->
- **reload/Publish 失败必须日志 `retaining previous config` 并返回**：仅 Publish 成功记 `config applied`；前者是 confcenter 唯一正向产出方证据，缺了 e2e 无法证明热更落地/坏配置拒绝。（manager.go:128-150；lessons 2026-06-23） <!-- rule: kratos/confcenter-retain-log-is-evidence | sev: warn -->
- **Publish 对慢订阅者非阻塞 skip**：`select default` 发送、满缓冲就跳过（不报错、继续迭代）；阻塞等待会与 mu.Lock 一起死锁所有 Current()/Subscribe()。（manager.go:75-80） <!-- rule: kratos/confcenter-nonblocking-publish | sev: warn -->

## 指针
- 弹性模型：`../../../../docs/decisions/0002-kratos-base-architecture-and-resilience.md`
- 启动期 fail-fast 入口：`../bootstrap/bootstrap.go`
- 单测（回滚/保留/断言设计）：`./manager_test.go`
- 热更 e2e：`../../test/resilience/scen_cc_runtime_down.sh`、`scen_conf_*.sh`
