# 错题本 lessons

每踩一次坑记一条，三段式，按日期倒序（新的在上）。反复出现的，升级进对应 skill 或 rule（rule-0007）。

格式：

```
## YYYY-MM-DD：一句话标题
- Mistake：错在哪
- Prevention：以后怎么防
- Earlier signal：怎么更早发现
```

**整理标记**（晋升流程 / step 4 用，标在标题行尾，`scripts/lessons-promote-check.sh` 据此计数）：无标记 = 未整理 ｜ `<!-- opt: seen -->` = 提醒过·待你决定 ｜ `<!-- opt: skip -->` = 看过·不升 ｜ `<!-- opt: rule-00NN -->` = 已升成该编号规则。攒够未整理的，钩子会提醒整理（升规则走 `add-rule`）。

---

## 2026-06-29：并行的两个 worker 共享一套可变命名空间（都造 FP 号）→ 必撞号
- Mistake：prd 编排把 PRD本体员 + 功能点员 + 原型员 设成**纯并行**，而 PRD本体员和功能点员**都会造 `FP-NN` 编号**。并行时俩看不到对方、各编各的 → 同号指不同物（`FP-08` 一份是"桌面通知"、另一份是"权限降级"）、跨文档追溯断裂。**end-to-end dogfood 才暴露**——`make verify` 的结构检查查不出这种语义级撞号。
- Prevention：拆并行 worker 前先看**有没有共享的可变产物 / 命名空间**（ID 空间、同一份清单、同一个文件）。共享可变的**不能多头写**——要么定**单一权威**（只一个 worker 造，其余引用）、要么按**依赖图串起来**（下游读上游成品再并行真正无依赖的）。"几个步骤概念上独立" ≠ "可并行"；真判据是**数据依赖 + 是否共享可变态**。
- Earlier signal：两个"并行"worker 的产物会**互相引用**（PRD 引 FP、FP 映射又引 PRD 正文）——互引即有依赖，纯并行必出"引用了还没生成 / 对方旧版本"的错位。

## 2026-06-29：把"dogfood"用成了拿旧 code-reviewer 审代码，新建的 prd-reviewer 一次没跑过
- Mistake：prd 编排收尾我说"dogfood 挑刺"，实际派的是 **code-reviewer** 审 skill 的代码/配置/文档——而本任务新建的正是 **prd-reviewer + 6 worker + 编排 workflow，它们一次都没真跑过**，只做了结构验证（双栈齐、索引不漂）。审"代码/控制面产物"用 code-reviewer 本身没错（reviewer 按**产物类型**选、不按话题：skill 源码=code-reviewer，需求产出=prd-reviewer），但这恰恰暴露：新 prd-reviewer 的**功能从未被验证**，而我还把这轮叫"dogfood"。
- Prevention：建了新执行体（subagent / skill / workflow），收尾要**真正运行它一遍**（端到端跑它该处理的产物）——"结构齐全 / 索引不漂" ≠ "功能可用"；"dogfood X" 必须是"让 X 实际跑一次干它的活"，不是拿别的东西审 X 的源码。验 reviewer 能不能用 → 让它真审一份产物；审 reviewer 的源码 → 那是 code-reviewer 的事，两码事。见 [[prd-reviewer-vs-code-reviewer]]。
- Earlier signal：收尾说"dogfood 了 X"时回头查 X 的 agentType 本次有没有被真调用过——没有，就不是 dogfood，只是"审了 X 的代码"。

## 2026-06-29：切任务时没清上一任务的 `## Review` 段，stop-check 拿旧 Review 误判新任务"在收尾"
- Mistake：开 prd-orchestration 新任务时，我只改了 `todo.md` 顶部的"当前任务"头，**把上一任务（dev-skill）整段连同它的 `## Review` 留在了文件里**。stop-check（rule-0005 收尾闸）按"当前 `## Review` 存在 = 在收尾"判定，撞到那段旧 Review，就在我**任务中途**（T6 还在跑、eval 还没到）⛔ 拦截，要我先跑 eval。是 todo 卫生没跟上，不是闸的 bug。
- Prevention：**切任务的当轮就把上一任务整块滚进 `archive/` 或压成一行"已闭"注脚**，绝不让旧 `## Review` 跟新任务并存；`todo.md` 任一时刻只该有"当前任务"那一个 `## Review`（且只在真收尾时才补）。见 [[stop-check-eval-gate-midtask]]（闸把 Review 当收尾信号的机制）。
- Earlier signal：编辑 todo 头部时若下方还留着上一任务的 `- [x]` 清单 / `## Review`——就是没清干净，stop-check 下一次 turn-end 必误触。

## 2026-06-29：每条消息结尾"认吗/认不认"问个不停，用户嫌"像审犯人认罪"
- Mistake：brainstorming 的"逐段确认"被我做成**每条消息结尾都甩一句"认吗/认不认/对吗"**，一个 refinement 问一次。用户烦："跟认罪审犯人似的"。把"增量确认"做成了"反复逼供式索要批准"。
- Prevention：(a) 别用闭合的"认不认"逼 yes/no——**陈述我要做什么 + 开放邀请"要改直说"**，或方向已明就**直接做**、让用户在产物上 review；(b) 用户已多次同意大方向时，**信任累积的共识、往前推**（写出来给他看），别每个微调都回头要批准；(c) 确认点**攒到一个自然节点**（如"写完 spec 给你过目"），不是每轮。
- Earlier signal：连续几条消息都以"认吗?/对吗?/这样行吗?"收尾——这就是在反复逼供，该改成"我去写/我去做，完了你看"。

## 2026-06-29：把"交互对话"当成不能拆 subagent，漏了"需求采集"本身也是一个 subagent <!-- opt: seen -->
- Mistake：聊 PRD 编排时我说"对话留人、不 fan-out"，把"跟用户多轮聊需求"当成必须留主 agent、不可编排的一步。用户纠正：需求采集**本身也是一个 subagent**（需求采集员），职责就是跟用户对话；总监（编排 agent）照样调度它。用户："刚刚我不说，你就不提醒"——我框窄了。
- Prevention：编排建模时 **"交互式 / 人在环" ≠ "不能是 subagent"**。一个专职跟用户对话的 subagent 完全成立（它 own 对话、收齐需求再交回）；"人在环"可以是"人跟某个 worker subagent 在环"，不必是主 agent。别把"需要人确认"自动等同于"这步不能拆出去"。
- Earlier signal：我在编排图里把某一步标成"留主 agent / 不编排"，理由仅是"它要跟人交互"——这理由本身站不住。

## 2026-06-28：把 loop engineering 理解窄成"审→修→再审"，实际是 O-R-A-E 闭环架构 + 5 组件 <!-- opt: seen -->
- Mistake：用户问"了解 loop engineering 吗"，我答成"收敛循环（审→修→再审到零）"。用户贴外部资料纠正"不是简单的循环吧"——实际是 **O-R-A-E 闭环**（Observe 观察/读记忆+环境 → Reason 推理/结合全局目标定动作 → Act 执行/调工具改码 → Evaluate 独立裁判打分 + 失败原因喂回下一轮），并由 **5 大技术组件**支撑（Automations/Cron、Worktrees、Skills、Connectors/MCP、Sub-agents「制作者-验证者」分离）。我只抓了 Evaluate→feedback 一环，漏了整体架构。
- Prevention：被问"了不了解 X"时先给理解 + **明说置信度**，别把熟悉的子集当全貌；遇成熟概念（尤其 agent 架构类）先想"有没有公认的完整结构（如 O-R-A-E）"再答。注意外部资料 rule-0008 不自动采信——采纳前对照本仓现状验收（本仓其实已具备这 5 组件：hooks/Cron、`using-git-worktrees`、SKILL 体系、MCP、`eval`/`code-reviewer` 子 agent）。
- Earlier signal：对一个有名词的成熟概念只给"一句话直觉"，没有结构/阶段/组件的拆解——多半只懂了一个面。

## 2026-06-28：删 skill 连带漂移又复发——活引用没扫全，枚举清单无 --check 必漂 <!-- opt: seen -->
- Mistake：删 feature-delivery/bugfix、建 dev 后，第二轮挑刺又揪出一片"删旧建新连带漂移"：`AGENTS.md`「已有子代理」清单漏 code-reviewer（且 self-optimize 早就漏）、`self-evolution/SKILL.md` 仍把 bugfix 列为"缺口/待补 playbook"、`references/docs.md` 案例还提已删的 bugfix。我只改了 ADR-0009 点名的几处（process-coverage/subagents），漏了同体系其它文件。这是上轮 eval（20260626）已点名、subagents.md 自家判据也写明的**同型问题第 N 次复发**。
- Prevention：删/改一个 skill 时，**全仓 grep 该 skill 名 + 它代表的"缺口/样板"语义**，逐一判活引用（改）vs 历史记录（不动），别只改 ADR 受影响栏点名的文件；ADR 受影响栏自己也要把"连带要扫哪些"列全。**根治**：枚举型清单（AGENTS.md 子代理行、SKILL 里"缺口如 X"措辞）无 `--check` 必随增删漂——能指针化的指向自动索引（rule-0012），本次把 AGENTS.md 子代理行改为指向 `.claude/agents/README.md`。
- Earlier signal：删了个 skill 只改了 ADR 点名的 2-3 个文件就收手；全仓 grep 该 skill 名仍有命中、且不全是历史记录。

## 2026-06-28：用户问"我刚说的 code-reviewer 子 agent"，我答成了平台自带的 /code-review <!-- opt: seen -->
- Mistake：用户问"你提的 code-reviewer 子 agent，Claude Code 自带 workflow 了、还要不要配"，我把"自带的"理解成内置 `/code-review` skill，大篇幅讲 /code-review 分档 / ultra——答错了对象。用户纠"我问的不是 /code-review，是你刚说的 code-reviewer"。
- Prevention：用户用"你刚说的 / 我们提的 X"指代时，**锚回我自己对话里说过的那个 X**，别因为平台有个名字相近的功能（`/code-review` vs `code-reviewer` 子 agent）就偷换成它；名字相近 ≠ 同一个东西，先确认指代再答。
- Earlier signal：我的回答里引入了一个用户没提过、名字相近的平台功能，并围着它展开——多半已偏题。

## 2026-06-28：问"范围/定位"时堆 skill 内部名词，用户"没懂" <!-- opt: seen -->
- Mistake：设计新 dev skill 时，用多选问"新 skill 取代/收编哪些（feature-delivery / bugfix / prd-elicitation / test-case）、用户可见 vs 工程代码"——一口气塞 4 个内部 skill 名 + 抽象分类，用户回"没懂"。同型于 2026-06-27"名词多"，这次发生在【提问】环节。
- Prevention：(a) 问用户决策点用**大白话 + 用户视角**，别让用户裁决 harness 内部 taxonomy（哪个内部 skill 该 retire 是我该**提案**的实现细节，不是用户该冷选的）；(b) 范围/定位这类先给**具体大白话的设计草图**让用户改，别抛抽象多选；(c) 一条问题别塞 ≥3 个专有名词。
- Earlier signal：提问选项里出现 ≥3 个内部 skill 名 / "用户可见 vs 非可见"这类内部分类术语——用户大概率没懂。
- 续（用户再纠"范围划分根本不是一个逻辑"）：我反复拿"旧 skill 怎么切"去框新 skill 的范围——**用旧结构的逻辑套新设计**。新东西按它**自己的活儿（用户给的 scope）**建，别映射到旧 taxonomy 上；旧的都是参考、迟早整体替换，不必现在逐一裁"谁对应谁"。

## 2026-06-28：审计脚本"宽容解析分隔符"是 whack-a-mole，第 4 轮才被顿号 blocker 戳穿——该收口为最简严格契约 <!-- opt: seen -->
- Mistake：test-cases-audit 声明解析为支持单行多 id，做了"首 id + 分隔符续接"，分隔符字符类只写了 `[,，]`（半/全角逗号）。注释却宣称"逗号/顿号续接"。第 4 轮实跑：`- AC-1、AC-2：`（**顿号**，中文最常用并列符、仓内 156 文件在用）第二个 AC-2 被静默丢 → 真未覆盖却判绿（**blocker 假阴**）；连带全角分号/空格/斜杠/"和"等任意非逗号分隔符都漏。根因不是"漏了顿号"，是**"枚举分隔符"这条路本身无穷尽**——每加一种宽容就多一类静默丢的假阴面。
- Prevention：判闸脚本对"可多取"的字段，要么**枚举对账 fail-closed**（行内 id 总数 > 取出数即判红，无论什么分隔符都不静默少取），要么**收口为最简严格契约**（一行一个 id、id 紧跟锚点紧接冒号，多 id/括注/杂样一律判红）——本次选后者：声明一行一 id（`- AC-n：`），covers 行才收"所有 id"（covers 是纯 id 列表、无标注歧义）。**别走"逐个宽容分隔符/写法"的中间路**，它既不收敛又留静默假阴。注释承诺的能力（"顿号续接"）必须有守护测试钉住，否则就是会撒谎的契约。
- Earlier signal：解析器里出现"支持 A、B 两种分隔符"的字符类枚举，且没有"取少了就判红"的对账兜底——每多一种合法写法就是一个新的静默丢失面；注释写"支持 X"但测试样本里没有一个 X。

## 2026-06-28：新写的覆盖审计脚本裸 grep 解析 markdown，两轮对抗评审挖出一片假阴/假阳 <!-- opt: seen -->
- Mistake：`test-cases-audit.sh`（管"用例对 AC/FP 覆盖"的硬闸）初版用 `grep '^- AC-'` / `grep 'covers:'` 全文扫判覆盖闭合。两轮对抗评审（共 17 真问题）揭穿：① 围栏代码块/附录里的示例 `- covers: AC-x`、`- AC-n：` 被当真覆盖/真声明 → **假阴**（缺口判绿、硬闸失守）+ **假阳**（合法判红）；② 声明锚 `^-` 不容缩进、covers 容缩进 → 缩进声明静默漏算=**假绿**；③ 改 awk 段限定后又带进新洞——声明侧单 `match` 只取单行多 id 的首个、段标题写歪致声明落段外 vacuous 假绿、守护样本把围栏放错段没真杀变异（注释撒谎，rule-0009）。守护测试初版 7 条全绿却对这些场景全盲。
- Prevention：写"解析结构化文本判闸"的审计脚本，默认要：(a) **剥围栏**再解析（示例/附录不算数）；(b) **按段限定**提取（声明只在声明段、引用只在引用段，标题前缀锚定别用子串）；(c) **两端匹配对称**（缩进/加粗/全角冒号/多 id 容忍一致），别一边松一边紧；(d) 加 **fail-closed 护栏**——"有疑似内容却没解析出预期"→判红，把 vacuous 假绿变响亮红；(e) 守护测试必须含**对抗式 fixture**（围栏内示例、缩进/加粗、单行多 id、标题漂移、未闭合围栏），且**每条都变异自证**（neuter 对应行→样本变红），否则守护是装饰。呼应已有 lesson"单轮零=假收敛"：第二轮专挖第一轮修复本身的洞。
- Earlier signal：审计脚本只用 `^-`/`grep` 单行正则扫全文、不分段不剥围栏；守护测试样本全是"教科书式整洁"输入、没有一个畸形/对抗样本——这时绿得好看但闸是漏的。

## 2026-06-28：讲 Claude Code 操作默认成 CLI、给了终端命令，用户其实用 app <!-- opt: seen -->
- Mistake：用户问"怎么进目录续任务"，我默认是 CLI，给了 `cd` + `claude --continue` 终端命令；实际用户用的是**桌面 / 网页 app**。被纠正"不是 cli，是 app"。
- Prevention：讲"怎么操作 Claude Code"前，先确认客户端类型（CLI / 桌面 app / 网页 / IDE 插件）——各自操作完全不同；不确定就先问、或分客户端给，别默认 CLI。
- Earlier signal：给 `cd` / 命令行步骤前，没确认对方在不在用终端。

## 2026-06-28：把用户的"通用/行业流程"提问误当成"问 harness 现状"，跑去翻本地文件 <!-- opt: seen -->
- Mistake：用户问"通常需求评审后是不是该进研发 + 测试用例生产"——是个**行业通用流程**的确认性提问（"通常"），我却当成"问 harness 怎么做"，去读 feature-delivery SKILL + 模板。被纠正"不是本地情况，是行业流程"。
- Prevention：答前先分清问题是**通用知识**还是**本仓现状**——信号词"通常 / 一般 / 行业 / 是不是应该"=通用，直接答通用知识；"我们这 / 现在 / 这个 skill / 本仓"=现状，才去核本地文件。别一见问题就翻代码。
- Earlier signal："通常/一般"这类泛化词在问句里，我却在 grep/read 本地文件。

## 2026-06-28：单轮 green eval 漏掉 blocker+7major，对抗式多轮（换视角+复查自身）才照出 <!-- opt: seen -->
- Mistake：prd-workflow 重做收尾，单个 eval 子 agent 判 **green**（010/011 pass）。用户要求"对抗评审"后，10 视角 ×2 轮 + 逐条独立证伪挖出 **16 真问题**：1 blocker（我设计的"用户故事先于 PRD"中间态——只有 `user-stories.md` 无 `prd.md`——直接挂 `prds-audit` → `make verify` 红）、我的 stop-check B 修复其实**回归**（全文件 grep `## Review` 命中暂挂/归档块旧 Review，mid-task 又误拦）、守护测试**存活变异**（`-ge 2→-ge 3` 边界、正则弱化成裸 Review 都全绿）、一片文档/护栏漂移（rule-0010、prds README、`dir:` 字段、原型措辞 vs ADR-0003）。green eval 盲区：只在空账本态验过、过度声称"五层一致 / load-bearing"。
- Prevention：(a) L2+/高价值改动收尾，**单轮 eval green ≠ 收敛**；该上对抗式多轮（换没用过的视角 + 专设"复查自己刚做的修复"视角）——这次正是后者抓到 B 回归（呼应 2026-06-24"单轮零=假收敛"）。(b) 守护测试必须变异闸门的**边界**（测 L2 不只 L3）和**锚点语义**（`## 标题` 不只字面），单次变异通过 ≠ 钉死契约（rule-0009）。(c) 新流程引入"中间态/新产物"时，立刻问"它会不会挂现有机器闸（make verify）"。
- Earlier signal：eval/自评说"green / 一致 / load-bearing"，却只在一种状态下验过、关键变异只试过一次。

## 2026-06-27：eval 闸 mid-task 误触发——rule-0013"开局标 level"撞上 stop-check"每轮拦" <!-- opt: seen -->
- Mistake：按 rule-0013 在 todo 开局标 `level: L3`，stop-check 立刻每个 turn-end 都拦"L3 无 eval 评审"；但任务才做到 ADR、实现没动，无从 eval。两规则在多轮任务上冲突：rule-0013 要"开局声明档位"，stop-check 把每个 turn-end 当"收尾"——分不清"进行中"与"要收尾"。
- Prevention：eval 闸应只在"任务真要收尾/声明完成"时触发，而非每 turn-end。最小修法：stop-check 多看一个**完成信号**（todo 该任务有 `## Review` 段——rule-0013 本就要求"收尾前补 Review"）才要求 eval。**绝不靠降 level 到 L1 绕**（rule-0013/0005 明令反对的低报）。
- Earlier signal：todo 标 L2+ 后第一个 turn-end 就被拦，而任务实际刚起步。

## 2026-06-27：讲实现找不准"高度"——先堆术语被嫌"名词多"，改大白话又被嫌"不专业、没实现路径" <!-- opt: seen -->
- Mistake：讲 step 4，先一口气抛术语 / 自造标记名（中转站 / opt / parked / done / domain…）被嫌"名词有点多"；我矫枉过正改成纯大白话（打勾 / 数数 / 提醒），又被嫌"太不专业、没实现路径、不知道怎么实现"。两头都没踩准。本会话已多次因表达失准被打断（"没懂""太长了"）。
- Prevention：讲实现的正确高度 = **具体（点名文件 + 每个干啥 + 标记长什么样）+ 说人话（不堆自造名词、一次一个概念）**。不是二选一：要可落地的细节，但用平实的话讲；别在"术语墙"和"空泛"之间来回荡。
- Earlier signal：一条回复要么冒出 ≥3 个自造名词、要么通篇没有一个文件名 / 具体动作——两种都是高度不对。

## 2026-06-27：用户要"一步步来"，我却把下一步全量分析了——front-run 了用户的节奏 <!-- opt: seen -->
- Mistake：用户说"落进去吧，然后再看第二步"，我把"看第二步"当成"立刻产出第二步的完整设计"，第一步刚落就甩一长篇第二步方案。用户当场纠正"我们只做第一步，谁让你看后续，一步步分析"。我超车了用户的节奏。
- Prevention：用户给"一步步 / 一个个来 / 然后再看 X"这类节奏信号时，做完当前步就**停**，下一步只留一句"待指令"，不自动展开成分析。节奏跟用户走，不抢跑。
- Earlier signal：本会话用户已多次要"短""一个个来"；当我正在写一长篇前瞻分析时，就该察觉已越过"当前这一步"。

## 2026-06-27：检测器自己漂——turn-backstop 用的是 doc-sync 清单的过期子集，所以没拦住漂移 <!-- opt: seen -->
- Mistake："改完该查什么"这份清单存了两份：`doc-sync/SKILL.md`（14 行，全）和 `turn-backstop.sh` 的 Haiku 提示（4 个例子，子集）。两份各自漂；turn-backstop 那份没列"新建 skill→CURRENT_STATUS 行"，于是 skill 7→8 在它眼皮底下没被捞到。**检测器的判据本身是个漂移面**——同一份 checklist 复制成两份，消费方就会瞎一块。
- Prevention：清单/判据**单一来源**——一处权威（doc-sync 表），消费方（turn-backstop）运行时去读它，别自抄。本次把 doc-sync 表加 `谁兜底` 列（`✅机检` make verify 兜 / `🔴手` 只能人改），turn-backstop 只读 `🔴手` 行当判据 + 守护测试（变异自证：去引用/去标记都判红）盯住这条接线不被静默掐断。泛化：凡发现"同一份清单/枚举有第二份拷贝"，就是漂移源，收敛到一处。同型于当轮 rule-0012（状态文档别硬编码可自动生成的枚举），都是"同一信息存了两份会漂"。
- Earlier signal：写检查/提示时手敲了一串"典型：A→改A'、B→改B'"的例子，而别处已经有一份更全的同类清单——你正在造第二份拷贝。

## 2026-06-27：CURRENT_STATUS 硬编码 skill 清单第 3 次漂移——改去硬编码而非又补个数字 <!-- opt: seen -->
- Mistake：`docs/context/CURRENT_STATUS.md` 的 `.agents/skills/` 行硬编码"N 个技能 + 逐个列名"，已第 3 次漂移：4→7（eval `20260626T075532Z` 记）、6→7 漏 bugfix（同评审）、本次 7→8 漏 `doc-sync`。机器查不到（`skills-index --check` 只守自动生成的 `.agents/skills/README.md`，不守 CURRENT_STATUS 这段散文），每次新增 skill 就漂一次。同型还有 `references/docs.md` 把 kratos 就近 AGENTS.md 写死"13 处"（实际 12）。
- Prevention：状态/索引类文档凡能交给自动生成索引的，**不要硬编码计数+枚举**，直接写"以 `<自动生成的 README>` 为准"。本次把 CURRENT_STATUS skill 行改为指向 `.agents/skills/README.md`（skills-index 自动生成、`--check` 兜底）、docs.md 去掉"13 处"硬数。范本是 `PROJECT_BRIEF.md:20`"首个已接 kratos-base，实时状态见 CURRENT_STATUS.md"——只给指针、不复刻状态。**已晋升成规则**：rule-0012（根 `AGENTS.md`，sev warn），挂了机器检查（`verify-control-plane.sh` 拦 CURRENT_STATUS 的 skill 行枚举 ≥4 名）+ eval 题 014。
- Earlier signal：在状态文档里写下"N 个 X：a / b / c …"这种带计数的逐项枚举，而同一信息已有 `*-index` 自动生成的权威清单——这就是未来的漂移源。

## 2026-06-27：反复凭印象描述 harness 状态/机制，不核源就下结论 <!-- opt: seen -->
- Mistake：本会话多次断言后被用户当场推翻——"5K=膨胀"（实际 61 行，不膨胀）、"projects/ 空"（实际挂着 kratos-base）、"钩子失效"（实际只是触发阈值没到）、"drift"打成"draft"、描述漂移链时漏了 self-optimize、skills.md "7 个 skill"已过期没发现。共因：凭记忆/印象说，没先 `ls`/`cat`/`grep`/`git status` 核当前事实。
- Prevention：描述 harness 任何状态/计数/机制前，先跑命令核源再下结论；尤其"X 是空的/失效了/有 N 个"这类**可枚举断言**必须有当轮命令证据（呼应 rule-0008 事实源=代码现状）。改文档计数/清单时同步想"这里以后会不会漂"，能交给自动索引的别硬编码枚举。
- Earlier signal：自己正要写"应该是/大概/记得/我印象中"这类词，且紧跟一个能被一条命令证伪的事实断言。

## 2026-06-26：兜底把超长 transcript 喂给 Haiku，"Prompt is too long" 被当发现记进 log <!-- opt: seen -->
- Mistake：turn-backstop 取 transcript 末尾 400 **行**喂 Haiku，但 JSONL 单行含工具输出可能极大→prompt 超限→Haiku 返回 "Prompt is too long"；脚本没识别这是报错，当成发现追加进 optimization-log（污染，还提交了一条）。
- Prevention：喂 LLM 的上下文按**字节**截（非行数，行大小不可控）；输出只认**预期格式**（findings 必须形如 `[类别] ...`），报错/NONE/空一律不记——别把 LLM 的错误消息当结果落库。
- Earlier signal：log/产出里冒出 "Prompt is too long"、"overloaded"、"budget" 这类 LLM 错误串。

## 2026-06-26：macOS 没有 timeout/gtimeout，钩子里别用 <!-- opt: seen -->
- Mistake：给 headless triage 加 `timeout 90 claude ...`，退出码 127（command not found），claude 根本没跑。本机（Darwin）默认无 `timeout`/`gtimeout`。
- Prevention：钩子/脚本要超时一律用 perl：`perl -e 'alarm shift @ARGV; exec @ARGV' <秒> <cmd...>`（macOS 自带 perl）；别假设 GNU coreutils 在。
- Earlier signal：脚本里出现 `timeout`/`gtimeout` 且目标平台是 macOS。

## 2026-06-26：管道里的环境变量只作用于左边那个命令 <!-- opt: seen -->
- Mistake：测递归 guard 写成 `HARNESS_TRIAGE=1 printf ... | bash stop-check.sh`，env 只给了 `printf`，`bash` 没拿到 → 误判"guard 漏了"。
- Prevention：要给管道右边的命令传 env，写在它前面：`printf ... | HARNESS_TRIAGE=1 bash stop-check.sh`。测试失败先怀疑测试本身。
- Earlier signal：`A=1 cmd1 | cmd2` 却期望 A 影响 cmd2。

## 2026-06-26：机械触发用"绝对阈值"会在大改动未提交时每轮误触发 <!-- opt: seen -->
- Mistake：兜底触发用"变更文件数 ≥ N"绝对值，结果一有大 changeset 没提交就每轮都 fire（正是想避免的每轮跑 LLM）。
- Prevention：周期性触发用"自上次基线以来的**增量**"（涨多少），把基线（轮号/HEAD/计数）存状态文件，比差值；绝对量只适合一次性门槛。
- Earlier signal：触发条件只看当前快照、不看"自上次以来变化"。

## 2026-06-26：声称"无损迁移/全保留"却实际偷改，被独立 eval 抓出 <!-- opt: seen -->
- Mistake：规则分布化时 ADR 写"severity / eval 映射全保留"，实际把 rule-0007 severity warn→blocker、给 rule-0005/0006/0008 编了不存在的 eval 指针（005/006/008）；凭记忆迁移、没对源核。eval 子 agent 逐条 `git show HEAD` 对比后判 yellow。
- Prevention：凡声称"X 保留/不变"，**必须对事实源（`git show HEAD:<file>`）机械核对**再写，别凭记忆；能机器查的就固化成检查（本次把"eval 标记必须指向存在考题"加进 `rules-index.sh --check` 并变异自证）。
- Earlier signal：ADR/总结里出现"全部保留""完全一致"这类绝对措辞，却没贴逐条 diff 证据。

## 2026-06-26：rule-0007 改了 skill 却没在 ADR 记录 = 判失败 <!-- opt: seen -->
- Mistake：架构大改时更新了 `add-rule` skill，但 ADR 漏掉 `templates/adr.md` 强制的"受影响的 skill（rule-0007）"栏，`context-loading` 也没回顾/声明 → eval-011 直接判 blocker fail（"做了没记"等于没履行）。
- Prevention：rule-0007 = **改 skill + 在 ADR 该栏逐条写**（已改写已改、不需改写"无需更新+理由"）；ADR 用 `templates/adr.md` 起草别手搓省栏。
- Earlier signal：写了 ADR 但没用模板、跳过"受影响 skill"段。

## 2026-06-26：bash 用 TAB 作 IFS 分隔会折叠空字段 <!-- opt: seen -->
- Mistake：扫描器 `printf '...\t...'` + `IFS=$'\t' read` 解析记录，某字段为空时（eval 缺省）TAB 作 IFS 空白被折叠，字段错位（brief 空、location 串到规则正文）。
- Prevention：解析可能含**空字段**的定长记录，用**非空白分隔符**（如 `\037` US），别用 TAB/空格。
- Earlier signal：去掉某可选字段后，后续字段整体"串位"。

## 2026-06-24：多轮对抗评审（独立证伪 + 迭代到零）能抓出单轮漏掉的，包括修复自身引入的回归 <!-- opt: seen -->
- Mistake：单轮评审/自审会漏两类——①**修复本身的不彻底**：R1 给 SDK 包全局 `EnableSsl` 加的 `buildMu` 只锁"构造"，罩不住拨号期(detached goroutine)的无锁读，是 R2 才抓出的"假修 + 注释撒谎(rule-0009 §C)"；②**同型问题的兄弟实例**：共因污染/WARN 放行散落在多个 `scen_*.sh`，R1 只修了一部分，R2 才清完。
- Prevention：成规模评审用"**多维并行出 findings → 每条独立 agent 对抗证伪(只留站得住的) → 修 → 下一轮，直到某轮零确认**"的迭代法。对抗证伪挡住"为覆盖而覆盖/过度报"（本次 27→11、22→17、各轮均有驳回）；迭代收敛挡住"漏网 + 修复回归"。补测试一律 **mutation 自证 load-bearing**（把被测逻辑改坏、确认测试会红），杜绝为过而测的牵强测试。
- Earlier signal：某一轮确认数不降反升（11→17）通常不是发散，而是上一轮修得不彻底被照出来——别慌，继续迭代会收敛（→4→1→1→1→0）。
- 更大的教训（同主题，14 轮全量评审后补）：**"用同一套视角跑出单轮零"≠收敛**。R7 用第一循环的四维(覆盖/牵强/e2e/正确性)跑出 0，我据此判收敛想停；用户追问"不是循环吗"后改用【怀疑式轮换视角】(并发 TOCTOU、优雅关停时序、热更一致性、可观测、中间件、注册生命周期、安全、k8s、文档漂移、infra，外加**专门复查自己刚做的修复**)，R8-R13 又挖出 36+ 真问题——包括我自己 R10 的 DLQ 修复无效(经典队列 Nack 不累加 x-death)+ 配套是喂合成数据的空转测试、R9 的 metrics 测试不绑真实链、registryx backoff 指数溢出。**Prevention 升级**：(a) 收敛判据改为"换一批**从没用过的视角**跑仍零"，而非"同视角零"；(b) 每轮必含"复查自身最近修复"维度——自己的修复和测试同样会牵强/无效；(c) 凡补测试，问"喂的输入生产路径真会产生吗"——喂合成/不可达输入来通过的是空转测试(R10 DLQ、R9 metrics 同型)，等于没测。

## 2026-06-23：弹性 builder 忽略调用方 ctx，把"热更落地"的证据搞成脆弱的超时巧合 <!-- opt: seen -->
- Mistake：`redisx.Open`/`pgxpool.Open` 连通性探测自建 `context.Background()+DialTimeout`，无视调用方 ctx。后果有二：①热更到坏地址时 `Provider.Build` 里的 eager ping 阻塞、不被 readyz 请求 ctx 取消（kratos HTTP 默认请求超时 1s），readyz 卡满 1s 再报 `context deadline exceeded`；②AC-CR1 "坏 redis 地址→/readyz 翻 503" 之所以成立，靠的竟是"Open 阻塞耗光那 1s → self-heal 回退旧好句柄后 Health 再探时 ctx 已过期"这个**超时巧合**，而非干净的"provider 改连坏地址、探活失败"。读码审计据此（误）判它必为 200 假阳性；实跑确是 503，但机制脆弱、与脚本注释不符。
- Prevention：弹性数据面 builder 一律**吃调用方 ctx**（`Open(ctx, cfg)`，ping ctx = min(调用方 deadline, DialTimeout)）。**别让断言依赖看不见的 ctx 超时竞态**——S6 给 readyz 设 15s 合理探测超时后，"坏 redis 地址→503"当场失效（self-heal 回退旧好句柄、readyz 保持 200，这才是对的弹性）。最终把"热更续上"改成 ctx 无关的硬证据：恢复后推非法配置 → confcenter 新增 `retaining previous config` 日志（BEFORE/AFTER 计数对比，杜绝旧行假命中）。教训：一个断言若随"超时设多少"而翻盘，它锚的就不是真行为。
- Earlier signal：readyz 失败信息是 `context deadline exceeded`（超时）而非 `connection refused`（干净的不可达）——错误类型不对就该追：谁在等谁的 ctx。

## 2026-06-23：共因污染断言——"坏 DSN + 停 pg → 503"无法区分热更落地与 pg 宕 <!-- opt: seen -->
- Mistake：`scen_cc_runtime_down.sh` CR1-b/c 用"推坏 DSN + 停 pg → 503"证明热更续上，但 pg 停了本身就会让 readyz 翻 503；如果 watch 根本没有重连、新配置未落地，503 照样出现——两种失败路径共用同一个可观测信号（共因污染），断言无法区分"热更确实落地"与"pg 宕导致"。
- Prevention：正解 = 改一个**不停对应容器**的依赖地址。改 redis 地址（6390，redis 容器仍在 6379 运行）：readyz 翻 503 的唯一路径是"新配置经 watch 落地 → provider 按坏地址重建 → redis 探活失败"；pg/redis 容器均活着排除共因。凡断言"某依赖不可达 → readyz 翻 503"，均需确认**只有热更路径**能导致该依赖不可达。
- Earlier signal：断言脚本里同时操作了"推坏配置"和"停某个容器"两个动作——多了一个动作就应怀疑共因。

## 2026-06-12：e2e 日志断言被"访问日志回显"骗过，掩盖了 100% 消息丢失 <!-- opt: seen -->
- Mistake：场景断言用裸 payload grep 全文匹配日志——发布请求的 HTTP 访问日志会回显 args（含 payload），断言命中它就 PASS，而实际路由键错误（用了随机事件 id 而非队列名）导致消息被默认交换机静默丢弃、消费者从未收到。我和实现/复跑都被骗，**独立 eval 评委用 rabbitmqctl 查队列 + 管理 API 注入对照才揭穿**。
- Prevention：日志断言必须**锚定产出方的结构化字段**（如 `"consumer":"received"` + `"key":"<事件id>"`），禁止裸串全文 grep；消息链路验收配队列侧证据（积压数/消费者数）；发布方要么 mandatory 要么先声明队列，杜绝静默丢弃。
- Earlier signal：断言里出现"找到任意 consumer 行也算过"的兜底分支；payload 同时出现在请求日志里。

## 2026-06-12：池类依赖会掩盖"探活失败不标记重建"的缺口 <!-- opt: seen -->
- Mistake：`resource.Provider.Healthy()` 探活失败时不置 `ready=false`，死句柄因配置版本/指纹未变而永远命中缓存、不重建。PG/Redis 没暴露（`sql.DB`/go-redis 是**池**，句柄下面自愈）；rabbitmq 的 `*amqp.Connection` 是**单条 TCP**，死了句柄即死，S3 e2e 才炸出来。
- Prevention：弹性抽象的失败路径要用**最脆的依赖形态**（单连接句柄）做 e2e 验证，别只用自带池的依赖；Healthy 失败必须驱动下次 Get 重建。
- Earlier signal：依赖恢复后 readyz 永不转绿、而单测全绿——说明缓存命中路径绕开了重建。

## 2026-06-02：宣称"全绿"但没跑项目唯一门禁命令 <!-- opt: seen -->
- Mistake：T3 自核只跑了 go build/test/tidy 就说"全绿"，没跑 `make verify`——其中 golangci-lint 是红的，被审查者抓出。
- Prevention：宣称通过前跑**项目登记的门禁命令本身**（`make verify`），不是自选子集；引用它的真实输出。
- Earlier signal：要说"绿"时，回想是否亲眼见过 `verify OK` 那行。

## 2026-06-02：e2e 脚本隐含 CWD 假设，登记的验证命令实际是坏的 <!-- opt: seen -->
- Mistake：弹性脚本假设 CWD=工程根（裸 `make`/`go build`），而 `verification.yaml` 登记的命令从 harness 根调用——亲跑即全挂；子代理在工程目录跑全过，掩盖了问题。
- Prevention：脚本一律开头 `cd "$(dirname "$0")/../.."` 做到 CWD 无关；**登记路由前用路由里的原始命令、从路由的工作目录亲跑一次**。
- Earlier signal：脚本注释出现 "must be run from project root"。

## 2026-06-02：Kratos config.Scan 走 json.Unmarshal，缺 json tag 的字段静默零值 <!-- opt: seen -->
- Mistake：配置结构体只写 yaml tag；`config.Scan` 实际用 json.Unmarshal，snake_case 字段（池参数、sample_ratio）全部静默解析为零值，单测/lint 均不报。
- Prevention：Kratos 配置结构体一律 **json+yaml 双 tag**；e2e 断言**真实业务值**（计数、采样行为），不只断言 200。
- Earlier signal："配置了但行为像默认值"（如采样率 1.0 却无 span）。

## 2026-06-02：子代理长任务流式超时（idle timeout / 大依赖下载阻塞） <!-- opt: seen -->
- Mistake：T3/T4 子代理两次中断——prompt 不自包含导致先读一堆文件长时间静默；首次 `go get` otel 全家桶下载阻塞数分钟，触发流超时。
- Prevention：派发前**预热依赖模块缓存**；prompt **自包含**（接口/语义/测试用例全给，明示"别读文件直接写"）；机械实现型任务用快输出模型（sonnet）。
- Earlier signal：子代理 tool_uses 不少但 0 token 输出、目标目录无产物。

## 2026-05-29：目录改名后直接写旧逻辑路径会失败 <!-- opt: seen -->
- Mistake：`mv` 重命名目录后，对新路径直接 Write/Edit 报 "File has not been read yet"。
- Prevention：改名 / 移动后，已存在文件先 `Read` 再 `Edit`；新文件可直接 `Write`。
- Earlier signal：凡涉及 `mv` / 移动文件后还要改它，先 Read。
