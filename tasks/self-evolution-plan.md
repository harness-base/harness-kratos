# Plan：harness 自进化（self-evolution）skill

> 状态：draft，待确认后建。

## 目标

一个**规范检查层** skill：当要改 harness、或发现 harness 漏洞时，引导 agent 按 harness 结构**定位"哪一环出问题 / 该怎么改"**，逐维度审查、不靠记忆漏项。

- 它是**检查层**，区别于**操作层** skill（`hc-add-rule`/`hc-prd`/`feature-delivery` 等——它们是**被检查对象** + 发现缺口后的**修复工具**，不作为审查逻辑被复用）。
- 落文档提醒（capture）是独立机制，不属本 skill。

## 何时用（两个触发）

1. **要改 harness 某处**（方向已定）→ 引导按结构改对、查连带影响。
2. **发现 harness 漏洞/症状**（如"某规则工作时没被加载"）→ 诊断是哪一环断了。

## 诊断方法

harness 每个能力是一条**链路**；漏洞 = 某环断了。

1. 把症状写成「X 本该发生却没有」。
2. 拆出交付 X 的链路（环节序列）。
3. 逐环查：在不在 / 接没接 / 真起作用——机器能查的跑 `*-audit`、`*-index --check`；查不了的判断。
4. 定位断环 → 用对应维度手册修 → 查有没有连带破坏别的环 → 记 lesson。

**例**：症状「某规则工作时没进上下文」→ 链路 `定义(AGENTS.md) → 加载(CLAUDE.md @import/就近) → 选取(按任务挑相关) → 遵守 → 拦截(hook/eval)` → 逐环发现「选取」环没有 task→规则 选择器 = 断点 → 修复走 rules 维度手册。

## 审查维度（统一表，每行 = 一份 reference）

| # | 维度 | 规范（健康长什么样/不变量） | 怎么检索现状 | 检查问什么（含漏洞模式） | 修复工具 |
|---|---|---|---|---|---|
| 1 | **skills** | 会触发的流程都有 skill；description 触发准、不重叠不漏；进自动索引防漂移；arch 变了回顾 | `.agents/skills/README` + 各 SKILL.md；`skills-index --check` | 常见任务无 skill？description 误触发/漏触发？version/last_reviewed 过期？ | 写/改 SKILL.md + 重生成索引 |
| 2 | **rules** | 就近入驻 AGENTS.md(带标记)、放最浅覆盖位；该用时进上下文；有考题或标注无需；catalog 自动生成防漂移、引用不悬空；severity 准 | `docs/rules/index.yaml`；grep 标记；`rules-index --check`；加载链路 | 热点无规则？定义了却加载不到(链路断)？catalog 漂移/坏指针？severity 被改？ | `hc-add-rule` |
| 3 | **文档** | 事实源单一；根 AGENTS.md 红线精简、全文就近；凡 AGENTS.md 必有 CLAUDE.md shim；context 反映真实状态 | `docs/README`；`find AGENTS/CLAUDE`；`docs-audit`；`CURRENT_STATUS` | 与现状漂移？shim 漏配？related_docs 悬空？知识没就近？ | 编文档 + docs-audit/shim |
| 4 | **eval** | L2+/关键点收尾必评；题库独立按号引规则；blocker 规则有考题；免 key 子 agent 可用；考题↔规则不悬空 | `docs/eval/`(index/prompts/rubric/evaluator/task-reviews)；`verify-eval-materials` | 重要规则无考题？eval 流程/方式该改？考题牵强/过时？指针悬空？ | 加考题 + 登记 + 指针校验 |
| 5 | **templates** | 每类标准产出(ADR/feature/PRD/plan/skill)有模板；字段反映当前规范；起草用模板不手搓 | `templates/` + 产出是否依模板 | 常产出却无模板？字段跟规范脱节(如 ADR 漏"受影响 skill"栏)？该有索引？ | 改/加模板 |
| 6 | **gates-hooks** | 声称的检查 load-bearing(mutation 自证)、不可绕、**codex&claude 对等**；机判→脚本进 verify、人判→eval；hook 分软(agent)/硬(git/CI)；触发频率合理 | `verify-control-plane`；各 `*-audit`；`hook-policy(.test)`；`.claude/settings.json`；`.githooks`；CI；`HOOKS.md` | 哪个是花架子(无 mutation)？能绕？codex 等价缺(只做了 CC)？该机器化的靠自觉？频率空转/漏？ | 加 mutation+进 verify；补 codex 等价(就近/git/CI)；同步 HOOKS.md |
| 7 | **decisions/context/features** | 各区有 index、内容一致；大决策有 ADR 且被按号引用；context 反映真实状态 | `docs/decisions/index`；`docs/features/index`；`docs/context` | index 与目录一致？大改没落 ADR？context 漂移？ | 补 ADR/登记 + 更新 context |
| 8 | **sandbox** | 被管工程有可复现本地验证环境；CWD 无关；一键起/销；与 `verification.yaml` 路由对得上 | `projects/*/deploy/sandbox`；`test/resilience`；`workspace/verification.yaml` | 能一键起/销？CWD 假设？路由登记的命令真能跑？ | 改脚本 CWD 无关 + 亲跑路由 |
| 9 | **subagents** | 独立任务用子 agent；清晰 system prompt/工具范围/免 key；与 skill 分工清(skill=流程, subagent=独立执行) | `.claude/agents/` + 谁 spawn | 该独立却塞主 agent？定义过期？与 skill 重叠空转？ | 加/改 `.claude/agents/<name>.md` |
| 10 | **lessons/memory** | 踩坑当场记(三段式)；反复出现晋升成规则；memory 存用户偏好；log 是中转、必晋升不空转 | `tasks/lessons.md`；memory；`optimization-log` | 反复 lesson 没晋升成规则？log 躺着不动(空转)？memory 过期？ | 晋升 lesson→规则 / 写 memory |
| 11 | **被管工程接入** | projects/ 有 onboarding；`verification.yaml` 路由准；就近规则随工程进 projects/ | `PROJECT_ONBOARDING`；`workspace/verification.yaml`；`VERIFICATION_ROUTING` | 接新工程流程齐？路由准？ | 改 onboarding + 路由 |
| 12 | **索引体系（横切）** | 每个资产区都有索引；尽量自动生成(防手维护漂移)、有 `--check` 进 verify；"默认不加载、自检时查" | 列全部区 + 各自 index + 是否自动生成 | 哪些区没索引(context/harness/templates/sandbox 现在可能没)？哪些手维护会漂？引用悬空？ | 补索引 + 写生成器(仿 rules/skills-index) + 进 verify |
| 13 | **流程覆盖** | 常见任务类型(需求/实现/bugfix/评审/重构/迁移/loop…)都有流程；过时流程演进 | 技能索引 + 任务类型盘点 | 哪类常做任务没流程？哪个旧流程该换(如档位→按场景进 workflow-skill)？ | 新建 skill / 改造旧流程 |
| 14 | **self（别漏）** | 自身也按上面被审；维度列表完整；不空转、不太吵(误报率) | 本 skill + references | 维度有没有漏？references 过期？这套审查本身该改？ | 改本 skill / references |

## 结构：一个总 skill + references

```
.agents/skills/hc-self-evolution/
  SKILL.md             总入口（轻、常读、可带参直达某维度）
  references/<维度>.md  审查手册（按需读，平时零占用）
```

不做成多个子 skill：每个 skill 的 description 会进每个会话**常驻的技能列表**；自进化偶尔用，~13 个子 skill = ~13 条描述常占上下文，违反"少读、按需读"。references 只在调用总 skill 时才读。某维度复杂到成了独立可复用任务时，再单拎成 skill。

## reference 统一模板

```
# <维度> 审查手册
## 规范（健康长什么样 / 不变量）
## 怎么检索现状（索引 / 文件 / 机器检查入口，给真实路径）
## 怎么判（符合 / 缺口 / 漏洞 的判据）
## 常见漏洞模式（引真实案例）
## 修复用哪个操作 skill / 脚本
```

## 总 SKILL.md 大纲

定位（检查层；与操作层的区别）→ 何时用（两触发）→ 诊断方法（链路 5 步 + 例）→ 维度索引（指向各 reference，带"何时看哪份"）→ 4 条 meta（① 全覆盖不靠记忆 ② 每项给检索·方向·边界 ③ 列表外是否补新流程 ④ 自身也审）→ 演进(rule-0007)。

## 构建步骤

1. 确认本 plan（维度 / 模板 / 结构）。
2. **workflow 并行写 13 份 reference**：每 agent 读对应 harness 区，按模板 grounded 写（检索入口给真实路径、漏洞模式引真实案例）。
3. 写**总 SKILL.md** + self 维度 + 注册 `skills-index`。
4. `make verify` 收口。

## 验收标准

- 总 skill 轻（方法 + 维度索引）；13 份 reference 各含五段（规范/检索/判据/漏洞/修复）。
- 拿"规则没被加载"走一遍：能从症状 → 链路 → 定位断环 → 指到修复工具。
- 技能索引只新增 `self-evolution` 一个（不污染常驻列表）。
- `make verify` 绿。

## 开放问题（建前定）

1. 命名：`self-evolution`（建议）还是 `harness-self-check`？
2. 维度 13 项够吗？哪项要拆细/合并？
3. 索引体系维度若发现"某区没索引"，本次顺手补，还是只登记为缺口待办？
4. Codex 对等：references 里只写"该有等价机制"，还是顺手补 Codex 侧（更大，建议单独立项）？
