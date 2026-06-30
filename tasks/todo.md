# 当前任务

> 只记手头这一件事；干完清空、旧的 roll 进 `archive/`。保持轻。
> 元：level: L3 ｜ task: hc-design

## 当前：hc-design 研发方案/设计阶段 skill（hc-prd → hc-design → api 用例 的关键一环）
- [x] 设计敲定：交互式设计 skill（≈hc-dev 做→挑刺，非 hc-prd 编排）；硬原则=参考项目代码/资产·不确定查+问·决策点用户拍·**全明确才落可执行方案(零 TBD)**·用户审核门·对抗评审·**模板通用不掺项目内容**
- [x] 模板定稿：design.md(9 段)+ api-contract.md(单独，端点索引+每端点 请求/响应/Mock/错误码/关联)；异常 & 安全&风险 拆两段；删项目专属"多租户"
- [x] 建：`templates/design.md` + `templates/api-contract.md` + `hc-design` skill + `hc-design-reviewer` 双栈（workflow 并行建+复核；并行踩踏致复核矛盾→查磁盘理清，真问题只 2：补 config 注册 + 删空壳 README + 2 minor）
- [x] 接流程 + 留档：testing-flow api 线指向 hc-design 为契约源、`docs/designs/` 账本、ADR-0015 登记、索引已 regen 无漂移
- [x] 收尾：`make verify`✓ + `docs-audit`✓(46) + **hc-eval green**（考题 010/011 pass；`docs/eval/task-reviews/20260630T072602Z-hc-design/`）
- [x] **对抗评审 3 轮**（用户逼出的硬环节，eval/verify 都漏的）：R1 揪 blocker(模板偷设 REST、对 kratos gRPC 不通用)+7 major 并修 → R2 口径自洽(reviewer 判据回灌 skill/模板/文档登记 audit) → R3 收敛、修 R2 自相矛盾(逐行→并集、机检词表 4→7 统一、config 不列500→约定/未约定)；建 `scripts/designs-audit.sh` 机检(两层防线)
- [ ] 提交（待授权）
- **follow-up（记 ADR-0015，本批不做）**：reviewer 无对外接口 N/A 回 source 核、补分页/限流硬动作、api-contract 写端点幂等槽位、③多表写法

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
