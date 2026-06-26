---
title: ADR-0004 规则分布化：入驻 AGENTS.md + 自动生成 catalog + CLAUDE.md import shim
status: accepted
date: 2026-06-26
last_updated: 2026-06-26
source_files: []
related_docs:
  - ../../AGENTS.md
  - ../rules/index.yaml
  - 0001-harness-skeleton-design.md
---

# ADR-0004：规则分布化——入驻 AGENTS.md + 自动生成 catalog + CLAUDE.md import shim

## 背景

诊断发现旧规则模型有两层断裂：

1. **投送断裂**：规则全文在 `docs/rules/00NN.md`，但**没有任何机制按任务把规则送到 agent 面前**。红线之外的规则只能靠"skill 碰巧引用 / eval 收尾兜底 / agent 自觉"。典型：`rule-0009`（验收断言锚定）既不在 AGENTS.md 红线、也无 skill 引用——**写测试的当下根本不可达**，只在收尾 eval 才被读到。
2. **加载靠自觉**：`AGENTS.md`（常驻规则源）此前靠 `CLAUDE.md` 用自然语言"叫 agent 去读"，不是机制加载。

且 `@import`（CC 专属）、`applies_when`（自造字段）都**不跨工具**：Codex 无 import、无条件规则机制（官方文档证实），只认目录层级合并。

## 决策

1. **规则入驻 AGENTS.md，就近生效**：harness 全局规则进**根 `AGENTS.md`**；项目专属规则沉淀在 `projects/**/AGENTS.md`。规则放在"能覆盖其所有目标的最浅 `AGENTS.md`"。这是 CC（向上读最近 AGENTS.md）与 Codex（层级合并）**唯一共通的"按情境加载"原语**。`rule-0009` 因此进根 AGENTS.md，工作中始终可达。
2. **编号 `rule-00NN` 保留为稳定引用键**：被 eval 考题 / ADR / feature 按号引用（全仓 160+ 处，含不可变的 eval 评审历史）。ID 无业务语义，只是交叉引用键——故**不换 slug**（换 slug 要改 160 处含历史档案，纯亏）。
3. **catalog 自动生成、默认不加载**：`scripts/rules-index.sh` 扫各 AGENTS.md 的 `<!-- rule: id | sev | eval -->` 隐形标记，生成 `docs/rules/index.yaml`（编号+简述+位置+severity+eval）。供人审查 / agent 自进化时按需查；`--check` 进 `make verify` 防漂移。
4. **CLAUDE.md import shim 机制化加载**：凡有 `AGENTS.md` 处配一个同级 `CLAUDE.md`，内容仅 `@AGENTS.md`。CC 经 import 自动加载、Codex 原生读 AGENTS.md——**两边都不靠"自然语言叫你去读"**。`make verify` 校验"凡 AGENTS.md 必有 CLAUDE.md shim"。
5. **eval 题库独立维护**：`docs/eval/prompts/` + `index.yaml` 自成体系，按编号引用规则，不被规则文件位置绑死。

## 跨工具与硬门禁

- 可移植主干 = **目录就近 AGENTS.md**（放规则）+ **git hooks / CI**（谁都绕不过的硬门禁）。
- `@import`、PreToolUse 钩子等是 **CC 专属的锦上添花**，不作为可移植机制依赖。

## 受影响的 skill（rule-0007）

- skill：add-rule ／ 是否已更新：是（version 2——加规则流程从"建 `docs/rules/00NN.md`"改为"入驻 `AGENTS.md` + 隐形标记 + 重生成 catalog"）
- skill：context-loading ／ 是否已更新：否（其职责是判"读多少档"；加载机制由 `AGENTS.md`/`CLAUDE.md` 承担，档位规则未变，§启动顺序第 5 步"向上读最近 AGENTS.md"仍准确）
- skill：feature-delivery / prd-elicitation / git-workflow ／ 是否已更新：否（不涉及规则的产生 / 加载机制）

## 备选与取舍

- **保留 docs/rules/ 全文 + 加 `applies_when`**：否决——投送仍断裂，且 applies_when 无原生消费者（死元数据）。
- **换命名空间 slug 作 ID**：否决——160+ 引用含不可变历史，churn 大、零收益。
- **规则全删进 AGENTS.md、不留 catalog**：否决——丢掉 eval↔规则编号的可审计骨架。

## 影响

- 重写根 `AGENTS.md`（规则入驻 + 标记 + 启动顺序更新）；根 `CLAUDE.md` 改为 `@import`。
- 新增 `scripts/rules-index.sh`；`docs/rules/index.yaml` 转为生成物（禁手改）；删除 `docs/rules/00NN.md`（全文已入 AGENTS.md）。
- `projects/kratos-base/`、`.../internal/data/` 各加 `CLAUDE.md` shim。
- `make verify` 纳入 rules 漂移 + shim 校验。规则语义不变（编号、eval 映射、severity 全保留）。
