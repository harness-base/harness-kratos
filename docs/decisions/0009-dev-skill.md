---
title: ADR-0009 dev skill——写代码统一入口（两级 + 挑刺 + 共享 code-reviewer 双栈），替代 feature-delivery / bugfix
status: accepted
date: 2026-06-28
last_updated: 2026-06-28
source_files:
  - ../../.agents/skills/dev/SKILL.md
  - ../../.claude/agents/code-reviewer.md
  - ../../.codex/agents/code-reviewer.toml
related_docs:
  - 0003-prd-elicitation-and-prototype.md
  - 0007-prd-workflow-redesign.md
  - 0008-test-case-skill.md
---

# ADR-0009：dev skill——写代码统一入口，替代 feature-delivery / bugfix

## 背景

harness 的"写代码"流程散在两处且有缺口：`feature-delivery` 只管"用户可见的需求/行为变化"（显式排除纯 bug / 控制面 / 工程代码），`bugfix` 单管修 bug；**工程性代码（重构、内部模块、工具、基建——如 test-case skill 本身）没有任何 skill 统管**（self-evolution 的流程覆盖维度早点名 `bugfix`/`重构`/`迁移` 是缺口）。讨论确认：建一个**独立的"写代码"统一入口**，把好的部分从旧 skill 借过来，旧的逐步替换删掉。

## 决策

1. **新建 `dev` skill 作"写代码"统一入口**：功能 / 工程代码 / 重构 / 改 bug 都走它。主线：想清楚 → 列 plan → 用户确认 → 写（**全程不假设、决策点问用户、防技术债**）→ **挑刺**（对抗 review）→ 提醒用户测。**纪律全程常开，仪式按级别伸缩**。

2. **两级（常规 / 深度），默认 agent 按升级信号自判、用户可指定**：
   - 常规（小而明确）：plan → 写 → 1-2 个 `code-reviewer` 挑刺循环到无 bug → 提醒测。不强制 brainstorm / eval。
   - 深度（动核心 / 接口 / 不可逆 / 要写 ADR / 复杂）：brainstorm → 正式 plan（用户可见功能先立需求包）→ 写（决策点必问）→ 多视角对抗挑刺循环到零 → 收尾 eval → 提醒测。
   - 升级信号：核心逻辑 / 对外接口 / 不可逆 / 要写 ADR / 关联多且不明确 / 要读很多 / 用户可见行为变化。
   - **不做硬档位机器闸**——用户明确要"纪律常开 + 仪式现场拿捏 + 可指定"，给启发式指引而非一刀切闸。

3. **挑刺 = 对抗 review，派共享 `code-reviewer` 子 agent（双栈）**：Claude Code 用 workflow 编排（`agent(..., {agentType:'code-reviewer'})` 并行 / 循环）；Codex 用其原生机制派同名；想要云端多 agent 深审则提醒用户触发 `/code-review ultra`（用户触发，agent 启不了）。借用 superpowers（brainstorming / writing-plans / TDD / review）与收尾 eval，**引用不重写**。

4. **删 `feature-delivery` + `bugfix` skill（能力并入 dev）**：feature-delivery 的"需求包门禁 → 验收 → 收尾 eval"并入 dev 深度级；bugfix 的"复现 → 真根因 → 能复现的守护测试 → 防回归"并入 dev 的改 bug 子模式。**需求包门禁（rule-0001）+ eval 001 + `templates/feature-package.md` + `docs/features/` 数据保留**（dev 深度级接管），只删 skill 壳。

5. **旧的逐步替换、暂留参考**：本批先删 feature-delivery / bugfix；其余（prd-elicitation / test-case 等）保留、把指向已删 skill 的**活引用**改为 dev；历史记录（各 ADR 受影响栏、eval task-reviews、归档 plan）不改写。

## 受影响的 skill（rule-0007）
- skill：dev ／ **新建**——本 ADR 即其确立。
- skill：feature-delivery ／ **删除**——并入 dev 深度级 + 需求包门禁（rule-0001 等保留）。
- skill：bugfix ／ **删除**——并入 dev 改 bug 子模式。
- skill：prd-elicitation ／ **是**——下游引用 feature-delivery 改为 dev（产出需求流程本身未改）。
- skill：test-case ／ **是**——下游引用 feature-delivery 改为 dev（覆盖流程本身未改）。
- skill：self-evolution ／ **是**——references 多处把 feature-delivery/bugfix 当操作层样板 / 流程覆盖缺口，更新为 dev（bugfix / 重构 / 迁移 缺口已由 dev 填）；并补本 ADR 新引入的 `code-reviewer` 子 agent 进 subagents 维度事实锚点 + `.codex/config.toml` 注册。
- skill：其余（context-loading / add-rule / doc-sync / git-workflow）／ 否——与写代码流程无关。

## 备选方案
- **薄编排层（只路由到已有 skill）/ 增强 feature-delivery**：拒——用户明确要"独立新建、旧的逐步替换删掉"。
- **硬档位机器闸（L0–L6 强制不同流程）**：拒——用户要"纪律常开 + 仪式现场拿捏 + 可指定"，给启发式不给一刀切闸。
- **为 review 不封子 agent、纯靠 Claude workflow 自带**：部分否——只用 Claude Code 时 workflow 够；但用户要 Claude Code + Codex 跨工具一致，故仍封一个**共享** `code-reviewer` 子 agent（双栈可移植），且与 workflow 不冲突（workflow 用 `agentType` 调它）。

## 影响
- 写代码有统一入口；bugfix / 重构 / 工程代码缺口被填。
- 需求包门禁 / 数据不丢，dev 接管；rule-0001 / eval 001 不变。
- 删两 skill 连带改活引用 + ADR-0003 `related_docs`（曾有一项指向 feature-delivery/SKILL.md，删后须移除否则 `docs-audit` 红）；skills-index 自动去旧增新。
- `code-reviewer` 双栈，跨 Claude Code / Codex 一致；review 走 workflow（Claude）/ 原生（Codex）/ 云端用户触发（ultra）。
- 后续：其余旧 skill 逐步评估是否并入 dev（暂留参考，不在本批）。
