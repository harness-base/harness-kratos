# 已归档：搭 agent-harness 骨架（L1，2026-05-29 完成）

> 自 `tasks/todo.md` 归档。

### Checklist
- [x] 设计定稿（spec + ADR-0001）
- [x] 根 + docs 治理层（context / rules / decisions / harness）
- [x] eval 全套
- [x] tasks / templates / scripts / skills / .claude / hooks / CI / 空壳
- [x] `make verify` 自检通过
- [x] Stop hook（收尾触发 eval + 错题本提醒）

## Review
- 跑了 `make verify`：结构齐、docs-audit 过（20 篇）、hook-policy 自测 6/6、eval 资产 OK、skills 无漂移——全绿。
- 本任务属控制面 scaffold（level L1）：无业务验收目标、且未配 `EVAL_API`，故不触发 eval（rule-0005 针对 L2+ 业务任务）。
- 可选后续：`add-rule` skill（待定）、接 `docs-maintainer` / sandbox（等真实工程）。
