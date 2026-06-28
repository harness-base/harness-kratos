---
title: ADR-0010 prd-elicitation 编排式重构——产品总监调度专职 worker（必选/可选·权重 + 确认门 + 并行 + review loop）
status: accepted
date: 2026-06-29
last_updated: 2026-06-29
source_files:
  - ../../.agents/skills/prd-elicitation/SKILL.md
related_docs:
  - 0003-prd-elicitation-and-prototype.md
  - 0007-prd-workflow-redesign.md
  - ../superpowers/specs/2026-06-29-prd-orchestration-design.md
---

# ADR-0010：prd-elicitation 编排式重构

## 背景

`prd-elicitation`（ADR-0007）是一条线性交互 skill。讨论后要把它重构成**编排式**：一个**产品总监**（编排逻辑）调度一队**专职 worker**，每个 worker 可各配模型 / 提示词 / 数量，并用 必选/可选·权重 防小需求过度走流程、又不盲目省略。动因：模块化、轻重自适应、地基（用户故事 + AC）早审。设计详见 `../superpowers/specs/2026-06-29-prd-orchestration-design.md`。

## 决策

1. **产品总监 = 编排逻辑**，**主 agent 当总监**（按 SKILL 总谱 + Workflow 编排模板派 worker）。常驻自主"产品总监 agent"**押后**到把 harness 封装成自主 agent 那一步。

2. **三层优先级**：**用户明确指令**（最高，覆盖一切默认，含必选；体现 `AGENTS.md`"用户指令 > 本文件"）> **必选**（默认 always）> **可选·权重**（按权重 + 触发判据）。覆盖必选时先**提示后果** → 用户坚持照办 → **留痕**（审稿/eval 不当缺陷扣）；跳过可选步留一句理由。

3. **7 个 worker**：需求采集员（必选·人在环）、外部调研员（可选·权重低，**复用 `deep-research`**，过 rule-0008 验收）、用户故事+AC 员（必选）、PRD 本体员（必选）、功能点清单员（必选）、原型员（可选·权重中）、PRD 审稿员（必选）。前 6 个建成 **subagent 双栈**（`.claude/agents/*.md` + `.codex/agents/*.toml` + `config.toml` 注册）；外部调研复用现成 skill。

4. **时序**：需求采集 →（可选）外部调研 → 用户故事+AC →（**轻审 loop** 1-2 轮：AC 可观测/故事完整/内部一致/对齐采集）→ **确认门（用户 approved）** → 并行产出 [PRD 本体 ∥ 功能点 ∥（可选）原型] → **PRD 审稿员重审 loop（框住并行产出）** → 收尾确认。review loop 的"修"=**回原 worker 角色重跑**（每轮只重跑被审出问题的 worker），独立性靠审稿员每轮复审；"方向整个错"才回更上游。

5. **两个 review 点（轻重分明）**：用户故事+AC **轻审**（地基四项，1-2 轮，确认门前 shift-left）；整套 PRD **重审**（多轮对抗到零，查下游跟地基一致）。两处用**同一个 prd-reviewer**，轮数/侧重不同。

6. **PRD 审稿 ≠ code review**：`prd-reviewer` 独立子 agent，rubric = eval 013 / rule-0010；与 `code-reviewer` 分开。

## 受影响的 skill（rule-0007）
- skill：prd-elicitation ／ **是**——本 ADR 即其编排式重写（SKILL 改为总谱、version→3）。
- skill：test-case / dev / feature 体系 ／ 否——下游不变，仍松耦合（PRD 非强制）。
- skill：其余（context-loading / add-rule / doc-sync / git-workflow / self-evolution）／ 否——与产出需求流程无关。
- 子 agent：新增 6 个 worker（prd-reviewer / requirements-gatherer / user-story-writer / prd-writer / feature-point-writer / prototype-builder，双栈）；`deep-research` 被复用为外部调研员。

## 备选方案
- **薄编排 / 不拆 worker（保持线性 skill）**：拒——用户要"为效果"把 worker 全建成独立 subagent，享受各配模型/提示词 + 并行。
- **常驻自主"产品总监 agent"现在就建**：押后——现阶段是交互 harness、人在环，主 agent 当总监足够；自主总监等封装成 agent 时再建。
- **PRD review 复用 code-reviewer**：拒——PRD 审稿逻辑（覆盖/可观测/四态/原型可点）≠ 代码审，另建 prd-reviewer。

## 影响
- prd-elicitation 模块化、轻重自适应、地基早审；轮次更多但每步更稳。
- 新增 6 个 worker 子 agent（双栈）+ 一个 Workflow 编排模板（`prd-elicitation/references/`）。
- 护栏对齐：rule-0010 / eval 013（PRD 标准）、`prds-audit`（结构机检）随实现保持；用户覆盖必选/留痕是新约定。
- ADR-0003/0007 不改写（历史）；本 ADR 是对其流程的编排化细化。
- 押后：常驻自主总监、通用 loop-engineering 引擎、harness 自身 observability、外部 MCP 连接器。
