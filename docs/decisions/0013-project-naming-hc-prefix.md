---
title: ADR-0013 项目定名 harness-control，skill / 子 agent 统一前缀 hc-（存量全量改名，历史不改写）
status: accepted
date: 2026-06-30
last_updated: 2026-06-30
source_files:
  - ../../.agents/skills/README.md
  - ../../.claude/agents/README.md
related_docs:
  - 0010-prd-orchestration.md
  - 0011-demote-context-loading.md
  - 0012-doc-sync-redesign.md
---

# ADR-0013：项目定名 harness-control，skill / 子 agent 统一前缀 hc-

## 背景

skill 与子 agent 的名字此前随场景起（`prd-elicitation` / `dev` / `eval` / `code-reviewer` …），项目本身也没有一个统一标识。两个痛点：① 没法一眼看出某个 skill/agent 归属本控制面；② `dev` / `eval` 这类**通用词**当标识，跟"开发""评测"等普通词义混在一起，检索和指代都易混。讨论后用户拍板给项目定名并统一前缀。

## 决策

1. **项目定名 `harness-control`，所有 skill 与子 agent 统一前缀 `hc-`**（`hc-prd` / `hc-dev` / `hc-eval` / `hc-code-reviewer` …）。今后**新建** skill / 子 agent 一律 `hc-` 开头。
2. **改名形态**：多数是**加前缀**（`dev→hc-dev`、`code-reviewer→hc-code-reviewer`）；`prd-elicitation` 名字过长，**缩短**为 `hc-prd`（不是 `hc-prd-elicitation`）。
3. **存量全量改名**（用户选「A」）：5 个 skill（`hc-prd` / `hc-dev` / `hc-add-rule` / `hc-git-workflow` / `hc-self-evolution`）+ 10 个子 agent 全部改名。`test-case` skill 暂留原名（另起一档重建为 `hc-test`）。**活引用**全跟改：workflow `agentType:` 派活、frontmatter `source_files`/`related_docs` 路径、prose 互引、`.codex/config.toml` 的 `[agents.X]` 注册。
4. **历史不改写（叙述保留 / 路径指针跟改）**：`docs/decisions/`（ADR）、`docs/eval/task-reviews/`、`tasks/lessons.md`、`tasks/optimization-log.md`、`tasks/archive/`、`docs/features/`、`docs/superpowers/`（specs+plans）里的**叙述 / 决策文字（含旧 skill/agent 名）一律保留**——改了等于篡改历史。但**文件路径引用一律随 `git mv` 跟改到新位置**，不分 frontmatter 还是正文：① frontmatter 的 `source_files`/`related_docs`（不改 `docs-audit` 会红）；② 正文里的 `文件:行` 路径锚点 / `修复入口：<路径>` / `新增 <路径>` 等——这些是**指向真实文件的指针**（不跟改就点到死链），不是"决策叙述"。判据：**名是叙述（留旧）、路径是指针（跟文件走）**。ADR 文件名本身不改（如 `0009-dev-skill.md`）。
5. **接受「活引用=hc- / 历史正文=旧名」的分裂**：拿"历史可信（`git mv` 保留改名追溯）+ 当前一致"换"不强行重写历史"。读旧 ADR 见 `prd-elicitation` 等旧名属正常。
6. **语义词不盲 sed**：`dev` / `eval` / `self-evolution` 既是标识又是普通词（`make eval`、`docs/eval/`、`/dev/null`、"自进化闭环"），逐处判语境，只有指那个 skill/agent 时才改。

## 受影响的 skill（rule-0007）
- skill：`hc-prd` / `hc-dev` / `hc-add-rule` / `hc-git-workflow` / `hc-self-evolution` ／ **是**——本 ADR 即其改名（仅 `name` + 互引变，内容逻辑不变）。
- skill：`test-case` ／ 否——暂留原名，待 `hc-test` 重建一并处理。
- 子 agent（10 个）／ **是**——全部加 `hc-` 前缀（`.claude/*.md` + `.codex/*.toml` + `config.toml` 注册同步）；workflow 模板 `hc-prd/references/orchestration-workflow.js` 的 `agentType` 跟改。
- 索引：`.agents/skills/README.md` / `.claude/agents/README.md` 自动重生成（`skills-index` / `dir-index --check` 守）。

## 备选方案
- **只改新建、存量不动**：拒——长期"新 hc- / 老旧名"混存更乱，用户明确要全量（A）。
- **连历史正文一起改写**：拒——篡改 ADR / lessons / eval-review 的决策叙述，失去历史可信；`git mv` 已足够追溯。
- **只定项目名、不加前缀**：拒——`dev`/`eval` 与通用词混淆是主要痛点，前缀才解耦。

## 影响
- 统一 `hc-` 前缀：一眼识别归属 + 与通用词解耦。
- 改动约 90+ 项（40 个 `git mv` 重命名 + 约 50 个改/新增内容，含本 ADR 与索引登记、收尾文档）；`make verify` + `docs-audit` 全绿、活文件 0 bare 旧名残留、无 `hc-hc-` 双前缀。
- 会话 agent registry 重载后按 `hc-` 名解析（workflow `agentType` 已指 `hc-`）。
- **后续约定**：新建 skill/agent 一律 `hc-` 开头（落入 `hc-self-evolution` / skill 新建流程的提示）。
- 一次踩坑已记 `tasks/lessons.md`（2026-06-30）：缩短式改名 ≠ 加前缀，批量 sed 误产 `hc-prd-elicitation`，自查 grep 新名修复。
