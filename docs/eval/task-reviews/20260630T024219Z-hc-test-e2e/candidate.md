# 候选产物副本 — hc-test 测试编排 · e2e build（ADR-0014）

> 评委亲取证副本。原文以仓内文件为准；本文留交付清单 + 关键产物落点，便于复核。

## 任务

把测试做成编排式（同 PRD ADR-0010）：测试总监调度专职 worker，删 `test-case` 线性 skill 并入 `hc-test`；本期实现 e2e 线，api/脚本占位（分阶段实现、整体发布）。

## 交付清单（候选声称）

- ADR：`docs/decisions/0014-hc-test-orchestration.md`（已登记 `index.yaml`）+ 流程总纲 `docs/harness/testing-flow.md`（唯一真相源）
- skill 总谱：`.agents/skills/hc-test/SKILL.md`（删了旧 test-case skill）
- 双栈子 agent：`.claude/agents/hc-e2e-qa.md` + `hc-e2e-reviewer.md`（+ `.codex/agents/hc-e2e-{qa,reviewer}.toml` + `.codex/config.toml` 注册）
- 模板：`templates/e2e-test-case.md`（含「交互点 × 类型 覆盖矩阵」）
- 机检扩展：`scripts/test-cases-audit.sh` 加「覆盖矩阵完整性 + 无·理由逃生口」+ 守护测试 `scripts/test-cases-audit.test.sh`
- 删 test-case skill + 修 9 处引用（4 frontmatter 路径 → hc-test/SKILL.md、5 prose → hc-test）

## 判据（考题）

- 015 测试用例产出标准（本批建的是 ENFORCE 该标准的 skill + 机检 + reviewer）
- 011 改架构/接口须回顾 skill（ADR-0014 受影响栏名实相符）
- 010 收尾 eval 综合质量

## 关键产物落点（评委读取确认）

- 流程唯一真相源：`docs/harness/testing-flow.md`（active）；北极星 vision：`docs/superpowers/specs/2026-06-29-testing-flow-vision.md`（存在）
- 矩阵硬闸：`scripts/test-cases-audit.sh` §④（MATRIX 段解析，成功/失败/边界三列每格须 TC-NN 或「无·理由:」，占位 `< > …` 判红）
- 守护测试：`scripts/test-cases-audit.test.sh` #25–#28（矩阵闸 4 例，与正向锚仅差一格 → 变异自证）
- 双栈：`.claude/agents/hc-e2e-{qa,reviewer}.md`（无 model 字段=会话模型；qa 有 Write、reviewer 无 Write）+ `.codex/agents/hc-e2e-{qa,reviewer}.toml` + `.codex/config.toml` `[agents.hc-e2e-qa]`/`[agents.hc-e2e-reviewer]` 注册
