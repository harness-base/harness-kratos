---
title: ADR-0008 测试用例 skill——独立产出测试用例 + 软硬分层管覆盖（AC+FP）
status: accepted
date: 2026-06-28
last_updated: 2026-06-28
source_files:
  - ../../.agents/skills/test-case/SKILL.md
  - ../../templates/test-case.md
  - ../../scripts/test-cases-audit.sh
related_docs:
  - 0003-prd-elicitation-and-prototype.md
  - 0007-prd-workflow-redesign.md
---

# ADR-0008：测试用例 skill——独立产出测试用例 + 软硬分层管覆盖（AC+FP）

## 背景

harness 有"产出需求"（`prd-elicitation`，ADR-0003/0007）与"实现需求"（`feature-delivery`）两环，但缺一环**产出测试用例**：把需求侧的验收点（AC）/ 功能点（FP）系统地转成测试用例、并保证"用例对需求的覆盖"不漏。

讨论确认：这一环**先做成独立 skill**——只管"产出用例 + 管覆盖"，**不碰"过没过"（执行结果）**；真跑 / 执行脚本是后面单独一段（另一 skill / 机制）。它与上游（`prd-elicitation`）/ 下游（`feature-delivery`）怎么衔接**后面再定**，现在 AC/FP 当通用输入。

## 决策

1. **定位：独立 skill `test-case`**。产出测试用例 + 管"用例对需求（AC + 功能点 FP）的覆盖率"——这里的覆盖率指**用例对需求的覆盖**，不是跑出来的代码覆盖率。**不碰执行结果**。AC/FP 当通用输入（用户给 / 指 `user-stories.md` / 指 feature 包均可），来源不绑定。

2. **覆盖锚点：AC + 功能点 FP**。两个轴都要全覆盖。

3. **软+硬分层，按"机器查不查得了"切**（同 rule-0009 / doc-sync"机检兜得了的上硬闸、兜不了的留人审"）：
   - **硬闸**（`scripts/test-cases-audit.sh` → `make verify` 红）：卡可机检的**存在性 / 完整性**——① 每条 AC、每个 FP 都被 ≥1 条用例 `covers:` 覆盖（无遗漏）；② `covers:` 引用的 AC/FP 都已声明（无悬空）；③ 目录 ↔ `index.yaml` 登记一致。配守护测试 `scripts/test-cases-audit.test.sh`（变异自证 load-bearing，rule-0009）。
   - **软**（SKILL 对抗评审 checklist + 收尾 eval 考题 015）：卡机器查不了的**质量**——用例是否真覆盖该 AC/FP 的语义与边界、正常 / 边界 / 异常齐不齐、有没有为凑覆盖率的空壳用例。

4. **`covers:` 是覆盖关系的唯一真相源**。每条用例用 `covers: AC-x, FP-y` 声明它覆盖什么；硬闸直接从 `covers:` 核全覆盖 + 无悬空，**不另存一张手维护的映射表**（避免"同一信息存两份会漂"，rule-0012；比"两份交叉校验"少一处漂移面）。模板里的覆盖映射表仅作人读示例，标注"权威以 `covers:` 为准"。

5. **产物落 `docs/test-cases/<id>/test-cases.md`**（独立账本，平行 `docs/prds/`），套新建 `templates/test-case.md`：`## 验收点 AC` / `## 功能点 FP` / `## 用例`（每条 `### TC-n` 带 `covers:`）。账本 `docs/test-cases/index.yaml`（`id` / `dir` / `title` / `status` / `source`（需求来源，可空，衔接上游后填），audit 按 `dir` 双向校验），空账本平凡通过。

6. **护栏 / 登记**：新增 rule-0014（测试用例产出标准，sev blocker，仅在产出时适用）+ eval 考题 015（用例覆盖完整性与质量）。

## 受影响的 skill（rule-0007）
- skill：test-case ／ **新建**——本 ADR 即其确立，随实现写 `SKILL.md`。
- skill：prd-elicitation ／ 否——独立 skill，现阶段不绑定来源；AC/FP 可来自其 `user-stories.md`/PRD，但衔接后定，上游产出流程未改。
- skill：feature-delivery ／ 否——下游不变，仍松耦合（不强制有测试用例产物）。
- skill：其余（context-loading / add-rule / doc-sync / git-workflow / bugfix / self-evolution）／ 否——与本流程无关。

## 备选方案
- **挂进 feature-delivery / prd-elicitation 而非独立 skill**：暂拒。先独立、来源不绑定，衔接后再定（用户明确"先独立，后面再考虑衔接"）；过早耦合会被未定的上下游设计拖住。
- **覆盖率全软（只映射表 + 评审）**：拒。用户要"软+硬都要"；存在性 / 完整性是机器查得了的，该上硬闸，留软只兜机器查不了的质量。
- **另存手维护映射表 + 硬闸交叉校验两份一致**：拒。`covers:` 已内联全部覆盖关系，另存一张表是冗余的第二真相源（rule-0012 反模式）；直接从 `covers:` 核更彻底防漂、少一处维护面。
- **覆盖率做成机器闸卡百分比**：本设计的硬闸即"100% 存在性覆盖"（缺任一 AC/FP 即红），已是硬闸；质量百分比不可机检，留 eval。

## 影响
- 多一条产出链（需求 → 用例），与执行（过没过）解耦，后续可接执行机制。
- 硬闸进 `make verify`；空账本不影响现状。
- 护栏对齐：rule-0014 + eval 015 随实现落；`docs/test-cases/` 进文档路由。
- 与实现体系松耦合，不强制测试用例必须存在（门禁只在产出时适用，同 rule-0010 对 PRD 的处理）。
