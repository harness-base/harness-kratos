# <项目名>（被管工程入口规则）

> 新工程根 `AGENTS.md` 骨架。本模板**通用、中性**：`hc-onboard` 接入时按真实项目填，填好的那份才项目专属。
> 本文件**精简**——只列工程简介 + 工程红线 + 就近下沉指针 + 验证入口。规则多就下沉到各层 `AGENTS.md`，别都堆这里（见 `docs/harness/PROJECT_ONBOARDING.md` 第 2 步）。
> 控制面常驻规则以仓库根 `AGENTS.md` 为准；冲突时**用户当前指令 > 本文件 > 根 AGENTS.md**。

## 工程简介

- **目标**：<这个工程要解决什么、给谁用>
- **概述**：<一句话说清它是什么形态的系统>
- **技术栈**：<框架 / 语言 / 关键组件——按真实项目填，别预设>

## 工程红线

> 工程专属红线（动手前必看）。每条用 `<!-- rule: rule-XXXX -->` 标记占位，**规则要真正落地走 `hc-add-rule` skill**（定范围 → 写下来 + 登记 → 挂执行），别只写文字没人挂执行。
> 刚接入时可能一条都没有，那就先空着——踩到坑 / 定了规范再按 `hc-add-rule` 补。

- <工程红线一句话，可观察、可验证> <!-- rule: rule-XXXX -->
- <工程红线一句话> <!-- rule: rule-XXXX -->
- **不擅自 git 写操作**（根 AGENTS.md 红线，工程内同样守）。

## 就近下沉指针

> 规则多就下沉到各层，**干哪层活只加载哪层**（就近优先，见 `docs/context/CONTEXT_LOADING.md`）。这里只放"去哪找"的指针，规则本体写在对应层的 `AGENTS.md`。

- <某层规矩，如"数据层用法">：`<dir>/AGENTS.md`
- <某层规矩>：`<dir>/AGENTS.md`
- 全部规则总览（harness + 本工程）：`../../docs/rules/index.yaml`

## 指针

- 设计 / 选型 / 决策记录：`../../docs/decisions/`（本工程第一个 ADR 由 `hc-onboard` 第 3 步落）
- 需求包账本：`../../docs/features/index.yaml`（第一个需求包按 rule-0001 立）
- 怎么 build / generate / test：`README.md`

## 验证

> 执行口登记在控制面 `workspace/verification.yaml`（`make verify` 和 CI 据此路由，见 `docs/harness/VERIFICATION_ROUTING.md`）。
> 每个接入点（verify / unit / api / e2e / sandbox）的值必须是三态之一——**不许静默空 / 留白 / 裸 TODO**：
> - 真命令（已接实）；
> - `PENDING: <为啥现在空 / 补的条件>`（待接实，`make verify` 会 warn 提醒，此处也留一条"待补"记录）；
> - `N/A: <理由>`（本工程不需要这个接入点）。

- 最小收口：`make verify`（路由到本工程的 `verify` 命令）
- 待补记录（对齐 `workspace/verification.yaml` 里的 PENDING 项，接实后删）：
  - <接入点，如 e2e>：PENDING: <为啥空 / 补的条件>

## CLAUDE.md shim

> 同级放一个 `CLAUDE.md`，内容仅一行 `@AGENTS.md`（外加注释），用于让 Claude Code 自动加载本 `AGENTS.md`（常驻规则源）。规则与流程都在 `AGENTS.md`，shim 不放内容。
