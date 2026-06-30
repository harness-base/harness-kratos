---
title: ADR-0001 Agent Harness 骨架设计
status: accepted
date: 2026-05-29
last_updated: 2026-05-29
source_files: []
related_docs:
  - ../context/PROJECT_BRIEF.md
---

# ADR-0001：Agent Harness 骨架设计

## 背景

从 0 搭一套通用 agent 控制面（harness），治理 AI agent 在被管工程上的开发。参考了 `cms-builder` 的落地机制和既有控制面的治理深度，目标是**完善而优雅**：该有的能力都在，但核心小、边界清。

## 决策

采用「**最小内核 + 可挂载模块**」：

- **内核**：`AGENTS.md`（入口 + 红线）、`tasks/`（任务状态 + 错题本）、`docs/`（按需加载的文档）、`scripts/`（验证）。
- **从 cms-builder 借的机制**：自包含技能（`.agents/skills/`）、错题本（`tasks/lessons.md` 三段式）、轻量 todo（干完即清空归档）、文档 frontmatter + `docs-audit` 自检、带测试的 hook policy、分层快验证。
- **保留的治理深度**：按需加载档位、改业务代码前的需求包闸门、验证路由、评分体系（eval）。
- **关键取舍**：
  - `rules/` 保留并第一天建——与 eval 强绑定，规则带编号供 eval/ADR 按号引用；`AGENTS.md` 只放红线 + 指向规则库，不复制。
  - `features/` 保留机制但建成空账本，真需求随被管工程填。
  - 不建 `workflows/` 目录——流程改用 `.agents/skills/`（流程即技能）。
  - 不照搬 `cms-builder` 的 JS 工具链产物与业务目录（`apps/`、`packages/`、`.turbo/` 等）——那些属于被管工程，进 `projects/`。
  - eval 默认走 **eval 子 agent**（`.claude/agents/hc-eval.md` + `.codex/agents/hc-eval.toml`，用会话模型、免 key，Claude Code / Codex 双运行时）；`run-eval.sh`（curl 调外部 LLM）为可选 CI/headless 路径。不用 Python、不接外部知识库服务。
  - 去掉 `knowledge/`（不接外部材料层）；去掉 `skills-lock.json`（产品运行时的件，非控制面）。
  - `docs-maintainer` skill 先不接（待接入）；`docs-audit` 脚本保留。
  - skill 随工程演进：错题本 + 大改时回顾（ADR 模板「受影响 skill」+ `rule-0007`）。
  - 脚本/工具语言 = **bash**（通用、零编译；通用框架不绑 Go），不引入 Node 工具链。
  - 增 `rule-0007`（改架构/接口须回顾相关 skill）、`rule-0008`（外部材料不自动采信）。
  - **Stop hook**（`scripts/stop-check.sh`）收尾兜底：L2+ 任务无 eval 产出则拦，并提醒记错题本。
  - 加 `add-rule` skill（规则落地标准流程）；起步 4 个 skill 用 skill-creator 写法，`skills-index` 防漏登记。
  - 嵌套加载收紧：读/改某目录代码前向上读最近 `AGENTS.md`，与档位叠加（见 `CONTEXT_LOADING.md`）。
  - **git：先不 init**（文件铺好，由用户 init / commit）。

## 受影响的 skill（rule-0007）

- skill：全部（`context-loading` / `feature-delivery` / `git-workflow` / `add-rule` 等）／是否已更新：n-a（奠基 ADR，skill 体系本身由本骨架确立）

## 备选方案

1. **文档分层式**（纯文档树 + 脚本）：简单但状态易散、靠自觉。
2. **契约优先 + 单一状态源**：机器可校验但前期 schema 重。
3. **最小内核 + 可挂载模块**（采用）：吸收方案 2 的状态集中做内核，重层做成按需挂载模块，最贴合"控制面先行、逐步接工程"。

## 影响

- 现在建实控制面核心；`.codex/` 已接 eval 子 agent（其余按需）；`architecture/`、`prds/`、`drift/`、`docs-maintainer` skill、sandbox/E2E 等待接 `projects/` 再建。
- 完整设计稿见 z-mate-control 仓 `docs/superpowers/specs/2026-05-29-agent-harness-skeleton-design.md`。
