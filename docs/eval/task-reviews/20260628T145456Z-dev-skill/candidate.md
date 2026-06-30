# 候选产出副本 — dev skill（L4）

任务：新建统一"写代码" `dev` skill（L4），替代并删除旧 `feature-delivery` + `bugfix`。两级（常规/深度）、全程常开纪律、挑刺=对抗 review（派共享 `code-reviewer` 子 agent 双栈）、借用 superpowers + 收尾 eval（引用不重写）、需求包门禁（rule-0001）保留并由 dev 深度级接管。

## 核验的候选产出（文件路径）
- `.agents/skills/hc-dev/SKILL.md` — dev 正文（两级 + 纪律 + 改 bug 子模式 + 重构/迁移子模式 + 升级信号 + 挑刺派活 + 硬规则 + 演进）
- `.claude/agents/hc-code-reviewer.md` — Claude 侧子 agent，`tools: Read, Glob, Grep, Bash`（无 Write）
- `.codex/agents/hc-code-reviewer.toml` — Codex 侧等价定义
- `.codex/config.toml` `[agents.code-reviewer]`（line 25-27，`config_file = "agents/hc-code-reviewer.toml"`）
- `docs/decisions/0009-dev-skill.md` — ADR，含「受影响的 skill（rule-0007）」栏（line 38-45）
- 删除：`.agents/skills/feature-delivery/SKILL.md`、`.agents/skills/bugfix/SKILL.md`（git status 显示 `D`）
- 改的活引用：`prd-elicitation/SKILL.md`、`test-case/SKILL.md`、`self-evolution/SKILL.md` + references（subagents/process-coverage/docs/templates/decisions-context-features/project-onboarding）、`templates/prd.md`、`docs/prds/README.md`、`docs/test-cases/README.md`、`docs/harness/PROJECT_ONBOARDING.md`、`docs/decisions/0003` related_docs、`AGENTS.md`「已有子代理」行
- 索引：`.agents/skills/README.md`、`.claude/agents/README.md`、`docs/decisions/index.yaml`

## 复核的证据（实跑）
- `make verify` → ✓ 控制面自检通过（含 skills/rules/.claude/agents 索引无漂移、rule-0012、AGENTS↔CLAUDE shim、docs-audit 28 篇）
- `make docs-audit` → ✓ 通过（28 篇）
- `ls .agents/skills/feature-delivery .agents/skills/bugfix` → 两者均 No such file or directory
- `.codex/config.toml` → 含 `[agents.code-reviewer]` + `config_file`
- 全仓 grep `feature-delivery` / `bugfix` → 残留均为历史 ADR 受影响栏 / 历史 plan / 历史 log / 任务类型种子 / dev「借自原 bugfix」标注 / lessons 记录，无误导性活路由
