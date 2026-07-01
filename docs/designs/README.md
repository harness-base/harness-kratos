---
title: 研发方案 / 技术设计产出账本（design + 接口契约）
status: active
owner: harness
last_updated: 2026-06-30
source_files: []
related_docs:
  - ../../.agents/skills/hc-tech-design/SKILL.md
  - ../../templates/design.md
  - ../../templates/api-contract.md
  - ../decisions/0015-hc-tech-design.md
  - ../harness/testing-flow.md
---

# 研发方案 / 技术设计（designs）

由 `hc-tech-design` skill 交互式产出的 **技术设计 + 接口契约**，填 `hc-prd`（需求）与 `hc-dev`（实现）之间的设计空档：`hc-prd → hc-tech-design →（api 用例 / hc-dev）`。接口契约被 **api 用例线**消费（ADR-0014 / 0015）。**方案是项目专属**，skill 与模板是通用控制面。

- 模板：`templates/design.md`（研发方案 9 段）、`templates/api-contract.md`（接口契约，单独文件）
- 账本：`index.yaml`（每条：`id` / `dir`（目录名）/ `title` / `status`）
- 每个设计一个目录：`docs/designs/<id>/`
  - `design.md`：研发方案主文档（背景&范围 / 业务流程 / 数据模型 / 接口设计 / 技术要点 / 关键决策+备选 / 影响范围 / 异常 / 安全&风险）；**全明确才落、零 TBD、可执行**
  - `api-contract.md`：接口契约（端点索引 + 每端点 请求/响应/Mock/错误码/关联）——api 用例的测试依据。错误码**只列约定内错误**（业务码 / 校验 400·422 / 鉴权 401·403 / 约定服务态如 503 DB_UNAVAILABLE）；**未约定的未预期故障**（裸 500 / panic）不进契约。**有对外接口才产 api-contract**；纯内部重构 / 数据迁移类、无对外接口的设计标 N/A、不强凑（契约可选）。
- **两层防线**：结构层（登记双向一致 / `design.md` 在 / 定稿零 TBD——扫 TBD·待确认·待定·待补·FIXME·TODO·留待实现）由 `scripts/designs-audit.sh` 机检（已进 `make verify`）；判断层（可执行 / 契约逐字段 / 决策有据 / 安全对账 / 忠于源 / 含糊措辞）由 `hc-tech-design-reviewer` 对抗评审。机器查结构、reviewer 查判断、不重复——同 `test-cases-audit ↔ hc-e2e-reviewer` 的分工。
- 流程与产出门槛见 `../decisions/0015-hc-tech-design.md`；与实现/测试体系**松耦合**。
