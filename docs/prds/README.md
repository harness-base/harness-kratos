---
title: 需求产出账本（PRD + 原型）
status: active
owner: harness
last_updated: 2026-06-24
source_files: []
related_docs:
  - ../../.agents/skills/prd-elicitation/SKILL.md
  - ../../templates/prd.md
  - ../decisions/0003-prd-elicitation-and-prototype.md
  - ../features/README.md
---

# 需求产出（PRDs）

由 `prd-elicitation` skill 通过引导式对话产出的 **PRD + 交互原型**，是 `feature-delivery`（实现需求）的**上游**。与实现体系**松耦合**：feature 实现不强依赖此处，有 PRD 时可派生 feature 包衔接。

- 模板：`templates/prd.md`
- 账本：`index.yaml`（每个 PRD 的 id / 标题 / prd_status / 派生 feature）
- 每个 PRD 一个目录：`docs/prds/<id>/`
  - `prd.md`：需求正文
  - `prototype/`：可点的 HTML 原型（浏览器直开、mock 数据、无后端；有现有前端则与之一致）
