---
name: doc-sync
description: 改了配置 / 脚本 / 接口 / 目录结构 / 规则 / ADR 之后用，对照 checklist 检查相关文档（README / AGENTS.md / CURRENT_STATUS.md / 等）是否要同步——挡在"改了代码忘改文档"前面。被 turn-backstop（钩子）兜底，但这里是主动提醒。
version: 2
last_reviewed: 2026-06-27
---

# 文档同步自检（doc-sync）

改东西要顺手改对应文档。本 skill 是**提醒型**——给你一份 checklist，按改动类型回查相关文档。**不是替你改，是不让你忘**。

被动兜底是 `scripts/turn-backstop.sh`（钩子级，Haiku 复查），那是漏了之后捞；本 skill 是漏之前先想到。

## 何时用 / 何时不用
- 用：刚改了脚本 / 配置 / 接口 / 目录结构 / 规则 / ADR 之后；或者准备收尾前回扫一遍。
- 不用：纯探索、读代码、不改文件——没什么可同步。

## 步骤
1. 看你这轮改了什么（`git status` / `git diff --stat`）。
2. 对照下方 **checklist** 找匹配项，回查对应文档是否要跟。
3. 真要改就改；不需要改就在心里勾一下（顺路写进 PR 描述也行）。

## checklist

**`谁兜底` 列**：`✅机检` = `make verify` 已自动兜底（索引/shim/audit 漂了就红），你不用专门记；`🔴手` = 无机器兜底、只能人手同步，**这几行才是真漂移面**。本表是"改完查什么"的**唯一来源**——`scripts/turn-backstop.sh`（钩子兜底）直接读本表的 `🔴手` 行当判据，不再自抄一份子集（改本表标记/行时，turn-backstop 自动跟着变）。

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
| 新建 / 改模板 `templates/*.md` | `templates/README.md`（由 `dir-index.sh` 自动生成，跑 `bash scripts/dir-index.sh templates` 重生成） | ✅机检 |
| 改了 PRD 内容 / 新增 PRD | `docs/prds/index.yaml`；`prds-audit.sh` 守章节齐全 | ✅机检 |

**没匹配上但改动看着不轻** → 跑一下 `make verify` 看是不是有索引漂移没看到（很多文档同步靠 `*-index.sh --check` 兜，verify 红了就是提示）。

## 硬规则
- 不复刻规则正文进 README / docs（事实源在 `AGENTS.md`，rule-0008）。
- 改完跑 `make verify`——任何索引漂移当场暴露，不要"以为同步了"。
- 真做不到同步的就在 `tasks/optimization-log.md` 留一条，下次自进化时处理（rule-0011），别假装没事。

## 演进（rule-0007）
新增加了某类资产 / 文档承诺 / hook，本 checklist 加对应行；改完跑 `bash scripts/skills-index.sh` 重生成技能索引（本 skill 的 frontmatter 改了也要 `version` / `last_reviewed` 同步）。
