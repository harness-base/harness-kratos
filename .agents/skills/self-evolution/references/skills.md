# skills 审查手册

> 维度 1 / self-evolution 检查层。skill = **操作层流程的固化体**；本维度审"流程有没有 skill、触发准不准、索引漂不漂、arch 变了跟没跟"。

## 规范（健康长什么样 / 不变量）

- **会触发的流程都有 skill**：常见任务类型（立规则 / 交付需求 / 出 PRD / git 写操作 / 定档 / 自检）都能在 `.agents/skills/` 找到对应 skill，不靠口头约定。
- **description 触发准、不重叠不漏**：每个 `description` 写清"何时用 / 何时不用"，覆盖该触发的场景，又不和别的 skill 抢同一类任务。
- **进自动索引、防漂移**：每个 skill 有 `SKILL.md`，frontmatter 含 `name / description / version / last_reviewed`；`.agents/skills/README.md` 由 `scripts/skills-index.sh` 从 frontmatter 自动生成，**禁手改**，`--check` 进 `make verify`。
- **arch 变了回顾（rule-0007）**：架构 / 接口 / 流程变了，相关 skill 必须跟着改或在 ADR 写"无需更新 + 理由"；`SKILL.md` 末尾的「演进」段指明它依赖谁、何时回顾。
- **常驻列表干净**：每条 description 都进每会话常驻技能列表，是上下文成本——偶尔用的能力做成 `references/`（按需读），别滥建子 skill。

## 怎么检索现状（命令可直接跑）

```bash
# 看技能索引（自动生成的目录）
cat .agents/skills/README.md

# 列全部 SKILL.md
ls .agents/skills/*/SKILL.md

# 逐个看 frontmatter（name/description/version/last_reviewed）
for f in .agents/skills/*/SKILL.md; do echo "== $f =="; \
  awk 'NR==1&&$0=="---"{i=1;next} i&&$0=="---"{exit} i' "$f"; done

# 机器检查：索引有没有漂移（加了 skill 忘登记 / 手改了 README）
bash scripts/skills-index.sh --check

# 重新生成索引（注意：是脚本，没有 make skills-index 目标）
bash scripts/skills-index.sh

# 全量控制面自检（内含 skills-index --check，见 verify-control-plane.sh:29）
make verify
```

真实路径锚点：索引脚本 `scripts/skills-index.sh`；`--check` 被 `scripts/verify-control-plane.sh` 调用进 `make verify`；**当前 skill 清单以自动生成的 `.agents/skills/README.md` 为准——别在本手册里硬编码枚举（会漂，本仓已踩过）**。注：`self-optimize` 是子 agent（`.claude/agents/`），不是 skill。

## 怎么判（逐条可判定）

- **符合**：常见流程都有 skill；`bash scripts/skills-index.sh --check` 输出"✓ skills 目录无漂移"；每个 description 有"何时用/何时不用"且彼此不抢同类任务；改了架构的 ADR 在「受影响的 skill」栏逐条交代了相关 skill。
- **缺口**：某类反复做的任务（如某种迁移 / 重构 / 评审）没有对应 skill，只靠口头或 lessons；或 description 漏掉了该触发的场景（漏触发）。
- **漏洞**：
  - **索引漂移**：新增/改了 `SKILL.md` 但没重生成 README → `--check` 非零；或有人手改了 README（脚本生成物，禁手改）。
  - **误/漏触发**：两个 skill description 边界含糊、抢同一类任务（误触发）；或措辞太窄/没写触发场景，该用时不被选（漏触发）。
  - **arch 变 skill 没跟（rule-0007）**：架构改了但相关 `SKILL.md` 没改、ADR「受影响的 skill」栏空着。
  - **过期**：`version` / `last_reviewed` 与正文实际状态脱节（流程已变但没 bump）。
  - **脏锚点**：`SKILL.md` 引用的脚本 / 文件路径已不存在（如「演进」段指向的脚本被删/改名）。

## 常见漏洞模式（本仓真实案例）

- **arch 变 skill 没在 ADR 记 = 判失败**（`tasks/lessons.md` 2026-06-26「rule-0007 改了 skill 却没在 ADR 记录」；eval 评审 `docs/eval/task-reviews/20260626T014408Z-harness-rules-distribution/decision.md` §4）：规则分布化大改时 `add-rule` 实际已更新（version 2、`last_reviewed: 2026-06-26`），但 ADR-0004 漏掉模板强制的「受影响的 skill（rule-0007）」栏，且最相关的 `context-loading` 既没回顾也没声明"无需更新" → eval-011 直接判 blocker fail。**教训：做了 ≠ 记了**；改 skill 必须在 ADR 该栏逐条写（改了的写改了、不需改的写"无需更新 + 理由"），ADR 用 `templates/adr.md` 起草别手搓省栏。这正是本维度 plan 里点名的"本仓 add-rule 差点漏"。
- **索引头与真实命令漂移**（现存）：`.agents/skills/README.md` 头部写"由 `make skills-index` 自动生成"，但 `Makefile` 里**没有 `skills-index` 目标**；真实命令是 `bash scripts/skills-index.sh`，`--check` 经 `make verify` 跑。文案指向不存在的入口，是个该顺手修的小漂移。

## 修复用哪个操作 skill / 脚本

- **加/改一个 skill**：写或改 `.agents/skills/<name>/SKILL.md`（frontmatter 四件套齐全；正文写"何时用/何时不用"+「演进」段）。
- **改完重生成索引**：`bash scripts/skills-index.sh`，再 `bash scripts/skills-index.sh --check` 确认无漂移。
- **arch 变了履行 rule-0007**：改相关 `SKILL.md` + 在 ADR 的「受影响的 skill」栏逐条交代（用 `templates/adr.md`）；同时 bump `version` / `last_reviewed`。
- **缺口是"该立规则而非 skill"**：用 `add-rule` skill。
- **收口**：`make verify`（含 skills-index `--check`）绿才算落地。
