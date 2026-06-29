# Candidate — test-case skill（L3）

新增独立 `test-case` skill：产出测试用例 + 管「用例对需求（AC + FP）的覆盖」，不碰执行结果。

## 候选产出（已逐一 Read 核验）
- `docs/decisions/0008-test-case-skill.md`（ADR，含「受影响 skill (rule-0007)」栏逐 skill 填 yes/no + 理由）
- `.agents/skills/test-case/SKILL.md`（skill 正文，已被 skills-index 自动收录）
- `templates/test-case.md`（模板 + 格式契约）
- `scripts/test-cases-audit.sh`（硬闸：覆盖闭合 + 账本一致，严格 + fail-closed 解析）
- `scripts/test-cases-audit.test.sh`（守护测试 25 条）
- `docs/test-cases/index.yaml` + `docs/test-cases/README.md`（账本，空：`test-cases: []`）
- `docs/eval/prompts/015-test-case-coverage.md` + `docs/eval/index.yaml`（015 已登记，rule-0014）
- `AGENTS.md` rule-0014（`<!-- rule: rule-0014 | sev: blocker | eval: 015 -->`）
- 接线：`scripts/verify-control-plane.sh`（L35 守护测试 + L91 硬闸两段，均为 live verify step）
- doc-sync：`docs/context/CURRENT_STATUS.md` / `docs/README.md` / `scripts/README.md`

## 复核命令与结果（评委实跑）
- `bash scripts/test-cases-audit.test.sh` → `pass=25 fail=0`
- `make verify` → ✓ 控制面自检通过（含「测试用例覆盖自检 ✓」「测试用例自检自测 pass=25」「rules 索引无漂移 + eval 指针有效」「状态文档未硬编码 skill 枚举 rule-0012」）
- `make docs-audit` → ✓ 通过（27 篇带 frontmatter 文档）
- 变异验证（评委自做，证守护测试 load-bearing）：
  - 阉割覆盖检查① → 守护测试 pass 25→19（6 条转红），还原后回 25。
  - 破坏剥围栏逻辑 → 守护测试 pass 25→22（3 条转红），还原后回 25。
  - 两处变异后均 `diff -q` 确认脚本完整还原（RESTORED-OK）。
