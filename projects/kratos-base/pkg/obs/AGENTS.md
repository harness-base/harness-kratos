# obs 就近规约（pkg/obs）

可观测（trace / metrics）。本文件是本包红线；工程全局红线见 `../../AGENTS.md`。

## 红线
- **空 Endpoint 强制 AlwaysSample（反直觉陷阱）**：本地开发（Endpoint=""）无视 sample_ratio 一律全采，`sample_ratio=0` 仍出全部 span 而非零；改前务必懂这个"藏掉'一个 span 都没有'回归"的坑。（tracing.go:117-119；tracing_test.go；R9F4） <!-- rule: kratos/obs-empty-endpoint-always-sample | sev: blocker -->
- **TraceConfig 字段必须 json+yaml 双 tag**：config.Scan 走 json.Unmarshal，缺 json tag 的 sample_ratio 静默零值→生产无 span。（../../app/demo/internal/conf/conf.go；lessons 2026-06-02） <!-- rule: kratos/obs-trace-config-dual-tag | sev: blocker -->
- **跨服务 ParentBased 时必须传播 W3C TraceContext 头**：0<ratio<1 时采样决策随父 span 上下文，头丢了下游独立采样、链路断裂。（tracing.go:114-127） <!-- rule: kratos/obs-propagate-tracecontext | sev: blocker -->
- **本地用 Simple、远程用 Batch span processor，别混**：Simple 同步导出→本地 stdout 立即可见（AC5 承诺）；Batch 排队走吞吐。（tracing.go:76-81） <!-- rule: kratos/obs-simple-vs-batch-processor | sev: warn -->
- **SetupTracer 全局只在启动调一次并 defer shutdown**：全局 TracerProvider 是单例，多次调用静默覆盖（错 exporter/sampler）。（tracing.go:38-98） <!-- rule: kratos/obs-setup-tracer-once | sev: warn -->

## 指针
- 弹性 / 可观测：`../../../../docs/decisions/0002-kratos-base-architecture-and-resilience.md`
- 双 tag 规约：`../../AGENTS.md`
- 启动序列：`../../app/demo/cmd/main.go`
- 中间件位置（tracing）：`../../app/demo/internal/server/http.go`
