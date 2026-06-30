# doc-sync 重构 — 完整方案

> 与用户逐条敲定的最终版。关联项/缺口由 self-evolution 维度审查得出（run wktbhq3t5）。

## 一、目标

我改了文件、忘了改关联的文档时，在我**收工前**被自动查出来、反馈给我、逼我改掉，并留记录 + 处理状态。

## 二、做完后，运行时怎么工作

1. 这一轮我改了几个文件（例：改了 `scripts/foo.sh`）。
2. 我准备结束这一轮 → Claude Code 先跑 Stop 钩子。
3. 钩子看触发条件（够 8 轮 / 这轮有过 commit / 改了 ≥10 文件 —— 三者任一才跑，不是每轮）。
4. 触发了 → 启动 `doc-sync-reviewer`，它读【我这轮的 `git diff`】+【对照表】→ 比出"`scripts/foo.sh` 改了、`scripts/README.md` 没动 = 漂移"。
5. 有漂移 → 钩子①把它**写进 log（标"待处理"）** ②**反馈给我** ③ 等我处理。
6. 我要么改掉 `scripts/README.md`，要么写"暂缓，因为 X" → 状态标"已处理/暂缓"。
7. 没漂移 / 全处理完 → 放我收工。

## 三、设计决策（含明确不做的）

- **删** `doc-sync` skill；它那张对照表**降为数据文件** `docs/harness/doc-sync-checklist.md`。
- **新建** `doc-sync-reviewer` 子 agent（haiku、双栈）做第 4 步的检查，只读不改。
- 钩子发现漂移 = **写 log（带状态）+ 反馈给我 + 标处理状态**（不是丢日志、也不是只写日志没人看）。
- 反馈走"能真送达我"的通道（`exit 2` 拦收工 / 或下一轮注入）—— **T3 先实测定**。
- **不做**：① 不动 `dev`/`git-workflow`（它俩要单独重构；git 时查漂移该是 git hook、不属 workflow，且 commit 已被钩子触发覆盖）；② 不立 AGENTS.md 通用规矩（太重 / 与本机制重复）。

## 四、实现任务

**T1 — 删 skill + 表降数据文件**
- 表搬到 `docs/harness/doc-sync-checklist.md`（保留 🔴手/✅机检 标记；frontmatter 引用不悬空）。
- 删 `.agents/skills/doc-sync/`。
- 跑 `skills-index.sh` + `dir-index.sh docs/harness` 重生成索引。

**T2 — `doc-sync-reviewer` 子 agent（双栈，haiku）**
- `.claude/agents/hc-doc-sync-reviewer.md`：`tools: Read, Grep, Bash`；读 diff + 清单 🔴手 行 → 报漂移、不改。
- `.codex/agents/hc-doc-sync-reviewer.toml` + `.codex/config.toml` 注册；跑 `dir-index.sh .claude/agents`。

**T3 — 改钩子（log + 反馈 + 状态）**
- 先实测反馈通道（`exit 2` vs 下一轮注入），选能真送达的，写进 ADR。
- 钩子 doc-drift 那条：漂移 → 写 log（带"待处理/已处理/暂缓"状态）+ 反馈我 + 清单源指新文件；保住安全不变量（递归/超时/预算/不卡死）。
- 改 `turn-backstop.test.sh`（**与 T1 删 skill 同一 commit**）：断言新接通（指新清单 + 派 reviewer + reviewer 注册齐），静态接线测试、不真跑，mutation 自证。

**T4 — rewire 关联项（self-evolution 枚举）**
- 改：根 `README.md:25`、`docs/harness/HOOKS.md:54`、`docs/context/CURRENT_STATUS.md:37`、`docs/README.md` harness 行、**本 spec/plan 自身 frontmatter**；self-evolution 6 处 reference（`gates-hooks.md:13,59` / `docs.md:74` / `subagents.md:35` / `lessons-memory.md` / `skills.md:7` / `decisions-context-features.md`）。
- 不动：ADR / eval / lessons（历史）、`stop-check.test.sh`。
- 收尾 `git grep doc-sync` 复扫，残留全为历史 / 新清单自身。

**T5 — ADR-0012（决策留档，与 verify 无关）**
- `docs/decisions/0012-doc-sync-redesign.md`：受影响 skill 栏 = `doc-sync(是/删)` + `self-evolution(是)` + 其余否；related_docs 含 0005/0006/0011，正文衔接 0006；写 haiku 依据 + doc-drift 自动检测 Claude-only。
- 在 `docs/decisions/index.yaml` 登记。

**T6 — dogfood + 验证 + eval + 提交**
- dogfood：throwaway 改个脚本不改它 README → 验 reviewer 真报出 + 反馈真到我；跑完删。
- 独立 agent 对抗复扫漏网；`make verify` + `docs-audit` 全绿。
- 收尾 eval（L3，考题 011/014/010）；补 todo Review；commit（单独 PR）。

## 五、会让 `make verify` 红的地方（与 ADR 是两条线）

| 动作 | 哪个检查红 | 修法 | 谁修 |
|---|---|---|---|
| 删 `.agents/skills/doc-sync/` | `skills-index --check` | 跑 `skills-index.sh` | ✅脚本自动重写 |
| 新建 `docs/harness/doc-sync-checklist.md` | `dir-index docs/harness --check` | 跑 `dir-index.sh docs/harness` | ✅脚本自动重写 |
| 新建 `.claude/agents/hc-doc-sync-reviewer.md` | `dir-index .claude/agents --check` | 跑 `dir-index.sh .claude/agents` | ✅脚本自动重写 |
| 删 `doc-sync/SKILL.md`（test 引用它） | `turn-backstop.test.sh` | 改 test | ✋我手改 |
| 删 `doc-sync/SKILL.md`（spec frontmatter 引用它） | `docs-audit` | 改 spec frontmatter | ✋我手改 |
| 新建 `ADR-0012` 文件 | `index-audit`（索引↔文件不符） | `index.yaml` 登记 | ✋我手改 |

> ADR 与 verify 的唯一交集 = 末行（ADR 文件要登记进索引，机械动作）。ADR 的**内容**给收尾 `eval` 看，与 verify 红不红无关。

## 六、押后 backlog（不在本方案）

self-optimize(②) 闭环、`optimization-log` 全局 drain、① capture 通道实测 0 产出根因、`.codex/agents/` 无索引、`lessons.md` 46 条 `seen` 从没裁决。
