# sandbox 索引（kratos-base 本地验证环境）

> 被管工程的可复现本地依赖环境。内容非 .md，本索引静态维护（不进 dir-index 自动生成）。

- `docker-compose.yaml` — PG / Redis / MQ 等本地依赖容器
- `initdb/` — PG 初始化迁移 SQL（与 ent schema 一致，容器 initdb 应用）
- `rocketmq/` — rocketmq broker 本地配置

**怎么用**：由 `../../test/resilience/run_all.sh` 驱动（一键起 / 跑弹性验收 / 销，CWD 无关）。验证路由见仓库根 `workspace/verification.yaml` + `docs/harness/VERIFICATION_ROUTING.md`。
