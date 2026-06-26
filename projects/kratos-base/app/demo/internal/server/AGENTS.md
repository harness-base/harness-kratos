# server 就近规约（app/demo/internal/server）

HTTP / gRPC transport 与中间件链。本文件是本包红线；工程全局红线见 `../../../../AGENTS.md`。

## 红线
- **metrics.Server 必须在 ratelimit / validate 之上（外层）**：被 ratelimit(429)/validate(400) 拒的请求若绕过 metrics，错误率静默少计。（grpc.go:36-41；metrics_order_test.go 变异自证；lessons 2026-06-24） <!-- rule: kratos/server-metrics-above-ratelimit-validate | sev: blocker -->
- **中间件链必须经 grpcMiddlewares()/httpMiddlewares() helper 构造，不内联**：测试白盒驱动这俩 helper 返回的真实切片来钉顺序；手写内联链绕过校验。（grpc.go:46、http.go:48、metrics_order_test.go） <!-- rule: kratos/server-middleware-via-helper | sev: blocker -->
- **metrics 测试必须绑真实中间件链 + 真实拒绝场景**：空输入夹具会"过"但真请求不过；这是 load-bearing 测试，须能变异（挪 metrics / 破不变量）才红。（metrics_order_test.go:42-45；lessons 2026-06-24 "R9 metrics 测试不绑真实链"） <!-- rule: kratos/server-metrics-tests-bind-real-chain | sev: blocker -->
- **HTTP 链必须含 ratelimit.Server()**：与 gRPC 过载保护对称；漏了 HTTP 入口裸奔且破坏不变量测试。（http.go:40,58；metrics_order_test.go） <!-- rule: kratos/server-http-includes-ratelimit | sev: blocker -->
- **/readyz 用独立 context.WithTimeout(Background, 15s)，不继承 Kratos 请求 ctx**：Kratos 默认 1s 请求超时会提前掐断慢探针（MQ ~10s），误报 503。（http.go:112-114；S6 eval；lessons 2026-06-23） <!-- rule: kratos/server-readyz-dedicated-ctx | sev: blocker -->

## 指针
- 弹性模型 / middleware 表：`../../../../../../docs/decisions/0002-kratos-base-architecture-and-resilience.md`
- 白盒顺序测试（变异自证）：`./metrics_order_test.go`
- 数据层规约：`../data/AGENTS.md`
- S6 readyz 取舍：`../../../../../../docs/eval/task-reviews/20260623T0600Z-kratos-base-s6/`
