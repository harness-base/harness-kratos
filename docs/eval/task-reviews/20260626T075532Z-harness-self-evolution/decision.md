# decision — harness-self-evolution

综合分档：**yellow**（无 blocker 级 fail，可有条件收尾；但有一批 warn 级文档漂移与悬空引用必须修，集中在新写的 references 与 CURRENT_STATUS）。

---

## 逐项 verdict

### 1. 新检查是否 load-bearing（防花架子）— mutation 自证

```yaml
prompt: "012"
verdict: pass
severity: warn
reason: 三个新检查全部变异自证为 load-bearing——改坏被检对象 make verify 真变红、还原真变绿。
```

亲手变异（均已还原，最终 `make verify` EXIT 0）：

- **`scripts/index-audit.sh`（decisions/features 双向一致性）**：从 `docs/decisions/index.yaml` 删掉 `0001-harness-skeleton-design.md` 登记 → `index-audit.sh docs/decisions` EXIT 1（"✗ 0001-... 未登记进"）；还原 EXIT 0。反向检查真生效。
- **`scripts/dir-index.sh --check`（目录索引漂移）**：① 向 `docs/context/README.md` 注入 `BOGUS.md` 行 → `--check` EXIT 1（"✗ docs/context 索引漂移"）；② 向 `templates/` 加一个未登记 `__bogus_test.md` → `--check` EXIT 1。两个方向（漂移 / 漏登）都拦，还原后绿。
- **`verify-control-plane.sh` 的"路由工程路径可达"**：把 `workspace/verification.yaml` 的 `path: projects/kratos-base` 改成 `projects/__nonexistent__` → `make verify` 打印"✗ verification.yaml 路由的工程路径不存在"，**全量 `make verify` EXIT 2**；还原后 EXIT 0。

三者都不是花架子。

---

### 2. references 是否 grounded（防 LLM 幻觉）

```yaml
prompt: "012"
verdict: pass
severity: warn
reason: 抽查 5 份 reference，命令/路径/案例锚点真实存在，案例与 lessons / task-reviews 实质对应；个别为 paraphrase 非逐字，不算编造。
```

抽查 `rules.md` / `gates-hooks.md` / `eval.md` / `process-coverage.md` / `index-system.md`：

- 引用的 task-review 目录全部真实存在：`20260626T014408Z-harness-rules-distribution`、`20260612T041709Z-kratos-base-s3`、`20260612T050146Z-kratos-base-s3-rereview`、`20260602T105017Z-kratos-base-s0`。
- 引用的脚本/文件全部存在（`hook-policy.test.sh` / `turn-backstop.test.sh` / `stop-check.sh` / `.githooks/*` / `verify.yml` / `HOOKS.md` 等）。
- `rules-index.sh` 的 `check_eval_pointers` 真在（49/65/71 行）；`turn-backstop.sh` 的"只认 `^\[` findings 行"过滤真在（74 行）。
- lessons 案例实质对应（个别关键词是 paraphrase：reference 写"声称全保留却实际偷改"，lessons 实为"声称'无损迁移/全保留'却实际偷改"——同一条，非杜撰）。

参考内容质量高、可复核。**唯一例外见第 5 项**：grounded 的"案例"对，但 references 里描述 harness 当前结构的"事实锚点"有过时/自相矛盾的硬伤。

---

### 3. ①/② 是否真拆干净

```yaml
prompt: "010"
verdict: pass
severity: warn
reason: 核心载体（rule-0011 / turn-backstop.sh / 两个子 agent / ADR-0005 订正）①②拆分干净、无概念重叠；但 references 多处仍把已删的 self-optimize 当 skill（见第 5 项）。
```

干净的部分：

- **rule-0011**（`AGENTS.md:30`）已①专属："落文档提醒（`scripts/turn-backstop.sh`，=①，非自进化审查）"。
- **`scripts/turn-backstop.sh:1-4`** header 正名"每轮落文档提醒（capture，①；≠ 自进化审查②=self-evolution skill）"，不再自称"自进化"。
- **`.claude/agents/hc-self-optimize.md:23`** 与 **`.codex/agents/hc-self-optimize.toml`** 都明确"知识捕获（决策/血泪有没有落文档）不归你——那是 ① 落文档提醒的活"，两端对齐为②深审执行器。
- **ADR-0005** 加了订正段（15-21 行）清楚拆①②。

残留概念漂移：references 与 SKILL.md 仍把"`self-optimize` skill"当存在（详见第 5 项）。不破坏①②的功能边界，但术语没收口。

---

### 4. "verify 绿 / ✅ 都做了"是否属实

```yaml
prompt: "010"
verdict: pass
severity: warn
reason: make verify / make docs-audit 亲跑均 EXIT 0；ADR-0001/0003 受影响栏、skills-index 头部修正、codex self-optimize 注册均属实；但 CURRENT_STATUS"已同步"不完全——漏了本批新增的 bugfix skill 与 index-audit 脚本。
```

属实：

- `make verify` EXIT 0、`make docs-audit` EXIT 0（亲跑，非声称）。
- ADR-0001:40-42 / ADR-0003:36-39 "受影响的 skill（rule-0007）"栏真已补。
- `scripts/skills-index.sh` 头部无 `make skills-index` 假命令（已修为 `bash scripts/skills-index.sh`）。
- `.codex/config.toml:21-23` 真注册了 `[agents.self-optimize]`。

**不完全属实（warn）**：optimization-log 第 14 行称"CURRENT_STATUS.md 漂移 → 已同步"，但 `docs/context/CURRENT_STATUS.md:28` 仍写"**6 个技能**"且列表**漏 `bugfix`**（实际 7 个，本批刚加的 bugfix 没进）；`CURRENT_STATUS.md:26` 的 scripts 行也**漏 `index-audit`**（本批新增）。这两处是本批自己产出却没回填，不破 verify（该处非机器校验），属文档漂移。

---

### 5. 连带破坏 / 悬空引用 ★本批最该修的一类

```yaml
prompt: "010"
verdict: fail
severity: warn
reason: 删老 self-optimize skill 后，新写的 references 多处仍把它当 skill，且两条"事实锚点"已被本批自己证伪——skills 清单数错、codex 对等缺口被本批补了却仍写"缺"。
```

老 `self-optimize` skill 目录确已删，自动生成的索引（`.agents/skills/README.md`、`.claude/agents/README.md`）都正确。**但手写的 references 没跟上**，悬空/自相矛盾点（均 `文件:行` + 证据）：

- **`.agents/skills/hc-self-evolution/references/skills.md:36`** —「当前 6 个 skill：`add-rule / context-loading / feature-delivery / git-workflow / prd-elicitation / self-optimize`」。**错**：实际 7 个（`add-rule / bugfix / context-loading / feature-delivery / git-workflow / prd-elicitation / self-evolution`）。列了**已删的 self-optimize**、漏了本批新增的 **bugfix** 与 **self-evolution**——连被审的 self-evolution skill 自己都不在自己维度 reference 的清单里。这行还自称"真实路径锚点"。
- **`.agents/skills/hc-self-evolution/references/subagents.md:44`** —「Codex 不对等…当前 `self-optimize` **只有 Claude 侧、无 `.codex/agents/hc-self-optimize.toml`**，是已知对等缺口」，且整段标"事实锚点（核对过）"。**已被本批证伪**：`.codex/agents/hc-self-optimize.toml` 本批已创建并在 `config.toml` 注册（第 4 项已核）。"核对过"的事实锚点是假的。
- **`references/subagents.md:10`** —「`self-optimize` skill（深审入口、列总览索引、晋升归档）调 `self-optimize` 子 agent」：描述了一对已不存在的 skill+subagent。
- **`references/subagents.md:35`** —「重判断子 agent 由主 agent / `self-optimize` skill 按需 spawn」：`self-optimize` skill 已删；实际由 `self-evolution` skill spawn。
- **`references/eval.md:58`** / **`references/index-system.md:83`** —「→ `self-optimize` skill 把发现晋升归档」：指向已删 skill。
- **`references/decisions-context-features.md:70`** —「判断与归档走 `/self-optimize`」：指向不存在的 skill/slash-command。
- **`docs/decisions/0005-self-evolution-loop.md:41`** —"受影响的 skill：self-optimize ／ 已更新：是（本 ADR 新建）"：与同文件订正段（19 行"self-optimize 子 agent"）矛盾，该栏未随订正更新（self-optimize 已非 skill，新 skill 是 self-evolution）。

判 fail（warn 级）：这些不让 `make verify` 变红（references / ADR 结构化栏目非机器校验内容），但正是本批"删旧建新"留下的连带漂移，且两条自我标注"核对过/真实锚点"的事实已被本批自己证伪——违反"references 要 grounded"的初衷。修法：把 references/SKILL.md/ADR-0005 表里所有"self-optimize skill"改为"self-evolution skill（入口）+ self-optimize 子 agent（引擎）"；skills.md:36 改成正确的 7 个；subagents.md:44 删除已被补上的"codex 对等缺口"。

---

### 6. rule-0007 履行（架构级改动同步 skill / ADR）

```yaml
prompt: "011"
verdict: pass
severity: warn
reason: ADR-0005 有订正段并 ①②拆清、related_docs 通；ADR-0001/0003 受影响栏已补；self-evolution / bugfix 自身带 rule-0007 演进段。仅 ADR-0005 的"受影响 skill"结构栏未随订正同步（见第 5 项），属同一类漂移。
```

- ADR-0005 订正段（15-21）就是 rule-0007 的回顾痕迹，related_docs 指向的 `self-optimize.md` 存在。
- `self-evolution/SKILL.md:58-59`、`bugfix/SKILL.md:29-30` 均有"演进（rule-0007）"段。
- 唯一欠账：ADR-0005:41 的结构化"受影响 skill"栏没随订正更新（已并入第 5 项）。

---

### 7. 诚实性（⏳ 是否如实标未做）

```yaml
prompt: "010"
verdict: pass
severity: warn
reason: 三条 ⏳ 经核均如实未做，无假称完成。
```

- **⏳ Codex 原生 hooks**：`.codex/config.toml:11` 仅 `hooks = true`，`.codex/` 下无任何 hook 脚本/定义——确未接，如实标 ⏳（硬层 git/CI 已对等兜底，措辞也诚实）。
- **⏳ 迁移 / loop-engineering 流程**：`.agents/skills/` 下确无 migration/loop skill——如实未做。
- **⏳ eval task-review 三联模板**：确无该模板，低优先，如实标。

无"假完成"。

---

## blocker / warn 清单（给修）

无 blocker。warn（建议收尾前修，至少修前两条）：

1. **[references 悬空 / 假事实锚点]** `references/skills.md:36`（6 个 skill 数错+列已删 self-optimize+漏 bugfix/self-evolution）、`references/subagents.md:44`（"codex 对等缺口"已被本批补上却仍写"缺"，且标"核对过"）、`subagents.md:10/35`、`eval.md:58`、`index-system.md:83`、`decisions-context-features.md:70`、`SKILL.md:56` 一并把"self-optimize skill"改为"self-evolution skill + self-optimize 子 agent"。
2. **[CURRENT_STATUS 漂移]** `docs/context/CURRENT_STATUS.md:28`"6 个技能"→7 并补 `bugfix`；`:26` scripts 行补 `index-audit`。optimization-log 第 14 行的"已同步" ✅ 与事实不符，应同步修正或降级措辞。
3. **[ADR-0005 结构栏未随订正同步]** `docs/decisions/0005-self-evolution-loop.md:41` 受影响 skill 栏 self-optimize→self-evolution（与 19 行订正对齐）。

---

## 复修记录（2026-06-26，评后即修，主 agent）

三条 warn 全修平，`make verify` 复跑绿：
- warn 1（references 悬空/假锚点）→ 11 处修：`skills.md:36`（7 个 skill + 注明 self-optimize 是子 agent）、`subagents.md:10/35/44/48`、`eval.md:58`、`decisions-context-features.md:70`、`index-system.md:83`、`docs.md:51` 全把"self-optimize skill"改为"self-evolution skill（入口）+ self-optimize 子 agent（引擎）"；`subagents.md:44` 的过期"codex 对等缺口"改为"eval/self-optimize 均已对等"。复核残留 `self-optimize` 出现处皆为正确的子 agent/文件路径/历史描述用法。
- warn 2（CURRENT_STATUS 漂移）→ `:28` 7 个技能含 bugfix、`:26` scripts 补 index-audit；optimization-log 对应 ✅ 据实。
- warn 3（ADR-0005 受影响栏）→ `:41` 改为 self-evolution（与订正段对齐）。

复修后状态：误用清零、`make verify` 绿。原 yellow 的 warn 已清。
