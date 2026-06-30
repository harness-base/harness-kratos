# PRD 编排式重构 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `prd-elicitation` 从线性交互 skill 重构成编排式——产品总监（编排逻辑，主 agent 当总监）调度一队专职 worker subagent，带 必选/可选·权重 + 确认门 + 并行产出 + review loop。

**Architecture:** SKILL.md 是「总谱」（描述 worker、权重模型、时序、两 review 点）；6 个 worker 各是一个 subagent 文件（双栈 `.claude/agents/*.md` + `.codex/agents/*.toml` + `config.toml` 注册）；一个 Workflow 模板（reference `.js`）把它们按总谱编排起来（主 agent 当总监跑）。外部调研走现成 `deep-research` skill（通用 subagent 调 Skill 工具跑，不另建 subagent）。常驻自主总监押后。

**Tech Stack:** markdown skills + subagent persona files（双栈 Claude/Codex）+ Claude Code Workflow 编排（`parallel`/`pipeline`/loop）+ harness 既有护栏（`make verify` / `skills-index` / `dir-index .claude/agents` / `docs-audit` / eval 子 agent / 对抗挑刺）。

**设计依据：** `docs/superpowers/specs/2026-06-29-prd-orchestration-design.md`（已 approved）。本计划不重复 spec，只把它拆成可执行任务。

**验证现实（本任务"测试"= 这些）：** 没有 pytest；每个产物的"测试"是 `make verify`（结构 + `skills-index --check` + `dir-index .claude/agents --check` + `docs-audit`）绿、`.codex/config.toml` 注册可解析、收尾跑**对抗挑刺（dogfood 新建的 prd-reviewer）** + **eval 子 agent**。

---

## File Structure

| 文件 | 职责 | 动作 |
|---|---|---|
| `docs/decisions/0010-prd-orchestration.md` | 编排式重构的 ADR（决策 + 受影响 skill 栏） | 新建 |
| `docs/decisions/index.yaml` | 登记 ADR-0010 | 改 |
| `.agents/skills/hc-prd/SKILL.md` | 编排「总谱」（worker 表 + 权重模型 + 时序 + 两 review 点 + 用户覆盖/留痕 + 门禁 + 演进） | 重写 |
| `.claude/agents/hc-prd-reviewer.md` | PRD 审稿员（对抗挑刺，rubric=eval 013/rule-0010；轻审地基 / 重审下游） | 新建 |
| `.claude/agents/hc-requirements-gatherer.md` | 需求采集员（多轮对话收需求） | 新建 |
| `.claude/agents/hc-user-story-writer.md` | 用户故事+AC 员（写 user-stories.md） | 新建 |
| `.claude/agents/hc-prd-writer.md` | PRD 本体员（写 prd.md） | 新建 |
| `.claude/agents/hc-feature-point-writer.md` | 功能点清单员（FP + 映射） | 新建 |
| `.claude/agents/hc-prototype-builder.md` | 原型员（可点原型） | 新建 |
| `.codex/agents/{prd-reviewer,requirements-gatherer,user-story-writer,prd-writer,feature-point-writer,prototype-builder}.toml` | 上述 6 个的 Codex 对等 | 新建 |
| `.codex/config.toml` | 注册 6 个 `[agents.<name>]` | 改 |
| `.agents/skills/hc-prd/references/orchestration-workflow.js` | Workflow 编排模板（总监逻辑：时序/并行/确认门/review loop/条件触发可选 worker/每 worker 配 model+prompt） | 新建 |
| `.claude/agents/README.md` / `.agents/skills/README.md` | 自动索引（dir-index / skills-index 重生成） | regen |
| `docs/context/CURRENT_STATUS.md` | `.agents/skills/` 行的 prd-elicitation 注脚（升级为编排式）；agents 行已指针化、无需动 | 改（小） |
| `docs/superpowers/specs/2026-06-29-prd-orchestration-design.md` | 设计稿（随本批提交） | 提交 |

**命名约定：** worker subagent 用 kebab 英文名（同 `code-reviewer`）。**历史不改写**：ADR-0003/0007 不动（ADR-0010 在 related_docs 引它们）。

---

### Task 1: ADR-0010 + 重写 SKILL 总谱

**Files:**
- Create: `docs/decisions/0010-prd-orchestration.md`
- Modify: `docs/decisions/index.yaml`
- Rewrite: `.agents/skills/hc-prd/SKILL.md`

- [ ] **Step 1: 写 ADR-0010**

照 `templates/adr.md` / ADR-0009 骨架写 `docs/decisions/0010-prd-orchestration.md`：frontmatter（title / status: accepted / date: 2026-06-29 / source_files 列 SKILL.md + workflow 模板 + 6 个 `.claude/agents/*.md` / related_docs 列 0003、0007、`../superpowers/specs/2026-06-29-prd-orchestration-design.md`）。正文：背景（prd-elicitation 线性 → 编排，模块化/权重/地基早审）→ 决策（1 产品总监=编排逻辑、主 agent 当、常驻自主押后；2 三层优先级 用户指令>必选>可选权重 + 覆盖必选提示后果+留痕；3 7 worker 表 + 落地双栈；4 时序：采集→可选调研(rule-0008 验收)→stories+AC 轻审 loop→确认门→并行产出→PRD 审稿 loop（框并行、回原 worker、只重跑有问题的）→收尾确认；5 两 review 点 轻审地基/重审下游；6 外部调研复用 deep-research）→ **受影响的 skill（rule-0007）栏**（prd-elicitation=是/重写；其余逐条否）→ 备选（薄编排/不拆 worker：拒，用户要全建为效果；常驻自主总监：押后）→ 影响。

- [ ] **Step 2: 登记 ADR-0010**

在 `docs/decisions/index.yaml` 末尾追加 `- id: ADR-0010 / title / status: accepted / date: 2026-06-29 / file: 0010-prd-orchestration.md`（照 ADR-0009 条目缩进）。

- [ ] **Step 3: 重写 `.agents/skills/hc-prd/SKILL.md` 成总谱**

保留 frontmatter（`name: prd-elicitation`，更新 description 为"编排式产出需求"、`version: 3`、`last_reviewed: 2026-06-29`，description 单行无半角竖线）。正文章节：① 开篇（编排式产出需求；总监=编排逻辑主 agent 当；依据 ADR-0010）→ ② 何时用/不用 → ③ **优先级与权重模型**（三层 + 覆盖必选规矩 + 跳过留痕，照 spec）→ ④ **worker 表**（7 行，照 spec 的表：干啥/必选·可选权重/形态/落地）→ ⑤ **时序**（照 spec 的时序块）→ ⑥ **两 review 点**（轻审地基 / 重审下游）→ ⑦ 门禁（rule-0010 / 考题 013；结构由 `prds-audit` 机检）→ ⑧ 演进（rule-0007，连同 6 worker + workflow 模板，改完跑 skills-index）。**贯穿约束**（不静默假设、rule-0008）保留。

- [ ] **Step 4: regen + verify**

Run: `bash scripts/skills-index.sh && make verify 2>&1 | tail -3`
Expected: `✓ 控制面自检通过`（含 skills 索引无漂移、decisions 索引一致、docs-audit 绿）。

- [ ] **Step 5: Commit**

```bash
git add docs/decisions/0010-prd-orchestration.md docs/decisions/index.yaml .agents/skills/hc-prd/ docs/superpowers/specs/
git commit -m "prd 编排式重构(ADR-0010): 总谱 + 权重模型 + 设计稿"
```
（pre-commit 若误报危险命令字符串，确认是文档引用后 `--no-verify`。）

---

### Task 2: PRD 审稿员 subagent（双栈，最关键 worker）

**Files:**
- Create: `.claude/agents/hc-prd-reviewer.md`
- Create: `.codex/agents/hc-prd-reviewer.toml`
- Modify: `.codex/config.toml`

- [ ] **Step 1: 写 `.claude/agents/hc-prd-reviewer.md`**

照 `code-reviewer.md` 结构（frontmatter `name`/`description`/`tools: Read, Glob, Grep, Bash` 只读 + system prompt）。角色独特内容：独立 PRD 审稿员（对抗式），rubric = eval 013 / rule-0010；**两种模式**：①轻审地基（输入=user-stories.md，只盯 AC 可观测可验证 / 故事完整无遗漏 / 内部一致 / 对齐采集需求，1-2 轮）；②重审下游（输入=整套 stories+PRD+FP+原型，查 PRD 合故事 / FP 覆盖映射齐 / 原型可点 / 四态，多轮到零）。回结构化清单（`文件:位置` / 严重度 / 问题 / 证据 / 修法 + 指出该回哪个 worker 改）。**只评不改**；对"用户强制跳过 X 且已留痕"的项不当缺陷扣分。

- [ ] **Step 2: 写 `.codex/agents/hc-prd-reviewer.toml`**

照 `code-reviewer.toml` 结构（`name`/`description`/`model_reasoning_effort = "high"`/`developer_instructions` 三引号），内容与 `.md` system prompt 一致。

- [ ] **Step 3: 注册进 `.codex/config.toml`**

在末尾加：
```toml
[agents.prd-reviewer]
description = "PRD 审稿员：对抗挑刺用户故事/AC 与整套 PRD（rubric=eval 013/rule-0010），轻审地基/重审下游。免 API key。"
config_file = "agents/hc-prd-reviewer.toml"
```

- [ ] **Step 4: regen + verify**

Run: `bash scripts/dir-index.sh .claude/agents && make verify 2>&1 | tail -2`
Expected: `✓ 控制面自检通过`（`.claude/agents` 索引无漂移）。

- [ ] **Step 5: Commit**

```bash
git add .claude/agents/hc-prd-reviewer.md .claude/agents/README.md .codex/agents/hc-prd-reviewer.toml .codex/config.toml
git commit -m "prd 编排: 新增 prd-reviewer 子 agent(双栈) 审稿员"
```

---

### Task 3: 5 个产出 worker subagent（双栈）

**Files:**
- Create: `.claude/agents/{requirements-gatherer,user-story-writer,prd-writer,feature-point-writer,prototype-builder}.md`
- Create: `.codex/agents/{同上}.toml`
- Modify: `.codex/config.toml`

每个 worker 都照 `code-reviewer.md` 的 frontmatter+system-prompt 结构。**各自独特内容（写进 system prompt）：**

| name | tools | 角色 / 该干啥 / 产出 |
|---|---|---|
| `requirements-gatherer` | Read, Glob, Grep, Bash | 需求采集员：多轮引导对话收原始需求（用户与 JTBD / 页面流程 / 数据 / 四态 / 边界 / 验收目标 / 非目标）；**不静默假设**（缺信息去查事实源或问用户，rule-0008）；产出结构化需求摘要交回总监。 |
| `user-story-writer` | Read, Glob, Grep, Write, Bash | 用户故事+AC 员：把需求写成 `docs/prds/<id>/user-stories.md`（每条 `US-NN` + 可观测 `AC`），套 `templates/user-story.md`；产出后由 prd-reviewer 轻审、再交用户确认。 |
| `prd-writer` | Read, Glob, Grep, Write, Bash | PRD 本体员：按已确认用户故事写 `docs/prds/<id>/prd.md`，套 `templates/prd.md`，业务逻辑清晰、旧场景简/繁分别处理（references/prd-writing.md）。 |
| `feature-point-writer` | Read, Glob, Grep, Write, Bash | 功能点清单员：写功能点 + `US↔FP↔正文` 三级双向映射；目标 100% 覆盖、无孤儿。 |
| `prototype-builder` | Read, Glob, Grep, Write, Bash | 原型员：可点 HTML 原型落 `docs/prds/<id>/prototype/`，mock 数据无后端，与现有前端内容/风格一致（不复刻，ADR-0003）。 |

- [ ] **Step 1: 写 5 个 `.claude/agents/*.md`**（各按上表的角色/tools/产出写 system prompt）。
- [ ] **Step 2: 写 5 个 `.codex/agents/*.toml`**（与各自 `.md` 一致，`model_reasoning_effort = "high"`）。
- [ ] **Step 3: 在 `.codex/config.toml` 注册 5 个 `[agents.<name>]`**（照 prd-reviewer 块格式，各指 `agents/<name>.toml`）。
- [ ] **Step 4: regen + verify** — `bash scripts/dir-index.sh .claude/agents && make verify 2>&1 | tail -2` → `✓ 控制面自检通过`。
- [ ] **Step 5: Commit** — `git add .claude/agents/ .codex/agents/ .codex/config.toml && git commit -m "prd 编排: 5 个产出 worker 子 agent(双栈)"`。

---

### Task 4: Workflow 编排模板

**Files:**
- Create: `.agents/skills/hc-prd/references/orchestration-workflow.js`

- [ ] **Step 1: 写编排模板**

一个 reference Workflow 脚本（`export const meta = {...}` + 脚本体），实现总监逻辑：
- 阶段 `pipeline`/顺序：`requirements-gatherer`（人在环）→（`budget`/`args` 判定）可选 `deep-research`（外部调研，过 rule-0008 验收）→ `user-story-writer`。
- **stories 轻审 loop**：`while` 循环 ≤2 轮，每轮 `agent(..., {agentType:'prd-reviewer'})` 轻审模式 → 有问题回 `user-story-writer` 重跑 → 到零或轮数尽 → 停（确认门交主 agent 找用户）。
- **确认门**：脚本停在此，由主 agent（总监）拿给用户 approved 再继续（注释写清这是人在环断点）。
- **并行产出**：`parallel([prd-writer, feature-point-writer, (条件)prototype-builder])`，每个 `agent(prompt, {agentType, model})` 配各自档。
- **重审 loop**：`while`（loop-until-dry：连续干净才停）：`prd-reviewer` 重审整套 → 把"有问题的 worker"挑出 → **只重跑那个 worker**（`agent` 同 agentType + 喂产物+发现）→ 复审 → 到零。
- 注释标清：必选/可选·权重、用户覆盖断点、留痕点。
- 用 `log()` 标阶段；schema 约束 reviewer 结构化输出。

- [ ] **Step 2: 语法自检**

Run: `node --check .agents/skills/hc-prd/references/orchestration-workflow.js 2>&1 || echo "需 node；或人工核 meta 字面量 + 无 TS 语法"`
Expected: 无语法错（meta 是纯字面量、无 `Date.now()`/`Math.random()`）。

- [ ] **Step 3: Commit** — `git add .agents/skills/hc-prd/references/ && git commit -m "prd 编排: Workflow 编排模板(总监逻辑)"`。

---

### Task 5: doc-sync + 全量 verify

**Files:**
- Modify: `docs/context/CURRENT_STATUS.md`（`.agents/skills/` 行给 prd-elicitation 加"编排式(ADR-0010)"注脚；agents 行已指针化无需动）
- 视情况：`docs/README.md`（若要给 `docs/superpowers/` 加路由行）

- [ ] **Step 1: 改 CURRENT_STATUS** — prd-elicitation 注脚标"编排式 ADR-0010"；确认 `.claude/`/`.codex/` 行（已指针化/双栈对齐）仍准。
- [ ] **Step 2: 全量验证** — `make verify && make docs-audit` → 两个都 `✓`。
- [ ] **Step 3: Commit** — `git add docs/ && git commit -m "prd 编排: doc-sync (CURRENT_STATUS)"`。

---

### Task 6: 收尾——对抗挑刺（dogfood）+ eval

- [ ] **Step 1: 对抗挑刺（用本批新建的 prd-reviewer + code-reviewer dogfood）**

跑一个 Workflow 多视角审本批改动：① 设计一致性（SKILL 总谱 ↔ 6 worker ↔ workflow 模板 ↔ ADR-0010 是否自洽、worker 表与 spec 一致）；② 删改/漂移（CURRENT_STATUS、索引、`.codex` 注册齐不齐、双栈一致、有无"删旧建新连带漂移"）；③ 准确性（worker tools 是否最小、prd-reviewer 轻/重审两模式是否都写清、用户覆盖/留痕是否落地）；④ workflow 模板（确认门断点、review loop 只重跑有问题 worker、条件触发可选 worker 是否对）。每条独立证伪 → 修平 → 复跑到零。

- [ ] **Step 2: 收尾 eval**

spawn `eval` 子 agent，task slug `prd-orchestration`，level L4，考题 010（收尾）/ 011（架构同步 skill）/ 013（用户故事/PRD 完整性——核 prd-reviewer rubric 对不对）/ 014（不硬编码枚举）；写 `docs/eval/task-reviews/<时间戳>-prd-orchestration/`。green 才算收。

- [ ] **Step 3: 补 `tasks/todo.md` Review 段 + Commit**

```bash
git add docs/eval/task-reviews/ tasks/
git commit -m "prd 编排: 对抗挑刺修平 + 收尾 eval"
```

---

## Self-Review（对照 spec）

- **Spec 覆盖**：三层优先级→Task1 SKILL ③+ADR；7 worker→Task2/3（审稿员+5 产出，外部调研复用 deep-research 在 SKILL 写明）；时序+两 review 点→Task1 SKILL ⑤⑥ + Task4 workflow；用户覆盖/跳过留痕→Task1 SKILL ③ + prd-reviewer 不扣分逻辑（Task2）；review loop 框并行+回原 worker+只重跑有问题的→Task4 workflow Step1。✓ 无遗漏。
- **押后项**（常驻自主总监 / 通用 loop 引擎 / observability / MCP）：不在任何 task，符合 spec YAGNI。✓
- **占位扫描**：worker 的 system-prompt 全文在实现时按上表角色写实（非"similar to"）；workflow 模板给了结构而非空 TODO。实现时每个 worker 写满、勿留 TBD。
- **一致性**：subagent 命名（prd-reviewer / requirements-gatherer / user-story-writer / prd-writer / feature-point-writer / prototype-builder）six 处一致；双栈三件套（.md + .toml + config 注册）每个都列。✓

## 风险 / 注意

- **大活**：6 个 subagent 双栈 = 18 文件 + skill 重写 + workflow + ADR。建议**分 commit**（已按 Task 切）。
- **prds-audit 现状**：它校验 `prd.md` 必备章节 + 登记一致；本次不改账本结构，空账本仍绿。若以后要机检"用户故事 approved + 留痁字段"，soft→hard 另起。
- **历史不改写**：ADR-0003/0007 不动；全仓 grep `feature-delivery`/旧措辞确认无新悬空（收尾挑刺维度②兜）。
- **pre-commit 误报**：文档提危险命令字符串会被拦，确认后 `--no-verify`（已知毛刺，有独立 task）。
