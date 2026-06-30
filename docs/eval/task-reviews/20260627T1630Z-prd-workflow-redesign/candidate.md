# candidate：prd-workflow-redesign（L3）

> 候选产物副本/清单。评审时各文件已逐个 Read，下面登记路径 + 关键锚点行，便于复核。

## 决策
- docs/decisions/0007-prd-workflow-redesign.md（5 步/两套优先级/三级覆盖/受影响 skill 栏）

## skill（重写）
- .agents/skills/hc-prd/SKILL.md（version: 2 / last_reviewed: 2026-06-28；贯穿约束 + 5 步工作流）

## 模板
- templates/user-story.md（新；US-NN / us_status: draft|in-review|approved）
- templates/prd.md（改；"## 功能点清单 + 覆盖映射" + US↔FP↔正文表）

## 写作指南（新）
- .agents/skills/hc-prd/references/prd-writing.md

## 护栏
- scripts/prds-audit.sh（必备章节加"## 功能点清单" + user-stories.md 存在校验）
- docs/eval/prompts/013-prd-completeness.md（更新：用户故事先行+功能点覆盖）

## 连带修复
- scripts/stop-check.sh（只在 todo 有 ## Review 段时才拦——mid-task 不误拦）
- scripts/stop-check.test.sh（守护测试，case1/2 仅差 Review 段）
- docs/harness/HOOKS.md（同步新行为 + 局限）

---

## 关键快照：scripts/stop-check.sh:25（修复行）
```sh
if [ -n "$level" ] && [ "${level#L}" -ge 2 ] 2>/dev/null && grep -qE '^##[[:space:]]*Review' "$TODO"; then
```

## 关键快照：scripts/prds-audit.sh:24（必备章节）
```sh
  for sec in "## 范围" "## 功能点清单" "## 页面与流程" "## 状态"; do
```

## 关键快照：ADR-0007 受影响 skill 栏（:37-40）
```
## 受影响的 skill（rule-0007）
- skill：prd-elicitation ／ 是否已更新：**是**——本 ADR 即其流程重做（5 步 + 贯穿约束 + 两套优先级 + 覆盖），随实现重写 `SKILL.md` 并 bump version/last_reviewed。
- skill：feature-delivery ／ 是否已更新：**否**——下游不变，仍与产出需求松耦合（PRD 非强制），衔接口（PRD 派生 feature 包）未改。
- skill：其余（context-loading / add-rule / doc-sync / git-workflow / bugfix / self-evolution）／ 否——与产出需求流程无关。
```
