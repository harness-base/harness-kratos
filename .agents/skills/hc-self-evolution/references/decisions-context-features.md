# decisions / context / features 审查手册

> 规范检查层用。审「决策记录（ADR）/ 当前状态（context）/ 需求账本（features）」三区是否真实、一致、不漂移。
> 红线口吻，逐条可判。给的命令都能从 harness 根直接跑。

## 规范（健康长什么样 / 不变量）

- **每区有索引，索引即真相骨架**：`docs/decisions/index.yaml`、`docs/features/index.yaml` 登记每条目的 id/title/file（features 还有 delivery_status/implementation_allowed）。
- **index ⇄ 目录双向一致**：index 登记的 `file` 都在目录里存在；目录里的每个 `NNNN-*.md` 都被 index 登记。无孤儿、无幽灵。
- **大改必有 ADR，且被按号引用**：架构/接口/机制级变更要有 `docs/decisions/NNNN-*.md`；ADR 编号 `ADR-NNNN` 是稳定引用键，被 feature / plan / skill / eval review 按号引用，不复制正文。
- **ADR 用模板、栏不缺**：照 `templates/adr.md`，背景/决策/**受影响的 skill（rule-0007）**/备选/影响齐全。「受影响 skill」栏不许空（大改尤其）。
- **context 反映真实状态、不漂移**：`docs/context/CURRENT_STATUS.md` 的 frontmatter `last_updated` 与正文（阶段、规则条数、ADR 条数、各区状态）必须跟代码/index 当前事实对齐。`docs/context/README.md` 是 `dir-index.sh` 自动生成、禁手改。
- **features 是 rule-0001 的账本**：动业务代码前在 `index.yaml` 登记需求包，状态字段（draft→tests_ready→…→done）反映真实交付阶段，不提前置 done。

## 怎么检索现状（能直接跑）

```bash
cd "$(git rev-parse --show-toplevel)"

# 三区索引与目录
cat docs/decisions/index.yaml
cat docs/features/index.yaml
ls docs/decisions/*.md docs/features/*.md

# context 真实状态 + 自动索引
cat docs/context/CURRENT_STATUS.md
cat docs/context/README.md          # dir-index.sh 生成，禁手改

# index ⇄ 目录一致性（手核——见下「注意」）
diff <(grep -E '^\s*file:' docs/decisions/index.yaml | sed -E 's/.*file:\s*//' | sort) \
     <(ls docs/decisions/*.md | xargs -n1 basename | grep -vE 'index|README' | sort)
diff <(grep -E '^\s*file:' docs/features/index.yaml | sed -E 's/.*file:\s*//' | sort) \
     <(ls docs/features/*.md | xargs -n1 basename | grep -vE 'README' | sort)

# ADR 是否被按号引用（每条 ADR 至少应在引用方出现一次）
git grep -oE 'ADR-[0-9]{4}' | grep -v '^docs/decisions/' | sort | uniq -c

# context README 漂移 / docs frontmatter 引用通不通
bash scripts/dir-index.sh docs/context --check
bash scripts/docs-audit.sh
```

**注意（已核实的机器检查边界）**：
- `make verify` / `verify-control-plane.sh` 只校验 `docs/decisions/index.yaml`、`docs/features/index.yaml` **文件存在**，**不**校验 index ⇄ 目录一致；`dir-index.sh --check` 只覆盖 `docs/context|docs/harness|templates|.claude/agents` 的 **README 漂移**，**不**覆盖 `CURRENT_STATUS.md` 内容真实性。
- 现成的「index ⇄ 目录双向一致」机器检查**只有 PRD 区有**（`scripts/prds-audit.sh`，正反向都查）——decisions / features **目前靠手核**（上面两条 `diff`）。`docs-audit.sh` 只查 frontmatter 的 `source_files/related_docs` 路径在不在，不查 index 登记。

## 怎么判（逐条可判定）

- **index 漏登记 / 幽灵条目**：上面 `diff` 非空即漏。目录有文件 index 没登 = 漏登记；index 登了目录没文件 = 幽灵。两者都 fail。
- **大改没落 ADR**：本轮改了架构/接口/加载机制/脚本骨架，但 `docs/decisions/` 没新增 ADR、index 没登记 → fail（blocker 级，触 rule-0007 精神）。
- **ADR 没被引用 / 引用悬挂**：`git grep ADR-NNNN`（排除自身目录）为空 = 写了没人按号引用，形同孤本；引用指向不存在的 ADR 号 = 悬挂。
- **ADR 缺「受影响 skill」栏**：`grep -niE '受影响|skill' docs/decisions/NNNN-*.md` 命中不到那一栏（或栏在但空着）→ fail。这是 eval-011 的直接判失败口径。
- **context 漂移**：`CURRENT_STATUS.md` 正文写的规则条数 / ADR 条数 / 各区状态 ≠ 当前事实，或 `last_updated` 早于正文已描述的最新阶段 → fail。对照命令：
  `grep -cE '^\s*- id: ADR' docs/decisions/index.yaml`（ADR 数）、`grep -cE 'rule: rule-' AGENTS.md`（规则数）、`grep -c 'delivery_status: done' docs/features/index.yaml`（已完成需求数）。
- **features 状态虚高**：`delivery_status: done` / `implementation_allowed: true` 但无真实验证证据（eval review / e2e）→ fail（撞 rule-0002/0003）。
- **「全保留 / 完全一致」无 diff 证据**：ADR/总结出现绝对措辞却不贴逐条 `git show HEAD:<file>` 对比 → 高度可疑，按偷改对待，逐条核。

## 常见漏洞模式（本仓真实案例）

- **context 漂移（活样本，写本手册时实测）**：`docs/context/CURRENT_STATUS.md` `last_updated: 2026-06-11`，但正文已描述 S6（2026-06-23 落地）；控制面表仍写「8 条规则（rule-0001~0008）」「ADR-0001 奠基设计」「`docs/features/` skeleton 空账本」，而实际是 **11 条规则、5 条 ADR、6 个 features 全 done**。典型「内容往前跑、状态页没跟」。
- **大改没把 ADR 栏填全 = 判失败**：`docs/eval/task-reviews/20260626T014408Z-harness-rules-distribution/`（verdict yellow）——ADR-0004 是大改（重写 AGENTS.md / 删全部规则文件 / 换加载机制 / 新建 scanner），却漏掉 `templates/adr.md` 强制的「受影响的 skill（rule-0007）」栏，`context-loading` 未回顾未声明 → eval-011 直接 **blocker fail**（「做了没记」=没履行）。
- **凭记忆迁移、没对源核（catalog/索引指针造假）**：同一评审 + `tasks/lessons.md`「2026-06-26：声称『保留/不变』凭记忆没对源核」——ADR 写「severity / eval 映射全保留」，实际偷偷把 rule-0007 severity warn→blocker、给 rule-0005/0006/0008 编了不存在的 eval 指针（005/006/008）。教训：凡声称「X 保留/不变」必须 `git show HEAD:<file>` 机械核对再写。

## 修复用哪个操作 skill / 脚本

- **补 / 改 ADR**：照 `templates/adr.md` 起草（别手搓省「受影响 skill」栏），写 `docs/decisions/NNNN-*.md`，登记进 `docs/decisions/index.yaml`。大改连带回顾 `.agents/skills/` 并在该栏逐条写（已改写已改 / 不需改写「无需更新+理由」）——rule-0007。
- **补登需求包**：`/dev` skill 深度级（`.agents/skills/hc-dev/`），照 `templates/feature-package.md` 建包、登记 `docs/features/index.yaml`、推进 delivery_status。改业务代码前必须就绪（rule-0001）。
- **修 context 漂移**：直接改 `docs/context/CURRENT_STATUS.md`（含 `last_updated` + 正文事实），然后 `bash scripts/dir-index.sh docs/context` 重生成 README、`bash scripts/docs-audit.sh` 复核引用。
- **固化 / 加红线**：要把「index⇄目录一致」「context 不漂移」变成机器门禁，用 `/hc-add-rule` skill；可仿 `scripts/prds-audit.sh` 给 decisions/features 写双向一致校验并挂进 `make verify`。
- **自检收尾**：`make verify`（结构 + docs-audit + 索引漂移）、`make docs-audit`（frontmatter 引用通不通）；判断与归档走 `hc-self-evolution` skill（复杂时 spawn `hc-self-optimize` 子 agent）。
