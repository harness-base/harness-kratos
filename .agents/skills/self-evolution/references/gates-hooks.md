# gates-hooks（护栏 / 门禁 / 触发）审查手册

> 审「声称在守的检查到底守不守得住」。红线：**无 mutation 自证的检查＝花架子；能绕＝没有；只做了 Claude Code 不算对等。**

## 规范（健康长什么样 / 不变量）

- **load-bearing 自证**：每个声称的检查都能被「把被测逻辑改坏 → 检查变红 → 还原 → 变绿」证明它真在守。验证脚本本身要带测试（`*.test.sh`）。
- **不可绕**：机判的检查必须进 `make verify`（被 pre-push + CI 跑），不靠 agent 自觉调用。绕过路径（`--no-verify`）只在确属误报时人工用，不是默认。
- **机判进脚本、人判进 eval**：能确定性判定的（结构齐不齐、索引漂没漂、密钥在不在）做成脚本进 verify；需要质量判断的（语义无损、断言是否锚真实信号）走 `docs/eval/` 评委。别把人判塞进脚本（假精确），也别把机判推给 eval（空耗）。
- **hook 分层清楚**：软 hook = agent 侧（`.claude/settings.json` 的 PreToolUse/Stop，只 Claude Code 吃）；硬 hook = git/CI 侧（`.githooks/`、`.github/workflows/`，任何客户端都吃）。**红线检查必须落硬层**，软层只是早提醒。
- **codex ⇄ claude 对等**：硬层（就近 `AGENTS.md`、`.githooks/`、CI）Codex 也吃，是对等的根基。`.claude/settings.json` 里的 hook 只有 Claude Code 吃——**任何只挂在 settings.json 的红线，Codex 侧是裸奔的**，必须在硬层有等价物兜底。
- **触发频率合理**：周期性触发按「自上次基线以来的增量」而非绝对快照（防大改未提交时每轮空转）；纯讨论轮不烧 LLM；best-effort 的兜底任何失败都 `exit 0`，绝不阻断收尾。
- **文档同步机制自洽（doc-sync ⇄ turn-backstop ⇄ 资产类型）**：漂移处理是「预防层 `doc-sync` skill 的 checklist + 检测层 `turn-backstop.sh` Haiku prompt 的 (f) 项 + 仓内**实际资产类型**」三者咬合。新增一类资产（新目录 / 新索引 / 新 hook）时，doc-sync checklist 要有对应行、turn-backstop (f) 的典型例子要跟上——否则该类漂移没人预防也没人检测。这条横跨 skills 维度与本维度，别让它落进"谁都不管"的灰带（见 ADR-0006：漂移处理并入自进化闭环、不另建 drift/ 区）。

## 怎么检索现状（能直接跑）

```bash
ROOT="$(git rev-parse --show-toplevel)"

# 1) 总门禁 + 它真跑了哪些子检查
cat  $ROOT/scripts/verify-control-plane.sh        # 列出所有进 verify 的子检查
make -C $ROOT verify                              # 亲跑，看真实 exit 0（别只读不跑）

# 2) 软 hook（仅 Claude Code 吃）
cat  $ROOT/.claude/settings.json                  # PreToolUse(Bash→hook-policy) / Stop(stop-check)

# 3) 硬 hook（git，任何客户端吃，含 Codex）
cat  $ROOT/.githooks/pre-commit   $ROOT/.githooks/pre-push
cat  $ROOT/scripts/install-hooks.sh               # core.hooksPath -> .githooks（靠 make hooks 装）

# 4) CI（远端兜底）
cat  $ROOT/.github/workflows/verify.yml           # push/PR 跑 make verify

# 5) policy 检查 + 它的自测（mutation 入口）
cat  $ROOT/scripts/hook-policy.sh                 # 密钥/危险命令扫描
bash $ROOT/scripts/hook-policy.test.sh            # 应拦的拦/应放的放（pass=6 fail=0）

# 6) 收尾闸门 + 自进化兜底 + 兜底自测
cat  $ROOT/scripts/stop-check.sh                  # L2+ 强制 eval 产出，否则 exit 2 拦收尾
cat  $ROOT/scripts/turn-backstop.sh               # 机械触发(K轮/commit/增量)→headless Haiku
bash $ROOT/scripts/turn-backstop.test.sh          # 递归guard/不触发静默/永不阻断

# 7) 各 *-audit / *-index --check（都进了 verify）
ls   $ROOT/scripts/*-audit.sh $ROOT/scripts/*-index.sh
grep -n 'mutation\|变异\|check_eval_pointers' $ROOT/scripts/rules-index.sh   # 固化的自证

# 8) 文档是否同步
cat  $ROOT/docs/harness/HOOKS.md                  # 必须与上面脚本实际行为一致
```

## 怎么判（逐条可判定）

- **是不是 load-bearing**：找它的 `*.test.sh` 或 `--check` 自测。没有自测，或自测不含「故意改坏→变红」的反例 → 判**花架子**。`hook-policy.test.sh` 有 `expect_block`/`expect_ok`；`rules-index.sh` 有 `check_eval_pointers` 注释里记了变异自证（注入 `eval:999`→FAIL→还原 PASS）——这是合格样板。
- **能不能绕**：该检查进 `verify-control-plane.sh` 没有？没进＝只能靠 agent 想起来跑＝可绕。进了，pre-push（`.githooks/pre-push` → `make verify`）和 CI（`verify.yml`）才会强制它。
- **codex 等价缺不缺**：逐条问「这个 hook 挂在哪层」。挂 `.claude/settings.json`（PreToolUse hook-policy、Stop stop-check）的 → **Codex 侧没有等价软 hook 机制**，只能靠 `.githooks/pre-commit`（也调 hook-policy）+ CI 兜底。判据：该红线在 `.githooks/`/CI 有没有等价拦截？没有＝Codex 裸奔，缺口。
- **该机器化的是否靠自觉**：能确定性判的检查若散在 `AGENTS.md` 当口头规则、没进脚本 → 判缺口（如「eval 标记必须指向存在考题」曾只是约定，后来才固化进 `rules-index.sh --check`）。
- **软当硬用**：红线只挂软层（settings.json Stop）而硬层无等价 → 判 severity 错配。`stop-check.sh` 自己诚实标了局限：档位由 agent 在 todo.md 自报，**故意低报档位仍能绕**——这类「半强制」必须在 HOOKS.md 写明，不能假装是 100% 硬门禁。
- **频率空转/漏**：周期触发看的是快照还是增量？只看快照（绝对阈值）→ 大改未提交时每轮误触发（空转）。`turn-backstop.sh` 用「增量 ≥ 阈值」+ 基线文件（`.turn-count`/`.last-backstop`）比差值——这是对的。
- **doc-sync/turn-backstop 漏新资产**：新增的资产类型在 `doc-sync` checklist 找不到对应行，或 `turn-backstop.sh` prompt (f) 的典型例子没覆盖 → 该类文档漂移没人预防也没人检测，缺口。检索：`grep -n '回查' .agents/skills/doc-sync/SKILL.md`（看 checklist 覆盖哪些资产）对照 `find . -name index.yaml` / 各 `README.md` 实际资产类型。
- **文档对得上吗**：`HOOKS.md` 描述的触发常量/拦截项是否与脚本实际一致（source_files 列了 4 个脚本，`docs-audit` 会查它们存在但不查行为一致，行为一致靠人审）。

## 常见漏洞模式（本仓真实案例）

- **检查不 load-bearing，把垃圾当结果落库**（`tasks/lessons.md` 2026-06-26「兜底把超长 transcript 喂给 Haiku」）：turn-backstop 把 Haiku 返回的 `Prompt is too long` 当成「发现」追加进 `optimization-log.md`——检查在「跑」但产出是噪声。修法：输出**只认预期格式**（findings 必须 `^\[类别\]`），报错/NONE/空一律不记（见 `turn-backstop.sh:73`）。
- **测试不绑真实链 = 空转测试**（`tasks/lessons.md` 2026-06-26「14 轮对抗评审」补记）：R9 的 metrics 测试不绑真实链、R10 的 DLQ 测试喂合成不可达输入——「跑过」但没测到真东西。修法：补测试一律 **mutation 自证**（改坏被测逻辑确认变红），并问「喂的输入生产路径真会产生吗」。
- **声称无损实则偷改，机判没固化**（`docs/eval/task-reviews/20260626T014408Z-harness-rules-distribution/decision.md`）：ADR 称「eval 映射全保留」，实际给 rule-0005/0006/0008 编了不存在的 eval 指针（005/006/008），独立 eval 抓出。**根因＝该机判没进脚本**。修法：把「eval 标记必须指向存在的 prompt 文件」固化进 `rules-index.sh` 的 `check_eval_pointers`，进 `--check`/verify，并变异自证。
- **宣称全绿却没跑门禁命令本身**（`docs/eval/task-reviews/20260602T105017Z-kratos-base-s0`，对照 `tasks/lessons.md` 2026-06-02「宣称全绿但没跑门禁命令」）：自核只跑 go build/test 子集就说「全绿」，没跑 `make verify`（golangci-lint 是红的）。教训：门禁是「项目登记的那条命令」，不是自选子集——审查时**亲跑 `make verify` 看 exit 0**，别采信声称。
- **soft hook 的「半强制」局限**（`docs/harness/HOOKS.md:46`、`scripts/stop-check.sh:21`）：Stop hook 的 L2+ 强制 eval 靠 agent 在 todo.md 自报档位，故意低报可绕——本仓已诚实标注，审查时确认这类局限**写明了**、没被吹成硬门禁即可。

## 修复用哪个操作 skill / 脚本

- **加 mutation 自证**：照 `scripts/hook-policy.test.sh`（expect_block/ok）或 `scripts/turn-backstop.test.sh`（hermetic、不调外部）模板写 `<脚本>.test.sh`；把「故意改坏→变红」记进脚本注释（见 `rules-index.sh:48` 的 `check_eval_pointers`）。
- **进 verify**：把新检查加进 `scripts/verify-control-plane.sh`（一行 `bash scripts/<x>.sh || fail=1`）——自动被 `.githooks/pre-push` 与 `.github/workflows/verify.yml` 强制。
- **补 codex 等价**：红线落硬层——就近 `AGENTS.md`（Codex 原生按目录读）、`.githooks/`（`make hooks` 装）、CI。settings.json 的软 hook 只是早提醒，不是对等机制；**Codex 专属钩子机制目前缺，仅 git/CI 层对等**，审查发现的缺口照此兜底。
- **改/加规则当红线**：用 `add-rule` skill（定范围→写进就近 AGENTS.md 带 `<!-- rule: -->` 标记 + 登记→挂执行），保证「写完有人理」。
- **同步文档**：改了任何 hook/触发逻辑，更新 `docs/harness/HOOKS.md`（含触发常量、局限说明）；其 frontmatter `source_files` 列的脚本由 `scripts/docs-audit.sh` 校验存在。
- **装/重置 git hooks**：`make hooks`（`scripts/install-hooks.sh` → `core.hooksPath .githooks`）。
