---
title: ADR-0017 hc-onboard 工程接入 skill（本 ADR：新项目分支）——引导式搭骨架、接线不接假内容、占位不许静默
status: accepted
date: 2026-07-01
last_updated: 2026-07-01
source_files:
  - ../../workspace/verification.yaml
related_docs:
  - ../harness/PROJECT_ONBOARDING.md
  - 0015-hc-tech-design.md
  - 0009-dev-skill.md
---

# ADR-0017：hc-onboard 工程接入 skill（新项目分支）

## 背景

现在"把工程接进 harness"只有一份**被动文档** `docs/harness/PROJECT_ONBOARDING.md`（10 步指南 + 清单，靠用户自己照着填）。要把它做成**引导式 skill**（同 `hc-tech-design` 交互形态），agent 领着用户把工程接进来。

接入分两种：**老项目**（有代码有历史 → 扫、消化、搬进规范、对齐不飘）和**新项目**（从零 → 照规范搭骨架、顺着长出来）。**本 ADR 只做「新项目」分支**（较简单）；老项目分支后续加进同一 skill。

## 决策

1. **`hc-onboard` = 引导式接入 skill**（主 agent 当接入向导），形态 ≈ `hc-tech-design`「做 → 用户确认 → 对抗评审 → 回改到过」。本 ADR 落**新项目分支**；入口先问「新项目 / 老项目」再分流（老项目分支占位、后续 ADR 补）。

2. **全程用户确认（铁律）**：选型 / 结构 / 规矩都是**选择**——skill 摆选项 + 讲取舍**让用户拍**，不替用户定、不"看见啥就落啥"（新项目里体现为「选择先确认」，老项目里体现为「扫到的先确认」）。

3. **新项目流程（7 步，每步先确认再落）**：
   1. **收基本信息**：项目名（kebab）、**目标 / 概述 / 技术栈（框架·语言）**；怎么问由 agent 临场把握。
   2. **搭最小骨架**：`projects/<名>/` + 精简 `AGENTS.md`（红线 / 栈 / 指针）+ 同级 `CLAUDE.md` shim。**只搭壳、不搭代码结构**（代码结构是 `hc-tech-design`/`hc-dev` 的活，接入 skill 不越界）。
   3. **记第一个决策 ADR**：选型 / 结构为啥这么定，落项目自己的决策记录。
   4. **接执行口（接线、不接假内容）**：`workspace/verification.yaml` 给项目占一条（`verify`/`unit`/`api`/`e2e`/`sandbox`），CI 认得它，**约定好将来脚本放哪、叫啥命令**；**脚本本体先占位**（新项目多半还没有真实运行内容）。
   5. **对抗评审搭出来的骨架**：派 `hc-onboard-reviewer` 挑刺——骨架**最小没越界**（没替 `hc-dev` 搭代码结构）/ `AGENTS.md` 红线合理、该下沉的下沉 / 选型 ADR 有据（备选 + 理由、用户拍过）/ `verification.yaml` 条目对、**每个占位都显式标记 + 有待补记录**（无静默空占位）/ 忠于用户确认的选择（没替用户擅自定）；回改到过。
   6. **收尾**：`make verify` 绿（占位都是显式标记态、非静默空）、项目登记好。
   7. **交棒**：骨架过审 → 指路 `hc-prd`/`hc-tech-design`/`hc-dev` 开发，第一个需求包按 rule-0001 立。

4. **占位不许静默空着（关键设计）**：第 4 步每个接入点（verify/unit/api/e2e/sandbox）的值是**三态之一**——
   - **真命令**（已接实）；
   - **`PENDING: <为啥现在空 / 补的条件>`**（待接实：`make verify` warn 提醒，同时项目 `AGENTS.md` 里留一条"待补"记录，agent 以后一进来就看见）；
   - **`N/A: <理由>`**（这项目不需要这个接入点）。
   - **静默空 / 留白 / 裸 `TODO` = 红**（fail-closed）。目标：占位**看得见 + 绕不过去**，防"开发过了却没补"。同 `test-cases-audit`「无·理由」逃生口的思路（显式标记 + 机检）。

5. **`create-X` 的取舍**：填占位的动作里——
   - **`create-sandbox` 单独成 skill**（sandbox 复杂：docker / 虚拟机 / 本地 / 远程，还要管起 / 停 / 查；且反复用——接入时、以后补 e2e 时都调它）。**本 ADR 不建，下一轮 ADR 建**；接入 skill 本轮只留 sandbox 占位。
   - **verify / ci 不单独成 skill**（太薄：verify=一条最小检查命令、ci=把命令塞进 GitHub workflow），当 `hc-onboard` 的**内部步骤**；哪天真变复杂再拆。

6. **sandbox 形式无关**：sandbox 契约管**统一入口（起 / 停 / 查健康）+ 语义**，形式（docker/vm/本地/远程/…）由项目自己实现、控制面只调那几个命令——这是 `create-sandbox` + sandbox 契约（后续摊）的活，接入 skill 不掺。

## 受影响

- 新增 `hc-onboard` skill（新项目分支）+ `hc-onboard-reviewer`（对抗评审，双栈，比照 `hc-tech-design-reviewer`）+ 可能 `templates/project-agents.md`（工程 AGENTS.md 骨架，通用中性、rule-0015）。
- `docs/harness/PROJECT_ONBOARDING.md`：从被动指南**瘦身为 skill 的引用 / 清单**（避免与 skill 复刻漂移，rule-0012）；**修其第 8 步过时引用**（"docs-maintainer skill" 已被 ADR-0012 的 `hc-doc-sync` 机制取代）。
- `workspace/verification.yaml` + `VERIFICATION_ROUTING.md`：加「占位三态」约定；新增 `scripts/verification-audit.sh`（占位显式态机检：真命令 / `PENDING:` warn / `N/A:` pass / 静默空红），进 `make verify`。

## 备选（拒）

- **接入 skill 连代码结构一起搭**：拒——代码结构是 `hc-tech-design`/`hc-dev` 的活，接入只搭壳、别越界重叠。
- **verify / ci 也各成 skill**：拒——太薄，三个 create-X 里只有 sandbox 够格独立；其余当内部步骤，避免维护负担。
- **占位就空着、后面记得补**：拒——"记得"靠不住；必须显式记录 + 机检盯，防开发过了没补。
- **接入 skill 里把 sandbox 契约一次做深**：拒——sandbox 形式多、是独立复杂契约，塞进接入 skill 会糊；拆给 `create-sandbox`（下一轮）。

## 影响 / follow-up

- 新项目从零就长在规范里、接入即被治理；占位有据可查 + 机检兜底，不留隐性空洞。
- **follow-up**：① `create-sandbox` skill + sandbox 契约（起/停/查、形式无关）；② `hc-onboard` 老项目分支（扫 → 消化历史资产 → 搬进规范 → 对齐）。
