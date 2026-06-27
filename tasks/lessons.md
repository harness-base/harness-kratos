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
