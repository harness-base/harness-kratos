# candidate — rename-hc-prefix（L2）

## 候选产物（当前工作树，未提交）

任务：项目定名 `harness-control`，存量 5 个 skill + 10 个子 agent 全量改名加 `hc-` 前缀（`prd-elicitation` 缩短为 `hc-prd`，其余加前缀），改全所有活引用。

- **5 skill 目录改名**（`git mv`）：`prd-elicitation→hc-prd`（缩短）、`dev→hc-dev`、`add-rule→hc-add-rule`、`git-workflow→hc-git-workflow`、`self-evolution→hc-self-evolution`（连同其 references 子目录全移）。`test-case` 暂留原名（待 `hc-test` 重建）。
- **10 子 agent 改名**（双栈 + 注册）：`.claude/agents/*.md`（code-reviewer / doc-sync-reviewer / eval / feature-point-writer / prd-reviewer / prd-writer / prototype-builder / requirements-gatherer / self-optimize / user-story-writer 全加 `hc-`）+ `.codex/agents/*.toml` 同步 + `.codex/config.toml` 的 `[agents.X]` 注册跟改。
- **活引用跟改**：workflow `hc-prd/references/orchestration-workflow.js` 的 8 处 `agentType:`；frontmatter `source_files`/`related_docs` 路径；prose 互引（`dev`/`eval`/`self-evolution` 语义词逐处判，保 `make eval`/`docs/eval`/`/dev/null`/自进化闭环不动）；顺带 `agent-harness→harness-control`。
- **历史不改写**：`docs/decisions/`、`docs/eval/task-reviews/`、`tasks/lessons.md`、`tasks/optimization-log.md`、`tasks/archive/`、`docs/superpowers/` 正文 prose 保留旧**名**；但移动了的文件的**路径引用**（frontmatter `related_docs` + prose 内 `文件:行` 路径锚点）跟改，指向真实存在的新路径。
- **决策落文档**：`docs/decisions/0013-project-naming-hc-prefix.md`（ADR-0013，已入 decisions/index.yaml）+ `tasks/lessons.md` 2026-06-30 坑（缩短式改名误产 `hc-prd-elicitation`）+ `tasks/todo.md` Review 段。
- 自动索引 `skills-index` / `dir-index` 重生成：`.agents/skills/README.md`（5 hc- skill）、`.claude/agents/README.md`（10 hc- agent）。

候选声称：`make verify` 全绿、活文件 0 bare 旧名残留、无 `hc-hc-` 双前缀、`/dev/null` 完好；改动 89 文件（40 git mv + 48 改内容）。
