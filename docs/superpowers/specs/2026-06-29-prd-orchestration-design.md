---
title: PRD 编排式重构 设计稿
status: draft
owner: harness
last_updated: 2026-06-29
source_files: []
related_docs:
  - ../../../.agents/skills/prd-elicitation/SKILL.md
  - ../../decisions/0007-prd-workflow-redesign.md
  - ../../decisions/0003-prd-elicitation-and-prototype.md
---

# PRD 编排式重构 设计稿

## 背景 / 目标

`prd-elicitation` 现是一条线性交互 skill（ADR-0007：需求澄清 → 用户故事 → PRD → 可选原型 → 评审）。本次把它重构成**编排式**：一个**产品总监**（编排逻辑）调度一队**专职 worker**，带 **必选 / 可选 · 权重** + **确认门** + **并行** + **review loop**。

目标：① 模块化——每个 worker 专职、可各配模型 / 提示词 / 数量；② 轻重自适应——用权重防小需求过度走流程，又不盲目省略；③ 地基早审、下游重审——基础（用户故事 + AC）在往下之前就夯实。

**定位边界**：本次建的是编排"模式"那一层（skill 总谱 + worker subagents + 权重模型 + Workflow 编排），跑的时候**主 agent 当总监**。常驻自主"产品总监 agent"**押后**到真把 harness 封装成自主 coding agent 那一步。

## 优先级与权重模型

三层优先级（高 → 低）：

1. **用户明确指令**——最高，**覆盖一切默认**（含必选）。体现 `AGENTS.md`"用户当前明确指令 > 本文件"。
2. **必选**——默认 always 跑。
3. **可选 · 权重**——默认按权重 + 触发判据决定跑 / 跳。`权重高`=默认跑、明确理由才跳；`权重低`=默认跳、明确触发才跑。

规矩（既听用户、又不静默、可追溯）：
- **覆盖必选**：先**提示后果**（如"跳过 AC：后续 PRD / 功能点 / 审稿缺锚点、`US↔FP↔正文` 映射对不齐、衔接别扭"）→ 用户坚持就照办 → **留痕**（标"用户强制跳过 X，已告知后果"，审稿 / eval **不当缺陷扣分**）。
- **跳过可选步**：留一句为什么（"跳过外部调研：纯逻辑、无外部依赖"），审稿 / eval 抽查跳得有没有理由。

## 角色（workers）

| worker | 干啥 | 标 | 形态 | 落地 |
|---|---|---|---|---|
| 需求采集员 | 多轮对话收原始需求（JTBD / 页面 / 数据 / 四态 / 边界） | 必选 | 交互 · 人在环 | subagent 文件（双栈） |
| 外部调研员 | 查外部 SOP / 事实，**过 rule-0008 验收**再喂入 | 可选 · 权重低 | 用户要查 / 有市场 SOP 且摸不透才跑 | **走 `deep-research` skill**（可用的 research skill，通用 subagent 调 Skill 工具跑，不另建 subagent） |
| 用户故事 + AC 员 | 写 `user-stories.md`（US-NN / AC） | 必选 | 轻审 → 确认门 | subagent 文件（双栈） |
| PRD 本体员 | 写 `prd.md`（合已确认故事） | 必选 | 并行 | subagent 文件（双栈） |
| 功能点清单员 | 写功能点 + `US↔FP↔正文` 映射 | 必选 | 并行 | subagent 文件（双栈） |
| 原型员 | 可点 HTML 原型 | 可选 · 权重中 | 有界面 / 用户要才跑 | subagent 文件（双栈） |
| PRD 审稿员 | 对抗挑刺整套，rubric = eval 013 / rule-0010 | 必选 | review loop | subagent 文件（双栈，**逻辑 ≠ code-reviewer**） |

## 编排时序

```
需求采集员（必选 · 人在环）
   └─（可选 · 权重低）外部调研员 → rule-0008 验收 → 喂入
→ 用户故事 + AC 员（必选）
      → 轻审 loop（1-2 轮，只盯地基四项：AC 可观测可验证 / 故事完整无遗漏 / 内部一致 / 对齐采集需求）→ 修
      → 【确认门：用户 approved】才往下
→ 并行产出： [ PRD 本体员 ∥ 功能点清单员 ∥（可选 · 权重中）原型员 ]
→ ┌─────────── PRD 审稿员 重审 loop（loop 框住「并行产出」）───────────┐
   │  审整套（PRD 合故事？FP 覆盖？原型可点？四态？）                       │
   │   → 挑出有问题的 worker **回原 worker 角色重跑**（产物 + 发现喂回）      │
   │   → 审稿员复审 → 还有问题再重跑 →  到零                               │
   └──────────────────────────────────────────────────────────────────┘
→ 【收尾确认：用户】
```

要点：
- **review loop 的"修" = 回原 worker 角色重跑**（workflow 里 agent 无状态，"回原"= 同角色 / 提示词 + 把产物和发现喂进去重跑）。**每轮只重跑"被审出问题的 worker"**（只有功能点有问题就只重跑功能点员），不全部重跑。独立性靠**审稿员每轮复审**保证。**例外**：审稿员判"方向整个错"（非细节）→ 总监回更上游（重采集 / 重写），不在原产物上改。
- **两个 review 点，轻重分明**：① **用户故事 + AC = 轻审**（地基四项，1-2 轮，确认门前）——shift-left，地基歪了下游白做；② **整套 PRD = 重审**（多轮对抗循环到零，主查下游跟地基一致：PRD 合故事、FP 覆盖、原型可点），不重新 litigate 地基。两处用**同一个 PRD 审稿员**，只是轮数 / 侧重不同。

## 建什么

- **重写** `.agents/skills/prd-elicitation/SKILL.md` 成编排总谱：workers 表（必选 / 可选 · 权重 + 触发判据）+ 时序 + 两 review 点 + 用户指令覆盖 / 跳过留痕 规矩。
- **新建 worker subagent 双栈**（`.claude/agents/<name>.md` + `.codex/agents/<name>.toml` + `.codex/config.toml` 注册）：需求采集员 / 用户故事AC员 / PRD本体员 / 功能点清单员 / 原型员 / **PRD 审稿员**（共 6 个）；外部调研**走 `deep-research` skill**（可用的 research skill，通用 subagent 调 Skill 工具跑），不另建 worker subagent。
- **Workflow 编排模板**（主 agent 当总监）：`parallel` 并行产出、确认门穿插、review 用 loop、可选 worker 条件触发、每个 worker 调 `agent(prompt, {model, ...})` 配各自模型 / 提示词。
- **收口**：写 ADR（编排式重构决策 + 受影响 skill 栏）；改 ADR-0007 / 0003 衔接（prd-elicitation 升级为编排式）；视情况调 `templates/prd.md` / `user-story.md` / `prds-audit`；`make verify` + 收尾 eval。

## 押后（YAGNI，不在本次）

- 常驻自主"产品总监 agent"（现在主 agent 当总监）。
- 通用 loop-engineering 引擎（本次只在 PRD 内落 review loop）。
- harness 自身 observability（metrics / 仪表盘）、外部 MCP 连接器（飞书 / Figma 等）。

## 未决 / 待 review 时定

- 6 个 worker subagent 各自的"提示词 / 模型档位"细节（实现时定）。
- 轻审与重审是否真共用一个 PRD 审稿员，还是轻审用更省的提示词变体。
- `prds-audit` 是否要加"用户故事 approved + 留痕字段"的机检（soft → hard 视复发）。
