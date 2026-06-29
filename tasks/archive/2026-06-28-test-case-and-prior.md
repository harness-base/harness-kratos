# 归档 2026-06-28（test-case + 之前）

> 详账在各 commit 与 `docs/eval/task-reviews/`；此处只留指针级摘要。

## test-case-skill（L3，已提交 c0c94f6，eval green）
独立"测试用例"skill：产出用例 + 管"用例对 AC/FP 覆盖"，不碰执行结果。软+硬分层（硬闸 `scripts/test-cases-audit.sh` 严格 + fail-closed 解析 + 守护 25 条全变异自证；软靠对抗评审 + eval 015）；covers 单一真相源。产物：`templates/test-case.md`、`docs/test-cases/`、rule-0014、eval 015、ADR-0008。4 轮对抗评审修 25 真问题、驳 ~16 牵强；收尾 eval green（010/011/014 pass，015 空账本 n/a）。详见 `docs/eval/task-reviews/20260628T112905Z-test-case-skill/`。

## prd-workflow-redesign（L3，已提交 cbfbc7b）
重做"产出需求"流程（分阶段确认门 + 真相/需求源优先级 + 功能点覆盖）+ stop-check 收尾闸修复 + 对抗评审修 16 处；ADR-0007。详见 `docs/eval/task-reviews/20260627T1630Z-prd-workflow-redesign/`。

## kratos-base 切片（标准上下文）
S0~S6 done，F-0001~0006，全量 24 AC PASS。详见各 `docs/eval/task-reviews/` 与 `docs/features/`。
