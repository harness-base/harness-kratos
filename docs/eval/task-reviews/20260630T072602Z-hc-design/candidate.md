---
title: hc-design build 收尾评审 — candidate（候选副本/清单）
status: active
owner: hc-eval
task: hc-design
generated_at: 2026-06-30T07:26:02Z
---

# 候选产物清单（本批交付，ADR-0015）

git status 实测本批改动（??=新建，M=改）：

- ?? docs/decisions/0015-hc-design.md — ADR（已登记 decisions/index.yaml L74-78）
- ?? .agents/skills/hc-design/SKILL.md — 交互式设计 skill
- ?? .claude/agents/hc-design-reviewer.md — Claude Code 栈 reviewer（tools 无 Write、无 model）
- ?? .codex/agents/hc-design-reviewer.toml — Codex 栈 reviewer（model_reasoning_effort=high）
- M  .codex/config.toml — 注册 [agents.hc-design-reviewer]
- ?? templates/design.md — 研发方案 9 段模板
- ?? templates/api-contract.md — 接口契约模板（单独文件）
- ?? docs/designs/ — 产物区（README + index.yaml，空账本）
- M  docs/harness/testing-flow.md — api 用例线指向 hc-design 为契约来源
- M  .agents/skills/README.md / .claude/agents/README.md / templates/README.md / docs/decisions/index.yaml — 各 index 同步
- M  tasks/lessons.md / tasks/todo.md — 任务台账

候选全文见上述各源文件路径（评委已逐一 Read 取证，不在此重复粘贴）。
逐题判定与取证见同目录 decision.md；分档见 summary.md。
