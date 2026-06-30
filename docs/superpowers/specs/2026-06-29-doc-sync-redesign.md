---
title: doc-sync 重构设计稿（v2，经 self-evolution 维度审查补全）
status: draft
owner: harness
last_updated: 2026-06-29
source_files: []
related_docs:
  - ../../decisions/0005-self-evolution-loop.md
  - ../../decisions/0006-drop-drift-area.md
  - ../../decisions/0011-demote-context-loading.md
  - ../../../scripts/turn-backstop.sh
---

# doc-sync 重构设计稿（v2）

> v1 是我 freehand 写的，漏了一片关联项与缺口。v2 把 `self-evolution` 维度审查（8 维度，run wktbhq3t5）查出的 9 blocker + 一批 major 全补进来。

## 要解决什么

`doc-sync` skill 同时是**两样东西**：① 一张"改了 A → 去查 B"的对照清单（数据）；② 一个"改完主动提醒我去查"的入口（预防层）。问题：作为 skill 它几乎不被主动 invoke（advisory 死穴）；而钩子兜底（turn-backstop）发现漂移只写 `optimization-log` → 烂着没人修（实证：我踩的 README 漂移从没被捞到）。

## 设计（含 v2 修正的决策）

把它拆成各归各位，并**补回预防层 + 闭环送达**：

1. **删 `.agents/skills/doc-sync/` skill。**
2. **清单降数据**：搬到 `docs/harness/doc-sync-checklist.md`（唯一判据源）。它在 `dir-index docs/harness` 循环里 → **搬完必 regen `docs/harness/README.md`**，否则 verify 红（blocker）。frontmatter 的 `source_files/related_docs` 必须真实存在，否则 docs-audit 红。
3. **新建 `doc-sync-reviewer` 子 agent**（双栈）：读本轮 diff + 上面清单 → 报关联项漂移、只评不改。`tools: Read, Grep, Bash`（不要 Glob）。model 取 haiku（**依据写进 ADR**：doc-drift 是轻量对照判断、且是 hook-headless 廉价兜底，非会话子 agent；或由派发方 `--model` 指定、不硬钉 frontmatter）。
4. **turn-backstop 的 doc-drift 那一项**：改为用 doc-sync-reviewer 的 prompt（**先验**能否从 hook headless 派命名子 agent；不行就内联 reviewer 的 prompt 正文，像现在内联 a~f），清单源指新路径。
5. **闭环送达（命门）**：发现漂移要**真到达主 agent**。现 turn-backstop `exit 0 + stderr` 不注入上下文（HOOKS.md 自陈"没人看见"）→ 必须改走**真能送达的通道**：`exit 2` 软拦收尾（可 defer）或仿 correction-nudge 走 UserPromptSubmit 下一轮注入。**实现前先定死走哪条，并加守护测试证"反馈真进了 agent 上下文"。**
6. **预防层补回**（v2 新增，治"agent 没意识"）：在 `dev` 收尾段、`git-workflow` commit 前各挂一句"改完比对 `docs/harness/doc-sync-checklist`"——把"改完主动查"这个被删掉的入口补回常驻流程，不是无声删掉。
7. **立通用关联项规则**（v2 新增，治根）：走 `add-rule` 立一条 warn 规则"编辑任意产物 / 收尾前必比对关联项（索引 / README / 镜像 / 文档）"，doc-sync-reviewer 作为它的机检执行体之一。**这条比 doc-sync 大，是反复栽（3 天 ≥6 条同型 lesson）的真正治本。**

**保留不动**：`make verify` 机检那半（index/shim/audit）；turn-backstop 的 a~e 捕获（仍 Haiku→log）。

## 全部关联项（self-evolution 维度审查枚举，非手记）

**必改（删 skill / 移 checklist 后失准，漏了 verify 会红或闭环断）：**
- `scripts/turn-backstop.sh` + `scripts/turn-backstop.test.sh`：派 reviewer + 新清单路径 + 反馈通道；test #4 重写为新"接通闸"（断言引用新清单 + 派 reviewer + reviewer 双栈注册齐，任一断→红），**hermetic 静态接线测试、不真 spawn**；**必与删 skill 同 commit**。
- `docs/harness/doc-sync-checklist.md`（新）+ `dir-index docs/harness` regen。
- `.claude/agents/hc-doc-sync-reviewer.md` + `.codex/agents/hc-doc-sync-reviewer.toml` + `.codex/config.toml` 注册 + `dir-index .claude/agents` regen。
- 根 `README.md:25`：技能集例子删 `doc-sync`。
- `docs/harness/HOOKS.md:54`：机制描述改"派 reviewer + 反馈口语义 + Codex 局限"。
- `docs/context/CURRENT_STATUS.md:37`：① 机制描述（doc-drift 改走 reviewer + 当轮反馈，a~e 仍 log）+ `dir-index docs/context` regen。
- `docs/README.md` harness 行：补"文档同步对照清单"（minor）。
- **本设计稿 / plan 自身 frontmatter**：v1 的 `related_docs` 指 `doc-sync/SKILL.md` → 删 skill 后悬空 → 已在 v2 改指 checklist（自伤悬空，blocker）。
- **self-evolution 自己的 references**（删 skill 连带，meta④）：
  - `gates-hooks.md:13,59`：重写"文档同步自洽"不变量为新拓扑（数据文件=唯一判据源、reviewer 检测、turn-backstop 反馈），删 :59 失效的 `grep .../doc-sync/SKILL.md` 命令改指新路径，补"Codex 无自动触发"判据。
  - `docs.md:74`："用 doc-sync skill…主动版" → reviewer，并反映"预防层入口去向"。
  - `subagents.md:35`：agent 计数"3 个"已过期（实 9→10）→ 指针化"以 README 为准"；补"hook 可 headless 派命名子 agent 照样算硬触发"。
  - `lessons-memory.md:27/44/51`："optimization-log 0 条 / 闭环没跑过"已过期 → 改为现状；补 `opt: seen/skip/rule` marker 检索。
  - `skills.md:7`：补一句"被 reviewer 子 agent + 数据文件承接的能力不必硬做 skill"（援引 ADR-0011/0012 判据）。
  - `decisions-context-features.md` 漏洞模式：加样本"删 A skill 但改了 B skill 的 refs，受影响栏须标 B=是"。
- `docs/decisions/0012-doc-sync-redesign.md`（新）+ 登记 index：**受影响 skill 栏 = doc-sync(是/删) + self-evolution(是，列出改的 refs) + 其余否**；related_docs 含 0005/0006/0011；正文衔接 ADR-0006（它确立 doc-sync 为预防层，本 ADR 删之、职责转 reviewer+数据清单）；写明 model 取 haiku 依据；写明 doc-drift 自动检测 = Claude-only、Codex 靠 make verify + 根 AGENTS.md 规则。

**历史不改写（保留）：** ADR-0006~0011 正文、`docs/eval/task-reviews/*`、prd-orchestration plan、`tasks/lessons.md`。
**非引用 / 不动：** `stop-check.test.sh`（reviewer/review 整词消歧已加，保持绿）；`AGENTS.md`/`CURRENT_STATUS`(skills 行)/`docs/README` 经 grep 不含 doc-sync skill 名。

## 不做 / 押后（显式 backlog）

- self-optimize(②) 闭环、`optimization-log` 全局 drain。
- **① capture 通道实测 0 产出**（turn-backstop a~e 从没写进过一条）——根因待查，本次不动，立 backlog。
- `.codex/agents/` 无索引、无 --check（覆盖盲区，另立项）。
- lessons 46 条 `opt: seen` 从没裁决过升/skip（晋升流程空转）——收尾顺手裁一批。

## 风险 / 验证

- **闭环命门**（设计 5）：反馈通道不验清，整个改白做 → 实现前先定通道 + 守护测试证送达。
- **hook 派子 agent 未验**（设计 4）→ 先验 `claude -p --agent`，不行走内联 prompt。
- **关联项漂移**：靠上面 self-evolution 枚举 + 收尾再 `git grep doc-sync` 复扫 + 独立 agent 对抗复扫。
- **eval-011 老坑**：ADR-0012 受影响栏含 self-evolution=是（漏标=fail）。
- **dogfood**：造 throwaway 漂移（改脚本不改其 README）→ reviewer 真报出 + 反馈真到主 agent；跑完删。
- 收尾 eval（考题 011/014/010）+ `make verify` + `docs-audit` 全绿。
