# 数据层规约（demo/internal/data）

本文件是数据层红线；工程全局红线见 `../../../../AGENTS.md`。

## 红线

- **用 ent，非必要不写 raw SQL**：数据模型变更走 ent schema（`ent/schema/`），改完跑 `make ent`；复杂查询写 ent builder，不绕过 ent 写 `database/sql` 或字符串拼 SQL。
- **DB 访问经 `Data.Ent(ctx)`**（懒加载 provider）：不直接持有 `*ent.Client`，始终通过 `data.Ent(ctx)` 取客户端；首次调用触发连接，之后按 fingerprint 缓存并自愈。
- **错误经 `errs` 包映射**：基础设施故障（连不上、驱动错误）→ `errs.DBUnavailable(cause)`；业务缺失（行不存在）→ `errs.NotFound(...)` 且**不触发熔断**；其他 ent 错误包进 `errs.DBUnavailable`。
- **健康检查注册进 Registry**：`data.Healthy`（封装 `provider.Healthy`，最终调 `db.PingContext`）必须在 wire 装配时注册到 `resource.Registry`（key `"postgres"`），驱动 `/readyz` 的 readiness 判断。
- **熔断在数据层**：`GreetRepo`（以及未来新 repo）用 `sre.Breaker` 包住 DB 调用；breaker 只包基础设施路径，不包纯内存路径；连续失败开路→快速失败→上报 `DBUnavailable`。

## 指针

- ent schema：`ent/schema/`
- provider 层（薄 provider + Registry）：`../../../../../pkg/resource/`
- 错误助手：`../../../../../pkg/errs/`
- 配置类型 + SelectPool：`../conf/conf.go`
