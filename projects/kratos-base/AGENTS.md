# kratos-base（被管工程入口规则）

Kratos v2 微服务 monorepo。本文件**精简**：只列工程红线 + 指针。
控制面常驻规则以仓库根 `AGENTS.md` 为准；冲突时**用户当前指令 > 本文件 > 根 AGENTS.md**。

## 工程红线

- **生成代码入库、且只由 codegen 产出**：`api/**/*.pb.go`（buf）、`app/**/internal/data/ent/`（ent）不手改；改 `.proto` / ent schema 后跑 `make generate` 重生成并提交。
- **codegen 离线可复现**：proto 导入依赖 vendored 在 `third_party/`（不走 buf BSR，本环境访问不到）。新增导入先补 `third_party/`，别引 BSR 依赖。详见 `third_party/README.md`。
- **DI 用 wire**（编译期注入）：装配改了跑 `make wire` 重生成 `wire_gen.go`（生成物入库）。不手写注入图。
- **依赖懒加载、不宕服务**（ADR-0002 核心）：外部依赖（PG/Redis/MQ/配置中心/服务发现）启动不连、首用才连、连不上清晰报错但进程存活、恢复自动续上。新依赖按 ADR §4 弹性策略表接入。
- **版本锁**：直接依赖按 ADR-0002 版本矩阵，**事实源是干净的 `go mod tidy`**；只引真正用到的包（别为凑矩阵引 unused）。otel 全锁同一版本；contrib 锁伪版本、禁 `-u`。
- **测试**：表驱动；同步等待用可注入 clock / 轮询断言，**绝不 `time.Sleep`**；不许 mock/fake 冒充真实行为。
- **e2e 断言锚定产出方证据**（rule-0009）：锁结构化字段+业务 id（如 `"consumer":"received"` 且 key==事件 id），禁裸串全文 grep（访问日志回显入参会假阳性）、禁"任意相关行也算过"兜底分支；跨系统链路配对端侧证据（如队列深度）。S3 实案见 `tasks/lessons.md` 2026-06-12。
- **Kratos 配置结构体一律 json+yaml 双 tag**：`config.Scan` 走 json.Unmarshal，缺 json tag 的 snake_case 字段会**静默解析为零值**（已踩坑：池参数/采样率全默认，单测不报；见控制面 `tasks/lessons.md` 2026-06-02）。
- **三种弹性模型不混用**（ADR-0002 澄清）：数据面=懒加载+自愈+熔断（resource.Provider）；配置中心=**急加载** fail-fast + watch 热更；注册=非致命+后台重试（registryx.Runner，不用 kratos.Registrar——它注册失败会致命）。
- **不擅自 git 写操作**（根 AGENTS.md 红线）。

## 指针

- 设计/选型 + 版本矩阵 + 弹性模型澄清：`../../docs/decisions/0002-kratos-base-architecture-and-resilience.md`
- 需求包账本（F-0001 地基 / F-0002 Redis / F-0003 MQ / F-0004 配置+发现）：`../../docs/features/index.yaml`
- 实施计划（按片）：`../../tasks/kratos-base-s0-plan.md`、`...-s1-plan.md`、`...-s3-plan.md`、`...-s4-plan.md`
- 怎么 build / generate / test：`README.md`；弹性验收场景：`test/resilience/`（`run_all.sh`，CWD 无关）
- 数据层规矩（用 ent、provider 取 client、非必要不 raw SQL）：`app/demo/internal/data/AGENTS.md`
- 各包就近红线（resource / mq·rabbitmq·rocketmq / registryx / confcenter / pgxpool / redisx / obs / server / test）：见各自 `AGENTS.md`；全部规则总览（harness + 项目）：`../../docs/rules/index.yaml`

## 验证

见 `README.md` 与控制面 `workspace/verification.yaml` 路由。最小收口：`make verify`（build+vet+lint+test）。
