---
title: 测试用例账本
status: active
owner: harness
last_updated: 2026-06-28
source_files: []
related_docs:
  - ../../.agents/skills/hc-test/SKILL.md
  - ../../templates/test-case.md
  - ../../scripts/test-cases-audit.sh
  - ../decisions/0008-test-case-skill.md
---

# 测试用例账本

由 `hc-test`（`hc-e2e-qa`） 产出的测试用例集在此登记。它管"**产出用例 + 用例对需求（验收点 AC + 功能点 FP）的覆盖**"，**不碰"过没过"**（执行结果是后面单独一段机制）。**独立**：AC/FP 当通用输入，与上游 `hc-prd` / 下游 `hc-dev` 的衔接后定。决策见 `../decisions/0008-test-case-skill.md`。

- 模板：`templates/test-case.md`
- 账本：`index.yaml`（每个用例集一条：`id` / `dir`（目录名，`test-cases-audit` 按此键校验）/ `title` / `status`（draft|reviewed）/ `source`）
- 每个用例集一个目录：`docs/test-cases/<id>/`
  - `test-cases.md`：`## 验收点 AC`（`- AC-n：…`）+ `## 功能点 FP`（`- FP-n：…`）+ `## 用例`（每条 `### TC-n`，带单行 `covers: AC-x, FP-y`）
  - **段标题承重**：硬闸按确切段切解析——声明只在以「验收点」/「功能点」起始的 `## ` 段内识别、`covers:` 只在以「用例」/「测试用例」起始的 `## ` 段内识别；改标题名 / 声明落段外 / `covers:` 折行会被判红或漏算（套 `templates/test-case.md` 照写）
- **覆盖软+硬分层**：硬闸 `scripts/test-cases-audit.sh`（进 `make verify`）从 `covers:` 机检"每条 AC/FP 都被 ≥1 用例覆盖 + 无悬空引用 + 账本一致"；质量（用例真覆盖语义 / 边界异常齐不齐）由 eval 考题 015 / rule-0014 判。
- **`covers:` 是覆盖关系的唯一真相源**，不另存手维护映射表（防漂，rule-0012）。
