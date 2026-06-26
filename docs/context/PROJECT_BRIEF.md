---
title: 项目简报
status: active
owner: harness
last_updated: 2026-06-12
source_files: []
related_docs:
  - CURRENT_STATUS.md
  - ../decisions/0001-harness-skeleton-design.md
---

# 项目简报

## 目标

从 0 搭一套**完善而优雅**的 agent 控制面（harness）：用「最小内核 + 可挂载模块」治理 AI agent 在被管工程上的开发。

## 边界

- **控制面先行**，被管工程挂进 `projects/`（首个已接：`kratos-base`，实时状态见 `CURRENT_STATUS.md`）。
- 是**通用、内容轻的骨架**，不塞具体业务内容。
- 控制面只做「路由 / 执行 / 评分 / 收证据」，**不存放业务代码与业务测试本体**——那些在 `projects/<name>/` 里。

## 核心理念

1. 小内核 + 按需长。
2. 任务状态集中在 `tasks/`，不散。
3. 规矩能机检自动拦（hook，带测试），不靠自觉。
4. agent 有错题本（`tasks/lessons.md`），越做越聪明。
5. 文档能自检（frontmatter 依赖 + `docs-audit`）。
6. 流程即技能（`.agents/skills/`），不写成没人点开的说明文。
7. 质量有评委（`docs/eval/`），不靠 agent 自评。

设计来源与取舍见 `../decisions/0001-harness-skeleton-design.md`。
