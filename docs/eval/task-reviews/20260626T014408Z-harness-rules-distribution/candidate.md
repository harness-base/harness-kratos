---
title: candidate — harness 规则系统分布化
task: harness-rules-distribution
level: L2
prompts: ["010", "011"]
reviewed_at: 2026-06-26
---

# 被评对象与范围

L2 架构变更：规则系统分布化（ADR-0004 `docs/decisions/0004-rules-distribution-and-loading.md`）。

## 改了什么

1. **重写根 `AGENTS.md`**：原 `docs/rules/0001..0009.md` 9 条规则全文 + 新增 rule-0010（PRD 标准）入驻为 bullet，每条带隐形标记 `<!-- rule: rule-00NN | sev | eval -->`；启动顺序更新（§启动顺序 1-6）。
2. **新增扫描器 `scripts/rules-index.sh`**：扫各 `AGENTS.md` 标记 → 生成 `docs/rules/index.yaml`（catalog，禁手改）；`--check` 进 `make verify` 防漂移。
3. **CLAUDE.md import shim**：根 `CLAUDE.md` 改为 `@AGENTS.md`；新增 `projects/kratos-base/CLAUDE.md`、`projects/kratos-base/app/demo/internal/data/CLAUDE.md`；`scripts/verify-control-plane.sh` 加"凡 AGENTS.md 必有 CLAUDE.md shim 且含 @AGENTS.md"校验。
4. **删除 `docs/rules/0001..0009.md`**（全文已入 AGENTS.md），`docs/rules/` 仅留 `index.yaml`。
5. **更新 `add-rule` skill（version 2）**：加规则流程改为"入驻 AGENTS.md + 标记 + 重生成 catalog"。
6. **新增 ADR-0004** 并登记 `docs/decisions/index.yaml`。

## 套用考题

- **011**（rule-0007 架构变更同步 skill）
- **010**（任务收尾综合评审，含 002/003 诚实性、004 加载档、语义无损/引用未断/catalog 忠实/shim 机制）

## 核查方法

逐项怀疑式核：`git show HEAD:docs/rules/00NN.md` 对比被删规则原意；`bash scripts/rules-index.sh --check` 验漂移；`make verify` / `make docs-audit` 自跑验诚实性；`git grep -E 'rule-[0-9]{4}'` 统计引用；逐个读 3 个 shim、2 个 project AGENTS.md、context-loading 与 add-rule skill。
