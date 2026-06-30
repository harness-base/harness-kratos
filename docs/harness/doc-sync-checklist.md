---
title: 文档同步对照表（doc-sync checklist）
status: active
owner: harness
last_updated: 2026-06-29
source_files: []
related_docs:
  - HOOKS.md
---

# 文档同步对照表（doc-sync checklist）

"改了 X → 须查 Y 是否跟改"的**唯一真相源**（数据）。原 `doc-sync` skill 降级后（ADR-0012），这张表独立成数据文件，由两处消费：

- `scripts/turn-backstop.sh`（Stop 钩子）：触发时 grep 本表的 `🔴手` 行当判据，复查"动了文件但没同步文档"。
- `hc-doc-sync-reviewer` 子 agent：读本表 + 本轮 `git diff` 报漂移。

**`谁兜底` 列**：`✅机检` = `make verify` 已自动兜底（索引/shim/audit 漂了就红），不用人专门记；`🔴手` = 无机器兜底、只能人手同步——**这几行才是真漂移面、钩子/reviewer 只读这几行**。

| 你改了什么 | 回查什么 | 谁兜底 |
|---|---|---|
| `scripts/*.sh` 加 / 删 / 重命名 | `scripts/README.md` 对应章节是否要更新 | 🔴手 |
| 顶层目录加 / 删 | 根 `README.md` 的目录结构表 | 🔴手 |
| `docs/` 加 / 删子目录 | `docs/README.md` 路由表 | 🔴手 |
| `docs/decisions/*.md` 新建或大改（ADR）| 相关 skill 是否要更新（rule-0007，必填"受影响 skill"栏）+ `docs/decisions/index.yaml` 登记 | 🔴手（skill 回顾要判断；index 那半机检） |
| `AGENTS.md` 加 / 改 `<!-- rule: -->` 标记 | `docs/eval/prompts/` 是否要新增 / 更新考题；跑 `bash scripts/rules-index.sh` 重生成 catalog | 🔴手（考题要判断；catalog 机检） |
| `.claude/agents/*.md` 新建 / 改子 agent | `.codex/agents/*.toml` 对等是否要同步 | 🔴手 |
| `docs/features/*` 状态变更 / 加新 feature | `docs/context/CURRENT_STATUS.md` 被管工程表；`docs/features/index.yaml` 登记 | 🔴手（CURRENT_STATUS 那半；index 机检） |
| harness 模块状态变更（done / planned / skeleton）| `docs/context/CURRENT_STATUS.md` 控制面表 | 🔴手 |
| `scripts/turn-backstop.sh` 或其它 hook 触发逻辑变 | `docs/harness/HOOKS.md`；对应 `*.test.sh` 必须红得起来 | 🔴手 |
| 改了 `workspace/verification.yaml` 路由 | `docs/harness/VERIFICATION_ROUTING.md` | 🔴手 |
| 新建 `AGENTS.md` | 同级必补 `CLAUDE.md`（一行 `@AGENTS.md`）—— `verify-control-plane.sh` shim 段会拦 | ✅机检 |
| 新建 skill 目录 | 跑 `bash scripts/skills-index.sh` 重生成 `.agents/skills/README.md` | ✅机检 |
| 新建 / 改模板 `templates/*.md` | `templates/README.md`（`dir-index.sh` 生成，跑 `bash scripts/dir-index.sh templates`） | ✅机检 |
| 改了 PRD 内容 / 新增 PRD | `docs/prds/index.yaml`；`prds-audit.sh` 守章节齐全 | ✅机检 |

**维护**：新增一类资产 / 文档承诺 / hook，本表加对应行（否则该类漂移没人预防也没人检测）。本表是 `🔴手` 判据的唯一来源，turn-backstop 与 hc-doc-sync-reviewer 都读这里，不另抄子集。
