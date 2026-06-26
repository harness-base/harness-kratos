---
title: ADR-0006 不单独建 docs/drift/ 区——漂移处理并入自进化闭环
status: accepted
date: 2026-06-27
last_updated: 2026-06-27
source_files: []
related_docs:
  - 0001-harness-skeleton-design.md
  - 0005-self-evolution-loop.md
---

# ADR-0006：不单独建 `docs/drift/` 区——漂移处理并入自进化闭环

## 背景

ADR-0001 规划了 `docs/drift/` 区，用来登记"文档描述 ↔ 代码现状"的偏差，前提是"接入 `projects/` 后再建"。如今 `projects/` 已挂 kratos-base，前提满足，到了要不要建的决策点。

同时，自 ADR-0005 起，harness 已落地一套通用的"捕获 → 晋升"机制：

- **预防**：`doc-sync` skill（主动 checklist，改东西时回查相关文档）
- **检测**：`turn-backstop.sh`（①，每轮机械触发的 Haiku 轻扫，prompt (f) 专盯文件 ↔ README/AGENTS 同步遗漏）+ `self-optimize` 子 agent（②，按需深审）
- **中转**：`tasks/optimization-log.md`
- **晋升**：rule-0011（中转站候选必须晋升到 ADR / lessons / 就近 AGENTS.md / memory，不许烂在 log）

## 决策

**不建 `docs/drift/` 区。** "文档 ↔ 现状偏差"(drift) 是上述通用机制处理的**一个子集**，已被预防 + 检测 + 中转 + 晋升四段完整覆盖。单独建 drift/ 区会把这一支从共享中转站（optimization-log）拆出去单独立户——多一份 index、多一个 audit、多一道"算 drift 还是算 lesson"的分流判断——是**拆分而非收敛**，与"少建区、机制复用"的取向相悖。

drift 因此**并入**自进化闭环，不替代、不另起。

## 受影响的 skill（rule-0007）

- skill：`self-evolution` ／ 是否已更新：**是**——`references/skills.md` 去掉过期的"7 个 skill"硬编码枚举（改为以自动生成的索引为准）；`references/gates-hooks.md` 增"doc-sync ⇄ turn-backstop ⇄ 资产类型自洽"审查项。
- skill：`doc-sync` ／ 是否已更新：**否**（本会话新建，即承担预防层职责；无需因本 ADR 再改）。
- 其余 skill ／ **无需更新**（drift 不涉及其触发或流程）。

## 备选方案

- **建 drift/ skeleton（仿 prds/）**：否——与 optimization-log 中转职责重叠，增加维护面与分流成本。
- **用 lessons.md 兼任**：否——lessons 是"踩坑 + 教训"，与"客观偏差登记"语义不同，混记会糊。

## 影响

- 作废 ADR-0001 中"`drift/` 待接 `projects/` 再建"的规划（ADR-0001 状态不改，本 ADR 在其影响链上撤销该条）。
- 清理三处指向"待建 drift/"的文档：`docs/README.md` 目录职责注脚、`docs/context/CONTEXT_LOADING.md` L5 档"至少读"清单、`docs/context/CURRENT_STATUS.md` prds 行注。
- 漂移处理链路从此明确为"**两个检测器**"（turn-backstop ① + self-optimize ②），相关文档不得只提其一。
