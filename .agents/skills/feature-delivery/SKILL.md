---
name: feature-delivery
description: 管"用户可见的需求/行为/UI/流程/验收目标变化"的完整交付流程——立需求包→测试就绪→实现→验证→收尾。只要改动会被用户感知或改变验收目标（哪怕看着是小调整），动业务代码前就用本 skill；它挡在"先写了再补需求"前面。
version: 1
last_reviewed: 2026-05-29
---

# Feature Delivery（需求交付）

流程门禁挡在写代码之前，是为了避免"先写了再补需求"导致返工、以及验收和实现脱节。所以用户可见的改动必须先立项、先有测试，才动业务代码。

## 何时用 / 何时不用
- 用：用户可见的需求、行为、UI、流程、验收目标变化（不论大小）。
- 不用：纯控制面 / 文档、或无用户可见行为的轻量修补。

## 步骤
1. 用 `templates/feature-package.md` 建需求包，登记进 `docs/features/index.yaml`。
2. 补验收目标 + 测试设计，状态推进到 `tests_ready` 后**才动业务代码**（rule-0001）；没就绪 MUST STOP。
3. 实现 → 按 `workspace/verification.yaml` 跑验证 → 如实记结论分类（pass / fail / blocked / skipped，rule-0002 / 0003）。
4. 大改回顾相关 skill（rule-0007）。
5. 收尾跑 eval（rule-0005，用 eval 子 agent，免 key）。

## 硬规则
- 没就绪不写业务代码，MUST STOP。
- blocked / skipped 不当 pass；无真实证据不声称完成。

## 演进（rule-0007）
交付流程 / 状态机变化时回顾本 skill。
