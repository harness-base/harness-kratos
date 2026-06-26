# 实施计划：kratos-base S1（Redis 接入）

> 临时工作产物（放 `tasks/`）。设计源：需求包 `docs/features/0002-kratos-base-redis.md` + ADR-0002。
> 沿 S0 已验证的脊柱（`pkg/resource.Provider`）挂 Redis，照搬 PG 那套懒加载/自愈/健康/弹性场景的形状。

## 1. 概述

- 背景：S0 已用 PostgreSQL 证明"懒加载+断线自愈+不宕+恢复续上"。S1 把 Redis 沿同一脊柱挂上，证明"加适配器=照脊柱插"。
- 关键约束：Redis 用 go-redis/v9 `UniversalClient`（自适应单机/集群/分片）；封进 `resource.Provider`；不自研重连（go-redis 池自管），provider 层只做配置变更换池 + 健康聚合。版本 go-redis v9.18.0（已在 ADR 矩阵）。
- 产出即验收：AC-R1~R5（见需求包）。

## 2. 文件结构（S1 增量）

```
projects/kratos-base/
├── pkg/redisx/            client.go(UniversalClient+resource.Adapter) / *_test.go     ← 新
├── api/demo/v1/           demo.proto 加 Hits rpc（/v1/hits）→ 重生成
├── app/demo/internal/
│   ├── conf/conf.go       加 Redis 配置 + SelectRedis(cfg)→redisx.Config
│   ├── biz/hits.go        HitsUsecase（INCR 计数）                                   ← 新
│   ├── data/hits_repo.go  redis 仓储（sre.Breaker + errs 映射）                      ← 新
│   ├── data/redis.go      Data 持有 redis provider；RedisHealthy 注册进 registry      ← 新
│   ├── service/demo.go    实现 Hits
│   └── cmd/wire.go        provider/registry 装配加 redis
├── configs/runtime*.yaml  加 data.redis.addrs 等
├── deploy/sandbox/        docker-compose 加 redis:7 容器
└── test/resilience/       scen_redis_*.sh + run_all 纳入
```

## 3. 步骤（到代码级）

### S1-T1 `pkg/redisx`（核心适配器，纯 Go，TDD）
- `redisx.Config{ Addrs []string; Username,Password string; DB int; PoolSize,MaxRetries int; DialTimeout,ReadTimeout,WriteTimeout time.Duration; EnableTLS bool }`；`Fingerprint() string`。
- `func Open(cfg Config) (redis.UniversalClient, error)`：`redis.NewUniversalClient(&redis.UniversalOptions{...})` + 带超时 PING（失败 Close+返 err）。addrs 多个→cluster，单个→single（UniversalClient 自适应）。
- `resource.Adapter[redis.UniversalClient]`：Build=Open；Close=client.Close；Fingerprint=Config.Fingerprint；Health=client.Ping(ctx).Err()。
- 单测：Fingerprint 稳定/变更；不可达 addrs（短超时）→ Open 返 err 不 hang；多 addrs→UniversalClient 走 cluster 类型（断言类型/不报错）；Health 对不可达返错。
- 验证：`go test ./pkg/redisx/... -race`、`make verify` 绿。go.mod 加 go-redis v9.18.0（已预热）。

### S1-T2 接入 demo（hits 接口 + readiness）
- proto：`demo.proto` 加 `Hits(HitsRequest)→HitsReply [GET /v1/hits]`（HitsReply{count int64}）；error_reason 复用（redis 不可用→DB_UNAVAILABLE 或新增 CACHE_UNAVAILABLE，二选一，倾向复用 DB_UNAVAILABLE→503）。`make generate`。
- `conf`：加 `Data.Redis{ Addrs []string; ... }` + `SelectRedis(cfg any)(redisx.Config,error)`；`runtime.yaml`/`runtime.sandbox.yaml` 加 `data.redis.addrs: ["localhost:6379"]`。
- `data/redis.go`：`Data` 加 `redis *resource.Provider[redis.UniversalClient]`（`New` 增 selectRedis 入参）；`Redis(ctx)`、`RedisHealthy(ctx)`。
- `data/hits_repo.go`：`HitsRepo.Incr(ctx, key)`，sre.Breaker 包 INCR，redis 不可用→`errs.DBUnavailable`。`biz/hits.go`：`HitsUsecase.Hit(ctx)`。`service`：实现 Hits。
- wire：加 redis provider + 把 `RedisHealthy` 注册进 registry（readiness 现含 postgres + redis）。
- 单测：hits repo（不可达 redis→错误、断路器）；service Hits。
- 验证：`make verify` 绿；本地起（无 redis）`/readyz`=503 且 details 含 redis、`/v1/ping` 仍 200。

### S1-T3 sandbox + 弹性验收 + 回归
- `deploy/sandbox/docker-compose.yaml` 加 `redis:7-alpine`（healthcheck `redis-cli ping`）；sandbox 配置 redis addrs 指向它。
- `test/resilience/`：`scen_redis_boot_down.sh`(AC-R1)、`scen_redis_recover.sh`(AC-R2)、`scen_redis_drop.sh`(AC-R3，杀 redis→/v1/hits 快速失败+/readyz 503+进程活→恢复→续上)。纳入 `run_all.sh`。**回归**：S0 的 AC1–AC6 仍全过（AC-R5）。
- 脚本 CWD 无关（cd 到工程根，吸取 S0 教训）。
- 更新 `workspace/verification.yaml`（e2e 含 redis 场景）。
- 验证：`bash test/resilience/run_all.sh`（含 PG + Redis 全场景）E2E_EXIT=0；`make verify` 绿。

## 4. 验证 runbook（映射 AC-R）

- [ ] `make -C projects/kratos-base verify`（build+vet+lint+test -race 绿）
- [ ] **AC-R1** redis 关 → 起服务 → /healthz 200、/readyz 503(redis)、/v1/hits 5xx 结构化错误、/v1/ping 200、进程活
- [ ] **AC-R2** 起 redis（不重启）→ /readyz 200、/v1/hits 计数递增
- [ ] **AC-R3** 杀 redis → /v1/hits 快速失败(熔断)、/readyz 503、进程活 → 恢复 → 自动续上、/readyz 200
- [ ] **AC-R4** 单机 sandbox 通过；多 addrs 配置构造 cluster 客户端不报错（单测）
- [ ] **AC-R5** S0 `run_all.sh` 的 AC1–AC6 仍全 PASS（回归）
- [ ] 证据按 `workspace/verification.yaml` 记录

## 5. 失败模式与回滚

- go-redis 版本/依赖解析异常：以 `go mod tidy` 为准回填。
- 回滚：S1 全部在 `projects/kratos-base/` 内新增/扩展 + verification.yaml；删 redisx + 还原 demo/conf/sandbox 即回滚，不影响 S0。

## 6. 受影响 skill / 文档（rule-0007）

- feature-delivery / context-loading：无需更新。
- 收尾：L2+ 跑 eval（rule-0005）。
