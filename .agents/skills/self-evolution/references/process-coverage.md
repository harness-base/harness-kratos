# 流程覆盖 审查手册

> 维度 13。审查对象 = 「常见任务类型有没有对应流程（skill / 既定步骤）」+「旧流程有没有过时该演进」。
> 流程 ≠ 规则（rule，进 rules 手册）≠ 文档（docs 手册）。这里只问：一类反复做的任务，agent 是靠 skill 走流程，还是靠临场拍脑袋。

## 规范（健康长什么样 / 不变量）

- 每类**反复发生**的任务类型都有可走的流程：要么一支 skill（`.agents/skills/<name>/SKILL.md`），要么 AGENTS.md 里明文的既定步骤。盘点口径（种子）：产出需求 / 实现需求 / bugfix / 评审收尾 / 重构 / 迁移 / loop-engineering。
- 流程**挡在临场之前**：高返工 / 高风险的任务（动业务代码、改架构、做迁移）必须先进流程门禁，不允许"先做了再补"。`dev`（深度级，含需求包门禁）就是这条不变量的样板（rule-0001：未就绪 MUST STOP）。
- 流程**不淘汰旧的**就是债：一个流程被更好的范式取代后，旧 skill / 旧步骤必须改造或退场，不能两套并存让 agent 选错。
- 流程之间**接力关系明确**：上下游写在 description / 正文里（如 `prd-elicitation`【上游】→ `dev`），不靠 agent 猜。
- 覆盖盘点本身可复核：技能索引 `.agents/skills/README.md` 自动生成、`skills-index --check` 防漂移——但「索引里有没有这支 skill」是机器能查的，「这类任务该不该有 skill」要人判。

## 怎么检索现状（索引 / 文件 / 机器检查入口）

```bash
ROOT="$(git rev-parse --show-toplevel)"

# 1. 现有流程清单（自动生成的技能索引——别手改）
cat "$ROOT/.agents/skills/README.md"

# 2. 逐 skill 的"何时用/何时不用"边界（看覆盖了哪类任务、显式排除了哪类）
grep -rn "何时用\|何时不用\|不用：" "$ROOT/.agents/skills/"*/SKILL.md

# 3. 任务类型盘点：种子里这几类有没有对应 skill？
grep -rliE "bugfix|bug 修复|重构|refactor|迁移|migration|loop|评审|review" \
  "$ROOT/.agents/skills/"
#   （命中 0 = 该任务类型在 skills 层无流程，去 AGENTS.md 看有没有明文步骤）

# 4. AGENTS.md 里是否有不成 skill 但成文的流程（如收尾 eval = rule-0005）
grep -rn "收尾\|流程\|步骤\|MUST STOP" \
  "$ROOT/AGENTS.md" \
  "$ROOT/projects/"*/AGENTS.md

# 5. 索引漂移自检（加了 skill 没登记 → 红）
bash "$ROOT/scripts/skills-index.sh" --check
```

## 怎么判（符合 / 缺口 / 漏洞 的判据，逐条可判定）

逐条对盘点出的任务类型问：

- **有流程吗**：该任务类型在 `.agents/skills/README.md` 有 skill，**或** AGENTS.md 有明文步骤 → 符合；两处都没有 → **缺口**（靠临场）。
  - 现状判例（核对过）：`产出需求`=prd-elicitation✓ / `实现需求`=dev✓ / `产出用例`=test-case✓ / `评审收尾`=rule-0005 + eval 子 agent（成文，非 skill，可接受）/ `bugfix`=dev✓（改 bug 子模式）/ `重构`=dev✓（写代码统一入口）/ `迁移`=dev✓（深度级迁移子模式：逐条核源 / 禁凭记忆）/ `loop-engineering`=**缺口**（仓内无 loop skill）。
- **门禁挡在临场之前吗**：高返工任务的流程是不是 STOP-before-do（先立项/先有证据再动手）？只有事后清单、没有事前闸 → **缺口**。
- **旧流程过时吗**：某流程的范式被更好的取代却没退场 → **漏洞**。种子点名候选：`context-loading` 的 L0-L6 通用档位 → 是否该演进成"按任务场景（bugfix/迁移/评审…）直接进对应 workflow-skill 并带好该场景的固定上下文"。判据：若 agent 经常"定了档却仍漏读场景专属规则"，说明通用档位粒度太粗，旧流程该换。
- **接力清晰吗**：上下游 skill 的衔接写明了没有（description + 正文都点到）→ 符合；要 agent 猜先走哪支 → 缺口。
- **是真缺口还是"故意不做 skill"**：评审/收尾刻意用 rule + 子 agent 而非 skill（避免污染常驻技能列表，见 self-evolution-plan「结构」一节）——这是设计选择，**不算漏洞**；判缺口前先确认不是这种有意成文。

## 常见漏洞模式（引真实案例）

- **无流程的任务类型靠临场，错法五花八门**：`迁移`无 skill，`harness-rules-distribution` 迁移凭记忆做，ADR 宣称"severity / eval 映射全保留"实则偷改 rule-0007 severity（warn→blocker）、给 rule-0005/0006/0008 编了不存在的 eval 指针，被独立 eval 判 **yellow**。证据：`tasks/lessons.md` 2026-06-26「声称'无损迁移/全保留'却实际偷改」+ `docs/eval/task-reviews/20260626T014408Z-harness-rules-distribution/decision.md`。→ 该控制点（逐条对 `git show HEAD:<file>` 核源、禁凭记忆）现已并入 `dev` 迁移子模式（ADR-0009）。
- **bugfix 曾无专属流程，验收质量全靠人盯**：kratos-base 的 bug 修复（DLQ 假修、metrics 空转测试、backoff 溢出）反复需要"多轮对抗评审"才收敛，没有把"补测试必 mutation 自证 load-bearing""复查自身刚做的修复"固化成流程步骤。证据：`tasks/lessons.md` 2026-06-24「多轮对抗评审…」。→ 当初 bugfix 流程缺口的代价（已由 `dev` 改 bug 子模式填，ADR-0009）。
- **流程隐含 CWD 假设 = 登记的验证命令实际是坏的**：脚本假设 CWD=工程根，从 harness 根亲跑即挂；子代理在工程目录跑全过掩盖了它。证据：`tasks/lessons.md` 2026-06-02「e2e 脚本隐含 CWD 假设」。→ 这类应进 sandbox/gates 维度，但根因是"接入/验证流程"没强制 CWD 无关，属流程覆盖薄。
- **旧流程粒度太粗导致漏读**：`context-loading` 只给 L0-L6 通用档位，不绑场景专属规则，agent "定了档仍可能漏场景规则"——种子标记的待演进旧流程（尚无独立 eval 案例，作为演进候选盯）。

## 修复用哪个操作 skill / 脚本

- **某任务类型无流程 → 新建 skill**：用 `skill-creator`（写 `.agents/skills/<name>/SKILL.md`），按 self-evolution-plan「构建步骤」grounded 写，写完跑 `bash scripts/skills-index.sh`（重生成 `README.md`）→ `make verify` 收口（含 `skills-index --check`）。优先补：loop-engineering 流程（bugfix / 重构 / 迁移已由 `dev` 填，ADR-0009）。
- **旧流程该换 → 改造而非并存**：改对应 `SKILL.md` 正文 + 更新 frontmatter `version` / `last_reviewed`；范式级改动写 ADR（`templates/adr.md`，必填"受影响 skill"栏，rule-0007）并回顾连带 skill。
- **流程成文但不必成 skill（如收尾评审）→ 写进 AGENTS.md**：用 `add-rule` 把步骤落成就近规则 + 登记 catalog，避免污染常驻技能列表。
- **改完一律**：`make verify`（结构 + skills/rules 索引 + shim 全过）→ 把这次"补了哪类流程 / 退了哪个旧流程"记进 `tasks/lessons.md`（rule-0011），反复出现的晋升成规则。
