---
title: 需求产出账本（用户故事 + PRD + 可选原型）
status: active
owner: harness
last_updated: 2026-06-28
source_files: []
related_docs:
  - ../../.agents/skills/hc-prd/SKILL.md
  - ../../templates/user-story.md
  - ../../templates/prd.md
  - ../decisions/0003-prd-elicitation-and-prototype.md
  - ../decisions/0007-prd-workflow-redesign.md
  - ../features/README.md
---

# 需求产出（PRDs）

由 `hc-prd` skill 分阶段产出的 **用户故事 → PRD →（可选）交互原型**，是 `hc-dev`（实现需求）的**上游**。与实现体系**松耦合**：实现不强依赖此处，有 PRD 时可派生 feature 包衔接。流程见 `../decisions/0007-prd-workflow-redesign.md`。

- 模板：`templates/user-story.md`、`templates/prd.md`
- 账本：`index.yaml`（每个 PRD 一条：`id` / `dir`（目录名，`prds-audit` 按此键校验）/ `title` / `prd_status` / 派生 feature）
- 每个 PRD 一个目录：`docs/prds/<id>/`
  - `user-stories.md`：需求的事实视角，**先于 prd.md 产出、approved 才进 PRD**（套 `templates/user-story.md`）
  - `prd.md`：需求正文（须符合已确认用户故事，带功能点清单 + `US↔FP↔正文` 映射）
  - `prototype/`（**可选**）：可点的 HTML 原型（浏览器直开、mock 数据、无后端；有现有前端则与之内容/风格一致，不复刻还原）
