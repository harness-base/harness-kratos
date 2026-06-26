# pgxpool 就近规约（pkg/pgxpool）

PG 连接池 builder。本文件是本包红线；工程全局红线见 `../../AGENTS.md`，弹性模型见 ADR-0002。

## 红线
- **Open(ctx, cfg) 必须吃调用方 ctx**：ping deadline = min(ctx.Deadline(), ConnectTimeout)，禁止 `context.Background()`+仅 ConnectTimeout；否则热更探针阻塞 readyz 过其 deadline，把配置变更失败伪装成超时。（pool.go:69-70；pool_test.go TestOpen_HonorsCanceledContext；lessons 2026-06-23） <!-- rule: kratos/pgxpool-open-honors-ctx | sev: blocker -->
- **Open 返回前必须 PingContext，失败则 Close+返错**：跳过 ping 会把连接故障藏到首次使用，破坏急连通性检查、令 readyz 不可靠。（pool.go:72-75；ADR-0002 readiness=ping） <!-- rule: kratos/pgxpool-ping-before-return | sev: blocker -->
- **PoolConfig 本身不加 config 反序列化 tag**：它不被 config.Scan 直接扫；tag 在 wrapper（DataConfig.Database），SelectPool() 手工构造。错放 tag 会误导"可直扫"而藏 bug。（../../app/demo/internal/conf/conf.go；pool.go:18-25；lessons 2026-06-02） <!-- rule: kratos/pgxpool-no-config-tags-on-poolconfig | sev: warn -->

## 指针
- 弹性模型 / 自愈池：`../../../../docs/decisions/0002-kratos-base-architecture-and-resilience.md`
- 配置映射（SelectPool）：`../../app/demo/internal/conf/conf.go`
- 镜像设计（同样吃 ctx）：`../redisx/client.go`
- 热更 e2e：`../../test/resilience/scen_cc_runtime_down.sh`
