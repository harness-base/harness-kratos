---
name: self-evolution
description: harness 规范检查层。当要改 harness 本身、或发现 harness 漏洞（如"某规则工作时没被加载"）时用——引导按 harness 结构逐维度审查"哪一环出问题 / 该怎么改"，不靠记忆漏项。区别于操作层 skill（add-rule/prd-elicitation 等是被检查对象 + 修复工具）。
version: 2
last_reviewed: 2026-06-28
---

# 自进化 / harness 规范检查（self-evolution）

harness 不只治理被管工程，也要**管自己**。本 skill 是**规范检查层**：检查 harness 本身符不符合规范、有没有缺口/漏洞、该怎么改。

- **检查层 ≠ 操作层**：`add-rule` / `prd-elicitation` / `dev` / `git-workflow` 等是**操作层**——它们做事，是**被检查对象** + 发现缺口后的**修复工具**，不作为审查逻辑被复用。
- 落文档提醒（每轮 capture）是另一套机制，不属本 skill。

## 何时用（两个触发）
1. **要改 harness 某处**（方向已定）→ 引导按结构改对、查连带影响。
2. **发现 harness 漏洞 / 症状**（如"某规则工作时没进上下文"）→ 诊断是哪一环断了。

不用：纯被管工程开发任务、琐碎改动。

**触发后、开始改之前**：扫一眼 `tasks/lessons.md` 顶部最近 1-2 个月的条目（`grep '^## 202' tasks/lessons.md | head -20` 看标题），找跟本次要改领域同型的坑。反复出现 = 晋升成规则的信号（rule-0007），别又多记一条 lesson 就完事。

## 诊断方法（链路 / 不变量）
harness 每个能力是一条**链路**；漏洞 = 某环断了。
1. 把症状写成「X 本该发生却没有」。
2. 拆出交付 X 的链路（环节序列）。
3. 逐环查：在不在 / 接没接 / 真起作用——机器能查的跑 `*-audit`、`*-index --check`；查不了的判断。
4. 定位断环 → 读对应维度 reference 修 → **查有没有连带破坏别的环** → 记 lesson（反复出现晋升成规则）。

**例**：症状「某规则工作时没进上下文」→ 链路 `定义(AGENTS.md) → 加载(CLAUDE.md @import/就近) → 选取(按任务挑相关) → 遵守 → 拦截` → 逐环发现「选取」环没有 task→规则 选择器 = 断点 → 修复走 `references/rules.md`。

## 维度索引（审到哪个读哪份；逐项过、不靠记忆）
每份 reference 给该维度的 规范 / 检索 / 判据 / 漏洞 / 修复：

- `references/skills.md` — skill 体系
- `references/rules.md` — 规则
- `references/docs.md` — 文档（README / AGENTS / CLAUDE / context）
- `references/eval.md` — eval 题库 / 流程
- `references/templates.md` — 模板
- `references/gates-hooks.md` — verify / 护栏 / 门禁 / 触发（含 codex&claude 对等）
- `references/decisions-context-features.md` — 决策 / 上下文 / 需求 索引区
- `references/sandbox.md` — 被管工程本地验证环境
- `references/subagents.md` — 子 agent（`.claude/agents/`）
- `references/lessons-memory.md` — 错题本 / 记忆 / 晋升
- `references/project-onboarding.md` — 被管工程接入
- `references/index-system.md` — 索引体系（横切）
- `references/process-coverage.md` — 流程覆盖（任务类型→流程；缺口如 loop-engineering）

巡检/审查时先把**总览索引**当地图（默认不加载、自检时查）：`docs/rules/index.yaml`、`.agents/skills/README.md`、各区 `README.md`/`index.yaml`、`docs/decisions/index.yaml`、`docs/features/index.yaml`、`docs/eval/index.yaml`。

## 4 条 meta（强制）
1. **全覆盖**：维度逐项过，不许凭记忆只查一部分。
2. **每项给方法**：用对应 reference 的 检索 / 方向 / 规范边界，不空谈。
3. **列表外**：这些维度之外，是否要**补新 harness 流程**（如 loop-engineering playbook）。
4. **自身（别漏）**：harness 进化后，**本 skill 与 references 自己要不要改**——维度有没有漏、references 过没过期、这套审查本身该不该调。

## 深审执行器
复杂或要独立判断时，spawn `self-optimize` 子 agent（`.claude/agents/self-optimize.md`，免 key）按维度深审、写记录。

## 演进（rule-0007）
harness 结构 / 维度 / 诊断法变化时回顾本 skill 与 references。
