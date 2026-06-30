# candidate — demote-context-loading（L3）

把 `context-loading` 从 skill 降级为政策：删 skill 壳，入口落永远自动加载的 `AGENTS.md`，依据 ADR-0011。

## 候选改动集（git status，未提交）

- `?? docs/decisions/0011-demote-context-loading.md`（ADR）
- `M  docs/decisions/index.yaml`（登记 ADR-0011）
- `D  .agents/skills/context-loading/SKILL.md`（删 skill 壳）
- `M  .agents/skills/README.md`（skills-index regen，已无 context-loading 行）
- `M  AGENTS.md`（启动顺序第 2 条强化 + rule-0004 去"该 skill"、改纯指 CONTEXT_LOADING.md）
- `M  README.md`（技能例删 context-loading）
- `M  .agents/skills/hc-self-evolution/references/process-coverage.md`（两处候选行改为"已降级 ADR-0011 + L0-L6 粒度仍开放"）
- `M  docs/context/CONTEXT_LOADING.md`（修 L4 档引不存在的 docs/architecture/）
- `M  tasks/todo.md` / `M  tasks/lessons.md`（任务管理）

## 候选自述的验证

- 2 agent 独立扫漏网关联项 + 验入口可达，均 clean
- `make verify` + `make docs-audit`（32 篇）绿
- 历史不改写：旧 ADR 受影响栏、task-reviews、features、*-plan、self-evolution 讲"过去 eval-011 fail"的案例均保留
