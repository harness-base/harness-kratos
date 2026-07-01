---
title: ADR-0015 hc-tech-design 研发方案/技术设计阶段——交互式设计 skill，填 hc-prd 与 hc-dev 之间的设计空档
status: accepted
date: 2026-06-30
last_updated: 2026-06-30
source_files: []
related_docs:
  - 0010-prd-orchestration.md
  - 0009-dev-skill.md
  - 0014-hc-test-orchestration.md
  - ../harness/testing-flow.md
---

# ADR-0015：hc-tech-design 研发方案/技术设计阶段

## 背景

harness 现有 `hc-prd`（需求）→ `hc-dev`（实现），**中间没有"技术设计"这一档**。两个缺口因此暴露：① 设计错（接口怎么切、数据怎么存）要到写代码阶段才发现，纠错贵；② `hc-test` 的 **api 用例线**（ADR-0014 占位）需要**接口契约**当测试依据——而契约正是技术设计该产出的东西，没有它 api 用例无从写起。讨论确认：建 `hc-tech-design` 填这个设计空档。

## 决策

1. **`hc-tech-design` = 交互式设计 skill**（主 agent 当设计者），形态 ≈ `hc-dev` 的「做 → 派 reviewer 挑刺 → 回改」，**不是** `hc-prd` 那种总监 + 并行 worker（设计是一条连贯推演对话、不可拆并行）。链条：`hc-prd → hc-tech-design →（api 用例 / hc-dev）`。

2. **硬原则**（写进 skill 正文）：输入不限（PRD / 原型 / 用户故事 / 用户口述都行）；**参考项目代码与资产**（基于现状设计、不凭空）；不确定就**查 + 问**（rule-0008，不静默假设）；**决策点用户参与**（选型 / 接口 / 数据 让用户拍、留痕）；沟通**不抽象、讲前因后果**；**全明确才落可执行方案**（查/问/决策点把不确定消解掉，全明确 + 用户审核才落稿，**定稿零 TBD / 待确认、可直接实现**）；最后**对抗评审**。

3. **产物（项目专属，落 `docs/designs/<id>/`）**：`design.md`（9 段：背景&范围 / 业务流程 / 数据模型 / 接口设计[链接契约+原则] / 技术要点 / 关键决策+备选 / 影响范围 / 异常[业务码] / 安全&风险）+ `api-contract.md`（接口契约，单独文件：端点索引 + 每端点 请求/响应/Mock/错误码[只列约定内错误：业务码 / 校验 400·422 / 鉴权 401·403 / 约定服务态如 503 DB_UNAVAILABLE；未约定的未预期故障——裸 500 / panic——不进契约]/关联）。

4. **通用 / 项目隔离（控制面命根）**：`hc-tech-design` skill 与 `templates/design.md` `templates/api-contract.md` 是**通用控制面、不掺任何具体项目内容**（不预设多租户 / `tenant_id` 等）；**产出的方案是项目专属**。

5. **`hc-tech-design-reviewer` 子 agent（双栈）** 对抗评审：七块——基于现状不悬空 / 决策有据 / 接口契约逐字段可执行 / 异常+安全闭合 / **零 TBD** / 完整性(9 段) / 忠于需求源；rubric = rule-0008 + rule-0009；只评不改。

6. **解锁 api 用例线**：`hc-tech-design` 产出的接口契约即 ADR-0014 中 api 用例线的「研发方案 + 接口契约」前置；api 用例 worker（`hc-api-qa`/`hc-api-reviewer`）随后可建（仍占位、本 ADR 不建）。

## 受影响的 skill（rule-0007）

- skill：`hc-tech-design` ／ **是**——新建（交互式设计 + 用户审核门 + 对抗评审）。
- 子 agent：新增 `hc-tech-design-reviewer`（双栈，会话模型）。
- skill：`hc-test`（api 用例线）／ **是（前置解锁）**——其占位的「待研发方案」前置由 `hc-tech-design` 产出；`testing-flow.md` 的 api 用例线指向 `hc-tech-design` 为契约来源。
- skill：`hc-prd` / `hc-dev` ／ 否——上下游松耦合，`hc-prd` 出需求、`hc-dev` 实现（其"列 plan"是实现拆解，≠ hc-tech-design 的设计+契约，不重复）。
- 产物区：新增 `docs/designs/`（README + index.yaml）。
- 机检：新增 `scripts/designs-audit.sh`（进 `make verify`）——**两层防线**结构层：登记双向一致 / `design.md` 在 / 定稿零 TBD（扫 TBD·待确认·待定·待补·FIXME·TODO·留待实现）；与 `hc-tech-design-reviewer`（判断层：可执行 / 契约逐字段 / 决策有据 / 安全对账 / 忠于源 / 含糊措辞）分工不重复，同 `test-cases-audit ↔ hc-e2e-reviewer` 的机器查结构、reviewer 查判断口径。

## 备选方案

- **把接口契约塞进 `hc-dev` 的 plan 步**：拒——plan 是实现任务拆解，研发方案是设计 + 对外契约；混在一起则设计阶段评审弱、契约和实现绑死、api 用例无独立稳定依据。
- **`hc-tech-design` 做成 `hc-prd` 那种总监 + 并行 worker**：拒——设计是连贯推演对话、决策点环环相扣，拆并行反而割裂。
- **模板带项目内容（多租户 / `tenant_id` 等）**：拒——控制面命根是与具体项目隔离，模板必须通用。
- **方案定稿留"待确认"段**：拒——方案是全明确后才落、必须可执行；开放项在过程中（查/问/决策点）消解，不进定稿（lessons 2026-06-30）。

## 影响

- 填 `hc-prd → hc-dev` 之间的设计空档；接口契约让 api 用例线前置就绪。
- 新增 `hc-tech-design` skill + `hc-tech-design-reviewer` 双栈子 agent + `templates/design.md` + `templates/api-contract.md` + `docs/designs/` 产物区。
- 模板/skill 通用、方案项目专属——延续控制面 ↔ 项目隔离。
- 本 ADR 为决策留档；实现（skill / reviewer / 模板 / 账本）随本批，收尾 eval + `make verify`。
- **质量过程**：3 轮独立对抗评审——R1 揪出真 blocker（模板偷设 REST、对 kratos gRPC 不通用）+ 7 major 并修；R2 修口径自洽（reviewer 判据回灌 skill / 模板示例 / 文档登记 designs-audit）；R3 收敛到精修级，修掉 R2 引入的自相矛盾（"逐行对齐"→"⑧=各端点并集"、机检词表 4→7 统一、config 描述"不列 500"→约定/未约定）。
- **后续硬化（follow-up，本批不做、真用中迭代）**：reviewer 的「无对外接口 N/A」分支应回 `source` 核实属实（防假 N/A 逃避③对账）；reviewer 补**分页一致性**硬动作、**限流/熔断**视项目提示；`api-contract` 写端点模板补**幂等/并发槽位**（现 reviewer 要求但模板无槽，首轮易误判）；③数据模型多表写法 + async 协议骨架。均为 reviewer/模板的渐进硬化、非阻断。
