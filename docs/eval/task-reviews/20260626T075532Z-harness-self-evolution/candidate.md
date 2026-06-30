# candidate — harness-self-evolution（范围）

本批 harness 控制面改动（task `harness-self-evolution`），评审范围：

1. **新增 `self-evolution` skill**：`.agents/skills/hc-self-evolution/SKILL.md`（规范检查层编排）+ `references/` 13 份维度审查手册。
2. **① / ② 拆分**：`scripts/turn-backstop.sh` 正名"落文档提醒"（①）；`.claude/agents/hc-self-optimize.md` + `.codex/agents/hc-self-optimize.toml` 对齐为②深审执行器；rule-0011（`AGENTS.md`）改①专属；`tasks/optimization-log.md` header；ADR-0005（`docs/decisions/0005-self-evolution-loop.md`）加订正。老 `self-optimize` skill 已删。
3. **补缺口**：`docs/context/CURRENT_STATUS.md` 同步；`scripts/skills-index.sh` 头部假命令修正；ADR-0001/0003 补"受影响 skill"栏；新 `scripts/index-audit.sh`（decisions/features 索引一致性）；`scripts/verify-control-plane.sh` 加"路由工程路径可达"检查；新 `scripts/dir-index.sh`（context/harness/templates/.claude/agents 自动索引）+ 5 个区补索引；装 git hooks；新建 `bugfix` skill。
4. `tasks/optimization-log.md` 记 ✅/⏳ 缺口清单；`tasks/self-evolution-plan.md` 是方案。

评审方法：对每个新检查做 mutation 自证（改坏被检对象确认 `make verify` 变红、还原变绿）；抽查 references 的命令/路径/案例是否 grounded；核 ①/② 拆分是否干净、有无悬空引用；亲跑 `make verify` / `make docs-audit` 验真绿；逐条核 optimization-log 的 ✅ 是否属实、⏳ 是否如实标未做。

套用考题：010（收尾综合）、011（架构变更同步 skill）、012（锚定断言/load-bearing 检查）。
