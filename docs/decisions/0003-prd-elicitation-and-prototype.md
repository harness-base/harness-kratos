---
title: ADR-0003 需求产出（PRD + 交互原型）作为 feature-delivery 的上游
status: accepted
date: 2026-06-24
last_updated: 2026-06-24
source_files: []
related_docs:
  - ../../.agents/skills/prd-elicitation/SKILL.md
  - ../../templates/prd.md
  - ../prds/README.md
---

# ADR-0003：需求产出（PRD + 交互原型）作为 feature-delivery 的上游

## 背景

harness 现有 `feature-delivery` 管"**实现**需求"（立需求包 → 测试就绪 → 实现 → 验证 → 收尾），但缺"**产出**需求"的上游能力：从一个模糊想法或一个现有系统，通过沟通把"做什么"想清楚，并产出**可观测验收的 PRD** 与**能点的交互原型**。`feature-package.md` 是开发向的薄需求包，不承担产品向的需求澄清与原型。

## 决策

新增 skill `prd-elicitation`（需求产出），定位为 feature-delivery 的**上游**：

1. **引导式多轮对话**产出需求——分轮澄清（用户/JTBD → 页面/流程 → 数据 → 四态+边界 → 验收目标 → 非目标），关键抉择用 `AskUserQuestion`，不静默假设。
2. 产出**PRD**（`templates/prd.md`）+ **可点的交互 HTML 原型**——原型即"视觉 + 交互规格一体"；**项目若已有前端，原型与其内容/风格一致**（不为"还原前端"单开重活）。
3. 收尾用**对抗式完整性评审**（多 agent 证伪，复用代码评审已验证有效的法子）确认 AC 可观测、四态齐、流程闭合、原型可点通、假设已确认。

**产物归属与耦合**：PRD/原型放**独立目录** `docs/prds/<id>/`（账本 `docs/prds/index.yaml`、原型 `docs/prds/<id>/prototype/`）。与 feature 实现体系**松耦合**——`feature-delivery` 不强依赖 PRD 存在；有 PRD 时一份可派生 1..N 个 feature 包，平滑衔接。

## 标准（done 的判据）

- PRD：每条验收目标可观测可验证；范围 in+out 都写死；每页有四态+边界；假设显式；验收目标↔页面↔测试 可追溯。
- 原型：浏览器直开、mock 数据、无后端；能点通主流程 + 关键四态；有现有前端则与之一致。
- 过程：引导式、多轮、收敛、不静默假设。

## 受影响的 skill（rule-0007）

- skill：prd-elicitation ／ 是否已更新：是（本 ADR 新建）
- skill：feature-delivery ／ 是否已更新：否（松耦合上游，交付流程未变）

## 备选与取舍

- **并进 feature-delivery / feature-package.md**（不另立）：否决——会把产品向需求澄清与开发向切片混在一层；用户明确要"独立目录、松耦合、互相可衔接"。
- **重型"还原现有前端"子流程**（抓 DOM/像素级复刻）：否决——用户判定不必要；原型只需与现有前端**内容/风格一致**即可，保持轻量。

## 影响

- 新增 `.agents/skills/prd-elicitation/`、`templates/prd.md`、`docs/prds/`（README + index.yaml）。
- skills 目录索引经 `scripts/skills-index.sh` 重生成；`make verify` 纳入校验。
- 不改 `feature-delivery` 语义（松耦合，二者独立可用）。
