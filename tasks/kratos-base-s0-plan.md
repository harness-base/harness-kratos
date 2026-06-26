# 实施计划：kratos-base S0（地基/脊柱 + 以 PostgreSQL 证明弹性闭环）

> 计划是临时、会被纠正的工作产物（放 `tasks/`）。设计源：`docs/decisions/0002-kratos-base-architecture-and-resilience.md` + 需求包 `docs/features/0001-kratos-base-spine.md`。
> 实现前需求包须推进到 `tests_ready`（rule-0001）。

## 1. 概述

- **背景**：首个被管工程 `projects/kratos-base`，从 0 搭 Kratos 微服务地基。S0 只做地基 + 用 PostgreSQL 端到端证明"依赖懒加载 + 断线自愈 + 不宕服务 + 平滑恢复"。
- **当前状态**：`projects/` 为空（仅 `.gitkeep`）。控制面 `make verify` 绿。
- **关键约束**：
  - 弹性靠标准件组合，不自研大框架（ADR-0002）。自研只有 `pkg/resource` 薄 provider + 后续 MQ 消费监督。
  - 版本锁定见 ADR-0002 版本矩阵；otel 全锁 v1.44.0；contrib 锁伪版本禁 `-u`（S0 不引 contrib，但 go.mod 约定写明）。
  - DI = wire；codegen = buf v2 配置；ent 走 `database/sql`+pgx stdlib。
  - 可测单元先写表驱动测试再实现（TDD）；禁 `time.Sleep`，用可注入 clock / 轮询断言。
- **产出即验收**：AC1–AC6（见 §3 与需求包）。

## 2. 文件结构（S0 落地形态）

```
projects/kratos-base/
├── go.mod / go.sum / Makefile / buf.yaml / buf.gen.yaml / AGENTS.md / README.md
├── api/demo/v1/            demo.proto(+HTTP注解) / error_reason.proto / *.pb.go(gen)
├── pkg/
│   ├── resource/          provider.go(Provider[T]) / health.go(Registry) / *_test.go   ← 核心自研薄层
│   ├── bootstrap/         bootstrap.go(读 bootstrap.yaml+INFRA_MODE→config.Source)
│   ├── confcenter/        manager.go(snapshot/version/validate/rollback/subscribe) / *_test.go
│   ├── logx/              logger.go(slog JSON + trace_id + kratos log 适配)
│   ├── errs/              errs.go(kratos errors 助手 + reason 映射)
│   ├── obs/               tracing.go(otel) / metrics.go(prom exporter)
│   └── pgxpool/           pool.go(sql.Open("pgx")+池参数+ping / PoolConfig.Fingerprint)
├── app/
│   ├── demo/              cmd/main.go + internal/{conf,biz,data,service,server} + wire.go/wire_gen.go
│   │   └── internal/data/ent/   ent schema(greet 表)+生成物
│   └── gateway/           cmd/main.go + internal/server/http.go(路由→demo gRPC + /healthz /readyz /metrics) + wire
├── configs/               bootstrap.yaml(选源) / runtime.yaml(server/data/log)
├── deploy/                docker-compose.yaml(postgres) / migrations/(atlas)
└── test/resilience/       三幕弹性脚本(AC1/AC3/AC4) + 观测断言(AC5)
```

依赖顺序：T1 → {T2,T3,T4} → T5(用T4) → T6(用T4,T5,T2) → T7(用T2,T6,T3) → T8(用T7,T4) → T9(全部)。

## 3. 步骤（足够到代码级）

### T1 脚手架 + 工具链
- 建 `projects/kratos-base/`；`go mod init`，go.mod 按 ADR 版本矩阵写死直接依赖 + 工具链版本注释。
- `buf.yaml`(v2) + `buf.gen.yaml`(protoc-gen-go/-go-grpc/-go-http/-openapi)；`Makefile` 目标：`generate`(buf+ent)、`wire`(go generate ./... 或 wire ./...)、`build`、`test`、`lint`、`sandbox-up/down`、`migrate`、`verify`。
- 占位 `app/demo/cmd/main.go`（空 Kratos app 能起停）、工程 `AGENTS.md`(精简+指针)、`README.md`。
- 验证：`go build ./...` 过；`make generate` 跑通（先放最小 proto）；`make verify` 占位过。

### T2 proto + 错误模型
- `api/demo/v1/demo.proto`：`DemoService { Ping(PingReq)→PingResp [GET /v1/ping]; GetGreet(GetGreetReq)→Greet [GET /v1/greet/{id}] }`。`error_reason.proto`：枚举 `DB_UNAVAILABLE / NOT_FOUND / INVALID_ARGUMENT`（带 kratos `errors.proto` 注解）。
- `make generate` 出桩；`pkg/errs/errs.go`：`func DBUnavailable(cause error) error`（包 `kratos errors.New(503,"DB_UNAVAILABLE",...).WithCause`）、`NotFound(...)` 等。
- 验证：生成物存在；`go build ./...`；errs 单测断言 reason/code/HTTP 映射。

### T3 日志 + 可观测
- `pkg/logx/logger.go`：`func New(level string, svc,ver,env string) *slog.Logger`（JSON handler，固定字段）；`func KratosAdapter(*slog.Logger) log.Logger`；从 ctx 取 `trace_id`/`span_id` 注入（`func With(ctx) *slog.Logger`）。
- `pkg/obs/tracing.go`：`func SetupTracer(ctx, cfg TraceConfig) (shutdown func(context.Context) error, err error)`（otel SDK，OTLP 或 stdout exporter，全局 propagator）。`pkg/obs/metrics.go`：`func Registry() *prometheus.Registry` + otel metric→prom exporter；`func Handler() http.Handler`。
- 验证：logx 表驱动单测（输出含 `service/version/level`；ctx 有 trace 时含 `trace_id`）。

### T4 薄 provider 框架（核心，TDD 先行）
- `pkg/resource/provider.go`：
  ```go
  type Adapter[T any] struct {
      Build       func(ctx context.Context, cfg any) (T, error)
      Close       func(T) error
      Fingerprint func(cfg any) string
      Health      func(ctx context.Context, t T) error
  }
  type Snapshot struct { Version uint64; Value any }
  type Source  interface { Current() Snapshot }            // 由 confcenter 提供
  type Provider[T any] struct { /* mu; src; ad; cur T; ready bool; ver uint64; fp string; lastErr error */ }
  func New[T any](src Source, ad Adapter[T]) *Provider[T]
  func (p *Provider[T]) Get(ctx) (T, error)                // 见语义
  func (p *Provider[T]) Healthy(ctx) error                 // 用 cur 跑 ad.Health；未就绪→返回 lastErr/ErrNotReady
  func (p *Provider[T]) Close() error
  ```
  `Get` 语义：取 `src.Current()`；若 `ready && ver/fp 未变` → 返回 `cur`；否则 `Build`：成功→换池(`Close` 旧的)、`ready=true`、清 `lastErr`；失败→记 `lastErr`，**有旧值返回旧值，首次返回 err**。`mu` 保护，并发安全。
- `pkg/resource/health.go`：`Registry`，`Register(name string, check func(ctx)error)`、`Ready(ctx)(bool, map[string]error)`（任一 fail → not ready）。
- 验证（`provider_test.go` 表驱动）：①首次 Build 失败→Get 返 err；②有旧值时 Build 失败→返旧值且不报错；③fingerprint 变→换新值且旧值被 Close；④版本/指纹不变→不重建；⑤并发 Get 安全（`-race`）。health：注册多个 check，全过=ready，任一挂=not ready + details。

### T5 配置两段式（TDD 先行）
- `pkg/bootstrap/bootstrap.go`：`func Load(path string) (Bootstrap, error)`（读 `bootstrap.yaml`，`INFRA_MODE` 环变覆盖 `infra.mode`）；`func NewConfigSource(bs Bootstrap) (config.Source, error)`（S0 只 `file`；nacos/etcd/k8s 留 `case` 分支 + 明确 `errors.New("S5 实现")`，不静默）。
- `pkg/confcenter/manager.go`：
  ```go
  type Snapshot[T any] struct { Version uint64; Value T }
  type Manager[T any] struct { /* mu; cur Snapshot[T]; validate func(T)error; subs []chan Snapshot[T] */ }
  func NewManager[T any](initial T, validate func(T) error) (*Manager[T], error)  // 校验 initial
  func (m *Manager[T]) Current() Snapshot[T]
  func (m *Manager[T]) Publish(next T) error           // 校验失败→返 err 且保留上版；成功→ver++、通知 subs
  func (m *Manager[T]) Subscribe() <-chan Snapshot[T]
  func BindKratosWatch[T any](ctx, c config.Config, keys []string, reload func(config.Config)(T,error), m *Manager[T], log *slog.Logger) error
  ```
  `Manager` 暴露 `func (m *Manager[T]) ResourceSource() resource.Source`（把 `Snapshot[T]` 包装成 `resource.Snapshot{Version, Value:any}` 喂给 provider；避免同名 `Current()` 返回不同类型）。
- 验证（`manager_test.go`）：①合法 Publish→版本++ 且订阅者收到；②非法 Publish→返 err、`Current()` 仍是上版；③多订阅者各收一份；④`BindKratosWatch` 用内存 config 触发变更→Manager 收到（轮询断言，不 sleep）。

### T6 PG 池 + ent（懒加载接入）
- `pkg/pgxpool/pool.go`：`type PoolConfig struct{ DSN string; MaxOpen,MaxIdle int; ConnMaxLifetime,ConnMaxIdleTime,ConnectTimeout time.Duration }`；`func Open(cfg PoolConfig) (*sql.DB, error)`（`sql.Open("pgx",dsn)` + `SetMaxOpenConns/...` + 带超时 `PingContext`）；`func (PoolConfig) Fingerprint() string`（连接相关字段哈希）。
- ent：`app/demo/internal/data/ent/schema/greet.go`（`greet` 表：id、content、created_at；先不引多租户 mixin，S0 极简）；`make generate` 出 ent 代码；atlas 迁移 `make migrate`（CLI 方案，迁移文件入 `deploy/migrations/`）。
- `app/demo/internal/data/data.go`：用 `resource.New[*ent.Client](cfgManager, entAdapter)` 持有 ent；`entAdapter.Build`=`pgxpool.Open`→`entsql.OpenDB(dialect.Postgres, db)`→`ent.NewClient`；`Health`=`db.PingContext`；`Fingerprint`=`PoolConfig.Fingerprint`。`func (d *Data) Ent(ctx) (*ent.Client, error)` = `provider.Get(ctx)`。把 ent provider 的 `Healthy` 注册进 `resource.Registry`。
- 验证：`pgxpool` 单测（Fingerprint 稳定性 + 变更检测；池参数生效）；迁移文件生成；`data` 单测用 `Get` 在无 DB 时返 `DBUnavailable`。

### T7 demo 服务 + 中间件链
- `app/demo/internal/biz`：`GreetUsecase`（`Ping()` 纯内存；`Greet(ctx,id)` 经 repo 读 ent）。`internal/data/greet_repo.go`：实现 repo，DB 不可用时返 `errs.DBUnavailable`。`internal/service/demo.go`：实现 `DemoService` proto。
- `internal/server/grpc.go`：Kratos gRPC server，服务端中间件链 `recovery, tracing.Server, metrics.Server, metadata.Server, ratelimit.Server(BBR), validate`。熔断**不在服务端链上**——它保护**出向依赖**：S0 在 `internal/data` 用 `sre.Breaker` 包住 ent 的 `Get`+查询，连续失败开路、快速返 `DBUnavailable`（支撑 AC3 的"快速失败不挂起"）。
- `internal/conf/conf.go`：runtime 配置结构（server 地址、data.dsn+池参数、log.level、trace）。`wire.go` 装配 provider→data→biz→service→server；`make wire` 生成。
- 验证：service 单测（`Ping` 返回固定 payload；`Greet` 在 PG down 时返 `DB_UNAVAILABLE`、不 panic）；breaker 单测（连续失败后开路、快速失败）。

### T8 gateway + 健康端点
- `app/gateway/internal/server/http.go`：HTTP server；`/v1/*` 转发到 demo（S0 直接用生成的 `demo_http` client 或反向代理到 gRPC）；挂 `/healthz`（liveness：永远 200，只证明进程活）、`/readyz`（调 `resource.Registry.Ready`，全过 200 否则 503 + JSON details）、`/metrics`（`obs.Handler`）。
- `app/gateway/cmd/main.go` + wire；优雅启停（signal→`app.Stop`）。
- 验证：本地起 gateway（无 PG）→ `/healthz`=200、`/readyz`=503；起 PG 后轮询 `/readyz`→200。

### T9 sandbox + 弹性验收 + 接入收尾
- `deploy/docker-compose.yaml`：postgres:16 具名服务（按 onboarding §7 放 `workspace/local-sandbox-docker/` 或工程内，状态文件标识）。
- `test/resilience/`：脚本化三幕——`scen_boot_dep_down`(AC1)、`scen_recover`(AC2)、`scen_runtime_drop`(AC3)、`scen_config_hot`(AC4) + 观测断言(AC5：日志含 trace_id、/metrics 有计数、trace 产出) + 网关链路(AC6)。轮询断言，不 sleep 猜时序。
- 登记 `workspace/verification.yaml`（verify/unit/api/e2e/sandbox 命令与工作目录）；补工程 `internal/data/AGENTS.md`(数据层规矩：用 ent、provider 取 client、非必要不 raw SQL)。
- 验证：见 §3 runbook 全量跑。

## 4. 验证 runbook（映射 AC）

- [ ] `make -C projects/kratos-base build && go vet ./... && golangci-lint run`（编译/静态）
- [ ] `make -C projects/kratos-base test`（含 `-race`）：provider/confcenter/logx/errs/service/breaker 单测全绿
- [ ] **AC1** `scen_boot_dep_down`：PG 关 → 起服务 → `curl /healthz`=200、`/readyz`=503、`/v1/ping`=200、`/v1/greet/1`=503(DB_UNAVAILABLE)、进程存活
- [ ] **AC2** `scen_recover`：起 PG（不重启服务）→ 轮询 `/readyz`→200、`/v1/greet/1`→200
- [ ] **AC3** `scen_runtime_drop`：服务+PG 在用 → 杀 PG → `/v1/greet/1` 快速失败(熔断, <Xms 非超时)、`/readyz`=503、进程存活 → 恢复 PG → 自动转绿、接口成功
- [ ] **AC4** `scen_config_hot`：改 runtime.yaml 值 → 不重启生效；写非法值 → 被拒、用回上版、服务存活
- [ ] **AC5**：日志为 JSON 且含 `trace_id`；`/metrics` 有请求计数；trace 导出可见
- [ ] **AC6**：HTTP 经 gateway 到 demo gRPC 正确返回；中间件链生效（日志/trace/metric 有记录）
- [ ] 证据按 `workspace/verification.yaml` 记录（命令/时间/环境/结果/分类/case id，rule-0002）

## 5. 失败模式与回滚

- **`go mod tidy` 解析与 ADR 矩阵不符**（otel/contrib/http 插件）：以 tidy 实际解析为准，回填 ADR 版本矩阵（ADR 已声明"以 tidy 为准"）。
- **buf v2 配置在本布局 managed mode 不对**：调 `buf.gen.yaml` override；ADR 已标"待验证"。
- **breaker 误伤**（PG 抖动即开路）：调阈值/窗口；breaker 仅包 DB 调用，不影响 `/v1/ping`。
- **回滚**：S0 全部在 `projects/kratos-base/` 内新增，控制面仅动 `workspace/verification.yaml`、`docs/features`、`docs/decisions`、`tasks/`；删工程目录 + 还原这几处即回滚，不影响控制面其它部分。

## 6. 受影响 skill / 文档（rule-0007）

- feature-delivery / context-loading：无需更新（流程/档位未变）。
- 接入产物：工程 `AGENTS.md` + `internal/data/AGENTS.md` + `workspace/verification.yaml` 路由（onboarding 指南 §2/§3）。
- 收尾：L2+ 跑 eval（rule-0005）。
