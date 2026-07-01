# 当前任务

> 只记手头这一件事；干完清空、旧的 roll 进 `archive/`。保持轻。
> 元：level: L3 ｜ task: hc-onboard

## 当前：hc-onboard 新项目接入 skill（ADR-0017）
- [x] 设计敲定（多轮对话）：引导式接入 skill（≈hc-tech-design）；新项目分支先做、老项目占位；7 步（收信息/搭最小骨架/记选型 ADR/接执行口/对抗评审/make verify/交棒）；铁律=全程用户确认不替定；**只搭壳不越界**（代码结构归 hc-dev/hc-tech-design）；接入点占位**三态**（真命令/PENDING:理由/N/A:理由，静默空=红，看得见+绕不过去）；create-sandbox 拆下一轮、verify/ci 当内部步骤。
- [x] build（workflow 并行 3 块 disjoint：skill + reviewer 双栈 + templates/project-agents.md）+ 我串行：ADR-0017、占位机检 `verification-audit`（进 make verify）、config 注册、verification.yaml/VERIFICATION_ROUTING 三态、PROJECT_ONBOARDING 瘦身 + 修 docs-maintainer→hc-doc-sync、CURRENT_STATUS 清、索引 regen。
- [x] **对抗评审 2 栈 + 修**：① 正确性栈实跑挖出**机检假绿漏洞**（黑名单判红、被 `" "`/`"..."`/小写 todo/`# 注释`/裸 PENDING/待定 骗过）→ 改 **fail-closed** + 补变异自测；② 设计栈揪 major「PROJECT_ONBOARDING 只贴 banner 没真瘦身（rule-0012 复刻）」→ 真瘦身成口子速查（0 流程步）+ minor 清 CURRENT_STATUS docs-maintainer。
- [x] **多工程兼容（用户 2026-07-01 追加）**：projects/ 可多工程共存——skill 查重名 + ⑥ 隔离硬规则（只 append 自己那条、不碰别的工程）、reviewer 双栈核不撞名/不碰他人、机检加多工程逐工程测试。`verification-audit.test` **17/0**、make verify + docs-audit(48) 全绿。
- [x] 收尾 eval：**green**（考题 010/011/014/rule-0015 全 pass；评委独立造 fixture 亲验 fail-closed 堵住各种绕过 + 多工程逐工程核成立；`docs/eval/task-reviews/20260701T070010Z-hc-onboard/`）。评委另挖 **F-1**（单引号空值 `'  '` 绕过 fail-closed）→ 已修（clean 剥单引号 `\047` + 自测 18/0；坑：注释里单引号闭合了 awk，已记 lessons）。
- [ ] 提交（待授权；本批新一摊、与 PR #10 那两 commit 不相干）

## 已闭（本会话交付、下次 archive）：hc-tech-design 研发方案/设计阶段 skill（hc-prd → hc-tech-design → api 用例 的关键一环）
- [x] 设计敲定：交互式设计 skill（≈hc-dev 做→挑刺，非 hc-prd 编排）；硬原则=参考项目代码/资产·不确定查+问·决策点用户拍·**全明确才落可执行方案(零 TBD)**·用户审核门·对抗评审·**模板通用不掺项目内容**
- [x] 模板定稿：design.md(9 段)+ api-contract.md(单独，端点索引+每端点 请求/响应/Mock/错误码/关联)；异常 & 安全&风险 拆两段；删项目专属"多租户"
- [x] 建：`templates/design.md` + `templates/api-contract.md` + `hc-tech-design` skill + `hc-tech-design-reviewer` 双栈（workflow 并行建+复核；并行踩踏致复核矛盾→查磁盘理清，真问题只 2：补 config 注册 + 删空壳 README + 2 minor）
- [x] 接流程 + 留档：testing-flow api 线指向 hc-tech-design 为契约源、`docs/designs/` 账本、ADR-0015 登记、索引已 regen 无漂移
- [x] 收尾：`make verify`✓ + `docs-audit`✓(46) + **hc-eval green**（考题 010/011 pass；`docs/eval/task-reviews/20260630T072602Z-hc-design/` 时间戳存档名保留）
- [x] **对抗评审 3 轮**（用户逼出的硬环节，eval/verify 都漏的）：R1 揪 blocker(模板偷设 REST、对 kratos gRPC 不通用)+7 major 并修 → R2 口径自洽(reviewer 判据回灌 skill/模板/文档登记 audit) → R3 收敛、修 R2 自相矛盾(逐行→并集、机检词表 4→7 统一、config 不列500→约定/未约定)；建 `scripts/designs-audit.sh` 机检(两层防线)
- [x] 提交：commit `edaf216` → **PR [#9](https://github.com/harness-base/harness-control/pull/9)**（1 commit、CLEAN、不加 Co-Authored-By，待你合）
- [x] **尾活①（用户挑出）**：「控制面↔项目隔离」是全局命根，原写死进 hc-tech-design §⑦ → 升成 **rule-0015**（走 hc-add-rule，根 AGENTS.md，sev warn），§⑦ + ⑧硬规则那行改成指针；其余复刻处归类（reviewer/模板=规则在执行、ADR=历史记录，留）
- [x] **尾活②（用户改名）**：`hc-design` 像设计师用的 → 全仓 rename **`hc-tech-design`**（reviewer 跟改 `hc-tech-design-reviewer`、ADR-0015 文件、双栈、config、testing-flow、designs 账本、templates、索引 regen）；PR #9 未合、改名=上线前修，干净换；历史归档（lessons/optimization-log/eval 时间戳存档）按 narrative 名保留不动
- [x] 提交尾活①②：rebase onto main → 新开 PR #10（#9 已合，故新 PR；commit c5dc462、不加 Co-Authored-By）
- **follow-up（记 ADR-0015，本批不做）**：reviewer 无对外接口 N/A 回 source 核、补分页/限流硬动作、api-contract 写端点幂等槽位、③多表写法

## 当前（build 完成 · 收尾中）：hc-test 的 api 用例线（补 hc-test 占位的第 2 条线）
- **结构复刻 e2e**：建 `hc-api-qa`（写 api 用例）+ `hc-api-reviewer`（审）双栈子 agent（.md+.codex/.toml，注册 config）；`hc-test` 总监场景表接上 api 线（testing-flow 从占位→实现）；(可能)`templates/api-test-case.md` + 扩 `test-cases-audit` 矩阵机检；收尾 ADR（更新 0014 或新 ADR）+ eval + make verify。
- **核心硬门槛（用户定 2026-07-01）——接口来源是硬前置，无源即停**：
  - 来源优先级 ① `hc-tech-design` 的 `api-contract.md`（有→用它，且用例必与契约**逐端点/字段/错误码对账**、覆盖闭合、**不臆造契约外端点/字段**）＞ ② 用户指定接口来源（proto / OpenAPI / 路由表 / 现有接口代码）＞ ③ 契约与用户来源**都没有 → MUST STOP，提醒用户、不强做、不凭空臆造接口**。
  - 与 e2e 的差别：e2e 缺输入可降级（缺则略），**api 有硬地板**——不知接口长啥样，用例无从写起（否则违 rule-0008 凭空臆造）。
  - 落地：写进 `hc-api-qa` 上下文（产出前判来源、无源即停）+ `hc-api-reviewer` 对账 + testing-flow api 小节 + ADR。**决策 B（2026-07-01）：不升全局规则**——只管"产 api 用例"一件事、非横切（scope 不够，lessons：规则按 scope 判不按 form），落 agent 上下文即可。
  - **覆盖 = 一一对应（决策 A，2026-07-01）**：契约 =「接口清单」（协议无关）+ 每接口列举的业务异常；用例 ① 每个接口都对应上、全覆盖，② 每接口的每个业务异常各覆盖一个 case。机检查双向闭合：契约接口/业务异常码 ↔ 用例，缺一个红、引用契约外的也红。
  - **ADR（决策 C，2026-07-01 修订）**：① 新开 ADR-0016 记实现；② **补 ADR-0014 前向指针**——决策叙述不改写（历史），但加"api 占位已由 0016 实现、不再占位"状态标记 + `related_docs` 加 0016 + bump `last_updated`（免状态漂移，rule-0012）；③ 真相源 testing-flow 的 `场景×实现状态` 表 api 行 🔒占位 → ✅实现。
- **源驱动、不预设（用户定 2026-07-01）**：协议（gRPC / HTTP-REST / MQ event）、鉴权 / 限流等横切、Mock 来源、字段、错误码——**全按接口来源里实际有什么来覆盖**，agent 不硬编任何项目假设（"kratos 是 gRPC""必须测鉴权"都不行）；呼应 rule-0015（控制面不掺项目内容）。来源是 gRPC 就测 gRPC，契约有 Mock 就用契约 Mock、没有不硬凑。
- [x] **build 完成**：workflow 并行建 3 块 disjoint（`hc-api-qa`/`hc-api-reviewer` 双栈 + `templates/api-test-case.md`）+ 我串行扩机检（`test-cases-audit` 认 EP/EX，自测 34/0）+ 接线（config 注册 / testing-flow 小节+场景表 / ADR-0016 / 补 0014 前向指针 / 索引 regen）。`make verify` + `docs-audit`(47) 全绿。
- [x] **对抗评审 2 栈（干净、不被并行污染）**：① 正确性/格式契约——亲手造 `- EX-1：EP-1 · …` fixture 实跑，**无 blocker/major**（2 minor 文案，护栏已修）；② 设计忠实——揪出 **blocker：`hc-test/SKILL.md` 漏改、api 仍标占位**（我接线漏了总监入口 + rule-0012 复刻表漂移），**已修**（占位→实现 + de-复刻状态表指向 testing-flow + §④ 泛化 e2e/api）；lessons 已记。
- [x] 收尾 eval：**green**（考题 010/011/015/rule-0012/rule-0015 全 pass；评委独立复跑 make verify / docs-audit(47) / 机检 34/0 + 手造 fixture 验 EP/EX parser 真生效；`docs/eval/task-reviews/20260701T021419Z-hc-test-api-line/`）
- [x] 提交：commit `d466522` → **并进 PR #10**（用户选②合并；PR #10 现 2 commit=改名批 c5dc462 + api 线 d466522，CI verify 双绿、MERGEABLE，BLOCKED 仅缺 review 批准，待你合）
- **首次实战检验点（eval warn，非阻断）**：首次产真 api 用例集（`docs/test-cases/<id>/`）时让 `hc-api-reviewer` 回契约原文对账一遍——验「声明段↔契约原文」语义防线在真契约上抓不抓得住漏誊接口/错误码（机检覆盖不到的层）。

## Review（api 用例线 build）
- **做了什么**：填 ADR-0014 的 api 占位——照 e2e 同构建 api 用例 worker/reviewer 双栈 + 模板 + 扩机检 + 接线；覆盖=与接口来源一一对应、接口来源硬门槛（无源即停）、源驱动不预设。
- **对抗评审的价值**：`make verify` + `docs-audit` + 机检自测**全绿**都没逮到「SKILL.md 总监入口仍标 api 占位」这个功能断裂——干净对抗评审逮到（又一次"三者不可互替"）。根因是接线漏了最关键的消费方（总监总谱）+ rule-0012 复刻表漂移；顺手把 SKILL 复刻的状态表删了、指向真相源，免再漂。
- **质量**：机检扩展经实跑验证（EX 行的 EP 引用不误判、e2e 无回归、能力边界表述不夸大、两栈等价）；rule-0015 项目隔离守住（kratos 只在"别硬编"的否定句里）。

## 已闭·待提交滚动：给 turn-backstop 装诊断日志 + 修「① capture 0 产出」静默失效（PR #8）
- [x] 装诊断日志 `tasks/.turn-backstop.log`（gitignore）：记 触发/跳过原因、headless `exit`+输出前 160 字（claude stderr 接进来）、写没写 optimization-log；超 800 行自裁
- [x] 实跑揪根因：headless claude `--max-budget-usd 0.03` 偏紧 → 遇较长响应即 `Exceeded USD budget` 报错退出、0 产出，被 `2>/dev/null` 吞了（eval 复跑 n=4 纠偏：非每次必撞、视响应长度；0.005 必撞、0.03 多次 exit=0）
- [x] 修：默认预算 0.03→0.20（实跑验证 exit=0、产出正常）
- [x] 守护测试：turn-backstop.test 加 case 5/6（诊断留痕 + 不污染 optimization-log，变异自证）→ 6/0；HOOKS.md 记一笔 + gitignore
- [x] 修自引漏洞：stop-check.test 间接调 turn-backstop 漏设 `BACKSTOP_DLOG` → 污染真日志，补上；make verify 全绿、测试全 hermetic
- [ ] 提交（待用户授权）

## Review（turn-backstop 观测）
- **任务**：诊断「① capture 通道长期 0 产出」的静默失效——给 `turn-backstop.sh` 装诊断日志，让"执行了啥、卡哪了"有迹可循。
- **根因（日志一跑即现）**：headless claude 的 `--max-budget-usd 0.03` 偏紧 → 遇较长响应即 `Exceeded USD budget` 报错退出、0 产出；原 `2>/dev/null || true` 把报错全吞 → 看着在跑、失败没人看见。（hc-eval 复跑 n=4 纠偏：非"每次必撞"，视响应长度——但"预算偏紧 + 吞错致盲"方向确凿，提预算 + 装日志对症。）
- **修**：默认预算 0.03→0.20（实跑确认 exit=0、正常产出 `NONE`/findings）；诊断日志独立文件、gitignore、超 800 行自裁、`BACKSTOP_DLOG` 可覆写。
- **质量**：守护测试变异自证（neuter dlog → case 5/6 翻红）；并修了本次引入的 hermetic 漏洞（`stop-check.test` 间接调 turn-backstop 没隔离 DLOG → 污染真日志）；`make verify` + turn-backstop 6/0 + stop-check 10/0 全绿，测试不再污染真日志。
- **坑（已记 lessons 2026-06-30）**：best-effort 机制全程 `2>/dev/null` 吞 stderr → 总失败伪装成静默 0 产出；这类机制必须配诊断日志留痕。
- **押后**：a~e 捕获**质量**（现在能产出了，但实战产出多少/准不准待观察）；`optimization-log` 旧 ⏳ drain。

## 已闭（已合 / 已提交，下次清理滚 archive）
- **hc- 命名统一 + hc-test e2e（PR #7 已合）**：ADR-0013（改名 harness-control / hc-）+ ADR-0014（hc-test 总监 + e2e 双栈 + 矩阵硬闸 + 删 test-case），两摊 eval green、合 1 commit `2e6cc88`。
- doc-sync-redesign（L3，932ecef，ADR-0012）；demote-context-loading（L3，PR #6）；prd-orchestration（L4，PR #3/#5）；dev-skill（L4，7b6576d）；test-case-skill（L3，c0c94f6，已被 ADR-0014 superseded）。
