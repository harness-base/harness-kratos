# redisx 就近规约（pkg/redisx）

Redis client builder。本文件是本包红线；工程全局红线见 `../../AGENTS.md`，弹性模型见 ADR-0002。

## 红线
- **Open(ctx, cfg) 必须吃调用方 ctx**：`context.WithTimeout(ctx, DialTimeout)`，不是 `context.Background()`；否则热更探针阻塞 readyz 过其 deadline（eval S6 抓到过）。（client.go:88；lessons 2026-06-23） <!-- rule: kratos/redisx-open-honors-ctx | sev: blocker -->
- **Open ping 失败必须 Close 再返错**：go-redis 已分配连接池，泄漏未关 client 会在重试循环里累积 fd/goroutine。（client.go:91-93；TestOpen_Unreachable） <!-- rule: kratos/redisx-close-on-ping-failure | sev: blocker -->
- **Fingerprint 必须摘要所有影响连接池的字段**：Addrs/Username/Password/DB/PoolSize/超时/TLS 等；漏一个就用 stale 池（如换了地址仍连旧 host）。（client.go:102-117；client_test.go） <!-- rule: kratos/redisx-fingerprint-covers-fields | sev: blocker -->
- **模式由 Addrs 数量自动选**：单地址=standalone，多地址=cluster（NewUniversalClient）；别在 Config 里硬编模式。（client.go:67-72） <!-- rule: kratos/redisx-mode-by-addrs | sev: warn -->

## 指针
- 验收（懒加载/自愈/不阻塞 readyz）：`../../../../docs/features/0002-kratos-base-redis.md`
- adapter 用法：`../../app/demo/internal/data/redis.go`
- 配置映射（为何无 tag）：`../../app/demo/internal/conf/conf.go`
- 镜像设计：`../pgxpool/pool.go`
