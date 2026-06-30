---
title: ADR-0014 hc-test 测试编排——测试总监调度专职 worker、干掉 test-case 并入；e2e 本期实现、api/脚本占位
status: accepted
date: 2026-06-30
last_updated: 2026-06-30
source_files: []
related_docs:
  - 0008-test-case-skill.md
  - 0010-prd-orchestration.md
  - 0011-demote-context-loading.md
  - ../harness/testing-flow.md
  - ../superpowers/specs/2026-06-29-testing-flow-vision.md
---

# ADR-0014：hc-test 测试编排

## 背景

`test-case`（ADR-0008）是独立、线性的"产用例 + 覆盖闸"skill。要把测试做成**编排式**（同 PRD ADR-0010）：一个**测试总监**调度专职 worker，支持多场景（e2e / api / 脚本 / 回归）。完整流程见 `../harness/testing-flow.md`（总纲），北极星愿景见 `../superpowers/specs/2026-06-29-testing-flow-vision.md`。**分阶段【实现】、整体【发布】**。

## 决策

1. **`hc-test` = 测试总监 skill**（编排逻辑、**主 agent 当总监**），全量设计、**留拓展位**。派活：默认按"手上产物 + 到了哪一步"自动选场景（A），**用户指令最高优先级**可点名 / 跳过（沿用 `hc-prd` 总监"默认编排 + 用户覆盖"模式）。每步解耦、可跳过。

2. **干掉 `test-case` skill**；其规范（用例产出标准 / `covers:` 唯一真相源 / `test-cases-audit` 闸）**上提到 `hc-test` 总谱 + worker 上下文**。`rule-0014` + 考题 015 **不动**（是 skill 无关的产出标准）。

3. **本期实现 e2e 线**：`hc-e2e-qa`（写用例）+ `hc-e2e-reviewer`（审用例），**双栈子 agent、会话模型**（免 key，同其余 9 个；**非 haiku**）。输入优先级 **AC > FP > US > PRD**（缺则略）。要求**写进子 agent 上下文**（不只靠模板）。模板 `templates/e2e-test-case.md`（覆盖矩阵 + 用例字段）。

4. **两层覆盖防线**：
   - **机检**（扩 `test-cases-audit`）= **结构层**：交互点 × 类型覆盖矩阵完整性 + `covers:` 闭合，带 **"无·理由"逃生口**（成功/失败格要么有 TC、要么显式标 `无·理由:<…>`；空着没理由 = 红，防误伤）。
   - **`hc-e2e-reviewer`** = **判断层**：① 主观覆盖率、② 引用源符合 + 理解偏差、③ 用例质量（等价类/边界/预期锚定/牵强），判据 考题 015 + rule-0009。
   - 机器查结构、reviewer 查判断，**不重叠**。

5. **占位**（全量设计留位，待前置就绪、整体发布）：`hc-api-qa`/`hc-api-reviewer`（待研发方案 + 接口契约）、接口契约对照（待 routelist）、`hc-script-impl`/`hc-script-reviewer`（待 sandbox）、统一回归。

6. **流程总纲存 `docs/harness/testing-flow.md`**（唯一真相源；各 skill / 子 agent / ADR **按需引、不复制**）。

## 受影响的 skill（rule-0007）

- skill：`test-case` ／ **是**——删除，职责并入 `hc-test`（总监）+ `hc-e2e-qa`（worker）；**ADR-0008 被本 ADR 取代**（superseded，历史正文不改写）。
- skill：`hc-test` ／ **是**——新建（总监总谱）。
- skill：`hc-prd` / `hc-dev` 等 ／ 否——上下游松耦合不变。
- 子 agent：新增 `hc-e2e-qa` / `hc-e2e-reviewer`（双栈，会话模型）；api / 脚本 worker 占位待建。
- 机检：`test-cases-audit` ／ **扩**——加覆盖矩阵完整性（带逃生口）。
- 规则 / 考题：`rule-0014` / 考题 015 **不变**（产出标准，skill 无关）。

## 备选方案

- **保持 `test-case` 线性 skill**：拒——要编排 + 多场景 + 各配 worker（同 PRD 的理由）。
- **现在就建全部 worker（api / 脚本）**：拒——前置（研发方案 / sandbox）未就绪，建了空跑；占位即可，加时填空。
- **e2e 用例靠模板约束就够、不写进 agent 上下文**：拒——约束主体应在 agent 上下文，模板只定形状（lessons 2026-06-30）。
- **分阶段发布（e2e 先单独上线）**：拒——这是**分阶段实现、整体发布**；e2e-only 阶段是 WIP、不单独 merge，免得半套测试链误导。

## 影响

- 测试流程模块化、多场景、覆盖**双层防线**（机检结构 + reviewer 判断）。
- 删 `test-case` skill + 新建 `hc-test` 总谱 + 2 个 e2e 子 agent（双栈）+ `templates/e2e-test-case.md` + 扩 `test-cases-audit`；流程总纲落 `docs/harness/testing-flow.md`。
- 发布口径：e2e + api + 脚本全套实现完才发布（**分阶段实现 ≠ 分阶段发布**）。
- ADR-0008 superseded（历史正文不改写，本 ADR `related_docs` 引它并在此声明取代）。
- 本 ADR 为**决策留档**，实现（建 skill / 子 agent / 模板 / 扩机检 / 删 test-case）随后单独成批，收尾 eval + `make verify`。
