---
title: ADR-0005 自进化闭环——每轮机械兜底 + self-optimize 判断引擎
status: accepted
date: 2026-06-26
last_updated: 2026-06-26
source_files: []
related_docs:
  - ../../AGENTS.md
  - ../harness/HOOKS.md
  - ../../.claude/agents/hc-self-optimize.md
---

# ADR-0005：自进化闭环——每轮机械兜底 + self-optimize 判断引擎

## 订正（2026-06-26）

本 ADR 当时把两件事并称"自进化闭环"，后续拆清为两套独立机制：
- **① 落文档提醒（capture）**：每轮机械触发 + Haiku（`scripts/turn-backstop.sh`），catch"决策/知识没落文档"，由 rule-0011 钉。**本 ADR 的"每轮兜底"即此。**
- **② 自进化（规范检查）**：`self-evolution` skill + references（`.agents/skills/hc-self-evolution/`），刻意触发、按维度审 harness 本身；`self-optimize` 子 agent 是它的深审执行器。

二者触发频率 / 产出 / 记录分开，不再混称。

## 背景

本会话暴露两个缺口：(1) 重要决策/知识只活在对话里、没落地（如 kratos 各包就近规则缺失，是用户发现而非机制发现）；(2) 软约束靠 agent 自觉，会漏。需要一个**连续、独立于 agent 判断**的自进化机制。

## 决策

**分工：脚本管 WHEN（确定性），LLM 管 WHAT（被触发后才判）。** 两层：

1. **每轮廉价兜底**（`scripts/turn-backstop.sh`，Stop hook 调用）：
   - **机械触发**（全在脚本常量+逻辑）：`K` 轮到点（默认 8）/ commit 边界（HEAD 变）/ 变更文件数**增量** ≥ 阈值（默认 10，是"涨多少"非绝对值）。状态存 `tasks/.turn-count`、`tasks/.last-backstop`（gitignore）。
   - 触发后 **headless `claude -p --model haiku`** 复查最近 transcript，捞"做了决策/学了偏好/有知识却没写进文件"的遗漏，追加 `tasks/optimization-log.md`。
   - **触发独立于 agent 判断**——漏记的 agent 拦不住兜底（这是信任锚）。
2. **重判断引擎**（`.claude/agents/hc-self-optimize.md` 子 agent，会话模型）：一段工作落地后多维判断是否要优化（起步维度：知识捕获 / 产出质量 / skill 新鲜度 / 自身），写 `optimization-log.md`，有 blocker 提示先处理。与现有 eval（rule-0005 收尾评分）互补，不替代。

**安全**：递归 guard（headless 调用带 `HARNESS_TRIAGE=1` + 从中性目录 `/tmp` 跑、不加载项目钩子）；无 `timeout`/`gtimeout` 用 perl `alarm` 包超时；budget 封顶；全程 best-effort——**任何失败一律 exit 0，绝不阻断收尾**。

## 受影响的 skill（rule-0007）

- skill：self-evolution ／ 是否已更新：是（②规范检查层，见订正段；本 ADR 立的 `self-optimize` 是其深审**子 agent**，非 skill）
- skill：add-rule / context-loading / feature-delivery / prd-elicitation / git-workflow ／ 是否已更新：否（自进化闭环不改它们的流程）

## 备选与取舍

- **字面"每轮跑 eval"**：否决——纯讨论轮空转、烧 token/拖慢；改"主 agent 即时记 + 钩子机械触发的 Haiku 兜底"。
- **兜底由主 agent 自行判断是否启动**：否决——会忘记记的 agent 同样会忘记触发兜底，两者一起失效；故触发必须机械、独立。
- **变更用绝对阈值**：否决——大改动未提交时会每轮 fire；改为"自上次兜底以来的增量"。
- **triage 做成子 agent**：否决——子 agent 要主 agent 记得 spawn=软；兜底走钩子 headless 才硬。self-optimize（重判断）则仍是子 agent（同 eval 范式）。

## 影响

- 新增 `scripts/turn-backstop.sh`、`.claude/agents/hc-self-optimize.md`、`tasks/optimization-log.md`；`scripts/stop-check.sh` 扩两段（递归 guard + 取 transcript + 调兜底）；`.gitignore` 加状态文件；rule-0011 入 `AGENTS.md`。
- 不改现有 eval/rule-0005 收尾闸门语义。
