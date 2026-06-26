# 技能目录

> 由 `bash scripts/skills-index.sh` 从各 SKILL.md frontmatter 自动生成，请勿手改。

| name | description |
| --- | --- |
| add-rule | 把一条规则真正落地（团队规范、踩坑约束、编码红线）。用户说"以后都要/不许/必须…"、或你想固化一条规范时用本 skill，走"定范围→写下来+登记→挂执行"三步，确保规则会被加载、违反会被发现，而不是写完没人理。 |
| bugfix | 修 bug 的流程。bug / 测试失败 / 行为与预期不符 / 线上异常时用——先稳定复现 + 定位根因（不猜），再写能复现的守护测试、改根因、验证、防回归、记 lesson。挡在"看到报错就乱改表象"前面。 |
| context-loading | 判定一个任务该读多少文档/规则/上下文（渐进加载档位 L0-L6）。每个新任务开始、或拿不准要不要读某些文档/要不要升档时，先用本 skill 定档——避免全量通读浪费上下文，或读太少漏掉关键规则。 |
| doc-sync | 改了配置 / 脚本 / 接口 / 目录结构 / 规则 / ADR 之后用，对照 checklist 检查相关文档（README / AGENTS.md / CURRENT_STATUS.md / 等）是否要同步——挡在"改了代码忘改文档"前面。被 turn-backstop（钩子）兜底，但这里是主动提醒。 |
| feature-delivery | 管"用户可见的需求/行为/UI/流程/验收目标变化"的完整交付流程——立需求包→测试就绪→实现→验证→收尾。只要改动会被用户感知或改变验收目标（哪怕看着是小调整），动业务代码前就用本 skill；它挡在"先写了再补需求"前面。 |
| git-workflow | 做任何 git 写操作（分支/提交/合并/rebase/推送/worktree 清理）前用本 skill，约束安全习惯。尤其当你打算 commit、push、reset、合并或删分支时必须先看——防止未授权的 git 写操作和不可逆破坏。 |
| prd-elicitation | 产出需求（而非实现需求）：通过引导式多轮对话把一个想法或现有系统理清楚，产出 PRD + 可点的交互 HTML 原型。用户说"做需求/PRD/原型""把这个项目/功能先想清楚再动手""理一理需求""出个原型"时用。它是 feature-delivery 的【上游】——产物独立存放、与实现体系松耦合（feature 实现不强依赖它，有则可衔接）。 |
| self-evolution | harness 规范检查层。当要改 harness 本身、或发现 harness 漏洞（如"某规则工作时没被加载"）时用——引导按 harness 结构逐维度审查"哪一环出问题 / 该怎么改"，不靠记忆漏项。区别于操作层 skill（add-rule/prd-elicitation 等是被检查对象 + 修复工具）。 |
