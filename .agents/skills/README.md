# 技能目录

> 由 `bash scripts/skills-index.sh` 从各 SKILL.md frontmatter 自动生成，请勿手改。

| name | description |
| --- | --- |
| hc-add-rule | 把一条规则真正落地（团队规范、踩坑约束、编码红线）。用户说"以后都要/不许/必须…"、或你想固化一条规范时用本 skill，走"定范围→写下来+登记→挂执行"三步，确保规则会被加载、违反会被发现，而不是写完没人理。 |
| hc-design | 交互式产出研发方案 / 技术设计（而非需求、而非实现）：主 agent 当设计者，读项目现状 → 提方案 → 不确定就查/问 → 决策点让用户拍 → 全明确 + 用户审核才落稿 → 派 hc-design-reviewer 对抗评审到过。产出 项目专属 的 研发方案；有对外接口才附 接口契约（被 api 用例消费），纯内部无接口标 N/A。它填 hc-prd(需求) 与 hc-dev(实现) 之间的设计空档：hc-prd → hc-design →（api 用例 / hc-dev）。用户说「出研发方案 / 技术设计 / 设计接口 / 接口契约 / 怎么实现这块 / 把需求落成方案」时用。 |
| hc-dev | 写代码的统一入口（写功能 / 工程代码 / 重构 / 改 bug / 迁移 都走它）：想清楚 → 列 plan → 你确认 → 写（不假设、决策点问你、防技术债）→ 挑刺（对抗 review，循环到无 bug）→ 提醒你测。两级（常规 / 深度），默认按任务轻重自判、可指定。用户说「写 / 实现 / 改 / 重构 / 迁移 / 修 bug / 做个功能 / 开发」时用。 |
| hc-git-workflow | 做任何 git 写操作（建分支 / 提交 / rebase / 合并 / 解冲突 / 推送 / worktree 清理）前用本 skill：① 安全红线（没授权不写、不强推、别动 main）② 本仓 git 约定（feat/fix 分支从 main 切、本地 rebase main 解冲突、PR 走 merge commit、commit 格式）。打算 commit / push / reset / 合并 / 删分支 时必看。 |
| hc-prd | 编排式产出需求（而非实现）：产品总监（主 agent）调度一队专职 worker——需求采集 / 外部调研(可选) / 用户故事+AC / PRD本体 / 功能点 / 原型(可选) / PRD审稿——带 必选·可选权重 + 确认门 + 并行 + review loop，分阶段产出 用户故事 → PRD →（可选）原型，每阶段用户确认。用户说"做需求 / PRD / 原型 / 理一理需求 / 出个原型"时用。它是实现（hc-dev）的【上游】，产物独立存放、松耦合。 |
| hc-self-evolution | harness 规范检查层。当要改 harness 本身、或发现 harness 漏洞（如"某规则工作时没被加载"）时用——引导按 harness 结构逐维度审查"哪一环出问题 / 该怎么改"，不靠记忆漏项。区别于操作层 skill（hc-add-rule/hc-prd 等是被检查对象 + 修复工具）。 |
| hc-test | 编排式产出测试（而非实现）：测试总监（主 agent）按手上产物 + 到了哪一步自动选场景，调度专职 worker——e2e 用例（本期）/ api 用例 / 接口契约对照 / 测试脚本 / 回归（占位）——带 默认编排 + 用户覆盖、worker→reviewer 回改 loop、两层覆盖防线。用户说「写测试用例 / 写 e2e 用例 / 测试覆盖 / 用例覆盖率 / 把验收点转成用例 / 做测试」时用。流程唯一真相源 = docs/harness/testing-flow.md；产物独立落 docs/test-cases/<id>/，与实现体系松耦合。 |
