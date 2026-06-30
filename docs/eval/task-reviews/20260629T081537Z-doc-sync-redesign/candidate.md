# candidate — doc-sync-redesign（L3）

把 `doc-sync` 从 skill 重构为：数据文件 `docs/harness/doc-sync-checklist.md` + `doc-sync-reviewer` 子 agent + 钩子闭环（漂移写 log 带 `- [ ]` 状态 + `correction-nudge` 下一轮反馈 + 处理标 `- [x]`）。预防层取消、只留检测+反馈。依据 ADR-0012。

## 候选产物（当前工作树，未提交）

- 删 `.agents/skills/doc-sync/SKILL.md`；新建 `docs/harness/doc-sync-checklist.md`（数据，9 行 `🔴手` + 6 行 `✅机检`）。
- `.claude/agents/hc-doc-sync-reviewer.md`（model: haiku，tools: Read/Grep/Bash，无 Write）+ `.codex/agents/hc-doc-sync-reviewer.toml`（model_reasoning_effort=low）+ `.codex/config.toml` 注册。
- `scripts/turn-backstop.sh`：读表路径改新文件 + 发现写 `- [ ]` 状态。
- `scripts/correction-nudge.sh`：加"`- [ ]` 待处理 → 注入反馈"；`correction-nudge.test.sh` 加 case 6/7（守护测试）；`turn-backstop.test.sh` case 4 接通闸改指新文件。
- rewire：根 README、`docs/harness/HOOKS.md`、`docs/context/CURRENT_STATUS.md`、`docs/README.md`、`docs/harness/README.md`、`.agents/skills/README.md`、`.claude/agents/README.md`、self-evolution 5 处 references（gates-hooks/docs/subagents/skills/lessons-memory）。
- `docs/decisions/0012-doc-sync-redesign.md` + 登记 `docs/decisions/index.yaml`。
- 已 dogfood：派 doc-sync-reviewer 抓到种下的漂移、不误报真改动；`make verify` + `docs-audit`（36 篇）全绿。

> 评委独立实跑复核要点见同目录 `decision.md`。
