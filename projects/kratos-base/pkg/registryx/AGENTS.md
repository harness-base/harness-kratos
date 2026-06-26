# registryx 就近规约（pkg/registryx）

服务注册：非致命 + 后台重试。本文件是本包红线；工程全局红线见 `../../AGENTS.md`，弹性模型见 ADR-0002 §4。

## 红线
- **注册用 registryx.Runner（非致命后台重试），不用 kratos.Registrar**：Registrar 注册失败会致命、拖垮进程；注册失败只应让"跨服务调用降级"，进程必须活。（ADR-0002 弹性模型；registryx.go） <!-- rule: kratos/registryx-runner-not-kratos-registrar | sev: blocker -->
- **backoff 移位前必须先 clamp 指数**：`1<<uint(attempt)` 在 attempt≥34 溢出成负/零 duration，跳过 cap 判定，退避退化成无退避 CPU 忙等。（registryx.go:117-144；backoff_internal_test.go；lessons 2026-06-23 R13F1） <!-- rule: kratos/registryx-backoff-clamp-before-shift | sev: blocker -->
- **Deregister 必须用 context.Background()**：注销超时独立，不继承 Stop(ctx) 调用方 deadline；父 ctx 已过期会让注销立即失败、服务永远留在注册表。（registryx.go:271） <!-- rule: kratos/registryx-deregister-background-ctx | sev: blocker -->
- **每次 Register 用独立超时（5s）**：别让慢/挂的 etcd/nacos 拨号继承父 ctx，阻塞 Stop 与取消响应。（registryx.go:239-246） <!-- rule: kratos/registryx-register-per-attempt-timeout | sev: warn -->
- **注册成功后 Runner 即 park，不自动重注册**：恢复/热更后的续约靠注册 SDK keepalive，不靠应用循环；别假设应用会自己重注册（会留 stale entry）。（registryx.go:260-268；F-0005 AC-CR2 教训：旧证明是假的） <!-- rule: kratos/registryx-parks-after-success | sev: warn -->

## 指针
- 弹性模型（注册非致命）：`../../../../docs/decisions/0002-kratos-base-architecture-and-resilience.md` §4
- 单测：`./registryx_test.go`、`./backoff_internal_test.go`
- 验收：`../../../../docs/features/0004-kratos-base-conf-registry.md`、`../../../../docs/features/0005-kratos-base-nacos-runtime-resilience.md`
- backoff 同型实现：`../mq/supervisor.go`
