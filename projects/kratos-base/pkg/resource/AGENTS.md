# resource 就近规约（pkg/resource）

懒加载+自愈 Provider 脊柱。本文件是本包红线；工程全局红线见 `../../AGENTS.md`，弹性模型见 ADR-0002 §4。

## 红线
- **Healthy 置 ready=false 前必须比对句柄身份**：探针释放 `p.mu` 后并发 `Get` 可能已重建并装入新活句柄 `p.cur`；失败路径只能关自己探的那个 stale `t`，绝不能关 `p.cur`，否则误关健康资源、误判 not-ready（单连接句柄如 `*amqp.Connection` 才暴露，池化的会掩盖）。（provider.go:118-132；lessons 2026-06-12；eval s3-rereview） <!-- rule: kratos/resource-healthy-toctou-guard | sev: blocker -->
- **Healthy 失败必须置 ready=false**：即使 version/fingerprint 没变，健康探针失败也要清缓存逼下次 `Get` 重建；否则死句柄留缓存被反复返回、掩盖故障。（provider.go:126-131；lessons 2026-06-12） <!-- rule: kratos/resource-healthy-failure-marks-not-ready | sev: blocker -->
- **Adapter.Build 必须吃调用方 ctx**：急探针（ping/握手）尊重调用方 deadline，禁止 `context.Background()`+自带超时；否则坏 DSN 让 `/readyz` 干等 DialTimeout 而非随 ctx 取消即返。（正例 ../pgxpool/pool.go:69；lessons 2026-06-23；eval s5） <!-- rule: kratos/resource-builder-respects-ctx | sev: blocker -->
- **Fingerprint 必须含所有运行时可变的连接字段**：凭据/超时/池大小/TLS 任一遗漏，配置热更检测不到、继续返旧句柄，违背"配置变更→重建"契约。（../pgxpool/pool.go:82-93、../redisx/client.go:102-117；ADR-0002） <!-- rule: kratos/resource-fingerprint-covers-config | sev: blocker -->
- **Adapter.Close 必须幂等**：并发 rebuild 与 Healthy 失败路径可能对同一句柄双关，双关不许 panic / 坏状态。（provider.go:86-87,127-128） <!-- rule: kratos/resource-close-idempotent | sev: warn -->

## 指针
- 弹性模型：`../../../../docs/decisions/0002-kratos-base-architecture-and-resilience.md` §4
- 吃 ctx 的 Open 正例：`../pgxpool/pool.go`、`../redisx/client.go`
- 端到端自愈证明：`../../test/resilience/run_all.sh`
- 数据层用法：`../../app/demo/internal/data/AGENTS.md`
