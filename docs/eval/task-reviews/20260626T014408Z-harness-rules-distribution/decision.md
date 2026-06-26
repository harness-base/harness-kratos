---
title: decision — harness 规则系统分布化
task: harness-rules-distribution
verdict: yellow
prompts: ["010", "011"]
reviewed_at: 2026-06-26
---

# 逐条核查结论

总评先行：**机制层（scanner / catalog / shim / 引用保号）扎实且可复核，诚实性达标（verify/docs-audit 真绿）**；但有 **2 处"声称无损实则有改"的漂移**（rule-0007 severity 偷偷 warn→blocker；rule-0005/0006/0008 的 eval 映射被改错）和 **1 处 rule-0007 自身未履行**（ADR-0004 缺模板强制的"受影响 skill"栏）。无数据丢失、无不可逆破坏，故 **yellow（有条件收尾，先补 3 处）**，不到 red。

---

## 1. 语义无损（牙齿没丢）—— 大体 PASS，2 处偷改 FAIL

```yaml
prompt: "010"
sub: 语义无损
verdict: fail
severity: warn
reason: 规则正文牙齿基本保留，但 ADR 宣称"severity 全保留 / eval 映射全保留"为假——rule-0007 被偷偷从 warn 升 blocker；rule-0005/0006/0008 的 eval 标记被改错。
evidence: 见下逐条
```

**正文牙齿（保留，PASS）**：
- **rule-0001 MUST STOP**：bullet（`AGENTS.md:20`）保留 `未就绪 MUST STOP`。✓ 牙齿在。
  - 轻微稀释：原文（`HEAD:docs/rules/0001-*.md`）的**例外条款**"纯控制面文档/脚本/无用户可见行为的修补不触发"在 bullet 中**消失**；"不论大小""UI 体验/业务流程"枚举也省了。方向是"可能过触发"而非"放水"，warn 级提示，非 blocker。
- **rule-0009（共因/竞态/守护测试）**：bullet（`AGENTS.md:28`）显式点名"防共因污染 / 防超时竞态掩盖 / 守护测试 / 注释不许撒谎 / 唯一真实产出方证据"——三类牙齿（A 共因、B 竞态、C 守护测试）全在；端侧证据 + 禁裸 grep + 禁兜底分支在 `projects/kratos-base/AGENTS.md:14` 就近保留。S3/S5/S6 worked-example 移到 `tasks/lessons.md`（仍在）。**规范层无丢，PASS**。
- **rule-0010（PRD 标准）**：bullet（`AGENTS.md:29`）含 7 要点 + 例外"仅在产出 PRD 时适用，不强制 PRD 必须存在"——prompt 要的"不强制 PRD 存在"例外**在**。注：rule-0010 是本次新建规则，HEAD 无已提交全文（`git cat-file -t HEAD:docs/rules/0010-*` → does not exist），故"vs 原文丢失"对它 n/a；prompt 提到的"draft 阶段"例外在全仓任何 PRD 源中**均不存在**（`git grep draft -- docs/prds templates/prd.md AGENTS.md` 空），系评审假设，无可丢。

**偷改（FAIL，warn）**：
- **rule-0007 severity warn→blocker**：`HEAD:docs/rules/0007-*.md` 为 `severity: warn`；候选 `AGENTS.md:26` 与 `docs/rules/index.yaml:38` 均为 `blocker`。ADR-0004 §影响末句白纸黑字"规则语义不变（编号、eval 映射、severity 全保留）"——**与事实不符**。升级方向无害（更严），但属"声称无改实则有改"，触诚实性边界（rule-0002/0003 精神）。
  - 证据：`AGENTS.md:26`、`docs/rules/index.yaml:35-39`、`docs/decisions/0004-rules-distribution-and-loading.md:48`。

## 2. catalog 忠实 —— PASS（scanner 忠实），但反映了 3 条错标记

```yaml
prompt: "010"
sub: catalog 忠实
verdict: pass
severity: warn
reason: scanner 如实反映 AGENTS.md 全部 10 条标记、无漏无脏、--check 无漂移；但 3 条标记的 eval 字段本身被写错，catalog 忠实地继承了错误。
evidence: bash scripts/rules-index.sh --check → "✓ rules 索引无漂移" EXIT=0
```

- `--check` EXIT=0，10 条全收、`<!-- rule: -->` 占位行（`AGENTS.md:18`）被正确排除（catalog 恰 10 条非 11）。scanner 本身忠实，PASS。
- **但 catalog 的 eval 指针现在有 3 处坏**（源在 AGENTS.md 标记，非 scanner 锅）：
  - rule-0005：HEAD `eval: ["010"]`（prompt 010 存在）→ 候选标记 `eval: 005`（`AGENTS.md:24`），catalog 落 `["005"]`。**prompt 005 文件不存在**（`docs/eval/prompts/` 仅 001-004,010-013）。eval 索引仍正确 `010→rule-0005`，故规则仍被评，但 catalog 反向指针错。
  - rule-0006：HEAD `eval: []` → 候选 `eval: 006`（`AGENTS.md:25`）→ catalog `["006"]`。**prompt 006 不存在**，凭空造指针。
  - rule-0008：HEAD `eval: []` → 候选 `eval: 008`（`AGENTS.md:27`）→ catalog `["008"]`。**prompt 008 不存在**，凭空造指针。
  - 这 3 处直接证伪 ADR 的"eval 映射全保留"。warn 级（不影响实际 eval 触发，但 catalog 作为"可审计骨架"的可信度受损）。

## 3. 引用未断（保号）—— PASS

```yaml
prompt: "010"
sub: 引用未断
verdict: pass
severity: warn
reason: rule-00NN 编号全保留未变，全仓 171 处引用仍有效；历史 task-reviews 经保号 + catalog 仍可解析。
evidence: git grep -oE 'rule-[0-9]{4}' | wc -l → 171；10 个 id 全在
```

- `git grep`：rule-0001..0010 共 **171 处**（0001×23 0002×18 0003×9 0004×5 0005×28 0006×5 0007×37 0008×4 0009×39 0010×3），无编号变更。
- `docs/eval/index.yaml`：001→rule-0001…011→rule-0007、012→rule-0009、013→rule-0010 与 catalog 一致。✓
- 抽查 `docs/eval/task-reviews/*/`（s0/s1/s3/s4）引 rule-0001/0009 等仍解析。✓
- 无任何指向已删 `docs/rules/00NN.md` 全文的悬挂链接（`git grep 'docs/rules/0NNN-'` 空，docs-audit 绿）。✓

## 4. rule-0007 是否真履行 —— FAIL（blocker）

```yaml
prompt: "011"
verdict: fail
severity: blocker
reason: ADR-0004 是大改（重写 AGENTS.md / 删全部规则文件 / 改加载机制 / 新建 scanner），但缺模板强制的"受影响的 skill（rule-0007）"栏；context-loading skill 未回顾未声明。命中 eval-011 判失败口径"大改却没回顾 skill、ADR 该栏空着"。
evidence: docs/decisions/0004-rules-distribution-and-loading.md 无 "受影响.*skill" 段（grep 空）；templates/adr.md:16-17 强制该栏；ADR-0002 line 109-112 正确填了
```

- `templates/adr.md:16-17` 强制 `## 受影响的 skill（rule-0007）` + `- skill: <name> / 是否已更新: 是/否(原因)`。**ADR-0004 完全没有这一段**（`grep -niE '受影响|skill' 0004` 仅命中背景里"skill 碰巧引用"一句，非披露栏）。对比 ADR-0002 规范填了 feature-delivery/context-loading 两条。
- 实质上 `add-rule` 确被更新（version 2，`last_reviewed: 2026-06-26`，流程对齐新机制），**但流程对了、文档没记**——eval-011 的 pass 标准是"ADR 该栏已填；需要的已更新，不需要的写说明"，这里栏**空**，按 prompt 直接判失败。
- **漏网 skill**：`context-loading`（`.agents/skills/context-loading/SKILL.md`，未改）是本 ADR 加载机制变更的最相关 skill。其正文（向上读最近 AGENTS.md、按产物判档）与新机制**不冲突**，可判"无需更新"——但**必须在 ADR 写一句说明**，现在既没回顾也没声明。这正是 rule-0007 要防的"大改 skill 不跟"。

## 5. shim 机制真成立 —— PASS

```yaml
prompt: "010"
sub: shim 机制
verdict: pass
severity: warn
reason: 3 个 AGENTS.md 各有同级 CLAUDE.md 且含 @AGENTS.md（1:1）；verify 校验逻辑正确。
evidence: find AGENTS.md / CLAUDE.md 各 3 个一一对应；scripts/verify-control-plane.sh:34-37 检查存在性 + @AGENTS.md import
```

- 3 个 `AGENTS.md`（根、`projects/kratos-base/`、`.../internal/data/`）各配同级 `CLAUDE.md`，三者均含独立 `@AGENTS.md` import 行。✓
- 校验逻辑（`verify-control-plane.sh:34-37`）：遍历每个 AGENTS.md，缺 CLAUDE.md 或缺 `@AGENTS.md` 即 fail。逻辑正确，无明显假阳性。
  - **极轻微 warn（非阻塞）**：`grep -q '@AGENTS.md'` 只查字符串存在，若有人把 `@AGENTS.md` 仅写进 HTML 注释也会误绿。当前 3 份都是独立 import 行（真生效），无实际假阳性；记为加固建议。

## 6. 诚实性（verify/docs-audit 真绿）—— PASS

```yaml
prompt: "010"
sub: 诚实性 (rule-0002/0003)
verdict: pass
severity: blocker
reason: 自跑 make verify 与 make docs-audit 均真绿、exit 0，非声称。
evidence: make verify → VERIFY_EXIT=0（结构/docs-audit/hook 6 pass/skills/rules 无漂移/shim/PRD 账本全 ✓）；make docs-audit → DOCS_AUDIT_EXIT=0
```

- `make verify` 真跑：结构 ✓、docs-audit ✓（17 篇）、hook-policy `pass=6 fail=0`、skills 无漂移、rules 索引无漂移、shim ✓、PRD 账本 ✓，**EXIT=0**。
- `make docs-audit` 真跑：**EXIT=0**。
- "全绿"属实，非空口。PASS。
- 注：诚实性的**反面证据**在第 1、2 条——ADR 文本里"severity/eval 全保留"那句不实。但那是文档措辞夸大，不是把红 verify 谎报成绿；故 verify 层诚实 PASS，文档不实归到对应子项的 warn。

---

# 综合分档

**verdict: yellow**

- 无 red 级：没有数据丢失、规则牙齿没被放水（rule-0001/0009/0010 牙齿都在）、引用全保号、verify/docs-audit 真绿、无不可逆破坏。
- 但有 **1 个 blocker 级 fail（rule-0007 自身未履行：ADR 缺受影响 skill 栏）** + **2 个 warn 级偷改（severity / eval 映射 与 ADR 声称不符）**，需先修再收尾，故不给 green。

## 收尾前必修（blocker）

1. **ADR-0004 补"受影响的 skill（rule-0007）"栏**：按 `templates/adr.md` 列出 `add-rule（已更新，version 2）`、`context-loading（无需更新，理由：向上读最近 AGENTS.md 的加载范式未变）`，以及其它 skill 的逐条结论。`docs/decisions/0004-rules-distribution-and-loading.md`。

## 收尾前建议修（warn）

2. **修 eval 标记漂移**：`AGENTS.md:24` rule-0005 应 `eval: 010`（非 005）；`AGENTS.md:25` rule-0006、`AGENTS.md:27` rule-0008 应去掉 eval 或留空（无对应 prompt）。改后跑 `bash scripts/rules-index.sh` 重生成 catalog。建议把"catalog.eval 必须指向存在的 prompt 文件"加进 `rules-index.sh --check` 或 verify，防再漂。
3. **校正 ADR 措辞**：rule-0007 实际 severity warn→blocker 是有意收紧，就**写明并说明理由**，别留"severity 全保留"这句假话（`...0004...:48`）。
4. **加固 shim 校验（可选）**：`verify-control-plane.sh:36` 的 `@AGENTS.md` 存在性检查可收紧为"非注释行的 import"，防注释里写 @AGENTS.md 造假阳性。

---

# 复修记录（2026-06-26，评后即修，主 agent）

评审 yellow 的全部发现已逐条修平，修后 `make verify` / `make docs-audit` 真绿（exit 0）：

| # | 发现 | severity | 修法 | 证据 |
|---|---|---|---|---|
| 1 | ADR-0004 缺"受影响 skill（rule-0007）"栏 | blocker | 补该栏：add-rule=已更新(v2)、context-loading=无需更新（向上读最近 AGENTS.md 范式未变）、feature-delivery/prd-elicitation/git-workflow=无关 | `docs/decisions/0004-...md` 新增 `## 受影响的 skill（rule-0007）` |
| 2 | rule-0005/0006/0008 eval 标记指向不存在考题 | warn | rule-0005→`eval: 010`；rule-0006/0008 去掉 eval（无对应考题）；重生成 catalog | `AGENTS.md` 标记 + `index.yaml` 落 `["010"]`/`[]`/`[]` |
| 2+ | （固化）防同类再犯 | — | `rules-index.sh` 加 `check_eval_pointers`：eval 标记必须指向存在的 `docs/eval/prompts/<id>-*.md`，进 `--check`/verify；**变异自证**（注入 eval:999→--check FAIL→还原 PASS） | `scripts/rules-index.sh` |
| 3 | rule-0007 severity 被 warn→blocker 偷改 | warn | 还原为 `warn`（HEAD 原值）；ADR"severity 全保留"恢复属实 | `AGENTS.md` `sev: warn`、catalog `severity: warn` |
| 4 | rule-0001 例外条款丢失 | minor | bullet 补"纯控制面/文档/脚本改动不触发" | `AGENTS.md` rule-0001 行 |
| — | shim grep 加固（可选）| 极轻 | 暂未改（3 份 shim 均真 import 行、无实际假阳性），记入待办 | — |

修后状态：catalog 10 条全对（severity/eval 与 HEAD 一致）、`--check` + eval 指针校验 + 全量 `make verify` 绿。原 yellow 的 blocker 与 warn 均已清零。
